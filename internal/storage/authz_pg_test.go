package storage

import (
	"context"
	"testing"

	"chustapp/internal/identity/authz"
	"chustapp/internal/identity/authz/authztest"
)

// PgAuthzStore xotira ombori bilan BIR XIL shartnomaga bo'ysunadi.
func TestPgAuthzStoreContract(t *testing.T) {
	pool := realmTestPool(t)
	ctx := context.Background()
	clean := func() {
		for _, q := range []string{
			`DELETE FROM memberships WHERE id LIKE 'ctz_%'`,
			`DELETE FROM roles WHERE id LIKE 'ctz_%'`,
			`DELETE FROM businesses WHERE id LIKE 'ctz_%'`,
			`DELETE FROM organizations WHERE id LIKE 'ctz_%'`,
		} {
			if _, err := pool.Exec(ctx, q); err != nil {
				t.Fatalf("%s: %v", q, err)
			}
		}
	}
	cleanUsers := func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE id LIKE 'ctz_u_%'`) }
	clean()
	cleanUsers()
	t.Cleanup(func() { clean(); cleanUsers() })
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, phone, role) VALUES
		('ctz_u_owner', '+998987650201', 'restaurant'),
		('ctz_u_staff', '+998987650202', 'waiter'),
		('ctz_u_other', '+998987650203', 'restaurant')`); err != nil {
		t.Fatal(err)
	}
	store := NewPgAuthzStore(pool)
	set := func(table string) func(id, status string) {
		return func(id, status string) {
			// 'active' tashkilot CHECK bo'yicha tasdiq vaqtini talab qiladi.
			q := `UPDATE ` + table + ` SET status = $2 WHERE id = $1`
			if table == "organizations" {
				q = `UPDATE organizations SET status = $2, verified_at = COALESCE(verified_at, now()) WHERE id = $1`
			}
			if _, err := pool.Exec(ctx, q, id, status); err != nil {
				t.Fatalf("%s holati: %v", table, err)
			}
		}
	}
	authztest.RunStoreContract(t, func(t *testing.T) *authztest.Env {
		clean()
		return &authztest.Env{Store: store, Owner: "ctz_u_owner", Staff: "ctz_u_staff", Other: "ctz_u_other",
			SetOrgStatus: set("organizations"), SetBusinessStatus: set("businesses")}
	})
}

var _ authz.Store = (*PgAuthzStore)(nil)
