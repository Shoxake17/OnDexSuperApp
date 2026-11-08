package httpapi

import (
	"context"
	"sync"
	"testing"

	"chustapp/internal/identity"
	"chustapp/internal/users"
)

// A5 dan keyin merchant realmi (restoran, do'kon, kuryer, ofitsiant) eski
// HS256 token OLMAYDI — test fixturalari ham ularga production'dagi kabi
// markaziy sessiya (EdDSA) ochadi. Mijoz va admin hali eski tokenda.
//
// Har bir foydalanuvchi ombori uchun BITTA xotiradagi markaziy sessiya
// xizmati: fixtura tokenni `Deps` dan oldin yasaydi, `Deps.Identity` esa
// keyin xuddi shu xizmatni oladi (`testIdentityFor`).
var testIdentities sync.Map // users.Repository -> *identity.Service

func testIdentityFor(repo users.Repository) *identity.Service {
	if v, ok := testIdentities.Load(repo); ok {
		return v.(*identity.Service)
	}
	key, err := identity.GenerateKey()
	if err != nil {
		panic(err)
	}
	keys, err := identity.NewKeySet(key)
	if err != nil {
		panic(err)
	}
	iss, err := identity.NewIssuer(keys, "https://api.test.ondex", "ondex-api")
	if err != nil {
		panic(err)
	}
	svc, err := identity.NewService(identity.NewMemoryStore(), iss, repo, identity.NopAuditor{}, NewID, nil)
	if err != nil {
		panic(err)
	}
	v, _ := testIdentities.LoadOrStore(repo, svc)
	return v.(*identity.Service)
}

// testToken — rolga mos token: merchant realmi -> markaziy sessiya (rolga
// mos ilova klienti), qolganlar -> eski HS256.
func testToken(t testing.TB, tokens *users.TokenIssuer, repo users.Repository, u *users.User) (string, error) {
	t.Helper()
	if users.RealmForRole(u.Role) != users.RealmMerchant {
		return tokens.Issue(u)
	}
	client := "merchant-desktop"
	switch u.Role {
	case users.RoleCourier:
		client = "ondex-go"
	case users.RoleWaiter:
		client = "ondex-pro"
	}
	tok, err := testIdentityFor(repo).Create(context.Background(), identity.CreateInput{
		UserID: u.ID, ClientID: client, AMR: []string{"otp"},
	})
	if err != nil {
		return "", err
	}
	return tok.AccessToken, nil
}
