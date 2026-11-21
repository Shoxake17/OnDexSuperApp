package httpapi

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"chustapp/internal/identity"
	"chustapp/internal/users"
)

// testTokens — httpapi testlari uchun MARKAZIY sessiya chiqaruvchi.
//
// ONDEXECOSY-483: eski HS256 (`users.NewTokenIssuer`) butunlay olib
// tashlandi — testlar ham production bilan BIR XIL yo'ldan o'tadi
// (EdDSA, `identity.Authenticate`, akkaunt har so'rovda o'qiladi).
//
// Shart: foydalanuvchi `Issue` dan OLDIN serverning `UserRepo` siga
// yozilgan bo'lishi kerak (`parseBearer` akkauntni har so'rovda o'qiydi —
// o'chirilgan yoki roli o'zgargan akkaunt darhol 401).
type testTokens struct {
	svc   *identity.Service
	users *tokenUsers
}

// tokenUsers — identity sessiya ochishda akkauntning realmini o'qiydi.
type tokenUsers struct {
	mu sync.Mutex
	m  map[string]*users.User
	// fallback — server ombori: kirish oqimi (writeLogin) YANGI yaratgan
	// akkauntni ham ko'rish uchun (`WithUsers`).
	fallback identity.UserReader
}

func (t *tokenUsers) GetByID(_ context.Context, id string) (*users.User, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if u, ok := t.m[id]; ok {
		c := *u
		return &c, nil
	}
	if t.fallback != nil {
		return t.fallback.GetByID(context.Background(), id)
	}
	return nil, users.ErrUserNotFound
}

func newTestTokens(t testing.TB) *testTokens {
	t.Helper()
	key, err := identity.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	keys, _ := identity.NewKeySet(key)
	iss, err := identity.NewIssuer(keys, "https://api.test.ondex", "ondex-api")
	if err != nil {
		t.Fatal(err)
	}
	// Admin tokenlari alohida kalit bilan (production'dagi kabi, A6).
	pk, err := identity.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	pks, _ := identity.NewKeySet(pk)
	iss.WithPlatformKeys(pks)
	tu := &tokenUsers{m: map[string]*users.User{}}
	var seq int64
	svc, err := identity.NewService(identity.NewMemoryStore(), iss, tu, identity.NopAuditor{},
		func() string { return fmt.Sprintf("tsess_%06d", atomic.AddInt64(&seq, 1)) }, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &testTokens{svc: svc, users: tu}
}

// WithUsers — server ombori (kirish oqimi yaratgan akkauntlar uchun).
func (tt *testTokens) WithUsers(ur identity.UserReader) *testTokens {
	tt.users.mu.Lock()
	tt.users.fallback = ur
	tt.users.mu.Unlock()
	return tt
}

// Identity — `Deps.Identity` uchun.
func (tt *testTokens) Identity() *identity.Service { return tt.svc }

// Issue — oddiy kirish (parol bilan): "kod bilan kirdi" imtiyozi YO'Q.
func (tt *testTokens) Issue(u *users.User) (string, error) { return tt.issue(u, false) }

// IssuePhoneProven — kod bilan kirish (AMR otp): 15 daqiqa parolni joriy
// parolsiz o'rnatish imtiyozi (`Claims.PhoneProven`).
func (tt *testTokens) IssuePhoneProven(u *users.User) (string, error) { return tt.issue(u, true) }

func (tt *testTokens) issue(u *users.User, phoneProven bool) (string, error) {
	c := *u
	tt.users.mu.Lock()
	tt.users.m[u.ID] = &c
	tt.users.mu.Unlock()

	client, amr := identity.CustomerClientID, []string{users.AMRPassword}
	if phoneProven {
		amr = []string{users.AMROTP}
	}
	switch users.RealmForRole(u.Role) {
	case users.RealmMerchant:
		client, amr = "ondex-pro", []string{users.AMROTP}
	case users.RealmPlatform:
		// Admin sessiyasi faqat kuchli ikkinchi omil bilan (StrongAMR).
		client, amr = "admin-web", []string{users.AMRPassword, "totp"}
	}
	tok, err := tt.svc.Create(context.Background(), identity.CreateInput{UserID: u.ID, ClientID: client, AMR: amr})
	if err != nil {
		return "", err
	}
	return tok.AccessToken, nil
}
