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

// ┌─ MERCHANT WEB PANEL BFF (RS-7) ─────────────────────────────────────────┐
// Flutter web panel merchant.ondex.uz/r/ da ishlaydi, API'ni o'sha domenning
// /r/api/* yo'li orqali chaqiradi (Caddy -> shu jarayon). Bu qatlam:
//
//   - brauzerdan kelgan `Authorization` ni TASHLAYDI — kimlik faqat
//     HttpOnly cookie'dan (`__Host-ondex_panel`, SameSite=Strict);
//   - cookie sessiyasi bo'yicha qisqa access token chiqarib, so'rovga
//     o'zi qo'yadi (`identity.PanelWebAccess`);
//   - kirish bosqichlarida `client_id` ni majburan `merchant-panel-web`
//     qiladi va javobdagi access/refresh tokenlarni "bff" belgisiga
//     almashtiradi — token brauzer xotirasiga (localStorage) HECH QACHON
//     tushmaydi, XSS uni o'g'irlay olmaydi;
//   - holat o'zgartiruvchi so'rovlarda Origin'ni tekshiradi (CSRF).
//
// Flutter klienti (`ondex_core` ApiClient) o'zgarmaydi: u "bff" tokenini
// odatdagidek saqlaydi va yuboradi, 401 da refresh so'raydi — BFF refresh'ni
// cookie sessiyasi bo'yicha o'zi javoblaydi.
// └─────────────────────────────────────────────────────────────────────────┘

const (
	panelWebPrefix     = "/r/api"
	panelWebCookieName = "__Host-ondex_panel"
	panelWebMarker     = "bff"
	panelWebBodyMax    = 64 << 10
)

// panelWebLoginPaths — javobida token bo'lishi mumkin bo'lgan kirish
// bosqichlari (tokenlar cookie'ga aylanadi).
var panelWebLoginPaths = map[string]bool{
	"/v2/auth/merchant/password":   true,
	"/v2/auth/merchant/otp/verify": true,
	"/v2/auth/mfa/password":        true,
	"/v2/auth/mfa/factor":          true,
	"/v2/auth/mfa/verify":          true,
	"/v2/auth/mfa/totp/enroll":     true,
	"/v2/auth/mfa/totp/confirm":    true,
	"/v2/auth/passkey/login":       true,
}

// panelWebClientPaths — so'rov tanasida `client_id` bo'lgan bosqichlar.
var panelWebClientPaths = map[string]bool{
	"/v2/auth/merchant/password":   true,
	"/v2/auth/merchant/otp/verify": true,
	"/v2/auth/passkey/options":     true,
	"/v2/auth/passkey/login":       true,
}

// panelWebDenied — panel uchun kerak bo'lmagan yuzalar (platforma admini,
// OIDC provayder, sherik agentlar) BFF orqali ochilmaydi.
var panelWebDenied = []string{"/v2/auth/admin/", "/v2/oauth/", "/sso/", "/agent/"}

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

// withPanelWebBFF — /r/api/* so'rovlarini BFF orqali `next` ga uzatadi,
// qolganlari o'zgarishsiz o'tadi.
func (s *Server) withPanelWebBFF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != panelWebPrefix && !strings.HasPrefix(r.URL.Path, panelWebPrefix+"/") {
			next.ServeHTTP(w, r)
			return
		}
		s.servePanelWeb(w, r, next)
	})
}

func (s *Server) servePanelWeb(w http.ResponseWriter, r *http.Request, next http.Handler) {
	w.Header().Set("Cache-Control", "no-store")
	if s.Identity == nil {
		panelWebErr(w, http.StatusServiceUnavailable, "panel kirishi sozlanmagan")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, panelWebPrefix)
	if path == "" {
		path = "/"
	}
	for _, p := range panelWebDenied {
		if strings.HasPrefix(path, p) {
			panelWebErr(w, http.StatusNotFound, "topilmadi")
			return
		}
	}
	if !s.panelWebSameOrigin(r) {
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
	if c, err := r.Cookie(panelWebCookieName); err == nil {
		cookie = c.Value
	}

	switch path {
	case "/v2/auth/token/refresh":
		s.panelWebRefresh(w, r2, cookie)
		return
	case "/v2/auth/logout", "/auth/logout":
		if cookie != "" {
			s.Identity.PanelWebLogout(r.Context(), cookie)
			s.panelWebCache.drop(sha256.Sum256([]byte(cookie)))
		}
		clearPanelWebCookie(w)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	if cookie != "" {
		if tok, ok := s.panelWebToken(r2, cookie); ok {
			r2.Header.Set("Authorization", "Bearer "+tok)
		} else {
			clearPanelWebCookie(w)
		}
	}

	if panelWebClientPaths[path] && r2.Method == http.MethodPost {
		if !forcePanelWebClient(w, r2) {
			return
		}
	}
	if panelWebLoginPaths[path] {
		s.panelWebLogin(w, r2, next)
		return
	}
	next.ServeHTTP(w, r2)
}

// panelWebSameOrigin — CSRF: holat o'zgartiruvchi so'rov faqat panelning
// o'z sahifasidan (Origin aniq mos, yoki Origin'siz bo'lsa Sec-Fetch-Site
// same-origin). GET/HEAD/OPTIONS — cookie SameSite=Strict yetarli.
func (s *Server) panelWebSameOrigin(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	if o := r.Header.Get("Origin"); o != "" {
		return o == s.panelWebOrigin()
	}
	return r.Header.Get("Sec-Fetch-Site") == "same-origin"
}

func (s *Server) panelWebOrigin() string {
	if s.PanelWebOrigin != "" {
		return s.PanelWebOrigin
	}
	return "https://merchant.ondex.uz"
}

func (s *Server) panelWebToken(r *http.Request, cookie string) (string, bool) {
	k := sha256.Sum256([]byte(cookie))
	now := time.Now()
	if t, ok := s.panelWebCache.get(k, now); ok {
		return t, true
	}
	tok, exp, err := s.Identity.PanelWebAccess(r.Context(), cookie)
	if err != nil {
		s.panelWebCache.drop(k)
		return "", false
	}
	s.panelWebCache.put(k, panelWebAccess{token: tok, exp: exp})
	return tok, true
}

// panelWebRefresh — klient 401 dan keyin refresh so'raydi: cookie sessiyasi
// tirik bo'lsa "bff" belgisi bilan 200 (keyingi so'rov yangi access oladi),
// aks holda 401 — klient login ekraniga qaytadi.
func (s *Server) panelWebRefresh(w http.ResponseWriter, r *http.Request, cookie string) {
	if cookie != "" {
		s.panelWebCache.drop(sha256.Sum256([]byte(cookie)))
		if _, ok := s.panelWebToken(r, cookie); ok {
			writeJSON(w, http.StatusOK, map[string]string{
				"access_token": panelWebMarker, "refresh_token": panelWebMarker, "token_type": "Bearer"})
			return
		}
	}
	clearPanelWebCookie(w)
	panelWebErr(w, http.StatusUnauthorized, identity.ErrRefreshInvalid.Error())
}

// forcePanelWebClient — tanadagi `client_id` ni panel klientiga almashtiradi
// (boshqa qiymat kelsa ham): BFF orqali faqat panel sessiyasi ochiladi.
func forcePanelWebClient(w http.ResponseWriter, r *http.Request) bool {
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
	body["client_id"], _ = json.Marshal(identity.PanelWebClientID)
	out, _ := json.Marshal(body)
	r.Body = io.NopCloser(bytes.NewReader(out))
	r.ContentLength = int64(len(out))
	r.Header.Set("Content-Length", strconv.Itoa(len(out)))
	return true
}

func panelWebErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// panelWebLogin — kirish bosqichi javobini ushlaydi: token bo'lsa (sessiya
// ochildi) cookie qo'yiladi va tokenlar javobdan olib tashlanadi.
func (s *Server) panelWebLogin(w http.ResponseWriter, r *http.Request, next http.Handler) {
	rec := &panelWebRecorder{h: http.Header{}, code: http.StatusOK}
	next.ServeHTTP(rec, r)

	body := rec.buf.Bytes()
	var m map[string]json.RawMessage
	if rec.code == http.StatusOK && json.Unmarshal(body, &m) == nil {
		if raw, ok := m["access_token"]; ok {
			var access string
			_ = json.Unmarshal(raw, &access)
			sid, ok := s.Identity.PanelWebSessionOf(r.Context(), access)
			if !ok {
				// Boshqa klient sessiyasi (mfa tokeni tashqaridan olingan) —
				// tokenni brauzerga bermaymiz.
				panelWebErr(w, http.StatusBadRequest, "bu kirish web panel uchun emas")
				return
			}
			setPanelWebCookie(w, s.Identity.PanelWebCookie(sid))
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

func setPanelWebCookie(w http.ResponseWriter, v string) {
	// Sessiya 7 kun (idle = abs): cookie ham shuncha.
	const age = 7 * 24 * 60 * 60
	http.SetCookie(w, &http.Cookie{Name: panelWebCookieName, Value: v, Path: "/", MaxAge: age,
		Expires: time.Now().Add(age * time.Second), Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
}

func clearPanelWebCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: panelWebCookieName, Value: "", Path: "/", MaxAge: -1,
		Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
}
