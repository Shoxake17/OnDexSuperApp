package storage

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/agentapi"
)

// Migration 0039'dagi unikal indekslar. Ular buzilganda xom "23505"
// emas, tushunarli domen xatosi qaytadi.
const (
	idxAgentPartnerKey    = "idx_agent_partners_key"
	idxAgentPartnerPrefix = "idx_agent_partners_prefix"
	idxAgentGrantsPartner = "idx_agent_grants_partner_user"
)

type PgAgentRepo struct{ pool *pgxpool.Pool }

func NewPgAgentRepo(pool *pgxpool.Pool) *PgAgentRepo { return &PgAgentRepo{pool: pool} }

// ── Sheriklar ──

const partnerColumns = `id, name, key_prefix, environment, scopes, active, contact, created_at, revoked_at`

func (r *PgAgentRepo) CreatePartner(ctx context.Context, p *agentapi.Partner, keyHash string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO agent_partners
			(id, name, key_prefix, key_hash, environment, scopes, active, contact, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		p.ID, p.Name, p.KeyPrefix, keyHash, string(p.Environment),
		agentapi.JoinScopes(p.Scopes), p.Active, p.Contact, p.CreatedAt)
	return mapAgentErr(err)
}

// mapAgentErr — Postgres cheklovlarini domen xatosiga aylantiradi.
func mapAgentErr(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case idxAgentPartnerKey, idxAgentPartnerPrefix:
			// 192 bitli tasodifiy kalitda amalda imkonsiz, lekin
			// jimgina o'tkazib yuborilsa ikki sherik bitta kalitni
			// bo'lishardi.
			return errors.New("kalit kolliziyasi — qaytadan urinib ko'ring")
		case idxAgentGrantsPartner:
			return errors.New("bu integratsiya siz bilan allaqachon ulangan")
		}
	}
	return err
}

func scanPartner(row pgx.Row) (*agentapi.Partner, error) {
	var (
		p      agentapi.Partner
		env    string
		scopes string
	)
	err := row.Scan(&p.ID, &p.Name, &p.KeyPrefix, &env, &scopes,
		&p.Active, &p.Contact, &p.CreatedAt, &p.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, agentapi.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.Environment = agentapi.Environment(env)
	p.Scopes = agentapi.ParseScopes(scopes)
	return &p, nil
}

func (r *PgAgentRepo) PartnerByKeyHash(ctx context.Context, keyHash string) (*agentapi.Partner, error) {
	if keyHash == "" {
		return nil, agentapi.ErrNotFound
	}
	return scanPartner(r.pool.QueryRow(ctx,
		`SELECT `+partnerColumns+` FROM agent_partners WHERE key_hash = $1`, keyHash))
}

func (r *PgAgentRepo) PartnerByID(ctx context.Context, id string) (*agentapi.Partner, error) {
	return scanPartner(r.pool.QueryRow(ctx,
		`SELECT `+partnerColumns+` FROM agent_partners WHERE id = $1`, id))
}

func (r *PgAgentRepo) ListPartners(ctx context.Context) ([]*agentapi.Partner, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+partnerColumns+` FROM agent_partners ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*agentapi.Partner
	for rows.Next() {
		p, err := scanPartner(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *PgAgentRepo) SetPartnerActive(ctx context.Context, id string, active bool) error {
	// `revoked_at` faollik bilan birga boshqariladi: o'chirilganda
	// vaqt yoziladi, qayta yoqilganda tozalanadi — aks holda faol
	// sherik "bekor qilingan" ko'rinib turardi.
	tag, err := r.pool.Exec(ctx, `
		UPDATE agent_partners
		SET active = $2,
		    revoked_at = CASE WHEN $2 THEN NULL ELSE COALESCE(revoked_at, now()) END
		WHERE id = $1`, id, active)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return agentapi.ErrNotFound
	}
	return nil
}

// ── Ulanish so'rovlari ──

const linkColumns = `id, partner_id, secret_hash, code_hash, external_ref, scopes,
	status, COALESCE(user_id,''), COALESCE(grant_id,''), created_at, expires_at, decided_at`

func (r *PgAgentRepo) CreateLink(ctx context.Context, l *agentapi.LinkRequest) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO agent_link_requests
			(id, partner_id, secret_hash, code_hash, external_ref, scopes,
			 status, created_at, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		l.ID, l.PartnerID, l.SecretHash, l.CodeHash, l.ExternalRef,
		agentapi.JoinScopes(l.Scopes), string(l.Status), l.CreatedAt, l.ExpiresAt)
	return mapAgentErr(err)
}

func scanLink(row pgx.Row) (*agentapi.LinkRequest, error) {
	var (
		l      agentapi.LinkRequest
		status string
		scopes string
	)
	err := row.Scan(&l.ID, &l.PartnerID, &l.SecretHash, &l.CodeHash, &l.ExternalRef,
		&scopes, &status, &l.UserID, &l.GrantID, &l.CreatedAt, &l.ExpiresAt, &l.DecidedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, agentapi.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	l.Status = agentapi.LinkStatus(status)
	l.Scopes = agentapi.ParseScopes(scopes)
	return &l, nil
}

func (r *PgAgentRepo) LinkByID(ctx context.Context, id string) (*agentapi.LinkRequest, error) {
	return scanLink(r.pool.QueryRow(ctx,
		`SELECT `+linkColumns+` FROM agent_link_requests WHERE id = $1`, id))
}

func (r *PgAgentRepo) LinkByCodeHash(ctx context.Context, codeHash string) (*agentapi.LinkRequest, error) {
	if codeHash == "" {
		return nil, agentapi.ErrNotFound
	}
	return scanLink(r.pool.QueryRow(ctx,
		`SELECT `+linkColumns+` FROM agent_link_requests WHERE code_hash = $1`, codeHash))
}

// UpdateLink — `secret_hash`/`code_hash` ATAYLAB yangilanmaydi.
// Sirlar bir marta yaratiladi; ularni o'zgartiradigan yo'l bo'lmasin.
func (r *PgAgentRepo) UpdateLink(ctx context.Context, l *agentapi.LinkRequest) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE agent_link_requests
		SET status = $2, user_id = NULLIF($3,''), grant_id = NULLIF($4,''), decided_at = $5
		WHERE id = $1`,
		l.ID, string(l.Status), l.UserID, l.GrantID, l.DecidedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return agentapi.ErrNotFound
	}
	return nil
}

// ── Grantlar ──

const grantColumns = `g.id, g.partner_id, g.user_id, g.token_hash, g.scopes, g.status,
	g.per_order_limit_tiyin, g.daily_limit_tiyin, g.created_at, g.last_used_at,
	g.revoked_at, g.expires_at, COALESCE(p.name,'')`

// grantFrom — grantni HAR DOIM sherik nomi bilan birga o'qiymiz.
// Foydalanuvchiga "Ulangan ilovalar" ro'yxatida ID emas, NOM
// ko'rsatiladi; alohida so'rov qilish esa N+1 muammosi berardi.
const grantFrom = ` FROM agent_grants g LEFT JOIN agent_partners p ON p.id = g.partner_id`

func (r *PgAgentRepo) CreateGrant(ctx context.Context, g *agentapi.Grant) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO agent_grants
			(id, partner_id, user_id, token_hash, scopes, status,
			 per_order_limit_tiyin, daily_limit_tiyin, created_at, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		g.ID, g.PartnerID, g.UserID, g.TokenHash, agentapi.JoinScopes(g.Scopes),
		string(g.Status), g.PerOrderLimitTiyin, g.DailyLimitTiyin, g.CreatedAt, g.ExpiresAt)
	return mapAgentErr(err)
}

func scanGrant(row pgx.Row) (*agentapi.Grant, error) {
	var (
		g      agentapi.Grant
		scopes string
		status string
	)
	err := row.Scan(&g.ID, &g.PartnerID, &g.UserID, &g.TokenHash, &scopes, &status,
		&g.PerOrderLimitTiyin, &g.DailyLimitTiyin, &g.CreatedAt, &g.LastUsedAt,
		&g.RevokedAt, &g.ExpiresAt, &g.PartnerName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, agentapi.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	g.Scopes = agentapi.ParseScopes(scopes)
	g.Status = agentapi.GrantStatus(status)
	return &g, nil
}

func (r *PgAgentRepo) GrantByID(ctx context.Context, id string) (*agentapi.Grant, error) {
	return scanGrant(r.pool.QueryRow(ctx,
		`SELECT `+grantColumns+grantFrom+` WHERE g.id = $1`, id))
}

// GrantByTokenHash — bo'sh hash bilan qidiruv RAD ETILADI.
//
// Grant yaratilgandan keyin `token_hash` bir muddat BO'SH turadi
// (token faqat sherik uni olib ketganda hosil bo'ladi). Bo'sh
// qiymatga yo'l qo'yilsa, hali hech kimga berilmagan grant topilib
// qolardi. Bazadagi qisman unikal indeks ham aynan shu sababdan
// `WHERE token_hash <> ”`.
func (r *PgAgentRepo) GrantByTokenHash(ctx context.Context, tokenHash string) (*agentapi.Grant, error) {
	if tokenHash == "" {
		return nil, agentapi.ErrNotFound
	}
	return scanGrant(r.pool.QueryRow(ctx,
		`SELECT `+grantColumns+grantFrom+` WHERE g.token_hash = $1`, tokenHash))
}

func (r *PgAgentRepo) ActiveGrantFor(ctx context.Context, partnerID, userID string) (*agentapi.Grant, error) {
	return scanGrant(r.pool.QueryRow(ctx,
		`SELECT `+grantColumns+grantFrom+`
		 WHERE g.partner_id = $1 AND g.user_id = $2 AND g.status = 'active'`,
		partnerID, userID))
}

func (r *PgAgentRepo) ListGrantsByUser(ctx context.Context, userID string) ([]*agentapi.Grant, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+grantColumns+grantFrom+`
		 WHERE g.user_id = $1 ORDER BY g.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*agentapi.Grant
	for rows.Next() {
		g, err := scanGrant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// UpdateGrant — `partner_id`/`user_id` ATAYLAB o'zgarmaydi: grantning
// egasini almashtirish degan amal umuman bo'lmasligi kerak.
func (r *PgAgentRepo) UpdateGrant(ctx context.Context, g *agentapi.Grant) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE agent_grants
		SET token_hash = $2, scopes = $3, status = $4,
		    per_order_limit_tiyin = $5, daily_limit_tiyin = $6,
		    revoked_at = $7, expires_at = $8
		WHERE id = $1`,
		g.ID, g.TokenHash, agentapi.JoinScopes(g.Scopes), string(g.Status),
		g.PerOrderLimitTiyin, g.DailyLimitTiyin, g.RevokedAt, g.ExpiresAt)
	if err != nil {
		return mapAgentErr(err)
	}
	if tag.RowsAffected() == 0 {
		return agentapi.ErrNotFound
	}
	return nil
}

func (r *PgAgentRepo) TouchGrant(ctx context.Context, id string, at time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE agent_grants SET last_used_at = $2 WHERE id = $1`, id, at)
	return err
}

// ── Qoralamalar ──

const draftColumns = `id, grant_id, user_id, partner_id, restaurant_id, items,
	subtotal_tiyin, discount_tiyin, total_tiyin, payment_method, status,
	requires_user, COALESCE(order_id,''), created_at, expires_at, decided_at`

func (r *PgAgentRepo) CreateDraft(ctx context.Context, d *agentapi.Draft) error {
	itemsJSON, err := json.Marshal(d.Items)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO agent_order_drafts
			(id, grant_id, user_id, partner_id, restaurant_id, items,
			 subtotal_tiyin, discount_tiyin, total_tiyin, payment_method,
			 status, requires_user, created_at, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		d.ID, d.GrantID, d.UserID, d.PartnerID, d.RestaurantID, itemsJSON,
		d.SubtotalTiyin, d.DiscountTiyin, d.TotalTiyin, d.PaymentMethod,
		string(d.Status), d.RequiresUser, d.CreatedAt, d.ExpiresAt)
	return err
}

func scanDraft(row pgx.Row) (*agentapi.Draft, error) {
	var (
		d      agentapi.Draft
		status string
		raw    []byte
	)
	err := row.Scan(&d.ID, &d.GrantID, &d.UserID, &d.PartnerID, &d.RestaurantID, &raw,
		&d.SubtotalTiyin, &d.DiscountTiyin, &d.TotalTiyin, &d.PaymentMethod,
		&status, &d.RequiresUser, &d.OrderID, &d.CreatedAt, &d.ExpiresAt, &d.DecidedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, agentapi.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	d.Status = agentapi.DraftStatus(status)
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &d.Items); err != nil {
			return nil, err
		}
	}
	return &d, nil
}

func (r *PgAgentRepo) DraftByID(ctx context.Context, id string) (*agentapi.Draft, error) {
	return scanDraft(r.pool.QueryRow(ctx,
		`SELECT `+draftColumns+` FROM agent_order_drafts WHERE id = $1`, id))
}

// UpdateDraft — FAQAT holat maydonlari. Tarkib va summa
// o'zgarmaydi: qoralama — foydalanuvchi ko'rgan narsaning SURATI,
// uni tasdiqdan keyin qayta yozish butun ikki bosqichli tasdiqni
// ma'nosiz qilardi.
func (r *PgAgentRepo) UpdateDraft(ctx context.Context, d *agentapi.Draft) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE agent_order_drafts
		SET status = $2, order_id = NULLIF($3,''), decided_at = $4
		WHERE id = $1`,
		d.ID, string(d.Status), d.OrderID, d.DecidedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return agentapi.ErrNotFound
	}
	return nil
}

func (r *PgAgentRepo) ListOpenDraftsByUser(ctx context.Context, userID string) ([]*agentapi.Draft, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+draftColumns+` FROM agent_order_drafts
		 WHERE user_id = $1 AND status = 'awaiting_user' AND expires_at > now()
		 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*agentapi.Draft
	for rows.Next() {
		d, err := scanDraft(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *PgAgentRepo) SpentSince(ctx context.Context, grantID string, since time.Time) (int64, error) {
	var sum int64
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(total_tiyin), 0) FROM agent_order_drafts
		WHERE grant_id = $1 AND status = 'placed' AND created_at >= $2`,
		grantID, since).Scan(&sum)
	return sum, err
}

// ── Audit ──

func (r *PgAgentRepo) AppendAudit(ctx context.Context, e *agentapi.AuditEntry) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO agent_audit (partner_id, grant_id, user_id, action, detail, ok, ip, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		e.PartnerID, e.GrantID, e.UserID, e.Action, e.Detail, e.OK, e.IP, e.CreatedAt,
	).Scan(&e.ID)
}

func (r *PgAgentRepo) ListAuditByUser(ctx context.Context, userID string, limit int) ([]*agentapi.AuditEntry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT a.id, a.partner_id, a.grant_id, a.user_id, a.action, a.detail,
		       a.ok, a.created_at, COALESCE(p.name,'')
		FROM agent_audit a LEFT JOIN agent_partners p ON p.id = a.partner_id
		WHERE a.user_id = $1 ORDER BY a.created_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*agentapi.AuditEntry
	for rows.Next() {
		var e agentapi.AuditEntry
		// `ip` ATAYLAB o'qilmaydi: bu ro'yxat foydalanuvchiga
		// qaytariladi va sherik serverining manzili unga kerak emas.
		if err := rows.Scan(&e.ID, &e.PartnerID, &e.GrantID, &e.UserID, &e.Action,
			&e.Detail, &e.OK, &e.CreatedAt, &e.Partner); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}
