package e2e

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"chustapp/internal/e2e/report"
	"chustapp/internal/identity"
	"chustapp/internal/users"
)

// Senariy: OnDex OIDC (A4c) — Merchant web (ShoxPos BFF) OnDex orqali kiradi.
//
//	BFF -> /v2/oauth/authorize (PKCE S256, nonce, state)
//	    -> OnDex kirish sahifasi -> ondex-sso sessiyasi -> /login/complete
//	    -> SSO cookie + kod -> BFF /v2/oauth/token -> markaziy sessiya + id_token
//	    -> /v2/oauth/userinfo; keyingi kirish cookie bilan (parolsiz)
//
// Xavfsizlik: begona redirect_uri ga HECH QACHON yo'naltirilmaydi; PKCE'siz
// so'rov rad; kod bir martalik — qayta kelsa birinchi sessiya o'ladi; noto'g'ri
// sir/verifier rad; boshqa ilova sessiyasi SSO cookie'ga aylanmaydi.
func TestOIDCMerchantWeb(t *testing.T) {
	w := NewWorld(t, "OnDex OIDC (A4c): Merchant web BFF kirishi")
	ctx := context.Background()

	const ownerID, ownerPhone = "e2e-oidc-owner", "+998900000061"
	if err := w.Users.Create(ctx, &users.User{ID: ownerID, Phone: ownerPhone, Name: "OIDC Egasi",
		Role: users.RoleRestaurant, EntityID: RestA, PhoneVerified: true, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}

	const verifier = "e2e-oidc-pkce-verifier-0123456789-abcdefghijklmnop"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	authQ := func(over map[string]string) string {
		q := url.Values{"client_id": {"merchant-web"}, "redirect_uri": {OIDCRedirectURI}, "response_type": {"code"},
			"scope": {"openid profile"}, "state": {"e2e-state"}, "nonce": {"e2e-nonce"},
			"code_challenge": {challenge}, "code_challenge_method": {"S256"}}
		for k, v := range over {
			q.Set(k, v)
		}
		return "/v2/oauth/authorize?" + q.Encode()
	}
	noRedirect := &http.Client{Timeout: 15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	// send — yo'naltirishga ERGASHMAYDI (Location tekshiriladi), qamrovga yoziladi.
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
			Ms: float64(time.Since(start).Microseconds()) / 1000, Actor: "brauzer/BFF"})
		r := &Resp{Status: res.StatusCode, Body: raw, Header: res.Header}
		w.checkGeneric(t, method, path, r)
		return r
	}
	form := func(v url.Values) (map[string]string, string) {
		return map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, v.Encode()
	}
	type tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error"`
	}
	exchange := func(t *testing.T, code, secret, ver string, want int) tokenResp {
		t.Helper()
		h, b := form(url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {OIDCRedirectURI},
			"code_verifier": {ver}, "client_id": {"merchant-web"}, "client_secret": {secret}})
		var tr tokenResp
		send(t, "POST", "/v2/oauth/token", h, b).Expect(t, want).JSON(t, &tr)
		return tr
	}
	codeFrom := func(t *testing.T, loc string) string {
		t.Helper()
		u, err := url.Parse(loc)
		if err != nil || !strings.HasPrefix(loc, OIDCRedirectURI+"?") || u.Query().Get("state") != "e2e-state" {
			t.Fatalf("klientga qaytish manzili noto'g'ri: %q", loc)
		}
		return u.Query().Get("code")
	}

	w.Step(t, "discovery: issuer, faqat code + S256, EdDSA", func(t *testing.T) {
		var d map[string]any
		send(t, "GET", "/.well-known/openid-configuration", nil, "").Expect(t, 200).JSON(t, &d)
		if d["issuer"] != "https://api.e2e.ondex" || !strings.Contains(asString(d["code_challenge_methods_supported"]), "S256") ||
			strings.Contains(asString(d["code_challenge_methods_supported"]), "plain") {
			t.Fatalf("discovery: %v", d)
		}
	})

	w.Step(t, "xavfli so'rovlar: begona redirect -> sahifa (yo'naltirish YO'Q); PKCE'siz -> klientga xato", func(t *testing.T) {
		r := send(t, "GET", authQ(map[string]string{"redirect_uri": "https://evil.test/cb"}), nil, "").Expect(t, 400)
		if r.Header.Get("Location") != "" {
			t.Fatal("XAVFSIZLIK: begona redirect_uri ga yo'naltirildi")
		}
		r = send(t, "GET", authQ(map[string]string{"code_challenge_method": "plain"}), nil, "").Expect(t, 302)
		if loc := r.Header.Get("Location"); !strings.HasPrefix(loc, OIDCRedirectURI+"?") || !strings.Contains(loc, "error=invalid_request") {
			t.Fatalf("PKCE plain: %q", loc)
		}
		r = send(t, "GET", authQ(map[string]string{"prompt": "none"}), nil, "").Expect(t, 302)
		if !strings.Contains(r.Header.Get("Location"), "error=login_required") {
			t.Fatalf("prompt=none cookie'siz: %q", r.Header.Get("Location"))
		}
	})

	var reqBlob string
	w.Step(t, "cookie'siz authorize -> OnDex kirish sahifasi (CSP qat'iy)", func(t *testing.T) {
		r := send(t, "GET", authQ(nil), nil, "").Expect(t, 302)
		loc := r.Header.Get("Location")
		if !strings.HasPrefix(loc, "/v2/oauth/login?req=") {
			t.Fatalf("kirish sahifasiga yo'naltirilmadi: %q", loc)
		}
		u, _ := url.Parse(loc)
		reqBlob = u.Query().Get("req")
		page := send(t, "GET", loc, nil, "").Expect(t, 200)
		if csp := page.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") ||
			strings.Contains(csp, "unsafe-inline") {
			t.Fatalf("kirish sahifasi CSP: %q", csp)
		}
		send(t, "GET", "/v2/oauth/assets/login.js", nil, "").Expect(t, 200)
		send(t, "GET", "/v2/oauth/assets/login.css", nil, "").Expect(t, 200)
		// Parol "ko'zi": sahifa uni yuklaydi, tashqi skriptsiz va innerHTML'siz.
		if r := send(t, "GET", "/v2/oauth/assets/eye.js", nil, "").Expect(t, 200); !strings.HasPrefix(r.Header.Get("Content-Type"), "text/javascript") ||
			strings.Contains(string(r.Body), ".innerHTML") {
			t.Fatalf("eye.js: %q", r.Header.Get("Content-Type"))
		}
		if !strings.Contains(string(page.Body), `src="/v2/oauth/assets/eye.js"`) {
			t.Fatal("kirish sahifasi eye.js ni yuklamaydi")
		}
		if r := send(t, "GET", "/v2/oauth/assets/bg.jpg", nil, "").Expect(t, 200); r.Header.Get("Content-Type") != "image/jpeg" {
			t.Fatalf("fon turi: %q", r.Header.Get("Content-Type"))
		}
		send(t, "GET", "/v2/oauth/assets/logo.png", nil, "").Expect(t, 200)
		var info map[string]string
		send(t, "GET", "/v2/oauth/login/info?req="+url.QueryEscape(reqBlob), nil, "").Expect(t, 200).JSON(t, &info)
		if info["client_name"] == "" {
			t.Fatalf("info: %v", info)
		}
		send(t, "GET", "/v2/oauth/login/info?req=buzilgan", nil, "").Expect(t, 400)
	})

	w.Step(t, "kirish sahifasi merchant kirishini client_id=ondex-sso bilan boshlay oladi", func(t *testing.T) {
		// Parolsiz egasi: 401 (akkaunt mavjudligi bilinmaydi), lekin "noma'lum klient" (400) EMAS.
		w.Do(t, nil, "POST", "/v2/auth/merchant/password", map[string]any{"phone": ownerPhone, "password": "x",
			"client_id": identity.SSOClientID}).Expect(t, 401)
	})

	var cookie, firstAccess string
	w.Step(t, "login/complete: faqat ondex-sso tokeni; SSO cookie + kod", func(t *testing.T) {
		body := `{"req":"` + reqBlob + `"}`
		jsonH := map[string]string{"Content-Type": "application/json"}
		send(t, "POST", "/v2/oauth/login/complete", jsonH, body).Expect(t, 401)
		pro, err := w.Identity.Create(ctx, identity.CreateInput{UserID: ownerID, ClientID: "ondex-pro", AMR: []string{"otp"}})
		if err != nil {
			t.Fatal(err)
		}
		send(t, "POST", "/v2/oauth/login/complete",
			map[string]string{"Content-Type": "application/json", "Authorization": "Bearer " + pro.AccessToken}, body).Expect(t, 401)

		sso, err := w.Identity.Create(ctx, identity.CreateInput{UserID: ownerID, ClientID: identity.SSOClientID,
			AMR: []string{"pwd", "totp"}})
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]string
		r := send(t, "POST", "/v2/oauth/login/complete",
			map[string]string{"Content-Type": "application/json", "Authorization": "Bearer " + sso.AccessToken}, body).
			Expect(t, 200)
		r.JSON(t, &out)
		sc := r.Header.Get("Set-Cookie")
		if !strings.HasPrefix(sc, "__Host-ondex_sso=") || !strings.Contains(sc, "HttpOnly") || !strings.Contains(sc, "Secure") {
			t.Fatalf("SSO cookie: %q", sc)
		}
		cookie, _, _ = strings.Cut(sc, ";")
		code := codeFrom(t, out["redirect_to"])

		// Noto'g'ri sir — klient kod OCHILISHIDAN oldin rad etiladi: kod
		// "yonmaydi", haqiqiy BFF uni keyin almashtira oladi.
		exchange(t, code, "noto'g'ri-sir", verifier, 401)
		if tr := exchange(t, code, OIDCClientSecret, verifier, 200); tr.AccessToken == "" {
			t.Fatal("kod almashtirilmadi")
		}
	})

	w.Step(t, "kod almashinuvi: to'g'ri sir + verifier -> tokenlar; qayta kelsa birinchi sessiya o'ladi", func(t *testing.T) {
		r := send(t, "GET", authQ(nil), map[string]string{"Cookie": cookie}, "").Expect(t, 302)
		code := codeFrom(t, r.Header.Get("Location"))
		exchange(t, code, OIDCClientSecret, verifier+"x", 400)

		r = send(t, "GET", authQ(nil), map[string]string{"Cookie": cookie}, "").Expect(t, 302)
		code = codeFrom(t, r.Header.Get("Location"))
		tr := exchange(t, code, OIDCClientSecret, verifier, 200)
		if tr.AccessToken == "" || tr.RefreshToken == "" || tr.IDToken == "" || tr.ExpiresIn <= 0 {
			t.Fatalf("token javobi: %+v", tr)
		}
		firstAccess = tr.AccessToken
		w.Do(t, &Actor{Name: "BFF", Token: firstAccess}, "GET", "/v2/auth/sessions", nil).Expect(t, 200)

		exchange(t, code, OIDCClientSecret, verifier, 400)
		w.Do(t, &Actor{Name: "BFF", Token: firstAccess}, "GET", "/v2/auth/sessions", nil).Expect(t, 401)
	})

	w.Step(t, "SSO cookie bilan qayta kirish parolsiz; userinfo; refresh", func(t *testing.T) {
		r := send(t, "GET", authQ(nil), map[string]string{"Cookie": cookie}, "").Expect(t, 302)
		tr := exchange(t, codeFrom(t, r.Header.Get("Location")), OIDCClientSecret, verifier, 200)

		var ui map[string]any
		send(t, "GET", "/v2/oauth/userinfo", map[string]string{"Authorization": "Bearer " + tr.AccessToken}, "").
			Expect(t, 200).JSON(t, &ui)
		if ui["sub"] != ownerID || ui["phone_number"] != ownerPhone {
			t.Fatalf("userinfo: %v", ui)
		}
		if p, _ := ui["partner"].(map[string]any); p == nil || p["role"] != "restaurant" || p["entity_id"] != RestA {
			t.Fatalf("userinfo hamkor ma'lumoti: %v", ui["partner"])
		}
		send(t, "POST", "/v2/oauth/userinfo", map[string]string{"Authorization": "Bearer " + tr.AccessToken}, "").Expect(t, 200)
		send(t, "GET", "/v2/oauth/userinfo", nil, "").Expect(t, 401)

		h, b := form(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tr.RefreshToken}})
		h["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte("merchant-web:"+OIDCClientSecret))
		var r2 tokenResp
		send(t, "POST", "/v2/oauth/token", h, b).Expect(t, 200).JSON(t, &r2)
		if r2.AccessToken == "" || r2.RefreshToken == tr.RefreshToken {
			t.Fatalf("refresh: %+v", r2)
		}
		h, b = form(url.Values{"grant_type": {"password"}})
		send(t, "POST", "/v2/oauth/token", h, b).Expect(t, 400)
	})

	w.Step(t, "avatar: OnDex akkauntida (R2), userinfo picture; rasm bo'lmagan fayl rad", func(t *testing.T) {
		r := send(t, "GET", authQ(nil), map[string]string{"Cookie": cookie}, "").Expect(t, 302)
		tr := exchange(t, codeFrom(t, r.Header.Get("Location")), OIDCClientSecret, verifier, 200)
		bearer := map[string]string{"Authorization": "Bearer " + tr.AccessToken}
		multipartFile := func(name string, data []byte) (map[string]string, string) {
			var buf bytes.Buffer
			mw := multipart.NewWriter(&buf)
			fw, _ := mw.CreateFormFile("file", name)
			_, _ = fw.Write(data)
			_ = mw.Close()
			return map[string]string{"Authorization": "Bearer " + tr.AccessToken, "Content-Type": mw.FormDataContentType()}, buf.String()
		}
		img := image.NewRGBA(image.Rect(0, 0, 64, 48))
		for i := range img.Pix {
			img.Pix[i] = byte(i)
		}
		var pngBuf bytes.Buffer
		_ = png.Encode(&pngBuf, img)

		h, b := multipartFile("rasm.png", []byte("bu rasm emas"))
		send(t, "PUT", "/v2/me/avatar", h, b).Expect(t, 400)
		send(t, "PUT", "/v2/me/avatar", nil, "").Expect(t, 401)

		h, b = multipartFile("yuz.png", pngBuf.Bytes())
		var out map[string]string
		send(t, "PUT", "/v2/me/avatar", h, b).Expect(t, 200).JSON(t, &out)
		if !strings.HasSuffix(out["picture"], ".webp") {
			t.Fatalf("avatar manzili: %v", out)
		}
		var ui map[string]any
		send(t, "GET", "/v2/oauth/userinfo", bearer, "").Expect(t, 200).JSON(t, &ui)
		if ui["picture"] != out["picture"] {
			t.Fatalf("userinfo picture: %v != %v", ui["picture"], out["picture"])
		}
		send(t, "DELETE", "/v2/me/avatar", bearer, "").Expect(t, 204)
		ui = nil
		send(t, "GET", "/v2/oauth/userinfo", bearer, "").Expect(t, 200).JSON(t, &ui)
		if _, has := ui["picture"]; has {
			t.Fatal("o'chirilgan avatar userinfo'da qoldi")
		}
	})

	w.Step(t, "chiqish CSRF: hujumchining O'Z id_token'i bilan GET qurbonni chiqarmaydi", func(t *testing.T) {
		// Hujumchi o'z akkaunti bilan to'liq kirib, haqiqiy id_token oladi.
		const atkID = "e2e-oidc-attacker"
		if err := w.Users.Create(ctx, &users.User{ID: atkID, Phone: "+998900000062", Name: "Hujumchi",
			Role: users.RoleRestaurant, EntityID: RestB, PhoneVerified: true, CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
		u, _ := url.Parse(send(t, "GET", authQ(nil), nil, "").Expect(t, 302).Header.Get("Location"))
		atkSSO, err := w.Identity.Create(ctx, identity.CreateInput{UserID: atkID, ClientID: identity.SSOClientID, AMR: []string{"pwd", "totp"}})
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]string
		send(t, "POST", "/v2/oauth/login/complete", map[string]string{"Content-Type": "application/json",
			"Authorization": "Bearer " + atkSSO.AccessToken}, `{"req":"`+u.Query().Get("req")+`"}`).Expect(t, 200).JSON(t, &out)
		atk := exchange(t, codeFrom(t, out["redirect_to"]), OIDCClientSecret, verifier, 200)

		// Qurbon brauzeri (cookie) hujumchi havolasini ochadi.
		q := url.Values{"id_token_hint": {atk.IDToken}, "post_logout_redirect_uri": {"https://merchant.ondex.uz/"}}
		page := send(t, "GET", "/v2/oauth/logout?"+q.Encode(), map[string]string{"Cookie": cookie}, "").Expect(t, 200)
		if !strings.Contains(string(page.Body), `method="post"`) || page.Header.Get("Location") != "" {
			t.Fatal("XAVFSIZLIK: begona hint bilan GET tasdiqsiz chiqardi")
		}
		r := send(t, "GET", authQ(nil), map[string]string{"Cookie": cookie}, "").Expect(t, 302)
		codeFrom(t, r.Header.Get("Location")) // qurbon SSO sessiyasi tirik
	})

	w.Step(t, "chiqish: hint'siz GET faqat tasdiq (CSRF), hint bilan -> SSO va ilova sessiyasi yopiladi", func(t *testing.T) {
		r := send(t, "GET", authQ(nil), map[string]string{"Cookie": cookie}, "").Expect(t, 302)
		tr := exchange(t, codeFrom(t, r.Header.Get("Location")), OIDCClientSecret, verifier, 200)
		bff := &Actor{Name: "BFF", Token: tr.AccessToken}

		page := send(t, "GET", "/v2/oauth/logout", map[string]string{"Cookie": cookie}, "").Expect(t, 200)
		if !strings.Contains(string(page.Body), `method="post"`) {
			t.Fatal("hint'siz GET tasdiq sahifasini ko'rsatmadi")
		}
		w.Do(t, bff, "GET", "/v2/auth/sessions", nil).Expect(t, 200) // hali tirik

		q := url.Values{"id_token_hint": {tr.IDToken}, "post_logout_redirect_uri": {"https://merchant.ondex.uz/"}, "state": {"bye"}}
		r = send(t, "GET", "/v2/oauth/logout?"+q.Encode(), map[string]string{"Cookie": cookie}, "").Expect(t, 302)
		if r.Header.Get("Location") != "https://merchant.ondex.uz/?state=bye" {
			t.Fatalf("chiqishdan keyin qaytish: %q", r.Header.Get("Location"))
		}
		w.Do(t, bff, "GET", "/v2/auth/sessions", nil).Expect(t, 401)
		r = send(t, "GET", authQ(nil), map[string]string{"Cookie": cookie}, "").Expect(t, 302)
		if !strings.HasPrefix(r.Header.Get("Location"), "/v2/oauth/login?") {
			t.Fatal("XAVFSIZLIK: chiqishdan keyin SSO cookie hali ishlayapti")
		}
		// begona post_logout manzili -> yo'naltirish yo'q; POST ham ishlaydi
		h, b := form(url.Values{"post_logout_redirect_uri": {"https://evil.test/"}, "client_id": {"merchant-web"}})
		if r := send(t, "POST", "/v2/oauth/logout", h, b).Expect(t, 200); r.Header.Get("Location") != "" {
			t.Fatal("XAVFSIZLIK: begona post_logout manziliga yo'naltirildi")
		}
	})
}

func asString(v any) string {
	switch x := v.(type) {
	case []any:
		var parts []string
		for _, p := range x {
			if s, ok := p.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ",")
	case string:
		return x
	}
	return ""
}
