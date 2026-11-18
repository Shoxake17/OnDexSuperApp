package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"chustapp/internal/identity"
)

// ┌─ WEB PANEL BFF (RS-7 merchant, 479 admin) ───────────────────────────────┐
// Flutter web panellar API'ni o'z domenidagi prefiks orqali chaqiradi
// (Caddy -> shu jarayon). Har panel — alohida instansiya (`webBFF`):
//
//	merchant.ondex.uz/r/api  cookie __Host-ondex_panel  klient merchant-panel-web
//	admin.ondex.uz/a/api     cookie __Host-ondex_admin  klient admin-panel-web
//
// Bu qatlam:
//   - brauzerdan kelgan `Authorization` ni TASHLAYDI — kimlik faqat
//     HttpOnly cookie'dan (SameSite=Strict, __Host- prefiksi);
//   - cookie sessiyasi bo'yicha qisqa access token chiqarib so'rovga o'zi
//     qo'yadi (`identity.WebBFFAccess`, faol foydalanuvchida idle uzayadi);
//   - kirish bosqichlarida `client_id` ni MAJBURAN o'z klientiga qiladi va
//     javobdagi access/refresh tokenlarni "bff" belgisiga almashtiradi —
//     token brauzer xotirasiga HECH QACHON tushmaydi, XSS uni o'g'irlay olmaydi;
//   - holat o'zgartiruvchi so'rovlarda Origin'ni tekshiradi (CSRF);
//   - panelga kerak bo'lmagan yuzalarni yopadi (merchant'da admin kirishi,
//     admin'da merchant kirishi, OIDC, SSO, agentlar).
//
// Flutter klienti (`ondex_core` ApiClient) o'zgarmaydi: u "bff" tokenini
// odatdagidek saqlaydi va yuboradi, 401 da refresh so'raydi — BFF refresh'ni
// cookie sessiyasi bo'yicha o'zi javoblaydi.
// └──────────────────────────────────────────────────────────────────────────┘

const (
	panelWebMarker  = "bff"
	panelWebBodyMax = 64 << 10

	// Merchant panel (RS-7) — testlar va boshqa fayllar shu nomlarga tayanadi.
	panelWebPrefix     = "/r/api"
	panelWebCookieName = "__Host-ondex_panel"

	adminWebPrefix     = "/a/api"
	adminWebCookieName = "__Host-ondex_admin"
)

// webBFF — bitta web panel BFF sozlamasi.
type webBFF struct {
	name        string // log va xatolar uchun
	prefix      string
	cookieName  string
	clientID    string
	cookieAge   time.Duration
	origin      func(s *Server) string
	loginPaths  map[string]bool // javobida token bo'lishi mumkin
	clientPaths map[string]bool // tanasida client_id bor (majburlanadi)
	denied      []string        // BFF orqali ochilmaydigan yuzalar
	cache       *panelWebAccessCache
}

var merchantWebBFF = webBFF{
	name:       "merchant",
	prefix:     panelWebPrefix,
	cookieName: panelWebCookieName,
	clientID:   identity.PanelWebClientID,
	cookieAge:  7 * 24 * time.Hour, // sessiya 7 kun (idle = abs)
	origin:     (*Server).panelWebOrigin,
	loginPaths: map[string]bool{
		"/v2/auth/merchant/password":   true,
		"/v2/auth/merchant/otp/verify": true,
		"/v2/auth/mfa/password":        true,
		"/v2/auth/mfa/factor":          true,
		"/v2/auth/mfa/verify":          true,
		"/v2/auth/mfa/totp/enroll":     true,
		"/v2/auth/mfa/totp/confirm":    true,
		"/v2/auth/passkey/login":       true,
	},
	clientPaths: map[string]bool{
		"/v2/auth/merchant/password":   true,
		"/v2/auth/merchant/otp/verify": true,
		"/v2/auth/passkey/options":     true,
		"/v2/auth/passkey/login":       true,
	},
	denied: []string{"/v2/auth/admin/", "/v2/oauth/", "/sso/", "/agent/"},
}

var adminWebBFF = webBFF{
	name:       "admin",
	prefix:     adminWebPrefix,
	cookieName: adminWebCookieName,
	clientID:   identity.AdminPanelWebClientID,
	cookieAge:  12 * time.Hour, // sessiya mutlaq 12 soat (idle 30 daq — server)
	origin:     (*Server).adminWebOrigin,
	loginPaths: map[string]bool{
		"/v2/auth/admin/otp/verify": true,
		"/v2/auth/mfa/password":     true,
		"/v2/auth/mfa/factor":       true,
		"/v2/auth/mfa/verify":       true,
		"/v2/auth/mfa/totp/enroll":  true,
		"/v2/auth/mfa/totp/confirm": true,
		"/v2/auth/passkey/login":    true,
	},
	clientPaths: map[string]bool{
		"/v2/auth/admin/otp/verify": true,
		"/v2/auth/passkey/options":  true,
		"/v2/auth/passkey/login":    true,
	},
	// Admin web — faqat admin. Merchant kirishi va boshqa tashqi yuzalar yopiq.
	denied: []string{"/v2/auth/merchant/", "/v2/oauth/", "/sso/", "/agent/"},
}

// panelWebAccessCache — cookie bo'yicha chiqarilgan access token (har
// so'rovda baza/imzo ishlamasin). Sessiya bekor qilinsa token baribir
// `Authenticate` da (faollik keshi ≤30 s) rad etiladi.
type panelWebAccessCache struct {
	mu sync.Mutex
	m  map[[32]byte]panelWebAccess
}

type panelWebAccess struct {
	token string
	exp   time.Time
}

const panelWebCacheMax = 10000

func (c *panelWebAccessCache) get(k [32]byte, now time.Time) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	a, ok := c.m[k]
	if !ok || now.Add(time.Minute).After(a.exp) {
		return "", false
	}
	return a.token, true
}

func (c *panelWebAccessCache) put(k [32]byte, a panelWebAccess) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil || len(c.m) >= panelWebCacheMax {
		c.m = make(map[[32]byte]panelWebAccess)
	}
	c.m[k] = a
}

func (c *panelWebAccessCache) drop(k [32]byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.m, k)
}

// cacheFor — har panelning o'z keshi (bir cookie ikkinchi panelda ishlamaydi).
func (s *Server) cacheFor(b *webBFF) *panelWebAccessCache {
	if b.clientID == identity.AdminPanelWebClientID {
		return &s.adminWebCache
	}
	return &s.panelWebCache
}

// withPanelWebBFF — /r/api/* va /a/api/* so'rovlarini mos BFF orqali
// `next` ga uzatadi, qolganlari o'zgarishsiz o'tadi.
func (s *Server) withPanelWebBFF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, b := range []*webBFF{&merchantWebBFF, &adminWebBFF} {
			if r.URL.Path == b.prefix || strings.HasPrefix(r.URL.Path, b.prefix+"/") {
				s.serveWebBFF(b, w, r, next)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) serveWebBFF(b *webBFF, w http.ResponseWriter, r *http.Request, next http.Handler) {
	w.Header().Set("Cache-Control", "no-store")
	if s.Identity == nil {
		panelWebErr(w, http.StatusServiceUnavailable, "panel kirishi sozlanmagan")
		return
	}
	// Admin web — faqat ruxsat etilgan tarmoqdan (ADMIN_ALLOWED_CIDRS), HAR
	// so'rovda (kirishdan keyin ham): tarmoqdan tashqarida cookie ishlamaydi.
	if b.clientID == identity.AdminPanelWebClientID && !s.adminIPAllowed(r) {
		panelWebErr(w, http.StatusForbidden, errAdminIPDenied.Error())
		return
	}
	path := strings.TrimPrefix(r.URL.Path, b.prefix)
	if path == "" {
		path = "/"
	}
	for _, p := range b.denied {
		if strings.HasPrefix(path, p) {
			panelWebErr(w, http.StatusNotFound, "topilmadi")
			return
		}
	}
	if !webBFFSameOrigin(r, b.origin(s)) {
		panelWebErr(w, http.StatusForbidden, "so'rov manbasi ruxsat etilmagan")
		return
	}

	r2 := r.Clone(r.Context())
	r2.URL.Path = path
	r2.URL.RawPath = ""
	r2.RequestURI = ""
	r2.Header.Del("Authorization")
	r2.Header.Del("Cookie") // cookie API ichiga uzatilmaydi (SSO cookie bilan aralashmasin)

	cookie := ""
	if c, err := r.Cookie(b.cookieName); err == nil {
		cookie = c.Value
	}
	cache := s.cacheFor(b)

	switch path {
	case "/v2/auth/token/refresh":
		s.webBFFRefresh(b, w, r2, cookie)
		return
	case "/v2/auth/logout", "/auth/logout":
		if cookie != "" {
			s.Identity.WebBFFLogout(r.Context(), b.clientID, cookie)
			cache.drop(sha256.Sum256([]byte(cookie)))
		}
		clearWebBFFCookie(w, b)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	token := ""
	if cookie != "" {
		if tok, ok := s.webBFFToken(b, r2, cookie); ok {
			r2.Header.Set("Authorization", "Bearer "+tok)
			token = tok
		} else {
			clearWebBFFCookie(w, b)
		}
	}
	// Admin web: OnDexMap admin serveri shu BFF orqali (token server ichida).
	if b.clientID == identity.AdminPanelWebClientID && strings.HasPrefix(path, adminOndexMapPrefix) {
		s.proxyOndexMap(w, r2, path, token)
		return
	}

	if b.clientPaths[path] && r2.Method == http.MethodPost {
		if !forceWebBFFClient(w, r2, b.clientID) {
			return
		}
	}
	if b.loginPaths[path] {
		s.webBFFLogin(b, w, r2, next)
		return
	}
	next.ServeHTTP(w, r2)
}

// webBFFSameOrigin — CSRF: holat o'zgartiruvchi so'rov faqat panelning
// o'z sahifasidan (Origin aniq mos, yoki Origin'siz bo'lsa Sec-Fetch-Site
// same-origin). GET/HEAD/OPTIONS — cookie SameSite=Strict yetarli.
func webBFFSameOrigin(r *http.Request, origin string) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	if o := r.Header.Get("Origin"); o != "" {
		return o == origin
	}
	return r.Header.Get("Sec-Fetch-Site") == "same-origin"
}

// panelWebSameOrigin — merchant panel uchun (eski nom).
func (s *Server) panelWebSameOrigin(r *http.Request) bool {
	return webBFFSameOrigin(r, s.panelWebOrigin())
}

func (s *Server) panelWebOrigin() string {
	if s.PanelWebOrigin != "" {
		return s.PanelWebOrigin
	}
	return "https://merchant.ondex.uz"
}

func (s *Server) adminWebOrigin() string {
	if s.AdminWebOrigin != "" {
		return s.AdminWebOrigin
	}
	return "https://admin.ondex.uz"
}

func (s *Server) webBFFToken(b *webBFF, r *http.Request, cookie string) (string, bool) {
	k := sha256.Sum256([]byte(cookie))
	now := time.Now()
	cache := s.cacheFor(b)
	if t, ok := cache.get(k, now); ok {
		return t, true
	}
	tok, exp, err := s.Identity.WebBFFAccess(r.Context(), b.clientID, cookie)
	if err != nil {
		cache.drop(k)
		return "", false
	}
	cache.put(k, panelWebAccess{token: tok, exp: exp})
	return tok, true
}

// webBFFRefresh — klient 401 dan keyin refresh so'raydi: cookie sessiyasi
// tirik bo'lsa "bff" belgisi bilan 200 (keyingi so'rov yangi access oladi),
// aks holda 401 — klient login ekraniga qaytadi.
func (s *Server) webBFFRefresh(b *webBFF, w http.ResponseWriter, r *http.Request, cookie string) {
	if cookie != "" {
		s.cacheFor(b).drop(sha256.Sum256([]byte(cookie)))
		if _, ok := s.webBFFToken(b, r, cookie); ok {
			writeJSON(w, http.StatusOK, map[string]string{
				"access_token": panelWebMarker, "refresh_token": panelWebMarker, "token_type": "Bearer"})
			return
		}
	}
	clearWebBFFCookie(w, b)
	panelWebErr(w, http.StatusUnauthorized, identity.ErrRefreshInvalid.Error())
}

// forceWebBFFClient — tanadagi `client_id` ni panel klientiga almashtiradi
// (boshqa qiymat kelsa ham): BFF orqali faqat shu panel sessiyasi ochiladi.
func forceWebBFFClient(w http.ResponseWriter, r *http.Request, clientID string) bool {
	raw, err := io.ReadAll(io.LimitReader(r.Body, panelWebBodyMax+1))
	if err != nil || len(raw) > panelWebBodyMax {
		panelWebErr(w, http.StatusRequestEntityTooLarge, "so'rov juda katta")
		return false
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(raw, &body); err != nil || body == nil {
		panelWebErr(w, http.StatusBadRequest, "noto'g'ri so'rov")
		return false
	}
	body["client_id"], _ = json.Marshal(clientID)
	out, _ := json.Marshal(body)
	r.Body = io.NopCloser(bytes.NewReader(out))
	r.ContentLength = int64(len(out))
	r.Header.Set("Content-Length", strconv.Itoa(len(out)))
	return true
}

func panelWebErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// webBFFLogin — kirish bosqichi javobini ushlaydi: token bo'lsa (sessiya
// ochildi) cookie qo'yiladi va tokenlar javobdan olib tashlanadi.
func (s *Server) webBFFLogin(b *webBFF, w http.ResponseWriter, r *http.Request, next http.Handler) {
	rec := &panelWebRecorder{h: http.Header{}, code: http.StatusOK}
	next.ServeHTTP(rec, r)

	body := rec.buf.Bytes()
	var m map[string]json.RawMessage
	if rec.code == http.StatusOK && json.Unmarshal(body, &m) == nil {
		if raw, ok := m["access_token"]; ok {
			var access string
			_ = json.Unmarshal(raw, &access)
			sid, ok := s.Identity.WebBFFSessionOf(r.Context(), b.clientID, access)
			if !ok {
				// Boshqa klient sessiyasi (mfa tokeni tashqaridan olingan) —
				// tokenni brauzerga bermaymiz.
				panelWebErr(w, http.StatusBadRequest, "bu kirish web panel uchun emas")
				return
			}
			setWebBFFCookie(w, b, s.Identity.WebBFFCookie(b.clientID, sid))
			marker, _ := json.Marshal(panelWebMarker)
			m["access_token"] = marker
			if _, ok := m["refresh_token"]; ok {
				m["refresh_token"] = marker
			}
			delete(m, "token")
			body, _ = json.Marshal(m)
			rec.h.Del("Content-Length")
		}
	}
	for k, v := range rec.h {
		w.Header()[k] = v
	}
	w.WriteHeader(rec.code)
	_, _ = w.Write(body)
}

type panelWebRecorder struct {
	h     http.Header
	code  int
	buf   bytes.Buffer
	wrote bool
}

func (p *panelWebRecorder) Header() http.Header { return p.h }
func (p *panelWebRecorder) WriteHeader(c int) {
	if !p.wrote {
		p.code, p.wrote = c, true
	}
}
func (p *panelWebRecorder) Write(b []byte) (int, error) {
	p.wrote = true
	return p.buf.Write(b)
}

func setWebBFFCookie(w http.ResponseWriter, b *webBFF, v string) {
	age := int(b.cookieAge / time.Second)
	http.SetCookie(w, &http.Cookie{Name: b.cookieName, Value: v, Path: "/", MaxAge: age,
		Expires: time.Now().Add(b.cookieAge), Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
}

func clearWebBFFCookie(w http.ResponseWriter, b *webBFF) {
	http.SetCookie(w, &http.Cookie{Name: b.cookieName, Value: "", Path: "/", MaxAge: -1,
		Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
}
