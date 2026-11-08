package storage

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/identity"
)

// PgOAuthCodeStore — `oauth_codes` (migratsiya 0072). `identity.CodeStore`.
type PgOAuthCodeStore struct{ pool *pgxpool.Pool }

func NewPgOAuthCodeStore(pool *pgxpool.Pool) *PgOAuthCodeStore { return &PgOAuthCodeStore{pool: pool} }

const oauthCodeCols = `code_hash, client_id, redirect_uri, code_challenge, nonce, scope, user_id,
	parent_session_id, org_id, amr, auth_time, created_at, expires_at, COALESCE(issued_session_id, '')`

func scanOAuthCode(row pgx.Row) (*identity.AuthCode, error) {
	var c identity.AuthCode
	err := row.Scan(&c.Hash, &c.ClientID, &c.RedirectURI, &c.Challenge, &c.Nonce, &c.Scope, &c.UserID,
		&c.ParentSessionID, &c.OrgID, &c.AMR, &c.AuthTime, &c.CreatedAt, &c.ExpiresAt, &c.IssuedSessionID)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *PgOAuthCodeStore) Put(ctx context.Context, c *identity.AuthCode) error {
	amr := c.AMR
	if amr == nil {
		amr = []string{}
	}
	// Eskirgan kodlarni yo'l-yo'lakay tozalash (kunlik hajm kichik; alohida
	// fon ishi shart emas). Bir kundan eski — tekshiruv/audit uchun yetarli.
	if _, err := r.pool.Exec(ctx, `DELETE FROM oauth_codes WHERE expires_at < $1`, c.CreatedAt.Add(-24*time.Hour)); err != nil {
		return err
	}
	_, err := r.pool.Exec(ctx, `INSERT INTO oauth_codes (code_hash, client_id, redirect_uri, code_challenge, nonce,
		scope, user_id, parent_session_id, org_id, amr, auth_time, created_at, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		c.Hash, c.ClientID, c.RedirectURI, c.Challenge, c.Nonce, c.Scope, c.UserID, c.ParentSessionID,
		c.OrgID, amr, c.AuthTime, c.CreatedAt, c.ExpiresAt)
	return err
}

// Consume — bitta UPDATE bilan atomik: parallel ikki so'rovdan faqat
// bittasi `used_at` ni qo'ya oladi.
func (r *PgOAuthCodeStore) Consume(ctx context.Context, hash []byte, now time.Time) (*identity.AuthCode, identity.CodeResult, error) {
	c, err := scanOAuthCode(r.pool.QueryRow(ctx, `UPDATE oauth_codes SET used_at = $2
		WHERE code_hash = $1 AND used_at IS NULL AND expires_at > $2 RETURNING `+oauthCodeCols, hash, now))
	if err == nil {
		return c, identity.CodeOK, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, err
	}
	var used *time.Time
	c, err = scanOAuthCodeUsed(r.pool.QueryRow(ctx, `SELECT `+oauthCodeCols+`, used_at FROM oauth_codes
		WHERE code_hash = $1`, hash), &used)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, identity.CodeInvalid, nil
	}
	if err != nil {
		return nil, 0, err
	}
	if used != nil {
		return c, identity.CodeReused, nil
	}
	return nil, identity.CodeInvalid, nil
}

func scanOAuthCodeUsed(row pgx.Row, used **time.Time) (*identity.AuthCode, error) {
	var c identity.AuthCode
	err := row.Scan(&c.Hash, &c.ClientID, &c.RedirectURI, &c.Challenge, &c.Nonce, &c.Scope, &c.UserID,
		&c.ParentSessionID, &c.OrgID, &c.AMR, &c.AuthTime, &c.CreatedAt, &c.ExpiresAt, &c.IssuedSessionID, used)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *PgOAuthCodeStore) SetIssued(ctx context.Context, hash []byte, sessionID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE oauth_codes SET issued_session_id = $2 WHERE code_hash = $1`, hash, sessionID)
	return err
}
