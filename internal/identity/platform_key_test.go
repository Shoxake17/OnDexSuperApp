package identity_test

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"chustapp/internal/identity"
	"chustapp/internal/users"
)

// A6: admin (platform) tokenlari ALOHIDA kalit bilan. Asosiy (merchant)
// kalit bilan imzolangan platform tokeni va platform kaliti bilan
// imzolangan merchant tokeni — ikkalasi ham rad.
func TestPlatformKey_SeparateFromMerchant(t *testing.T) {
	f := newFixture(t)
	f.user(t, "u_admin01", users.RoleAdmin)
	f.user(t, "u_owner77", users.RoleRestaurant)

	adm := f.login(t, "u_admin01", "admin-web")
	if _, err := f.svc.Authenticate(f.ctx, adm.AccessToken, users.RealmPlatform); err != nil {
		t.Fatalf("haqiqiy admin tokeni: %v", err)
	}
	hdr, _, _ := jwt.NewParser().ParseUnverified(adm.AccessToken, jwt.MapClaims{})
	if hdr.Header["kid"] != f.pkey.KID {
		t.Fatalf("admin tokeni platform kaliti bilan imzolanmagan: kid=%v", hdr.Header["kid"])
	}

	forge := func(realm users.Realm, key *identity.SigningKey, sub, sid string) string {
		now := time.Now()
		c := identity.AccessClaims{Realm: realm, SessionID: sid, ClientID: "admin-web",
			RegisteredClaims: jwt.RegisteredClaims{Issuer: iss, Subject: sub, Audience: jwt.ClaimStrings{aud},
				IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute))}}
		tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, c)
		tok.Header["kid"] = key.KID
		s, err := tok.SignedString(key.Private)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	sid := adm.SessionID
	if _, err := f.svc.Issuer().Verify(forge(users.RealmPlatform, f.key, "u_admin01", sid)); !errors.Is(err, identity.ErrTokenInvalid) {
		t.Fatalf("XAVFSIZLIK: merchant kaliti bilan admin tokeni o'tdi: %v", err)
	}
	own := f.login(t, "u_owner77", "merchant-web")
	if _, err := f.svc.Issuer().Verify(forge(users.RealmMerchant, f.pkey, "u_owner77", own.SessionID)); !errors.Is(err, identity.ErrTokenInvalid) {
		t.Fatalf("platform kaliti bilan merchant tokeni o'tdi: %v", err)
	}

	// Platform kaliti yo'q — admin sessiyasi umuman ochilmaydi (yetim yozuv ham yo'q).
	k, _ := identity.GenerateKey()
	ks, _ := identity.NewKeySet(k)
	is, _ := identity.NewIssuer(ks, iss, aud)
	store := identity.NewMemoryStore()
	svc, err := identity.NewService(store, is, f.users, identity.NopAuditor{}, f.nextID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(f.ctx, identity.CreateInput{UserID: "u_admin01", ClientID: "admin-web"}); !errors.Is(err, identity.ErrPlatformKeyMissing) {
		t.Fatalf("kalitsiz admin sessiyasi: %v", err)
	}
	if list, _ := svc.ListSessions(f.ctx, "u_admin01"); len(list) != 0 {
		t.Fatalf("yetim sessiya yozuvi qoldi: %d", len(list))
	}
}
