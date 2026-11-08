package storage

import (
	"context"
	"crypto/sha256"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"chustapp/internal/identity"
	"chustapp/internal/users"
)

// PgOAuthCodeStore: bir martalik, atomik, muddatli (migratsiya 0072).
func TestPgOAuthCodeStore(t *testing.T) {
	pool := realmTestPool(t)
	ctx := context.Background()
	const uid, sid = "oac_user_pg", "oac_sso_pg"
	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM oauth_codes WHERE user_id = $1`, uid)
		_, _ = pool.Exec(ctx, `DELETE FROM auth_sessions WHERE user_id = $1`, uid)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, uid)
	}
	cleanup()
	t.Cleanup(cleanup)
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, phone, role) VALUES ($1, '+998987650777', 'merchant')`, uid); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	sess := &identity.Session{ID: sid, UserID: uid, Realm: users.RealmMerchant, ClientID: identity.SSOClientID,
		CreatedAt: now, LastUsedAt: now, IdleExpiresAt: now.Add(time.Hour), AbsExpiresAt: now.Add(time.Hour)}
	rh := sha256.Sum256([]byte("oac-sso-refresh"))
	if err := NewPgAuthSessionStore(pool).Create(ctx, sess, rh[:]); err != nil {
		t.Fatal(err)
	}
	st := NewPgOAuthCodeStore(pool)
	put := func(name string, ttl time.Duration) []byte {
		h := sha256.Sum256([]byte(name))
		c := &identity.AuthCode{Hash: h[:], ClientID: "merchant-web", RedirectURI: "https://pos.ondex.test/cb",
			Challenge: "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", Nonce: "n", Scope: "openid", UserID: uid,
			ParentSessionID: sid, AMR: []string{"pwd", "totp"}, AuthTime: now, CreatedAt: now, ExpiresAt: now.Add(ttl)}
		if err := st.Put(ctx, c); err != nil {
			t.Fatalf("Put: %v", err)
		}
		return h[:]
	}

	h1 := put("kod-1", time.Minute)
	c, res, err := st.Consume(ctx, h1, now)
	if err != nil || res != identity.CodeOK || c.ParentSessionID != sid || len(c.AMR) != 2 {
		t.Fatalf("birinchi Consume: %v %v %+v", err, res, c)
	}
	if err := st.SetIssued(ctx, h1, "issued_x"); err != nil {
		t.Fatal(err)
	}
	c, res, err = st.Consume(ctx, h1, now)
	if err != nil || res != identity.CodeReused || c == nil || c.IssuedSessionID != "issued_x" {
		t.Fatalf("qayta Consume: %v %v %+v", err, res, c)
	}

	h2 := put("kod-2", time.Minute)
	if _, res, _ := st.Consume(ctx, h2, now.Add(2*time.Minute)); res != identity.CodeInvalid {
		t.Fatalf("eskirgan kod: %v", res)
	}
	unknown := sha256.Sum256([]byte("yo'q"))
	if _, res, _ := st.Consume(ctx, unknown[:], now); res != identity.CodeInvalid {
		t.Fatalf("noma'lum kod: %v", res)
	}

	// Parallel: 20 ta so'rovdan faqat BITTASI kodni oladi.
	h3 := put("kod-3", time.Minute)
	var ok atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, res, err := st.Consume(ctx, h3, now); err == nil && res == identity.CodeOK {
				ok.Add(1)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 1 {
		t.Fatalf("XAVFSIZLIK: kod %d marta almashtirildi", ok.Load())
	}
}
