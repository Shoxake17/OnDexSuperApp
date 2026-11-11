package storage

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgLoginPrefs — `identity.LoginPrefs` (merchant_login_prefs, 0075).
// Yozuv yo'q = parol bilan kirish O'CHIQ.
type PgLoginPrefs struct{ pool *pgxpool.Pool }

func NewPgLoginPrefs(pool *pgxpool.Pool) *PgLoginPrefs { return &PgLoginPrefs{pool: pool} }

func (p *PgLoginPrefs) PasswordLogin(ctx context.Context, userID string) (bool, error) {
	var on bool
	err := p.pool.QueryRow(ctx, `SELECT password_login FROM merchant_login_prefs WHERE user_id = $1`, userID).Scan(&on)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return on, err
}

func (p *PgLoginPrefs) SetPasswordLogin(ctx context.Context, userID string, enabled bool, at time.Time) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO merchant_login_prefs (user_id, password_login, updated_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE SET password_login = EXCLUDED.password_login, updated_at = EXCLUDED.updated_at`,
		userID, enabled, at)
	return err
}
