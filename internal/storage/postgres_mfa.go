package storage

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/identity/mfa"
)

// PgMFAStore — `mfa_factors`, `mfa_recovery_codes` (0067). `mfa.Store`.
type PgMFAStore struct{ pool *pgxpool.Pool }

func NewPgMFAStore(pool *pgxpool.Pool) *PgMFAStore { return &PgMFAStore{pool: pool} }

const mfaCols = `id, user_id, secret_enc, confirmed_at, last_used_step, created_at`

func scanFactor(row pgx.Row) (*mfa.Factor, error) {
	var f mfa.Factor
	err := row.Scan(&f.ID, &f.UserID, &f.SecretEnc, &f.ConfirmedAt, &f.LastUsedStep, &f.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, mfa.ErrNotFound
	}
	return &f, err
}

func (r *PgMFAStore) Confirmed(ctx context.Context, userID string) (*mfa.Factor, error) {
	return scanFactor(r.pool.QueryRow(ctx, `SELECT `+mfaCols+` FROM mfa_factors
		WHERE user_id = $1 AND type = 'totp' AND confirmed_at IS NOT NULL`, userID))
}

func (r *PgMFAStore) Pending(ctx context.Context, userID string) (*mfa.Factor, error) {
	return scanFactor(r.pool.QueryRow(ctx, `SELECT `+mfaCols+` FROM mfa_factors
		WHERE user_id = $1 AND type = 'totp' AND confirmed_at IS NULL ORDER BY created_at DESC LIMIT 1`, userID))
}

// SavePending — foydalanuvchi qatorlari qulflanadi: parallel ikki ulash
// bir-birining tasdiqlanganini o'chirib yubormasin.
func (r *PgMFAStore) SavePending(ctx context.Context, f *mfa.Factor) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Commit muvaffaqiyatli bo'lsa no-op
	if _, err := tx.Exec(ctx, `SELECT 1 FROM users WHERE id = $1 FOR UPDATE`, f.UserID); err != nil {
		return err
	}
	var confirmed bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM mfa_factors
		WHERE user_id = $1 AND confirmed_at IS NOT NULL)`, f.UserID).Scan(&confirmed); err != nil {
		return err
	}
	if confirmed {
		return mfa.ErrAlreadyEnrolled
	}
	if _, err := tx.Exec(ctx, `DELETE FROM mfa_factors WHERE user_id = $1 AND confirmed_at IS NULL`, f.UserID); err != nil {
		return err
	}
	created := f.CreatedAt
	if created.IsZero() {
		created = time.Now()
	}
	if _, err := tx.Exec(ctx, `INSERT INTO mfa_factors (id, user_id, type, secret_enc, created_at)
		VALUES ($1, $2, 'totp', $3, $4)`, f.ID, f.UserID, f.SecretEnc, created); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PgMFAStore) Confirm(ctx context.Context, factorID string, step int64, now time.Time, recovery [][]byte) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Commit muvaffaqiyatli bo'lsa no-op
	var userID string
	err = tx.QueryRow(ctx, `UPDATE mfa_factors SET confirmed_at = $2, last_used_step = $3
		WHERE id = $1 AND confirmed_at IS NULL RETURNING user_id`, factorID, now, step).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return mfa.ErrNotFound
	}
	if err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23505" { // idx_mfa_one_totp
			return mfa.ErrAlreadyEnrolled
		}
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM mfa_recovery_codes WHERE user_id = $1`, userID); err != nil {
		return err
	}
	for _, h := range recovery {
		if _, err := tx.Exec(ctx, `INSERT INTO mfa_recovery_codes (user_id, code_hash) VALUES ($1, $2)`, userID, h); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// UseStep — bitta UPDATE: shart (`last_used_step < step`) va yozish atomik.
func (r *PgMFAStore) UseStep(ctx context.Context, factorID string, step int64) (bool, error) {
	tag, err := r.pool.Exec(ctx, `UPDATE mfa_factors SET last_used_step = $2
		WHERE id = $1 AND confirmed_at IS NOT NULL AND last_used_step < $2`, factorID, step)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 1 {
		return true, nil
	}
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM mfa_factors WHERE id = $1 AND confirmed_at IS NOT NULL)`,
		factorID).Scan(&exists); err != nil {
		return false, err
	}
	if !exists {
		return false, mfa.ErrNotFound
	}
	return false, nil
}

func (r *PgMFAStore) UseRecovery(ctx context.Context, userID string, hash []byte, now time.Time) (bool, error) {
	tag, err := r.pool.Exec(ctx, `UPDATE mfa_recovery_codes SET used_at = $3
		WHERE user_id = $1 AND code_hash = $2 AND used_at IS NULL`, userID, hash, now)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *PgMFAStore) RecoveryLeft(ctx context.Context, userID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM mfa_recovery_codes WHERE user_id = $1 AND used_at IS NULL`,
		userID).Scan(&n)
	return n, err
}

var _ mfa.Store = (*PgMFAStore)(nil)
