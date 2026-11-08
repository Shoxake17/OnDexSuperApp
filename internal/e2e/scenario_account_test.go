package e2e

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"chustapp/internal/e2e/report"
	"chustapp/internal/identity"
	"chustapp/internal/users"
)

// OnDex ID akkaunt sahifasi (passport.ondex.uz, Plane 270):
// cookie'siz -> kirish sahifasi (akkaunt rejimi); kirish -> SSO cookie;
// profil, qurilmalar; CSRF (sarlavha + Origin); qurilmani chiqarish;
// parolni almashtirish -> hamma joydan chiqish; hamma qurilmalardan chiqish.
func TestOnDexIDAccountPage(t *testing.T) {
	w := NewWorld(t, "OnDex ID akkaunt sahifasi")
	ctx := context.Background()

	const uid, phone, oldPwd, newPwd = "e2e-acc-owner", "+998900000081", "Eski-Parol-81x", "Yangi-Parol-81y"
	hash, err := users.HashPassword(ctx, oldPwd)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Users.Create(ctx, &users.User{ID: uid, Phone: phone, Name: "Akkaunt Egasi", Role: users.RoleRestaurant,
		EntityID: RestA, PhoneVerified: true, PasswordHash: hash, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}

	noRedirect := &http.Client{Timeout: 15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	send := func(t *testing.T, method, path string, hdr map[string]string, body string) *Resp {
		t.Helper()
		var rd io.Reader
		if body != "" {
			rd = strings.NewReader(body)
		}
		req, err := http.NewRequest(method, w.Srv.URL+path, rd)
		if err != nil {
			t.Fatal(err)
		}
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		start := time.Now()
		res, err := noRedirect.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		raw, _ := io.ReadAll(res.Body)
		res.Body.Close()
		pathOnly, _, _ := strings.Cut(path, "?")
		Rec.AddCall(report.Call{Scenario: w.scenario, Step: w.step, Method: method, Path: pathOnly,
			Route: MatchRoute(Routes, method, pathOnly), Status: res.StatusCode,
			Ms: float64(time.Since(start).Microseconds()) / 1000, Actor: "brauzer"})
		r := &Resp{Status: res.StatusCode, Body: raw, Header: res.Header}
		w.checkGeneric(t, method, path, r)
		return r
	}
	const origin = "https://api.e2e.ondex"
	var cookie string
	writeH := func(extra map[string]string) map[string]string {
		h := map[string]string{"Content-Type": "application/json", "Cookie": cookie,
			"X-OnDex-Account": "1", "Origin": origin}
		for k, v := range extra {
			if v == "" {
				delete(h, k)
			} else {
				h[k] = v
			}
		}
		return h
	}
	login := func(t *testing.T) {
		t.Helper()
		sso, err := w.Identity.Create(ctx, identity.CreateInput{UserID: uid, ClientID: identity.SSOClientID,
			AMR: []string{"pwd", "otp"}, Meta: identity.Meta{DeviceName: "Brauzer (OnDex kirish)"}})
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]string
		r := send(t, "POST", "/v2/oauth/login/complete", map[string]string{"Content-Type": "application/json",
			"Authorization": "Bearer " + sso.AccessToken}, `{"account":true}`).Expect(t, 200)
		r.JSON(t, &out)
		if out["redirect_to"] != "/v2/oauth/account" {
			t.Fatalf("akkaunt sahifasiga qaytmadi: %v", out)
		}
		sc := r.Header.Get("Set-Cookie")
		if !strings.HasPrefix(sc, "__Host-ondex_sso=") || !strings.Contains(sc, "HttpOnly") {
			t.Fatalf("SSO cookie: %q", sc)
		}
		cookie, _, _ = strings.Cut(sc, ";")
	}

	w.Step(t, "cookie'siz -> kirish sahifasi (akkaunt rejimi); API 401", func(t *testing.T) {
		r := send(t, "GET", "/v2/oauth/account", nil, "").Expect(t, 302)
		if loc := r.Header.Get("Location"); loc != "/v2/oauth/login?account=1" {
			t.Fatalf("yo'naltirish: %q", loc)
		}
		send(t, "GET", "/v2/oauth/account/api/me", nil, "").Expect(t, 401)
		// Tokensiz akkaunt kirishi — rad.
		send(t, "POST", "/v2/oauth/login/complete", map[string]string{"Content-Type": "application/json"}, `{"account":true}`).Expect(t, 401)
	})

	w.Step(t, "kirish -> SSO cookie; sahifa qat'iy CSP bilan; profil va qurilmalar", func(t *testing.T) {
		login(t)
		page := send(t, "GET", "/v2/oauth/account", map[string]string{"Cookie": cookie}, "").Expect(t, 200)
		if csp := page.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") || strings.Contains(csp, "unsafe-inline") {
			t.Fatalf("CSP: %q", csp)
		}
		send(t, "GET", "/v2/oauth/assets/account.js", nil, "").Expect(t, 200)
		if _, err := w.Identity.Create(ctx, identity.CreateInput{UserID: uid, ClientID: "merchant-desktop", AMR: []string{"otp"}}); err != nil {
			t.Fatal(err)
		}
		var me struct {
			User struct {
				Phone string `json:"phone"`
				Name  string `json:"name"`
			} `json:"user"`
			PasswordSet bool     `json:"password_set"`
			TOTP        bool     `json:"totp_enabled"`
			Apps        []string `json:"connected_apps"`
			Sessions    []struct {
				ID      string `json:"id"`
				App     string `json:"app"`
				Current bool   `json:"current"`
			} `json:"sessions"`
		}
		send(t, "GET", "/v2/oauth/account/api/me", map[string]string{"Cookie": cookie}, "").Expect(t, 200).JSON(t, &me)
		if me.User.Phone != phone || !me.PasswordSet || me.TOTP || len(me.Sessions) < 2 {
			t.Fatalf("profil: %+v", me)
		}
		if strings.Join(me.Apps, ",") != "OnDex Merchant (Windows)" {
			t.Fatalf("ulangan xizmatlar: %v", me.Apps)
		}
		current := 0
		for _, s := range me.Sessions {
			if s.Current {
				current++
			}
		}
		if current != 1 {
			t.Fatalf("joriy sessiya: %+v", me.Sessions)
		}
	})

	w.Step(t, "CSRF: sarlavhasiz, begona Origin yoki begona sayt — 403", func(t *testing.T) {
		send(t, "POST", "/v2/oauth/account/api/logout", writeH(map[string]string{"X-OnDex-Account": ""}), "{}").Expect(t, 403)
		send(t, "POST", "/v2/oauth/account/api/logout", writeH(map[string]string{"Origin": "https://evil.test"}), "{}").Expect(t, 403)
		send(t, "POST", "/v2/oauth/account/api/logout", writeH(map[string]string{"Origin": ""}), "{}").Expect(t, 403)
		send(t, "POST", "/v2/oauth/account/api/logout", writeH(map[string]string{"Sec-Fetch-Site": "cross-site"}), "{}").Expect(t, 403)
		// Sessiya hali tirik.
		send(t, "GET", "/v2/oauth/account/api/me", map[string]string{"Cookie": cookie}, "").Expect(t, 200)
	})

	w.Step(t, "qurilmani chiqarish: faqat o'ziniki; begona sessiya 404", func(t *testing.T) {
		desk, err := w.Identity.Create(ctx, identity.CreateInput{UserID: uid, ClientID: "merchant-desktop", AMR: []string{"otp"}})
		if err != nil {
			t.Fatal(err)
		}
		send(t, "POST", "/v2/oauth/account/api/sessions/revoke", writeH(nil), `{"id":"`+desk.SessionID+`"}`).Expect(t, 200)
		w.Do(t, &Actor{Name: "desk", Token: desk.AccessToken}, "GET", "/v2/auth/sessions", nil).Expect(t, 401)
		foreign, err := w.Identity.Create(ctx, identity.CreateInput{UserID: w.Owner.UserID, ClientID: "merchant-desktop"})
		if err != nil {
			t.Fatal(err)
		}
		send(t, "POST", "/v2/oauth/account/api/sessions/revoke", writeH(nil), `{"id":"`+foreign.SessionID+`"}`).Expect(t, 404)
		send(t, "POST", "/v2/oauth/account/api/sessions/revoke", writeH(nil), `{"id":""}`).Expect(t, 400)
	})

	w.Step(t, "parol: noto'g'ri joriy / zaif — 400; to'g'ri -> hamma joydan chiqiladi", func(t *testing.T) {
		desk, err := w.Identity.Create(ctx, identity.CreateInput{UserID: uid, ClientID: "merchant-desktop", AMR: []string{"otp"}})
		if err != nil {
			t.Fatal(err)
		}
		send(t, "POST", "/v2/oauth/account/api/password", writeH(nil),
			`{"current_password":"xato","password":"`+newPwd+`","password_confirm":"`+newPwd+`"}`).Expect(t, 400)
		send(t, "POST", "/v2/oauth/account/api/password", writeH(nil),
			`{"current_password":"`+oldPwd+`","password":"12345678","password_confirm":"12345678"}`).Expect(t, 400)
		send(t, "POST", "/v2/oauth/account/api/password", writeH(nil),
			`{"current_password":"`+oldPwd+`","password":"`+newPwd+`","password_confirm":"`+newPwd+`"}`).Expect(t, 200)
		send(t, "GET", "/v2/oauth/account/api/me", map[string]string{"Cookie": cookie}, "").Expect(t, 401)
		w.Do(t, &Actor{Name: "desk", Token: desk.AccessToken}, "GET", "/v2/auth/sessions", nil).Expect(t, 401)
		if _, err := w.AuthSvc.AuthenticatePasswordRealm(ctx, users.RealmMerchant, phone, newPwd); err != nil {
			t.Fatalf("yangi parol ishlamayapti: %v", err)
		}
	})

	w.Step(t, "hamma qurilmalardan chiqish va shu brauzerdan chiqish", func(t *testing.T) {
		login(t)
		desk, err := w.Identity.Create(ctx, identity.CreateInput{UserID: uid, ClientID: "merchant-desktop", AMR: []string{"otp"}})
		if err != nil {
			t.Fatal(err)
		}
		send(t, "POST", "/v2/oauth/account/api/logout-all", writeH(nil), "{}").Expect(t, 200)
		send(t, "GET", "/v2/oauth/account/api/me", map[string]string{"Cookie": cookie}, "").Expect(t, 401)
		w.Do(t, &Actor{Name: "desk", Token: desk.AccessToken}, "GET", "/v2/auth/sessions", nil).Expect(t, 401)

		login(t)
		send(t, "POST", "/v2/oauth/account/api/logout", writeH(nil), "{}").Expect(t, 200)
		send(t, "GET", "/v2/oauth/account", map[string]string{"Cookie": cookie}, "").Expect(t, 302)
	})
}
