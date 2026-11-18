package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"chustapp/internal/identity"
	"chustapp/internal/storage"
	"chustapp/internal/users"
)

// Admin web panel BFF (Plane 479): admin.ondex.uz/a/api, __Host-ondex_admin,
// admin-panel-web klienti.
type adminBFFFixture struct {
	s      *Server
	h      http.Handler
	svc    *identity.Service
	gotCID atomic.Value
}

func newAdminBFFFixture(t *testing.T) *adminBFFFixture {
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
	pkey, _ := identity.GenerateKey()
	pks, _ := identity.NewKeySet(pkey)
	issuer.WithPlatformKeys(pks) // admin tokenlari alohida kalit bilan (A6)
	ur := storage.NewMemoryUserRepo()
	ctx := context.Background()
	for _, u := range []*users.User{
		{ID: "u_adm00001", Phone: "+998900000009", Role: users.RoleAdmin, PhoneVerified: true, CreatedAt: time.Now()},
		{ID: "u_mrc00001", Phone: "+998900000001", Role: users.RoleRestaurant, PhoneVerified: true, CreatedAt: time.Now()},
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
	f := &adminBFFFixture{svc: svc, s: &Server{}}
	f.s.Identity = svc
	mux := http.NewServeMux()
	open := func(user string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			var b struct {
				ClientID string `json:"client_id"`
			}
			_ = json.NewDecoder(r.Body).Decode(&b)
			f.gotCID.Store(b.ClientID)
			tok, err := svc.Create(r.Context(), identity.CreateInput{UserID: user, ClientID: b.ClientID, AMR: []string{"otp", "pwd", "totp"}})
			if err != nil {
				writeJSON(w, 400, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, 200, tok)
		}
	}
	mux.HandleFunc("POST /v2/auth/admin/otp/verify", open("u_adm00001"))
	mux.HandleFunc("POST /v2/auth/merchant/password", open("u_mrc00001"))
	mux.HandleFunc("GET /v2/me", func(w http.ResponseWriter, r *http.Request) {
		c, err := svc.Authenticate(r.Context(), strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if err != nil {
			writeJSON(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		writeJSON(w, 200, map[string]string{"sub": c.Subject, "azp": c.ClientID})
	})
	f.h = f.s.withPanelWebBFF(mux)
	return f
}

func (f *adminBFFFixture) do(host, method, path, body string, cookies map[string]string, origin string) *httptest.ResponseRecorder {
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, "https://"+host+path, rd)
	if method != http.MethodGet {
		r.Header.Set("Origin", origin)
	}
	for k, v := range cookies {
		r.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	w := httptest.NewRecorder()
	f.h.ServeHTTP(w, r)
	return w
}

func cookieNamed(w *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestAdminWebBFFLoginForcesClientAndHidesTokens(t *testing.T) {
	f := newAdminBFFFixture(t)
	const admin = "https://admin.ondex.uz"
	w := f.do("admin.ondex.uz", "POST", "/a/api/v2/auth/admin/otp/verify",
		`{"phone":"+998900000009","code":"1","client_id":"admin-web"}`, nil, admin)
	if w.Code != 200 {
		t.Fatalf("kirish: %d %s", w.Code, w.Body)
	}
	if got, _ := f.gotCID.Load().(string); got != identity.AdminPanelWebClientID {
		t.Fatalf("client_id majburlanmadi: %q", got)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["access_token"] != panelWebMarker || strings.Contains(w.Body.String(), "eyJ") {
		t.Fatalf("token brauzerga chiqdi: %s", w.Body)
	}
	c := cookieNamed(w, adminWebCookieName)
	if c == nil || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode || c.MaxAge != 12*3600 {
		t.Fatalf("cookie: %+v", c)
	}
	me := f.do("admin.ondex.uz", "GET", "/a/api/v2/me", "", map[string]string{adminWebCookieName: c.Value}, "")
	if me.Code != 200 || !strings.Contains(me.Body.String(), identity.AdminPanelWebClientID) {
		t.Fatalf("cookie bilan /me: %d %s", me.Code, me.Body)
	}
	// Admin cookie merchant BFF'da ishlamaydi (alohida kalit va klient).
	if r := f.do("merchant.ondex.uz", "GET", "/r/api/v2/me", "", map[string]string{panelWebCookieName: c.Value}, ""); r.Code != 401 {
		t.Fatalf("admin cookie merchant panelda ishladi: %d", r.Code)
	}
}

func TestAdminWebBFFRejectsMerchantCookieAndSurfaces(t *testing.T) {
	f := newAdminBFFFixture(t)
	const admin = "https://admin.ondex.uz"
	// Merchant panel cookie'si admin'da ishlamaydi.
	mw := f.do("merchant.ondex.uz", "POST", "/r/api/v2/auth/merchant/password", `{"phone":"x"}`, nil, "https://merchant.ondex.uz")
	mc := cookieNamed(mw, panelWebCookieName)
	if mc == nil {
		t.Fatalf("merchant kirishi: %d %s", mw.Code, mw.Body)
	}
	if r := f.do("admin.ondex.uz", "GET", "/a/api/v2/me", "", map[string]string{adminWebCookieName: mc.Value}, ""); r.Code != 401 {
		t.Fatalf("merchant cookie admin'da ishladi: %d", r.Code)
	}
	// Yuzalar: admin BFF'da merchant kirishi, merchant BFF'da admin kirishi yo'q.
	if r := f.do("admin.ondex.uz", "POST", "/a/api/v2/auth/merchant/password", `{}`, nil, admin); r.Code != 404 {
		t.Fatalf("admin BFF merchant kirishini ochdi: %d", r.Code)
	}
	if r := f.do("merchant.ondex.uz", "POST", "/r/api/v2/auth/admin/otp/verify", `{}`, nil, "https://merchant.ondex.uz"); r.Code != 404 {
		t.Fatalf("merchant BFF admin kirishini ochdi: %d", r.Code)
	}
	// CSRF: boshqa manbadan holat o'zgartiruvchi so'rov.
	if r := f.do("admin.ondex.uz", "POST", "/a/api/v2/auth/admin/otp/verify", `{}`, nil, "https://merchant.ondex.uz"); r.Code != 403 {
		t.Fatalf("begona Origin o'tdi: %d", r.Code)
	}
}

func TestAdminWebBFFEnforcesAdminNetworks(t *testing.T) {
	f := newAdminBFFFixture(t)
	f.s.AdminAllowedNets = []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	r := httptest.NewRequest("GET", "https://admin.ondex.uz/a/api/v2/me", nil)
	r.RemoteAddr = "203.0.113.7:5000"
	w := httptest.NewRecorder()
	f.h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("ruxsatsiz tarmoqdan admin BFF ochildi: %d", w.Code)
	}
}

func TestAdminWebBFFProxiesOndexMapWithServerToken(t *testing.T) {
	var gotAuth, gotCookie, gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotCookie, gotPath = r.Header.Get("Authorization"), r.Header.Get("Cookie"), r.URL.Path+"?"+r.URL.RawQuery
		w.Header().Set("Set-Cookie", "evil=1")
		writeJSON(w, 200, map[string]string{"ok": "1"})
	}))
	defer upstream.Close()

	f := newAdminBFFFixture(t)
	f.s.DevMode = true // httptest — http
	f.s.OndexMapAdminURL = upstream.URL + "/admin"
	const admin = "https://admin.ondex.uz"
	lw := f.do("admin.ondex.uz", "POST", "/a/api/v2/auth/admin/otp/verify", `{"phone":"x","code":"1"}`, nil, admin)
	c := cookieNamed(lw, adminWebCookieName)
	if c == nil {
		t.Fatalf("kirish: %d", lw.Code)
	}
	w := f.do("admin.ondex.uz", "GET", "/a/api/x/ondexmap/api/places/meta?status=pending", "",
		map[string]string{adminWebCookieName: c.Value}, "")
	if w.Code != 200 || !strings.HasPrefix(gotAuth, "Bearer ey") || gotCookie != "" {
		t.Fatalf("proksi: %d auth=%q cookie=%q", w.Code, gotAuth, gotCookie)
	}
	if gotPath != "/admin/api/places/meta?status=pending" {
		t.Fatalf("yo'l: %q", gotPath)
	}
	if w.Header().Get("Set-Cookie") != "" {
		t.Fatal("OnDexMap cookie'si brauzerga o'tdi")
	}
	// Cookie'siz — token yo'q, 401; yo'l chetlab o'tish — 400.
	if r := f.do("admin.ondex.uz", "GET", "/a/api/x/ondexmap/api/places", "", nil, ""); r.Code != 401 {
		t.Fatalf("cookie'siz proksi: %d", r.Code)
	}
	if r := f.do("admin.ondex.uz", "GET", "/a/api/x/ondexmap/..%2f..%2fetc", "",
		map[string]string{adminWebCookieName: c.Value}, ""); r.Code != 400 && r.Code != 404 {
		t.Fatalf("yo'l chetlab o'tish: %d", r.Code)
	}
	// Merchant BFF'da bunday proksi yo'q.
	if r := f.do("merchant.ondex.uz", "GET", "/r/api/x/ondexmap/api/places", "", nil, ""); r.Code == 200 {
		t.Fatal("merchant BFF OnDexMap proksisini ochdi")
	}
}
