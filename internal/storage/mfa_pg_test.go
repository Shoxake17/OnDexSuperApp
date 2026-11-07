package storage

import (
	"context"
	"testing"

	"chustapp/internal/identity/mfa"
	"chustapp/internal/identity/mfa/mfatest"
)

// PgMFAStore xotira ombori bilan BIR XIL shartnomaga bo'ysunadi.
func TestPgMFAStoreContract(t *testing.T) {
	pool := realmTestPool(t)
	ctx := context.Background()
	const uid = "mfat_user_pg"
	clean := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM mfa_recovery_codes WHERE user_id = $1`, uid)
		_, _ = pool.Exec(ctx, `DELETE FROM mfa_factors WHERE user_id = $1`, uid)
	}
	clean()
	_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, uid)
	t.Cleanup(func() { clean(); _, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, uid) })
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, phone, role) VALUES ($1, '+998987650301', 'restaurant')`, uid); err != nil {
		t.Fatal(err)
	}
	store := NewPgMFAStore(pool)
	mfatest.RunStoreContract(t, func(*testing.T) mfa.Store { clean(); return store }, uid)
}
