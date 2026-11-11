package identity_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"chustapp/internal/identity"
	"chustapp/internal/identity/authz"
	"chustapp/internal/identity/mfa"
	"chustapp/internal/users"
)

// AUD-15: mfa_token BIR MARTALIK. Ilgari muvaffaqiyatli VerifyMFA'dan keyin
// o'sha token bilan (keyingi TOTP qadamida) yana sessiya ochish mumkin edi.
func TestMerchant_MFATokenSingleUse(t *testing.T) {
	f := newMerchantFixture(t)
	f.user(t, "u_once01", users.RoleRestaurant)
	phone := f.phoneOf(t, "u_once01")
	f.setPassword(t, phone, "parol-123")
	secret, _ := f.enroll(t, "u_once01")

	out, err := f.m.LoginPassword(f.ctx, phone, "parol-123", "merchant-web", meta)
	if err != nil || out.MFA == nil {
		t.Fatal(err)
	}
	f.clk.add(mfa.Period)
	code, _ := mfa.Code(secret, f.clk.now())
	if _, err := f.m.VerifyMFA(f.ctx, out.MFA.Token, code, meta); err != nil {
		t.Fatal(err)
	}
	f.clk.add(mfa.Period)
	next, _ := mfa.Code(secret, f.clk.now())
	if _, err := f.m.VerifyMFA(f.ctx, out.MFA.Token, next, meta); !errors.Is(err, identity.ErrMFATokenInvalid) {
		t.Fatalf("XAVFSIZLIK: ishlatilgan mfa_token yangi TOTP bilan ikkinchi sessiya ochdi: %v", err)
	}
	// Noto'g'ri kod esa tokenni YONDIRMAYDI (foydalanuvchi qaytadan boshlamaydi).
	out, _ = f.m.LoginPassword(f.ctx, phone, "parol-123", "merchant-web", meta)
	if _, err := f.m.VerifyMFA(f.ctx, out.MFA.Token, "000000", meta); !errors.Is(err, identity.ErrMFAInvalid) {
		t.Fatalf("noto'g'ri kod: %v", err)
	}
	f.clk.add(mfa.Period)
	code, _ = mfa.Code(secret, f.clk.now())
	if _, err := f.m.VerifyMFA(f.ctx, out.MFA.Token, code, meta); err != nil {
		t.Fatalf("xato koddan keyin to'g'ri kod o'tmadi: %v", err)
	}
}

// Oraliq bosqich (AddFactor) tokeni ham bir martalik: bitta parol+kod
// juftligidan ikki parallel "shox" ochib bo'lmaydi.
func TestMerchant_FactorTokenSingleUse(t *testing.T) {
	f := newMerchantFixture(t)
	f.user(t, "u_once02", users.RoleRestaurant)
	phone := f.phoneOf(t, "u_once02")
	f.setPassword(t, phone, "parol-123")
	out, err := f.m.LoginPassword(f.ctx, phone, "parol-123", "merchant-web", meta)
	if err != nil || out.MFA == nil || out.MFA.Method != "otp" {
		t.Fatalf("%+v %v", out, err)
	}
	if _, err := f.m.AddFactor(f.ctx, out.MFA.Token, f.fu.sendCode(phone), meta); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.AddFactor(f.ctx, out.MFA.Token, f.fu.sendCode(phone), meta); !errors.Is(err, identity.ErrMFATokenInvalid) {
		t.Fatalf("XAVFSIZLIK: oraliq token qayta ishlatildi: %v", err)
	}
}

// memGuardStore — Redis GuardStore semantikasi (storage testida haqiqiy Redis).
type memGuardStore struct {
	mu   sync.Mutex
	n, i map[string]int
	fail bool
}

func newMemGuardStore() *memGuardStore {
	return &memGuardStore{n: map[string]int{}, i: map[string]int{}}
}

var errRedisDown = errors.New("redis: connection refused")

func (s *memGuardStore) Begin(_ context.Context, key string, max int, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return false, errRedisDown
	}
	if s.n[key]+s.i[key] >= max {
		return false, nil
	}
	s.i[key]++
	return true, nil
}

func (s *memGuardStore) Finish(_ context.Context, key, res string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return errRedisDown
	}
	if s.i[key] > 0 {
		s.i[key]--
	}
	switch res {
	case "ok":
		delete(s.n, key)
	case "fail":
		s.n[key]++
	}
	return nil
}

func (s *memGuardStore) Fail(_ context.Context, key string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return errRedisDown
	}
	s.n[key]++
	return nil
}

func (s *memGuardStore) Reset(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.n, key)
	return nil
}

func (s *memGuardStore) Count(_ context.Context, key string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return 0, errRedisDown
	}
	return s.n[key] + s.i[key], nil
}

// Ikki API nusxasi (bitta Redis): hujumchi so'rovlarni nusxalar orasida
// bo'lib yuborsa ham jami 5 ta xatodan keyin akkaunt bloklanadi.
// Ilgari har nusxada alohida 5 ta urinish bor edi.
func TestMerchant_GuardSharedAcrossInstances(t *testing.T) {
	f := newMerchantFixture(t)
	store := newMemGuardStore()
	mk := func() *identity.MerchantAuth {
		m, err := identity.NewMerchantAuth(f.svc, identity.MerchantDeps{Users: f.fu, Repo: f.users,
			Authz: authz.New(f.az), MFA: f.mfaSt, Box: fakeBox{}, Guard: store})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	a, b := mk(), mk()
	f.user(t, "u_dist01", users.RoleRestaurant)
	phone := f.phoneOf(t, "u_dist01")
	f.setPassword(t, phone, "parol-123")
	f.m = a
	secret, _ := f.enroll(t, "u_dist01")

	login := func(m *identity.MerchantAuth) string {
		out, err := m.LoginPassword(f.ctx, phone, "parol-123", "merchant-web", meta)
		if err != nil || out.MFA == nil {
			t.Fatal(err)
		}
		return out.MFA.Token
	}
	for i := 0; i < 5; i++ {
		m := a
		if i%2 == 1 {
			m = b
		}
		if _, err := m.VerifyMFA(f.ctx, login(m), "000000", meta); !errors.Is(err, identity.ErrMFAInvalid) {
			t.Fatalf("%d-urinish: %v", i, err)
		}
	}
	f.clk.add(mfa.Period)
	code, _ := mfa.Code(secret, f.clk.now())
	for _, m := range []*identity.MerchantAuth{a, b} {
		if _, err := m.VerifyMFA(f.ctx, login(m), code, meta); !errors.Is(err, identity.ErrMFATooMany) {
			t.Fatalf("XAVFSIZLIK: hisob nusxalar orasida bo'linmadi: %v", err)
		}
	}
}

// Redis o'chsa — xotiradagi hisob ishlaydi (fail-open EMAS).
func TestMerchant_GuardFallsBackWhenRedisDown(t *testing.T) {
	f := newMerchantFixture(t)
	store := newMemGuardStore()
	m, err := identity.NewMerchantAuth(f.svc, identity.MerchantDeps{Users: f.fu, Repo: f.users,
		Authz: authz.New(f.az), MFA: f.mfaSt, Box: fakeBox{}, Guard: store})
	if err != nil {
		t.Fatal(err)
	}
	f.m = m
	f.user(t, "u_dist02", users.RoleRestaurant)
	phone := f.phoneOf(t, "u_dist02")
	f.setPassword(t, phone, "parol-123")
	f.enroll(t, "u_dist02")
	store.mu.Lock()
	store.fail = true
	store.mu.Unlock()
	var last error
	for i := 0; i < 8; i++ {
		out, err := m.LoginPassword(f.ctx, phone, "parol-123", "merchant-web", meta)
		if err != nil || out.MFA == nil {
			t.Fatal(err)
		}
		_, last = m.VerifyMFA(f.ctx, out.MFA.Token, "000000", meta)
	}
	if !errors.Is(last, identity.ErrMFATooMany) {
		t.Fatalf("XAVFSIZLIK: Redis o'chganda cheklov yo'qoldi: %v", last)
	}
}
