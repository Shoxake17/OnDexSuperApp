package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"chustapp/internal/identity"
	"chustapp/internal/storage"
	"chustapp/internal/users"
)

// bffFixture — haqiqiy identity xizmati + soxta API (kirish va "kimman").
type bffFixture struct {
	s      *Server
	h      http.Handler
	svc    *identity.Service
	gotCID atomic.Value // oxirgi kirishdagi client_id
}

func newBFFFixture(t *testing.T) *bffFixture {
	t.Helper()
	key, err := identity.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	ks, _ := identity.NewKeySet(key)
	issuer, err := identity.NewIssuer(ks, "https://api.ondex.test", "ondex-api")
	if err != nil {
		t.Fatal(err)
	}
	ur := storage.NewMemoryUserRepo()
	ctx := context.Background()
	for _, u := range []*users.User{
		{ID: "u_bff00001", Phone: "+998900000001", Role: users.RoleRestaurant, PhoneVerified: true, CreatedAt: time.Now()},
	} {
		if err := ur.Create(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	var seq int64
	svc, err := identity.NewService(identity.NewMemoryStore(), issuer, ur, identity.NopAuditor{},
		func() string { return fmt.Sprintf("sess_%06d", atomic.AddInt64(&seq, 1)) }, nil)
	if err != nil {
		t.Fatal(err)
	}
	f := &bffFixture{svc: svc}
	f.s = &Server{}
	f.s.Identity = svc

	mux := http.NewServeMux()
	// Soxta kirish: tanadagi client_id bilan sessiya ochadi (haqiqiy oqim kabi).
	mux.HandleFunc("POST /v2/auth/merchant/password", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			ClientID string `json:"client_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&b)
		f.gotCID.Store(b.ClientID)
		tok, err := svc.Create(r.Context(), identity.CreateInput{UserID: "u_bff00001", ClientID: b.ClientID, AMR: []string{"pwd", "totp"}})
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, tok)
	})
	mux.HandleFunc("GET /v2/me", func(w http.ResponseWriter, r *http.Request) {
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		c, err := svc.Authenticate(r.Context(), raw)
		if err != nil {
			writeJSON(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		writeJSON(w, 200, map[string]string{"sub": c.Subject, "azp": c.ClientID, "cookie": r.Header.Get("Cookie")})
	})
	mux.HandleFunc("POST /v2/echo", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"auth": r.Header.Get("Authorization")})
	})
	f.h = f.s.withPanelWebBFF(mux)
	return f
}

func (f *bffFixture) do(method, path, body, cookie string, hdr map[string]string) *httptest.ResponseRecorder {
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, "https://merchant.ondex.uz"+path, rd)
	if method != http.MethodGet {
		r.Header.Set("Origin", "https://merchant.ondex.uz")
	}
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: panelWebCookieName, Value: cookie})
	}
	for k, v := range hdr {
		if v == "" {
			r.Header.Del(k)
		} else {
			r.Header.Set(k, v)
		}
	}
	w := httptest.NewRecorder()
	f.h.ServeHTTP(w, r)
	return w
}

func panelCookieOf(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == panelWebCookieName {
			return c
		}
	}
	return nil
}

func TestPanelWebBFFLoginCookieAndProxy(t *testing.T) {
	f := newBFFFixture(t)

	// Klient merchant-desktop deb yuborsa ham — panel klienti majburlanadi.
	w := f.do("POST", "/r/api/v2/auth/merchant/password", `{"phone":"+998900000001","password":"x","client_id":"merchant-desktop"}`, "", nil)
	if w.Code != 200 {
		t.Fatalf("kirish: %d %s", w.Code, w.Body)
	}
	if got := f.gotCID.Load(); got != identity.PanelWebClientID {
		t.Fatalf("client_id majburlanmadi: %v", got)
	}
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out["access_token"] != panelWebMarker || out["refresh_token"] != panelWebMarker {
		t.Fatalf("token brauzerga chiqdi: %v", out)
	}
	if strings.Contains(w.Body.String(), "ort_") || strings.Count(w.Body.String(), ".") > 2 {
		t.Fatalf("javobda token qoldig'i: %s", w.Body)
	}
	c := panelCookieOf(t, w)
	if c == nil || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode || c.Path != "/" {
		t.Fatalf("cookie atributlari: %+v", c)
	}

	// Cookie bilan so'rov — BFF access qo'yadi; brauzer yuborgan
	// Authorization ("bff" yoki begona token) tashlanadi; Cookie API'ga o'tmaydi.
	w = f.do("GET", "/r/api/v2/me", "", c.Value, map[string]string{"Authorization": "Bearer bff"})
	if w.Code != 200 {
		t.Fatalf("me: %d %s", w.Code, w.Body)
	}
	var me map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &me)
	if me["sub"] != "u_bff00001" || me["azp"] != identity.PanelWebClientID || me["cookie"] != "" {
		t.Fatalf("me: %v", me)
	}

	// Cookie'siz — Authorization yo'q (brauzer tokeni o'tmaydi).
	w = f.do("POST", "/r/api/v2/echo", "{}", "", map[string]string{"Authorization": "Bearer stolen"})
	if strings.Contains(w.Body.String(), "stolen") {
		t.Fatalf("brauzer Authorization o'tib ketdi: %s", w.Body)
	}

	// Refresh — cookie tirik: 200 belgi bilan.
	w = f.do("POST", "/r/api/v2/auth/token/refresh", `{"refresh_token":"bff"}`, c.Value, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"access_token":"bff"`) {
		t.Fatalf("refresh: %d %s", w.Code, w.Body)
	}

	// Chiqish — cookie o'chadi, sessiya markazda yopiladi.
	w = f.do("POST", "/r/api/v2/auth/logout", "", c.Value, nil)
	if w.Code != 200 {
		t.Fatalf("logout: %d", w.Code)
	}
	if lc := panelCookieOf(t, w); lc == nil || lc.MaxAge >= 0 {
		t.Fatalf("chiqishda cookie o'chirilmadi: %+v", lc)
	}
	f.s.panelWebCache = panelWebAccessCache{} // boshqa nusxa: kesh yo'q
	w = f.do("GET", "/r/api/v2/me", "", c.Value, nil)
	if w.Code != 401 {
		t.Fatalf("chiqishdan keyin me: %d", w.Code)
	}
	w = f.do("POST", "/r/api/v2/auth/token/refresh", `{}`, c.Value, nil)
	if w.Code != 401 {
		t.Fatalf("chiqishdan keyin refresh: %d", w.Code)
	}
}

func TestPanelWebBFFCSRFAndDenied(t *testing.T) {
	f := newBFFFixture(t)
	body := `{"phone":"+998900000001","password":"x"}`
	if w := f.do("POST", "/r/api/v2/auth/merchant/password", body, "", map[string]string{"Origin": "https://evil.example"}); w.Code != 403 {
		t.Fatalf("begona Origin: %d", w.Code)
	}
	if w := f.do("POST", "/r/api/v2/echo", "{}", "", map[string]string{"Origin": "", "Sec-Fetch-Site": "cross-site"}); w.Code != 403 {
		t.Fatalf("Origin'siz cross-site: %d", w.Code)
	}
	if w := f.do("POST", "/r/api/v2/echo", "{}", "", map[string]string{"Origin": "", "Sec-Fetch-Site": "same-origin"}); w.Code != 200 {
		t.Fatalf("same-origin: %d", w.Code)
	}
	for _, p := range []string{"/r/api/v2/auth/admin/otp/verify", "/r/api/v2/oauth/token", "/r/api/sso/identity", "/r/api/agent/v1/x"} {
		if w := f.do("POST", p, "{}", "", nil); w.Code != 404 {
			t.Fatalf("%s: %d", p, w.Code)
		}
	}
	// Soxta cookie — so'rov tokensiz o'tadi va cookie o'chiriladi.
	w := f.do("GET", "/r/api/v2/me", "", "sess_000001.AAAA", nil)
	if w.Code != 401 {
		t.Fatalf("soxta cookie: %d", w.Code)
	}
	if lc := panelCookieOf(t, w); lc == nil || lc.MaxAge >= 0 {
		t.Fatal("soxta cookie o'chirilmadi")
	}
	// /r/api tashqarisi BFF'ga tegmaydi.
	if w := f.do("POST", "/v2/echo", "{}", "", map[string]string{"Authorization": "Bearer keep", "Origin": "https://evil.example"}); !strings.Contains(w.Body.String(), "keep") {
		t.Fatalf("oddiy yo'l o'zgardi: %s", w.Body)
	}
}

// mfa tokeni tashqaridan (boshqa klient uchun) olingan bo'lsa — BFF orqali
// yakunlangan kirish tokeni brauzerga berilmaydi.
func TestPanelWebBFFRejectsForeignClientSession(t *testing.T) {
	f := newBFFFixture(t)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v2/auth/mfa/verify", func(w http.ResponseWriter, r *http.Request) {
		tok, _ := f.svc.Create(r.Context(), identity.CreateInput{UserID: "u_bff00001", ClientID: "merchant-desktop", AMR: []string{"pwd", "totp"}})
		writeJSON(w, 200, tok)
	})
	f.h = f.s.withPanelWebBFF(mux)
	w := f.do("POST", "/r/api/v2/auth/mfa/verify", `{"mfa_token":"x","code":"1"}`, "", nil)
	if w.Code != 400 || strings.Contains(w.Body.String(), "ort_") || panelCookieOf(t, w) != nil {
		t.Fatalf("begona klient sessiyasi: %d %s", w.Code, w.Body)
	}
}
