package storage

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"chustapp/internal/identity/passkey"
)

// PgPasskeyStore (0074): qo'shish, takror ID, chegara (parallel ham),
// boshqa akkaunt kalitiga tegolmaslik, bir martalik challenge, CASCADE.
func TestPgPasskeyStore(t *testing.T) {
	pool := realmTestPool(t)
	ctx := context.Background()
	const uid, other = "passkey_user_pg", "passkey_other_pg"
	clean := func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1, $2)`, uid, other) }
	clean()
	t.Cleanup(clean)
	for i, id := range []string{uid, other} {
		if _, err := pool.Exec(ctx, `INSERT INTO users (id, phone, role) VALUES ($1, $2, 'merchant')`, id,
			fmt.Sprintf("+99898765099%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	s := NewPgPasskeyStore(pool)
	cred := func(user string, n int) *passkey.Credential {
		return &passkey.Credential{ID: fmt.Sprintf("cred-%s-%04d-abcdefgh", user, n), UserID: user, Name: "Kalit",
			Data: []byte(`{"publicKey":"AQID"}`)}
	}

	if err := s.Add(ctx, cred(uid, 0)); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(ctx, cred(uid, 0)); !errors.Is(err, passkey.ErrExists) {
		t.Fatalf("takror ID: %v", err)
	}
	// Chegara parallel so'rovlarda ham oshmaydi.
	var wg sync.WaitGroup
	for i := 1; i <= passkey.MaxPerUser+5; i++ {
		wg.Add(1)
		go func(i int) { defer wg.Done(); _ = s.Add(ctx, cred(uid, i)) }(i)
	}
	wg.Wait()
	if list, _ := s.List(ctx, uid); len(list) != passkey.MaxPerUser {
		t.Fatalf("chegara: %d", len(list))
	}

	id := cred(uid, 0).ID
	if err := s.Touch(ctx, other, id, []byte(`{}`), time.Now()); !errors.Is(err, passkey.ErrNotFound) {
		t.Fatalf("boshqa akkaunt kalitini yangiladi: %v", err)
	}
	if err := s.Delete(ctx, other, id); !errors.Is(err, passkey.ErrNotFound) {
		t.Fatalf("boshqa akkaunt kalitini o'chirdi: %v", err)
	}
	if err := s.Touch(ctx, uid, id, []byte(`{"publicKey":"BAUG"}`), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, uid, id); err != nil {
		t.Fatal(err)
	}

	h := sha256.Sum256([]byte(fmt.Sprintf("challenge-%d", time.Now().UnixNano())))
	if ok, err := s.UseChallenge(ctx, h[:], time.Now().Add(time.Minute)); !ok || err != nil {
		t.Fatalf("birinchi: %v %v", ok, err)
	}
	if ok, err := s.UseChallenge(ctx, h[:], time.Now().Add(time.Minute)); ok || err != nil {
		t.Fatalf("XAVFSIZLIK: challenge ikki marta qabul qilindi: %v %v", ok, err)
	}

	clean()
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM webauthn_credentials WHERE user_id = $1`, uid).Scan(&n)
	if n != 0 {
		t.Fatal("akkaunt o'chganda passkey qoldi")
	}
}
