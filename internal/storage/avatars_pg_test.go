package storage

import (
	"context"
	"testing"
)

// PgAvatarStore (0073): yozish, yangilash, o'chirish; akkaunt o'chsa rasm ham (CASCADE).
func TestPgAvatarStore(t *testing.T) {
	pool := realmTestPool(t)
	ctx := context.Background()
	const uid = "avatar_user_pg"
	_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, uid)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, uid) })
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, phone, role) VALUES ($1, '+998987650888', 'merchant')`, uid); err != nil {
		t.Fatal(err)
	}
	s := NewPgAvatarStore(pool)
	if got, err := s.Get(ctx, uid); err != nil || got != "" {
		t.Fatalf("bo'sh: %q %v", got, err)
	}
	for _, u := range []string{"https://img.ondex.test/avatars/a.webp", "https://img.ondex.test/avatars/b.webp"} {
		if err := s.Set(ctx, uid, u); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.Get(ctx, uid); got != u {
			t.Fatalf("Set/Get: %q != %q", got, u)
		}
	}
	if err := s.Set(ctx, uid, "javascript:alert(1)"); err == nil {
		t.Fatal("http(s) bo'lmagan manzil bazaga yozildi")
	}
	if err := s.Set(ctx, uid, ""); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(ctx, uid); got != "" {
		t.Fatalf("o'chirilmadi: %q", got)
	}
	_ = s.Set(ctx, uid, "https://img.ondex.test/avatars/c.webp")
	_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, uid)
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM user_avatars WHERE user_id = $1`, uid).Scan(&n)
	if n != 0 {
		t.Fatal("akkaunt o'chganda avatar qoldi")
	}
}
