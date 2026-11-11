package storage

import (
	"context"
	"testing"
	"time"
)

// Plane 317: yozuv yo'q = parol bilan kirish O'CHIQ; yoqish/o'chirish saqlanadi.
func TestPgLoginPrefs(t *testing.T) {
	pool := realmTestPool(t)
	ctx := context.Background()
	const id = "e2e-prefs-user"
	_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, phone, role, created_at) VALUES ($1, '+998907770099', 'restaurant', now())`, id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id) })
	p := NewPgLoginPrefs(pool)
	if on, err := p.PasswordLogin(ctx, id); on || err != nil {
		t.Fatalf("yozuvsiz akkauntda parol yoqiq: %v %v", on, err)
	}
	for _, want := range []bool{true, false, true} {
		if err := p.SetPasswordLogin(ctx, id, want, time.Now()); err != nil {
			t.Fatal(err)
		}
		if on, err := p.PasswordLogin(ctx, id); on != want || err != nil {
			t.Fatalf("kutilgan %v, keldi %v %v", want, on, err)
		}
	}
}
