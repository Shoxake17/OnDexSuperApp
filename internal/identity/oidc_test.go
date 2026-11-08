package identity_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"chustapp/internal/identity"
	"chustapp/internal/users"
)

const (
	rpID       = "merchant-web"
	rpSecret   = "shoxpos-bff-sirri-kamida-32-belgidan-iborat"
	rpRedirect = "https://pos.ondex.test/api/auth/oidc/callback"
	verifier   = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk-pkce-verifier"
)

func challengeOf(v string) string {
	s := sha256.Sum256([]byte(v))
	return base64.RawURLEncoding.EncodeToString(s[:])
}

type oidcFix struct {
	*fixture
	p     *identity.Provider
	codes *identity.MemoryCodeStore
}

func newOIDC(t *testing.T) *oidcFix {
	t.Helper()
	f := newFixture(t)
	codes := identity.NewMemoryCodeStore()
	p, err := identity.NewProvider(f.svc, codes, []identity.OIDCClient{{
		ID: rpID, Name: "OnDex Merchant (web)", SecretHash: identity.HashClientSecret(rpSecret),
		RedirectURIs: []string{rpRedirect}, PostLogoutRedirectURIs: []string{"https://pos.ondex.test/"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	f.user(t, "u_owner01", users.RoleMerchant)
	return &oidcFix{fixture: f, p: p, codes: codes}
}

func authorizeQuery(over map[string]string) url.Values {
	q := url.Values{
		"client_id": {rpID}, "redirect_uri": {rpRedirect}, "response_type": {"code"},
		"scope": {"openid profile"}, "state": {"st-123"}, "nonce": {"n-456"},
		"code_challenge": {challengeOf(verifier)}, "code_challenge_method": {"S256"},
	}
	for k, v := range over {
		if v == "<del>" {
			q.Del(k)
		} else {
			q.Set(k, v)
		}
	}
	return q
}

// ssoSession — kirish sahifasida muvaffaqiyatli kirish (ondex-sso sessiyasi).
func (o *oidcFix) ssoSession(t *testing.T) *identity.Session {
	t.Helper()
	tok := o.login(t, "u_owner01", identity.SSOClientID)
	sess, err := o.p.SSOSessionFromAccess(o.ctx, tok.AccessToken)
	if err != nil {
		t.Fatalf("SSOSessionFromAccess: %v", err)
	}
	return sess
}

func (o *oidcFix) code(t *testing.T, sso *identity.Session) string {
	t.Helper()
	req, _, err := o.p.ParseAuthorize(authorizeQuery(nil))
	if err != nil {
		t.Fatalf("ParseAuthorize: %v", err)
	}
	loc, err := o.p.IssueCode(o.ctx, req, sso)
	if err != nil {
		t.Fatalf("IssueCode: %v", err)
	}
	u, _ := url.Parse(loc)
	if !strings.HasPrefix(loc, rpRedirect+"?") || u.Query().Get("state") != "st-123" || u.Query().Get("iss") != iss {
		t.Fatalf("qaytish manzili noto'g'ri: %s", loc)
	}
	return u.Query().Get("code")
}

func TestOIDC_HappyPath(t *testing.T) {
	o := newOIDC(t)
	sso := o.ssoSession(t)

	// cookie -> o'sha SSO sessiyasi
	got, err := o.p.SSOSession(o.ctx, o.p.SSOCookie(sso.ID))
	if err != nil || got.ID != sso.ID {
		t.Fatalf("SSO cookie: %v", err)
	}

	code := o.code(t, sso)
	tr, err := o.p.ExchangeCode(o.ctx, rpID, rpSecret, code, rpRedirect, verifier, identity.Meta{IP: "5.6.7.8"})
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if tr.AccessToken == "" || tr.RefreshToken == "" || tr.IDToken == "" || tr.ExpiresIn <= 0 || tr.TokenType != "Bearer" {
		t.Fatalf("token javobi to'liq emas: %+v", tr)
	}

	// access token — oddiy markaziy token, klient merchant-web, ota — SSO
	c, err := o.svc.Authenticate(o.ctx, tr.AccessToken, users.RealmMerchant)
	if err != nil || c.ClientID != rpID || c.Subject != "u_owner01" {
		t.Fatalf("access token: %v %+v", err, c)
	}
	sess, _ := o.store.Get(o.ctx, tr.SessionID)
	if sess.ParentID != sso.ID || sess.ClientID != rpID {
		t.Fatalf("sessiya SSO'ga bog'lanmagan: %+v", sess)
	}

	// id_token: imzo, iss, aud, nonce, sid
	var idc identity.IDClaims
	_, err = jwt.ParseWithClaims(tr.IDToken, &idc, func(*jwt.Token) (any, error) { return o.key.Public, nil },
		jwt.WithValidMethods([]string{"EdDSA"}), jwt.WithIssuer(iss), jwt.WithAudience(rpID), jwt.WithTimeFunc(o.clk.now))
	if err != nil {
		t.Fatalf("id_token: %v", err)
	}
	if idc.Nonce != "n-456" || idc.SID != tr.SessionID || idc.Subject != "u_owner01" || idc.AuthTime == nil {
		t.Fatalf("id_token da'volari: %+v", idc)
	}
}

func TestOIDC_AuthorizeValidation(t *testing.T) {
	o := newOIDC(t)
	cases := []struct {
		name       string
		over       map[string]string
		redirectOK bool
		code       string
	}{
		{"noma'lum klient", map[string]string{"client_id": "evil-app"}, false, "invalid_client"},
		{"begona redirect", map[string]string{"redirect_uri": "https://evil.test/cb"}, false, "invalid_request"},
		{"redirect prefiks", map[string]string{"redirect_uri": rpRedirect + "/x"}, false, "invalid_request"},
		{"redirect query", map[string]string{"redirect_uri": rpRedirect + "?a=1"}, false, "invalid_request"},
		{"response_type token", map[string]string{"response_type": "token"}, true, "unsupported_response_type"},
		{"openid yo'q", map[string]string{"scope": "profile"}, true, "invalid_scope"},
		{"PKCE yo'q", map[string]string{"code_challenge": "<del>", "code_challenge_method": "<del>"}, true, "invalid_request"},
		{"PKCE plain", map[string]string{"code_challenge_method": "plain"}, true, "invalid_request"},
		{"challenge qisqa", map[string]string{"code_challenge": "abc"}, true, "invalid_request"},
		{"nonce yo'q", map[string]string{"nonce": "<del>"}, true, "invalid_request"},
		{"prompt consent", map[string]string{"prompt": "consent"}, true, "invalid_request"},
		{"max_age manfiy", map[string]string{"max_age": "-5"}, true, "invalid_request"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, redirectOK, err := o.p.ParseAuthorize(authorizeQuery(tc.over))
			var oe *identity.OAuthError
			if !errors.As(err, &oe) || oe.Code != tc.code || redirectOK != tc.redirectOK {
				t.Fatalf("kutilgan %s/%v, olindi %v/%v", tc.code, tc.redirectOK, err, redirectOK)
			}
		})
	}
	// takrorlangan parametr
	q := authorizeQuery(nil)
	q.Add("redirect_uri", "https://evil.test/cb")
	if _, ok, err := o.p.ParseAuthorize(q); err == nil || ok {
		t.Fatal("takrorlangan redirect_uri qabul qilindi")
	}
}

func TestOIDC_ExchangeBindings(t *testing.T) {
	o := newOIDC(t)
	sso := o.ssoSession(t)
	meta := identity.Meta{}

	bad := []struct {
		name                          string
		client, secret, redirect, ver string
	}{
		{"noto'g'ri sir", rpID, rpSecret + "x", rpRedirect, verifier},
		{"sirsiz", rpID, "", rpRedirect, verifier},
		{"begona klient", "ondex-pro", "", rpRedirect, verifier},
		{"boshqa redirect", rpID, rpSecret, "https://pos.ondex.test/other", verifier},
		{"noto'g'ri verifier", rpID, rpSecret, rpRedirect, verifier + "x"},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			code := o.code(t, sso)
			if _, err := o.p.ExchangeCode(o.ctx, tc.client, tc.secret, code, tc.redirect, tc.ver, meta); err == nil {
				t.Fatal("almashtirish o'tib ketdi")
			}
		})
	}

	t.Run("muddati o'tgan kod", func(t *testing.T) {
		code := o.code(t, sso)
		o.clk.add(identity.CodeTTL + time.Second)
		if _, err := o.p.ExchangeCode(o.ctx, rpID, rpSecret, code, rpRedirect, verifier, meta); err == nil {
			t.Fatal("eskirgan kod qabul qilindi")
		}
	})
}

// Kod ikkinchi marta kelsa — birinchi almashtirishda ochilgan sessiya YOPILADI.
func TestOIDC_CodeReuseRevokesIssuedSession(t *testing.T) {
	o := newOIDC(t)
	sso := o.ssoSession(t)
	code := o.code(t, sso)
	tr, err := o.p.ExchangeCode(o.ctx, rpID, rpSecret, code, rpRedirect, verifier, identity.Meta{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := o.p.ExchangeCode(o.ctx, rpID, rpSecret, code, rpRedirect, verifier, identity.Meta{}); err == nil {
		t.Fatal("kod ikkinchi marta almashtirildi")
	}
	if _, err := o.svc.Authenticate(o.ctx, tr.AccessToken); err == nil {
		t.Fatal("XAVFSIZLIK: qayta ishlatilgan kod bilan berilgan sessiya hali tirik")
	}
}

// SSO sessiyasi yopilgan bo'lsa (chiqish) — undan olingan kod almashtirilmaydi.
func TestOIDC_RevokedSSOBlocksExchange(t *testing.T) {
	o := newOIDC(t)
	sso := o.ssoSession(t)
	code := o.code(t, sso)
	if err := o.svc.RevokeOwn(o.ctx, "u_owner01", sso.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := o.p.SSOSession(o.ctx, o.p.SSOCookie(sso.ID)); err == nil {
		t.Fatal("yopilgan SSO sessiyasi cookie orqali tirik")
	}
	if _, err := o.p.ExchangeCode(o.ctx, rpID, rpSecret, code, rpRedirect, verifier, identity.Meta{}); err == nil {
		t.Fatal("yopilgan SSO'dan olingan kod almashtirildi")
	}
}

func TestOIDC_SSOCookieHardening(t *testing.T) {
	o := newOIDC(t)
	sso := o.ssoSession(t)
	good := o.p.SSOCookie(sso.ID)
	for _, c := range []string{"", sso.ID, sso.ID + ".", good + "x", "sess_999999." + strings.SplitN(good, ".", 2)[1]} {
		if _, err := o.p.SSOSession(o.ctx, c); err == nil {
			t.Fatalf("soxta cookie qabul qilindi: %q", c)
		}
	}
	// To'g'ri imzoli, lekin SSO bo'lmagan sessiya (masalan OnDexPro) cookie bo'la olmaydi.
	o.user(t, "u_staff001", users.RoleWaiter)
	pro := o.login(t, "u_staff001", "ondex-pro")
	if _, err := o.p.SSOSession(o.ctx, o.p.SSOCookie(pro.SessionID)); err == nil {
		t.Fatal("ondex-pro sessiyasi SSO cookie sifatida qabul qilindi")
	}
	if _, err := o.p.SSOSessionFromAccess(o.ctx, pro.AccessToken); err == nil {
		t.Fatal("ondex-pro tokeni SSO sessiyasiga aylandi")
	}
}

func TestOIDC_RefreshBoundToClient(t *testing.T) {
	o := newOIDC(t)
	tr, err := o.p.ExchangeCode(o.ctx, rpID, rpSecret, o.code(t, o.ssoSession(t)), rpRedirect, verifier, identity.Meta{})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := o.p.RefreshGrant(o.ctx, rpID, rpSecret, tr.RefreshToken, identity.Meta{})
	if err != nil || r2.RefreshToken == tr.RefreshToken {
		t.Fatalf("refresh: %v", err)
	}
	if _, err := o.p.RefreshGrant(o.ctx, rpID, "noto'g'ri", r2.RefreshToken, identity.Meta{}); err == nil {
		t.Fatal("noto'g'ri sir bilan refresh o'tdi")
	}
	// Boshqa klientning refresh tokeni bu klientda ishlamaydi va sessiyani yopadi.
	o.user(t, "u_staff002", users.RoleWaiter)
	pro := o.login(t, "u_staff002", "ondex-pro")
	if _, err := o.p.RefreshGrant(o.ctx, rpID, rpSecret, pro.RefreshToken, identity.Meta{}); err == nil {
		t.Fatal("begona klient refresh tokeni qabul qilindi")
	}
	if _, err := o.svc.Authenticate(o.ctx, pro.AccessToken); err == nil {
		t.Fatal("begona klientda ishlatilgan refresh sessiyasi yopilmadi")
	}
}

func TestOIDC_SignedRequest(t *testing.T) {
	o := newOIDC(t)
	req, _, err := o.p.ParseAuthorize(authorizeQuery(nil))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := o.p.SignRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	back, err := o.p.OpenRequest(raw)
	if err != nil || back.Nonce != req.Nonce || back.CodeChallenge != req.CodeChallenge || back.RedirectURI != rpRedirect {
		t.Fatalf("OpenRequest: %v %+v", err, back)
	}
	payload, sig, _ := strings.Cut(raw, ".")
	for _, bad := range []string{raw + "x", payload + ".AAAA", "x" + payload[1:] + "." + sig, ""} {
		if _, err := o.p.OpenRequest(bad); err == nil {
			t.Fatalf("buzilgan so'rov qabul qilindi: %q", bad)
		}
	}
	o.clk.add(16 * time.Minute)
	if _, err := o.p.OpenRequest(raw); err == nil {
		t.Fatal("eskirgan so'rov qabul qilindi")
	}
}

func TestOIDC_NeedsLogin(t *testing.T) {
	o := newOIDC(t)
	sso := o.ssoSession(t)
	req, _, _ := o.p.ParseAuthorize(authorizeQuery(nil))
	if o.p.NeedsLogin(req, sso) {
		t.Fatal("yangi SSO sessiyasi bilan qayta kirish so'raldi")
	}
	if !o.p.NeedsLogin(req, nil) {
		t.Fatal("sessiyasiz kirish so'ralmadi")
	}
	login, _, _ := o.p.ParseAuthorize(authorizeQuery(map[string]string{"prompt": "login"}))
	if !o.p.NeedsLogin(login, sso) {
		t.Fatal("prompt=login e'tiborsiz qoldi")
	}
	age, _, _ := o.p.ParseAuthorize(authorizeQuery(map[string]string{"max_age": "60"}))
	o.clk.add(2 * time.Minute)
	if !o.p.NeedsLogin(age, sso) {
		t.Fatal("max_age e'tiborsiz qoldi")
	}
}

func TestOIDC_ProviderConfigValidation(t *testing.T) {
	f := newFixture(t)
	codes := identity.NewMemoryCodeStore()
	bad := []identity.OIDCClient{
		{ID: rpID, RedirectURIs: []string{"http://pos.ondex.test/cb"}},    // http (localhost emas)
		{ID: rpID, RedirectURIs: []string{"https://pos.ondex.test/cb#x"}}, // fragment
		{ID: rpID}, // redirect yo'q
		{ID: "noma-lum", RedirectURIs: []string{rpRedirect}},                  // sessiya siyosati yo'q
		{ID: identity.SSOClientID, RedirectURIs: []string{rpRedirect}},        // SSO o'zi klient emas
		{ID: rpID, RedirectURIs: []string{rpRedirect}, SecretHash: []byte{1}}, // xesh uzunligi
	}
	for i, c := range bad {
		if _, err := identity.NewProvider(f.svc, codes, []identity.OIDCClient{c}); err == nil {
			t.Fatalf("%d: noto'g'ri klient qabul qilindi: %+v", i, c)
		}
	}
	if _, err := identity.NewProvider(f.svc, codes, []identity.OIDCClient{{ID: rpID,
		RedirectURIs: []string{"http://localhost:3300/api/auth/oidc/callback"}}}); err != nil {
		t.Fatalf("localhost dev manzili rad etildi: %v", err)
	}
}

// ── Chiqish (A4c.3) ──────────────────────────────────────────────────────

type bcCapture struct{ ch chan [2]string }

func (b *bcCapture) post(_ context.Context, uri, tok string) error {
	b.ch <- [2]string{uri, tok}
	return nil
}

func (b *bcCapture) next(t *testing.T) (string, string) {
	t.Helper()
	select {
	case m := <-b.ch:
		return m[0], m[1]
	case <-time.After(3 * time.Second):
		t.Fatal("back-channel logout yuborilmadi")
	}
	return "", ""
}

func newOIDCWithBC(t *testing.T) (*oidcFix, *bcCapture) {
	t.Helper()
	f := newFixture(t)
	codes := identity.NewMemoryCodeStore()
	p, err := identity.NewProvider(f.svc, codes, []identity.OIDCClient{{
		ID: rpID, SecretHash: identity.HashClientSecret(rpSecret), RedirectURIs: []string{rpRedirect},
		PostLogoutRedirectURIs: []string{"https://pos.ondex.test/"},
		BackchannelLogoutURI:   "https://pos.ondex.test/api/auth/oidc/backchannel",
	}})
	if err != nil {
		t.Fatal(err)
	}
	bc := &bcCapture{ch: make(chan [2]string, 8)}
	p.WithBackchannelPoster(bc.post)
	f.user(t, "u_owner01", users.RoleMerchant)
	return &oidcFix{fixture: f, p: p, codes: codes}, bc
}

func TestOIDC_BackchannelOnEveryRevocation(t *testing.T) {
	o, bc := newOIDCWithBC(t)
	parse := func(t *testing.T, raw string) identity.LogoutClaims {
		t.Helper()
		var c identity.LogoutClaims
		tok, err := jwt.ParseWithClaims(raw, &c, func(*jwt.Token) (any, error) { return o.key.Public, nil },
			jwt.WithValidMethods([]string{"EdDSA"}), jwt.WithIssuer(iss), jwt.WithAudience(rpID), jwt.WithTimeFunc(o.clk.now))
		if err != nil || tok.Header["typ"] != "logout+jwt" {
			t.Fatalf("logout_token: %v typ=%v", err, tok.Header["typ"])
		}
		if _, ok := c.Events["http://schemas.openid.net/event/backchannel-logout"]; !ok || c.SID == "" {
			t.Fatalf("logout_token da'volari: %+v", c)
		}
		if strings.Contains(raw, "nonce") {
			t.Fatal("logout_token'da nonce bo'lmasligi kerak")
		}
		return c
	}
	exchange := func(t *testing.T, sso *identity.Session) *identity.TokenResponse {
		t.Helper()
		tr, err := o.p.ExchangeCode(o.ctx, rpID, rpSecret, o.code(t, sso), rpRedirect, verifier, identity.Meta{})
		if err != nil {
			t.Fatal(err)
		}
		return tr
	}

	t.Run("SSO'dan chiqish bolaga xabar beradi", func(t *testing.T) {
		sso := o.ssoSession(t)
		tr := exchange(t, sso)
		if err := o.p.EndSSO(o.ctx, sso, "logout"); err != nil {
			t.Fatal(err)
		}
		uri, raw := bc.next(t)
		if uri != "https://pos.ondex.test/api/auth/oidc/backchannel" {
			t.Fatalf("back-channel manzili: %q", uri)
		}
		if c := parse(t, raw); c.SID != tr.SessionID || c.Subject != "u_owner01" {
			t.Fatalf("sid/sub: %+v", c)
		}
		if _, err := o.svc.Authenticate(o.ctx, tr.AccessToken); err == nil {
			t.Fatal("SSO yopilgach ilova sessiyasi tirik")
		}
	})

	t.Run("Qurilmalar sahifasidan yopish ham xabar beradi", func(t *testing.T) {
		tr := exchange(t, o.ssoSession(t))
		if err := o.svc.RevokeOwn(o.ctx, "u_owner01", tr.SessionID); err != nil {
			t.Fatal(err)
		}
		select {
		case m := <-bc.ch:
			if c := parse(t, m[1]); c.SID != tr.SessionID {
				t.Fatalf("sid: %q != %q", c.SID, tr.SessionID)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("RevokeOwn'da back-channel yuborilmadi")
		}
	})

	t.Run("parol almashishi (RevokeAllForUser) ham xabar beradi", func(t *testing.T) {
		tr := exchange(t, o.ssoSession(t))
		for len(bc.ch) > 0 {
			<-bc.ch
		}
		if _, err := o.svc.RevokeAllForUser(o.ctx, "u_owner01", "password_changed"); err != nil {
			t.Fatal(err)
		}
		deadline := time.After(3 * time.Second)
		for {
			select {
			case m := <-bc.ch:
				if parse(t, m[1]).SID == tr.SessionID {
					return
				}
			case <-deadline:
				t.Fatal("RevokeAllForUser'da ilova sessiyasiga xabar ketmadi")
			}
		}
	})
}

func TestOIDC_IDTokenHintAndPostLogout(t *testing.T) {
	o, _ := newOIDCWithBC(t)
	sso := o.ssoSession(t)
	tr, err := o.p.ExchangeCode(o.ctx, rpID, rpSecret, o.code(t, sso), rpRedirect, verifier, identity.Meta{})
	if err != nil {
		t.Fatal(err)
	}
	o.clk.add(time.Hour) // id_token muddati o'tgan — hint baribir ishlashi kerak
	h, err := o.p.ParseIDTokenHint(tr.IDToken)
	if err != nil || h.ClientID != rpID || h.SessionID != tr.SessionID || h.UserID != "u_owner01" {
		t.Fatalf("hint: %v %+v", err, h)
	}
	if _, err := o.p.ParseIDTokenHint(tamper(tr.IDToken)); err == nil {
		t.Fatal("buzilgan id_token_hint qabul qilindi")
	}
	if _, err := o.p.ParseIDTokenHint(tr.AccessToken); err == nil {
		t.Fatal("access token id_token_hint sifatida qabul qilindi")
	}
	if got := o.p.PostLogoutRedirect(rpID, "https://pos.ondex.test/", "s1"); got != "https://pos.ondex.test/?state=s1" {
		t.Fatalf("post logout: %q", got)
	}
	for _, bad := range []string{"https://evil.test/", "https://pos.ondex.test/x", ""} {
		if got := o.p.PostLogoutRedirect(rpID, bad, ""); got != "" {
			t.Fatalf("begona post_logout manzili qabul qilindi: %q", got)
		}
	}
	o.p.EndClientSession(o.ctx, h)
	if s, _ := o.store.Get(o.ctx, tr.SessionID); s.RevokedAt == nil {
		t.Fatal("hint bo'yicha ilova sessiyasi yopilmadi")
	}
	if s, _ := o.store.Get(o.ctx, sso.ID); s.RevokedAt == nil {
		t.Fatal("hint bo'yicha chiqishda SSO sessiyasi ham yopilishi kerak")
	}
}
