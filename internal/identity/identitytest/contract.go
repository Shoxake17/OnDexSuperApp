// Package identitytest — `identity.Store` implementatsiyalari uchun UMUMIY
// shartnoma testi: xotira va Postgres ombori AYNAN bir xil qoidalarda
// ishlashi shart (bitta to'plam ikkalasiga qo'llanadi).
package identitytest

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"testing"
	"time"

	"chustapp/internal/identity"
	"chustapp/internal/users"
)

const grace = 10 * time.Second

var base = time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

func h(s string) []byte { sum := sha256.Sum256([]byte(s)); return sum[:] }

func idleHour(string) time.Duration { return time.Hour }

type contract struct {
	ctx      context.Context
	newStore func(t *testing.T) identity.Store
	userID   string
	mu       sync.Mutex
	seq      int
}

// RunStoreContract — `newStore` har chaqirilganda toza ombor qaytarsin.
// `userID` — sessiya egasi (Postgres'da users jadvalida MAVJUD bo'lishi shart).
func RunStoreContract(t *testing.T, newStore func(t *testing.T) identity.Store, userID string) {
	c := &contract{ctx: context.Background(), newStore: newStore, userID: userID}
	t.Run("rotatsiya eski tokenni yaroqsiz qiladi", c.rotation)
	t.Run("eski token grace oynasidan keyin = o'g'irlik, sessiya bekor", c.theft)
	t.Run("halol poyga oynasida bekor qilinmaydi", c.race)
	t.Run("muddati o'tgan va noma'lum", c.expiredAndUnknown)
	t.Run("idle mutlaq muddatdan oshmaydi", c.idleCappedByAbs)
	t.Run("klient o'chirilgan = sessiya yopiladi", c.clientRemoved)
	t.Run("Revoke, ListActive, RevokeAllForUser", c.revocation)
	t.Run("parallel refresh: faqat BITTASI o'tadi", c.parallel)
	t.Run("bir xil refresh xeshi ikki sessiyada bo'lmaydi", c.duplicateHash)
	t.Run("parent_id saqlanadi, RevokeChildren faqat bolalarni yopadi", c.children)
}

// children — OIDC: SSO sessiyasi va undan ochilgan ilova sessiyalari.
func (c *contract) children(t *testing.T) {
	st := c.newStore(t)
	parent := c.session(t, st, "ch-parent")
	other := c.session(t, st, "ch-other")
	var kids []string
	for i, ref := range []string{"ch-k1", "ch-k2"} {
		c.mu.Lock()
		c.seq++
		n := c.seq
		c.mu.Unlock()
		k := &identity.Session{ID: fmt.Sprintf("ctrk_%d_%d_%d", time.Now().UnixNano(), n, i), UserID: c.userID,
			Realm: users.RealmMerchant, ClientID: "merchant-web", CreatedAt: base, LastUsedAt: base,
			IdleExpiresAt: base.Add(time.Hour), AbsExpiresAt: base.Add(24 * time.Hour), ParentID: parent.ID}
		if err := st.Create(c.ctx, k, h(ref+k.ID)); err != nil {
			t.Fatal(err)
		}
		kids = append(kids, k.ID)
	}
	got, err := st.Get(c.ctx, kids[0])
	if err != nil || got.ParentID != parent.ID {
		t.Fatalf("parent_id saqlanmadi: %v %+v", err, got)
	}
	if p, _ := st.Get(c.ctx, parent.ID); p.ParentID != "" {
		t.Fatalf("ota sessiyada parent_id bo'sh bo'lishi kerak: %q", p.ParentID)
	}
	revoked, err := st.RevokeChildren(c.ctx, parent.ID, "sso_logout", base.Add(time.Minute))
	if err != nil || len(revoked) != 2 {
		t.Fatalf("RevokeChildren: %v, %d ta", err, len(revoked))
	}
	for _, id := range kids {
		if s, _ := st.Get(c.ctx, id); s.RevokedAt == nil || s.RevokeReason != "sso_logout" {
			t.Fatalf("bola sessiya yopilmadi: %+v", s)
		}
	}
	for _, id := range []string{parent.ID, other.ID} {
		if s, _ := st.Get(c.ctx, id); s.RevokedAt != nil {
			t.Fatalf("begona sessiya yopildi: %s", id)
		}
	}
	again, _ := st.RevokeChildren(c.ctx, parent.ID, "sso_logout", base.Add(2*time.Minute))
	if len(again) != 0 {
		t.Fatalf("allaqachon yopilgan bolalar qayta qaytarildi: %d", len(again))
	}
	if none, _ := st.RevokeChildren(c.ctx, "", "x", base); len(none) != 0 {
		t.Fatal("bo'sh parent bilan sessiyalar yopildi")
	}
}

func (c *contract) session(t *testing.T, st identity.Store, refresh string) *identity.Session {
	t.Helper()
	c.mu.Lock()
	c.seq++
	n := c.seq
	c.mu.Unlock()
	s := &identity.Session{
		ID: fmt.Sprintf("ctr_%d_%d", time.Now().UnixNano(), n), UserID: c.userID,
		Realm: users.RealmMerchant, ClientID: "merchant-web", AMR: []string{"otp"},
		CreatedAt: base, LastUsedAt: base,
		IdleExpiresAt: base.Add(time.Hour), AbsExpiresAt: base.Add(24 * time.Hour),
	}
	if err := st.Create(c.ctx, s, h(refresh)); err != nil {
		t.Fatalf("Create: %v", err)
	}
	return s
}

func (c *contract) rotate(t *testing.T, st identity.Store, from, to string, at time.Time) (*identity.Session, identity.RotateResult) {
	t.Helper()
	s, res, err := st.Rotate(c.ctx, h(from+t.Name()), h(to+t.Name()), at, idleHour, grace)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	return s, res
}

func (c *contract) rotation(t *testing.T) {
	st := c.newStore(t)
	s := c.session(t, st, "r1"+t.Name())
	now := base.Add(10 * time.Minute)
	got, res := c.rotate(t, st, "r1", "r2", now)
	if res != identity.RotateOK || got.ID != s.ID {
		t.Fatalf("1-rotatsiya: %v", res)
	}
	if !got.IdleExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("idle uzaytirilmadi: %v", got.IdleExpiresAt)
	}
	if _, res := c.rotate(t, st, "r2", "r3", now.Add(time.Minute)); res != identity.RotateOK {
		t.Fatalf("2-rotatsiya: %v", res)
	}
}

func (c *contract) theft(t *testing.T) {
	st := c.newStore(t)
	s := c.session(t, st, "a1"+t.Name())
	now := base.Add(time.Minute)
	if _, res := c.rotate(t, st, "a1", "a2", now); res != identity.RotateOK {
		t.Fatal("rotatsiya")
	}
	got, res := c.rotate(t, st, "a1", "x", now.Add(time.Minute))
	if res != identity.RotateReused || got.ID != s.ID {
		t.Fatalf("kutilgan RotateReused, keldi %v", res)
	}
	g, _ := st.Get(c.ctx, s.ID)
	if g.RevokedAt == nil || g.RevokeReason != "refresh_reuse" {
		t.Fatalf("sessiya bekor qilinmadi: %+v", g)
	}
	// endi YANGI token ham ishlamaydi
	if _, res := c.rotate(t, st, "a2", "y", now.Add(2*time.Minute)); res != identity.RotateExpired {
		t.Fatalf("bekor qilingan sessiyaning yangi tokeni ishladi: %v", res)
	}
}

func (c *contract) race(t *testing.T) {
	st := c.newStore(t)
	s := c.session(t, st, "p1"+t.Name())
	now := base.Add(time.Minute)
	if _, res := c.rotate(t, st, "p1", "p2", now); res != identity.RotateOK {
		t.Fatal("rotatsiya")
	}
	if _, res := c.rotate(t, st, "p1", "z", now.Add(3*time.Second)); res != identity.RotateRaced {
		t.Fatalf("kutilgan RotateRaced, keldi %v", res)
	}
	if g, _ := st.Get(c.ctx, s.ID); g.RevokedAt != nil {
		t.Fatal("halol poygada sessiya bekor qilindi")
	}
}

func (c *contract) expiredAndUnknown(t *testing.T) {
	st := c.newStore(t)
	c.session(t, st, "e1"+t.Name())
	if _, res := c.rotate(t, st, "e1", "e2", base.Add(2*time.Hour)); res != identity.RotateExpired {
		t.Fatalf("idle o'tgan: %v", res)
	}
	if _, res := c.rotate(t, st, "yoq", "e3", base); res != identity.RotateNotFound {
		t.Fatalf("noma'lum: %v", res)
	}
}

func (c *contract) idleCappedByAbs(t *testing.T) {
	st := c.newStore(t)
	c.session(t, st, "m1"+t.Name())
	long := func(string) time.Duration { return 48 * time.Hour } // abs (24 soat)dan uzun
	got, res, err := st.Rotate(c.ctx, h("m1"+t.Name()), h("m2"+t.Name()), base.Add(30*time.Minute), long, grace)
	if err != nil || res != identity.RotateOK || !got.IdleExpiresAt.Equal(got.AbsExpiresAt) {
		t.Fatalf("idle mutlaq muddatga qisilmadi: %v %v", res, err)
	}
}

func (c *contract) clientRemoved(t *testing.T) {
	st := c.newStore(t)
	s := c.session(t, st, "c1"+t.Name())
	none := func(string) time.Duration { return 0 }
	if _, res, _ := st.Rotate(c.ctx, h("c1"+t.Name()), h("c2"+t.Name()), base.Add(time.Minute), none, grace); res != identity.RotateExpired {
		t.Fatalf("kutilgan RotateExpired: %v", res)
	}
	if g, _ := st.Get(c.ctx, s.ID); g.RevokedAt == nil {
		t.Fatal("sessiya ochiq qoldi")
	}
}

func (c *contract) revocation(t *testing.T) {
	st := c.newStore(t)
	a := c.session(t, st, "l1"+t.Name())
	b := c.session(t, st, "l2"+t.Name())
	now := base.Add(time.Minute)
	if list, err := st.ListActive(c.ctx, c.userID, now); err != nil || !has(list, a.ID) || !has(list, b.ID) {
		t.Fatalf("ListActive: %v", err)
	}
	if err := st.Revoke(c.ctx, a.ID, "logout", now); err != nil {
		t.Fatal(err)
	}
	if list, _ := st.ListActive(c.ctx, c.userID, now); has(list, a.ID) {
		t.Fatal("bekor qilingan ro'yxatda")
	}
	if err := st.Revoke(c.ctx, "yo'q-sessiya", "x", now); err != identity.ErrNotFound {
		t.Fatalf("noma'lum Revoke: %v", err)
	}
	if _, err := st.RevokeAllForUser(c.ctx, c.userID, "password_changed", now); err != nil {
		t.Fatal(err)
	}
	if g, _ := st.Get(c.ctx, b.ID); g.RevokedAt == nil || g.RevokeReason != "password_changed" {
		t.Fatalf("RevokeAll: %+v", g)
	}
	if _, err := st.Get(c.ctx, "yo'q"); err != identity.ErrNotFound {
		t.Fatalf("Get noma'lum: %v", err)
	}
}

func (c *contract) parallel(t *testing.T) {
	st := c.newStore(t)
	c.session(t, st, "par"+t.Name())
	now := base.Add(time.Minute)
	const n = 10
	results := make(chan identity.RotateResult, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, res, err := st.Rotate(c.ctx, h("par"+t.Name()), h(fmt.Sprintf("par-new-%d-%s", i, t.Name())), now, idleHour, grace)
			if err != nil {
				t.Errorf("Rotate: %v", err)
			}
			results <- res
		}(i)
	}
	wg.Wait()
	close(results)
	ok := 0
	for r := range results {
		switch r {
		case identity.RotateOK:
			ok++
		case identity.RotateRaced:
		default:
			t.Errorf("kutilmagan natija: %v", r)
		}
	}
	if ok != 1 {
		t.Fatalf("parallel refresh'da %d ta muvaffaqiyat (1 bo'lishi shart)", ok)
	}
}

func (c *contract) duplicateHash(t *testing.T) {
	st := c.newStore(t)
	c.session(t, st, "u1"+t.Name())
	dup := &identity.Session{ID: fmt.Sprintf("ctr_dup_%d", time.Now().UnixNano()), UserID: c.userID,
		Realm: users.RealmMerchant, ClientID: "merchant-web", CreatedAt: base, LastUsedAt: base,
		IdleExpiresAt: base.Add(time.Hour), AbsExpiresAt: base.Add(2 * time.Hour)}
	if err := st.Create(c.ctx, dup, h("u1"+t.Name())); err == nil {
		t.Fatal("takroriy refresh xeshi qabul qilindi")
	}
}

func has(list []*identity.Session, id string) bool {
	for _, s := range list {
		if s.ID == id {
			return true
		}
	}
	return false
}
