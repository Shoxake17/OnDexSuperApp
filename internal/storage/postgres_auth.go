package storage

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/identity"
	"chustapp/internal/users"
)

// PgAuthSessionStore — `auth_sessions` (migratsiya 0066). `identity.Store`.
type PgAuthSessionStore struct{ pool *pgxpool.Pool }

func NewPgAuthSessionStore(pool *pgxpool.Pool) *PgAuthSessionStore {
	return &PgAuthSessionStore{pool: pool}
}

const authSessionCols = `id, user_id, realm, client_id, org_id, amr, mfa_at, created_at, last_used_at,
	idle_expires_at, abs_expires_at, revoked_at, revoke_reason, ip_hash, user_agent, device_name,
	COALESCE(parent_id, '')`

func scanAuthSession(row pgx.Row) (*identity.Session, error) {
	var s identity.Session
	var realm string
	err := row.Scan(&s.ID, &s.UserID, &realm, &s.ClientID, &s.OrgID, &s.AMR, &s.MFAAt, &s.CreatedAt,
		&s.LastUsedAt, &s.IdleExpiresAt, &s.AbsExpiresAt, &s.RevokedAt, &s.RevokeReason, &s.IPHash,
		&s.UserAgent, &s.DeviceName, &s.ParentID)
	if err != nil {
		return nil, err
	}
	s.Realm = users.Realm(realm)
	return &s, nil
}

func (r *PgAuthSessionStore) Create(ctx context.Context, s *identity.Session, refreshHash []byte) error {
	amr := s.AMR
	if amr == nil {
		amr = []string{} // NULL emas: ustun NOT NULL
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO auth_sessions (id, user_id, realm, client_id, family_id, refresh_hash, org_id, amr, mfa_at,
			created_at, last_used_at, idle_expires_at, abs_expires_at, ip_hash, user_agent, device_name, parent_id)
		VALUES ($1,$2,$3,$4,$1,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,NULLIF($16, ''))`,
		s.ID, s.UserID, string(s.Realm), s.ClientID, refreshHash, s.OrgID, amr, s.MFAAt,
		s.CreatedAt, s.LastUsedAt, s.IdleExpiresAt, s.AbsExpiresAt, s.IPHash, s.UserAgent, s.DeviceName, s.ParentID)
	return err
}

// Rotate — bitta tranzaksiyada: joriy hash bo'yicha qatorni QULFLAB o'qiydi;
// topilmasa `prev` bo'yicha qidiradi (qayta ishlatish). Parallel ikki
// refresh bitta qatorni ikki marta aylantira olmaydi (FOR UPDATE).
func (r *PgAuthSessionStore) Rotate(ctx context.Context, oldHash, newHash []byte, now time.Time,
	idleFor func(string) time.Duration, grace time.Duration) (*identity.Session, identity.RotateResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Commit muvaffaqiyatli bo'lsa no-op

	s, err := scanAuthSession(tx.QueryRow(ctx,
		`SELECT `+authSessionCols+` FROM auth_sessions WHERE refresh_hash = $1 FOR UPDATE`, oldHash))
	switch {
	case err == nil:
		idle := idleFor(s.ClientID)
		if !s.Active(now) || idle <= 0 {
			if s.RevokedAt == nil && idle <= 0 {
				if _, err := tx.Exec(ctx,
					`UPDATE auth_sessions SET revoked_at = $2, revoke_reason = 'client_removed' WHERE id = $1`, s.ID, now); err != nil {
					return nil, 0, err
				}
			}
			return s, identity.RotateExpired, tx.Commit(ctx)
		}
		idleUntil := now.Add(idle)
		if idleUntil.After(s.AbsExpiresAt) {
			idleUntil = s.AbsExpiresAt
		}
		if _, err := tx.Exec(ctx, `
			UPDATE auth_sessions SET prev_refresh_hash = refresh_hash, refresh_hash = $2,
				last_used_at = $3, idle_expires_at = $4 WHERE id = $1`, s.ID, newHash, now, idleUntil); err != nil {
			return nil, 0, err
		}
		s.LastUsedAt, s.IdleExpiresAt = now, idleUntil
		return s, identity.RotateOK, tx.Commit(ctx)
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, 0, err
	}

	s, err = scanAuthSession(tx.QueryRow(ctx,
		`SELECT `+authSessionCols+` FROM auth_sessions WHERE prev_refresh_hash = $1 FOR UPDATE`, oldHash))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, identity.RotateNotFound, nil
	}
	if err != nil {
		return nil, 0, err
	}
	if s.RevokedAt == nil && now.Sub(s.LastUsedAt) <= grace {
		return s, identity.RotateRaced, tx.Commit(ctx)
	}
	if s.RevokedAt == nil {
		if _, err := tx.Exec(ctx,
			`UPDATE auth_sessions SET revoked_at = $2, revoke_reason = 'refresh_reuse' WHERE id = $1`, s.ID, now); err != nil {
			return nil, 0, err
		}
		t := now
		s.RevokedAt, s.RevokeReason = &t, "refresh_reuse"
	}
	return s, identity.RotateReused, tx.Commit(ctx)
}

func (r *PgAuthSessionStore) Get(ctx context.Context, id string) (*identity.Session, error) {
	s, err := scanAuthSession(r.pool.QueryRow(ctx, `SELECT `+authSessionCols+` FROM auth_sessions WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, identity.ErrNotFound
	}
	return s, err
}

func (r *PgAuthSessionStore) ListActive(ctx context.Context, userID string, now time.Time) ([]*identity.Session, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+authSessionCols+` FROM auth_sessions
		WHERE user_id = $1 AND revoked_at IS NULL AND idle_expires_at > $2 AND abs_expires_at > $2
		ORDER BY last_used_at DESC LIMIT 100`, userID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*identity.Session
	for rows.Next() {
		s, err := scanAuthSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *PgAuthSessionStore) Revoke(ctx context.Context, id, reason string, now time.Time) error {
	tag, err := r.pool.Exec(ctx, `UPDATE auth_sessions SET revoked_at = COALESCE(revoked_at, $2),
		revoke_reason = CASE WHEN revoked_at IS NULL THEN $3 ELSE revoke_reason END WHERE id = $1`, id, now, reason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return identity.ErrNotFound
	}
	return nil
}

func (r *PgAuthSessionStore) RevokeAllForUser(ctx context.Context, userID, reason string, now time.Time) (int, error) {
	tag, err := r.pool.Exec(ctx, `UPDATE auth_sessions SET revoked_at = $2, revoke_reason = $3
		WHERE user_id = $1 AND revoked_at IS NULL`, userID, now, reason)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// RevokeChildren — `parentID` dan ochilgan faol sessiyalarni yopadi va
// ularni qaytaradi (back-channel logout uchun).
func (r *PgAuthSessionStore) RevokeChildren(ctx context.Context, parentID, reason string, now time.Time) ([]*identity.Session, error) {
	rows, err := r.pool.Query(ctx, `UPDATE auth_sessions SET revoked_at = $2, revoke_reason = $3
		WHERE parent_id = $1 AND revoked_at IS NULL RETURNING `+authSessionCols, parentID, now, reason)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*identity.Session
	for rows.Next() {
		s, err := scanAuthSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// PgAuditLog — `audit_log` (0067, faqat qo'shiladi). Yozib bo'lmasa asosiy
// amal TO'XTATILMAYDI, lekin xato logga yoziladi.
type PgAuditLog struct{ pool *pgxpool.Pool }

func NewPgAuditLog(pool *pgxpool.Pool) *PgAuditLog { return &PgAuditLog{pool: pool} }

func (a *PgAuditLog) Record(ctx context.Context, e identity.Event) {
	details := []byte("{}")
	if len(e.Details) > 0 {
		if b, err := json.Marshal(e.Details); err == nil {
			details = b
		}
	}
	at := e.At
	if at.IsZero() {
		at = time.Now()
	}
	// Rune bo'yicha kesiladi: baytda kesish yarim UTF-8 belgi qoldirib,
	// Postgres yozuvni "invalid byte sequence" bilan rad etardi.
	ua := e.UserAgent
	if r := []rune(ua); len(r) > 300 {
		ua = string(r[:300])
	}
	// Asosiy so'rov bekor qilinsa ham jurnal yozilishi kerak — alohida, qisqa kontekst.
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if _, err := a.pool.Exec(wctx, `INSERT INTO audit_log (at, actor_user_id, realm, org_id, business_id,
		action, target, result, ip_hash, user_agent, details) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		at, e.ActorUserID, e.Realm, e.OrgID, e.BusinessID, e.Action, e.Target, e.Result, e.IPHash, ua, details); err != nil {
		slog.Error("audit_log yozilmadi", "action", e.Action, "err", err)
	}
}
