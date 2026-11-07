package identity_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"chustapp/internal/identity"
	"chustapp/internal/identity/identitytest"
	"chustapp/internal/storage"
	"chustapp/internal/users"
)

const (
	iss = "https://api.ondex.test"
	aud = "ondex-api"
)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) add(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

type fixture struct {
	svc    *identity.Service
	users  *storage.MemoryUserRepo
	store  *identity.MemoryStore
	key    *identity.SigningKey
	clk    *clock
	ctx    context.Context
	idSeq  int
	idLock sync.Mutex
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	key, err := identity.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	ks, _ := identity.NewKeySet(key)
	issuer, err := identity.NewIssuer(ks, iss, aud)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{users: storage.NewMemoryUserRepo(), store: identity.NewMemoryStore(), key: key,
		clk: &clock{t: time.Now().UTC().Truncate(time.Second)}, ctx: context.Background()}
	f.svc, err = identity.NewService(f.store, issuer, f.users, identity.NopAuditor{}, f.nextID, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.svc.WithClock(f.clk.now)
	return f
}

func (f *fixture) nextID() string {
	f.idLock.Lock()
	defer f.idLock.Unlock()
	f.idSeq++
	return fmt.Sprintf("sess_%06d", f.idSeq)
}

func (f *fixture) user(t *testing.T, id string, role users.Role) {
	t.Helper()
	if err := f.users.Create(f.ctx, &users.User{ID: id, Phone: "+99890" + id[len(id)-7:], Role: role,
		PhoneVerified: true, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) login(t *testing.T, userID, client string) *identity.Tokens {
	t.Helper()
	tok, err := f.svc.Create(f.ctx, identity.CreateInput{UserID: userID, ClientID: client, AMR: []string{"otp"},
		Meta: identity.Meta{IP: "1.2.3.4", UserAgent: "test"}})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return tok
}

// ── Ombor shartnomasi (xotira) ───────────────────────────────────────────
func TestMemoryStoreContract(t *testing.T) {
	identitytest.RunStoreContract(t, func(*testing.T) identity.Store { return identity.NewMemoryStore() }, "u_contract")
}

// ── Kalitlar ──────────────────────────────────────────────────────────────
func TestKeyPEMRoundTripAndJWKS(t *testing.T) {
	k, _ := identity.GenerateKey()
	pemBytes, err := identity.EncodePrivatePEM(k)
	if err != nil {
		t.Fatal(err)
	}
	back, err := identity.ParsePrivatePEM(pemBytes)
	if err != nil || back.KID != k.KID || !back.Public.Equal(k.Public) {
		t.Fatalf("PEM qaytmadi: %v", err)
	}
	if _, err := identity.ParsePrivatePEM(append(pemBytes, []byte("ortiqcha")...)); err == nil {
		t.Fatal("ortiqcha ma'lumotli PEM qabul qilindi")
	}

	old, _ := identity.GenerateKey()
	ks, _ := identity.NewKeySet(k, old)
	raw, _ := ks.JWKS()
	var doc struct {
		Keys []map[string]string `json:"keys"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || len(doc.Keys) != 2 {
		t.Fatalf("JWKS: %s %v", raw, err)
	}
	for _, j := range doc.Keys {
		if j["kty"] != "OKP" || j["crv"] != "Ed25519" || j["alg"] != "EdDSA" || j["kid"] == "" {
			t.Fatalf("JWK noto'g'ri: %v", j)
		}
		if _, has := j["d"]; has {
			t.Fatal("JWKS'da MAXFIY kalit bor!")
		}
	}
	if strings.Contains(string(raw), base64.RawURLEncoding.EncodeToString(k.Private.Seed())) {
		t.Fatal("JWKS maxfiy kalitni oshkor qildi")
	}
}

// ── Token tekshiruvi: hujumlar ───────────────────────────────────────────
func TestVerify_RejectsForgedAndForeignTokens(t *testing.T) {
	f := newFixture(t)
	f.user(t, "u_m0000001", users.RoleRestaurant)
	good := f.login(t, "u_m0000001", "merchant-web").AccessToken
	if _, err := f.svc.Authenticate(f.ctx, good, users.RealmMerchant); err != nil {
		t.Fatalf("haqiqiy token rad etildi: %v", err)
	}

	parts := strings.Split(good, ".")
	payload := parts[1]

	// alg: none
	noneHdr := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	// HS256 — ochiq kalitni HMAC siri qilib (klassik "algoritm aralashtirish" hujumi)
	hsTok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "u_m0000001"})
	hsTok.Header["kid"] = f.key.KID
	hs, _ := hsTok.SignedString([]byte(f.key.Public))
	// begona kalit bilan imzolangan
	other, _ := identity.GenerateKey()
	otherKS, _ := identity.NewKeySet(other)
	otherIss, _ := identity.NewIssuer(otherKS, iss, aud)
	otherSvc, _ := identity.NewService(identity.NewMemoryStore(), otherIss, f.users, nil, f.nextID, nil)
	foreign := mustLogin(t, otherSvc, "u_m0000001")

	cases := map[string]string{
		"alg none":         noneHdr + "." + payload + ".",
		"HS256 aralash":    hs,
		"begona kalit":     foreign,
		"imzo buzilgan":    parts[0] + "." + payload + "." + flip(parts[2]),
		"payload buzilgan": parts[0] + "." + tamper(payload) + "." + parts[2],
		"bo'sh":            "",
		"axlat":            "abc.def.ghi",
	}
	for name, tok := range cases {
		if _, err := f.svc.Authenticate(f.ctx, tok); err == nil {
			t.Errorf("%s: qabul qilindi", name)
		}
	}

	// noto'g'ri iss / aud
	ks, _ := identity.NewKeySet(f.key)
	for name, pair := range map[string][2]string{"iss": {"https://evil", aud}, "aud": {iss, "boshqa-api"}} {
		wrong, _ := identity.NewIssuer(ks, pair[0], pair[1])
		ws, _ := identity.NewService(identity.NewMemoryStore(), wrong, f.users, nil, f.nextID, nil)
		tok := mustLogin(t, ws, "u_m0000001")
		if _, err := f.svc.Issuer().Verify(tok); err == nil {
			t.Errorf("noto'g'ri %s qabul qilindi", name)
		}
	}
}

func mustLogin(t *testing.T, svc *identity.Service, uid string) string {
	t.Helper()
	tok, err := svc.Create(context.Background(), identity.CreateInput{UserID: uid, ClientID: "merchant-web"})
	if err != nil {
		t.Fatal(err)
	}
	return tok.AccessToken
}

func flip(s string) string {
	b := []byte(s)
	if b[0] == 'A' {
		b[0] = 'B'
	} else {
		b[0] = 'A'
	}
	return string(b)
}

func tamper(p string) string {
	raw, _ := base64.RawURLEncoding.DecodeString(p)
	s := strings.Replace(string(raw), `"realm":"merchant"`, `"realm":"platform"`, 1)
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

func TestVerify_ExpiryAndRealm(t *testing.T) {
	f := newFixture(t)
	f.user(t, "u_m0000002", users.RoleRestaurant)
	tok := f.login(t, "u_m0000002", "merchant-web").AccessToken

	if _, err := f.svc.Authenticate(f.ctx, tok, users.RealmCustomer); err == nil {
		t.Fatal("merchant tokeni customer marshrutida o'tdi")
	}
	if _, err := f.svc.Authenticate(f.ctx, tok, users.RealmPlatform); err == nil {
		t.Fatal("merchant tokeni admin marshrutida o'tdi")
	}
	f.clk.add(identity.AccessTTL + time.Minute)
	if _, err := f.svc.Authenticate(f.ctx, tok, users.RealmMerchant); err == nil {
		t.Fatal("muddati o'tgan token qabul qilindi")
	}
}

// ── Sessiyalar ────────────────────────────────────────────────────────────
func TestCreate_RealmBoundToClient(t *testing.T) {
	f := newFixture(t)
	f.user(t, "u_c0000003", users.RoleCustomer)
	f.user(t, "u_a0000004", users.RoleAdmin)
	_, err := f.svc.Create(f.ctx, identity.CreateInput{UserID: "u_c0000003", ClientID: "merchant-web"})
	if !errors.Is(err, identity.ErrRealmMismatch) {
		t.Fatalf("mijoz merchant ilovasida sessiya ochdi: %v", err)
	}
	_, err = f.svc.Create(f.ctx, identity.CreateInput{UserID: "u_a0000004", ClientID: "ondex-pro"})
	if !errors.Is(err, identity.ErrRealmMismatch) {
		t.Fatalf("admin OnDexPro'da sessiya ochdi: %v", err)
	}
	_, err = f.svc.Create(f.ctx, identity.CreateInput{UserID: "u_c0000003", ClientID: "noma-lum"})
	if !errors.Is(err, identity.ErrUnknownClient) {
		t.Fatalf("noma'lum klient: %v", err)
	}
	if _, err := f.svc.Create(f.ctx, identity.CreateInput{UserID: "u_a0000004", ClientID: "admin-web"}); err != nil {
		t.Fatalf("admin admin-web: %v", err)
	}
}

func TestRefresh_RotationAndTheftDetection(t *testing.T) {
	f := newFixture(t)
	f.user(t, "u_m0000005", users.RoleWaiter)
	first := f.login(t, "u_m0000005", "ondex-pro")

	f.clk.add(time.Minute)
	second, err := f.svc.Refresh(f.ctx, first.RefreshToken, identity.Meta{})
	if err != nil {
		t.Fatalf("1-refresh: %v", err)
	}
	if second.RefreshToken == first.RefreshToken || second.SessionID != first.SessionID {
		t.Fatal("rotatsiya bo'lmadi yoki sessiya almashdi")
	}

	// 3 soniyadan keyin eski token yana keldi — halol poyga: bekor QILINMAYDI.
	f.clk.add(3 * time.Second)
	if _, err := f.svc.Refresh(f.ctx, first.RefreshToken, identity.Meta{}); !errors.Is(err, identity.ErrRefreshInvalid) {
		t.Fatalf("poyga: %v", err)
	}
	if _, err := f.svc.Authenticate(f.ctx, second.AccessToken, users.RealmMerchant); err != nil {
		t.Fatalf("poygadan keyin sessiya yopilib qoldi: %v", err)
	}

	// 1 daqiqadan keyin o'sha eski token — o'g'irlik: sessiya yopiladi.
	f.clk.add(time.Minute)
	if _, err := f.svc.Refresh(f.ctx, first.RefreshToken, identity.Meta{}); !errors.Is(err, identity.ErrRefreshReuse) {
		t.Fatalf("o'g'irlik aniqlanmadi: %v", err)
	}
	if _, err := f.svc.Refresh(f.ctx, second.RefreshToken, identity.Meta{}); err == nil {
		t.Fatal("o'g'irlikdan keyin haqiqiy egasining tokeni ham ishlayapti")
	}
	if _, err := f.svc.Authenticate(f.ctx, second.AccessToken, users.RealmMerchant); err == nil {
		t.Fatal("bekor qilingan sessiyaning access tokeni ishlayapti")
	}
}

func TestRefresh_DeletedOrRealmChangedAccountClosesSession(t *testing.T) {
	f := newFixture(t)
	f.user(t, "u_m0000006", users.RoleCourier)
	tok := f.login(t, "u_m0000006", "ondex-go")
	if err := f.users.SoftDelete(f.ctx, "u_m0000006"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Refresh(f.ctx, tok.RefreshToken, identity.Meta{}); err == nil {
		t.Fatal("o'chirilgan akkaunt refresh oldi")
	}
	if s, _ := f.store.Get(f.ctx, tok.SessionID); s.RevokedAt == nil {
		t.Fatal("o'chirilgan akkaunt sessiyasi ochiq qoldi")
	}

	f.user(t, "u_m0000007", users.RoleWaiter)
	tok2 := f.login(t, "u_m0000007", "ondex-pro")
	if err := f.users.UpdateRole(f.ctx, "u_m0000007", users.RoleCustomer, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Refresh(f.ctx, tok2.RefreshToken, identity.Meta{}); err == nil {
		t.Fatal("realmi o'zgargan akkaunt refresh oldi")
	}
}

func TestRefresh_InputHardening(t *testing.T) {
	f := newFixture(t)
	for _, bad := range []string{"", "xyz", "ort_", "ort_" + strings.Repeat("a", 200)} {
		if _, err := f.svc.Refresh(f.ctx, bad, identity.Meta{}); !errors.Is(err, identity.ErrRefreshInvalid) {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

func TestLogoutAndRevokeOwn(t *testing.T) {
	f := newFixture(t)
	f.user(t, "u_m0000008", users.RoleRestaurant)
	f.user(t, "u_m0000009", users.RoleRestaurant)
	a := f.login(t, "u_m0000008", "merchant-web")
	b := f.login(t, "u_m0000008", "merchant-desktop")
	other := f.login(t, "u_m0000009", "merchant-web")

	list, _ := f.svc.ListSessions(f.ctx, "u_m0000008")
	if len(list) != 2 {
		t.Fatalf("sessiyalar: %d", len(list))
	}
	// begona sessiyani yopish — topilmadi (mavjudligi oshkor bo'lmaydi)
	if err := f.svc.RevokeOwn(f.ctx, "u_m0000008", other.SessionID); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("begona sessiya: %v", err)
	}
	if _, err := f.svc.Authenticate(f.ctx, other.AccessToken); err != nil {
		t.Fatal("begona sessiyaga tegildi")
	}
	if err := f.svc.RevokeOwn(f.ctx, "u_m0000008", b.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Authenticate(f.ctx, b.AccessToken); err == nil {
		t.Fatal("yopilgan sessiya tokeni ishlayapti")
	}
	c, err := f.svc.Authenticate(f.ctx, a.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Logout(f.ctx, c); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Authenticate(f.ctx, a.AccessToken); err == nil {
		t.Fatal("logout'dan keyin token ishlayapti")
	}
	if _, err := f.svc.Refresh(f.ctx, a.RefreshToken, identity.Meta{}); err == nil {
		t.Fatal("logout'dan keyin refresh ishlayapti")
	}
}

func TestRevokeAllForUser(t *testing.T) {
	f := newFixture(t)
	f.user(t, "u_m0000010", users.RoleRestaurant)
	a := f.login(t, "u_m0000010", "merchant-web")
	b := f.login(t, "u_m0000010", "merchant-desktop")
	n, err := f.svc.RevokeAllForUser(f.ctx, "u_m0000010", "password_changed")
	if err != nil || n != 2 {
		t.Fatalf("RevokeAll: %d %v", n, err)
	}
	for _, tok := range []*identity.Tokens{a, b} {
		if _, err := f.svc.Authenticate(f.ctx, tok.AccessToken); err == nil {
			t.Fatal("RevokeAll'dan keyin token ishlayapti")
		}
	}
}

func TestAccessTokenNeverOutlivesSession(t *testing.T) {
	f := newFixture(t)
	f.user(t, "u_a0000011", users.RoleAdmin)
	tok := f.login(t, "u_a0000011", "admin-web") // abs 12 soat
	c, err := f.svc.Issuer().Verify(tok.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if c.ExpiresAt.Sub(c.IssuedAt.Time) > identity.AccessTTL {
		t.Fatal("access token 10 daqiqadan uzun")
	}
	// harakatsiz 30 daqiqa — admin sessiyasi tugaydi
	f.clk.add(31 * time.Minute)
	if _, err := f.svc.Refresh(f.ctx, tok.RefreshToken, identity.Meta{}); err == nil {
		t.Fatal("admin sessiyasi 30 daqiqa harakatsizlikdan keyin ham yangilandi")
	}
}
