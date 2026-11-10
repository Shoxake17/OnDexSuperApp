package storage

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/identity/passkey"
)

// PgPasskeyStore — `webauthn_credentials`, `webauthn_used_challenges` (0074).
type PgPasskeyStore struct{ pool *pgxpool.Pool }

func NewPgPasskeyStore(pool *pgxpool.Pool) *PgPasskeyStore { return &PgPasskeyStore{pool: pool} }

func (r *PgPasskeyStore) List(ctx context.Context, userID string) ([]passkey.Credential, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, user_id, name, data, created_at, last_used_at
		FROM webauthn_credentials WHERE user_id = $1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []passkey.Credential
	for rows.Next() {
		var c passkey.Credential
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Data, &c.CreatedAt, &c.LastUsedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Add — chegara (MaxPerUser) va qo'shish BITTA so'rovda: ikki parallel
// ro'yxatdan o'tish chegarani oshirib yubora olmaydi (foydalanuvchi qatori
// FOR UPDATE bilan qulflanadi).
func (r *PgPasskeyStore) Add(ctx context.Context, c *passkey.Credential) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT 1 FROM users WHERE id = $1 FOR UPDATE`, c.UserID); err != nil {
		return err
	}
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM webauthn_credentials WHERE user_id = $1`, c.UserID).Scan(&n); err != nil {
		return err
	}
	if n >= passkey.MaxPerUser {
		return passkey.ErrTooMany
	}
	_, err = tx.Exec(ctx, `INSERT INTO webauthn_credentials (id, user_id, name, data) VALUES ($1, $2, $3, $4)`,
		c.ID, c.UserID, c.Name, c.Data)
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23505" {
		return passkey.ErrExists
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PgPasskeyStore) Touch(ctx context.Context, userID, id string, data []byte, at time.Time) error {
	tag, err := r.pool.Exec(ctx, `UPDATE webauthn_credentials SET data = $3, last_used_at = $4
		WHERE id = $1 AND user_id = $2`, id, userID, data, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return passkey.ErrNotFound
	}
	return nil
}

func (r *PgPasskeyStore) Delete(ctx context.Context, userID, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM webauthn_credentials WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return passkey.ErrNotFound
	}
	return nil
}

func (r *PgPasskeyStore) UseChallenge(ctx context.Context, hash []byte, expires time.Time) (bool, error) {
	if _, err := r.pool.Exec(ctx, `DELETE FROM webauthn_used_challenges WHERE expires_at < now()`); err != nil {
		return false, err
	}
	tag, err := r.pool.Exec(ctx, `INSERT INTO webauthn_used_challenges (hash, expires_at) VALUES ($1, $2)
		ON CONFLICT (hash) DO NOTHING`, hash, expires)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
