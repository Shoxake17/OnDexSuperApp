package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"chustapp/internal/ratelimit"
	"chustapp/internal/sso"
)

// SSO limitlari. Autentifikatsiyadan o'tgan endpoint (`/sso/authorize`)
// foydalanuvchi bo'yicha, ochiq ikkitasi IP bo'yicha cheklanadi.
//
// `/sso/token` ni chaqiruvchi — klient SERVERI (bitta IP), shuning uchun
// uning chegarasi umumiy: kirish tezligidan ancha yuqori, lekin sirni
// tanlashga yo'l qo'ymaydigan darajada.
var (
	ssoClientLimiter    = ratelimit.New(1, 30)
	ssoAuthorizeLimiter = ratelimit.New(0.5, 10)
	ssoTokenLimiter     = ratelimit.New(2, 30)
)

// ssoReady — SSO sozlanganmi. Sozlanmagan bo'lsa javobni o'zi yozadi.
func (s *Server) ssoReady(w http.ResponseWriter) bool {
	if s.SSO == nil {
		httpError(w, http.StatusServiceUnavailable,
			errors.New("yagona kirish (SSO) sozlanmagan"))
		return false
	}
	return true
}

// ssoStatus — SSO xatosining HTTP holati. Qaysi qism noto'g'ri ekani
// ataylab oshkor qilinmaydi (`sso` paketi izohiga qarang).
func ssoStatus(err error) int {
	switch {
	case errors.Is(err, sso.ErrInvalidRequest), errors.Is(err, sso.ErrInvalidGrant):
		return http.StatusBadRequest
	case errors.Is(err, sso.ErrClientAuth):
		return http.StatusUnauthorized
	case errors.Is(err, sso.ErrBusy):
		return http.StatusServiceUnavailable
	}
	return http.StatusInternalServerError
}

// OnDex yagona kirish manbai: boshqa xizmatlar (OnDex Merchant Panel)
// foydalanuvchini OnDex'ga yuboradi, OnDex bir martalik kod bilan qaytaradi.
//
// Oqim:
//  1. Xizmat brauzerni OnDex veb-saytidagi `/sso/authorize` sahifasiga yuboradi.
//  2. Foydalanuvchi kirgan (yoki kiradi) va qaysi ilovaga ruxsat
//     berayotganini KO'RADI (`GET /sso/client`).
//  3. U "Ruxsat berish" ni bosadi -> `POST /sso/authorize` -> kod bilan manzil.
//  4. Xizmat serveri kodni `POST /sso/token` bilan kimlikka almashtiradi.
func (s *Server) registerSSORoutes(mux *http.ServeMux) {
	// GET /sso/client?client_id=...&redirect_uri=... — tasdiqlash sahifasi
	// uchun: ilova nomi va qaytish manzili domeni. OCHIQ (foydalanuvchi
	// hali kirmagan bo'lishi mumkin), lekin faqat ro'yxatdagi JUFTLIK uchun
	// javob beradi.
	mux.HandleFunc("GET /sso/client", func(w http.ResponseWriter, r *http.Request) {
		if !s.ssoReady(w) {
			return
		}
		if rateLimited(w, ssoClientLimiter, clientIP(r)) {
			return
		}
		name, host, err := s.SSO.ClientInfo(
			r.URL.Query().Get("client_id"), r.URL.Query().Get("redirect_uri"))
		if err != nil {
			httpError(w, ssoStatus(err), err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"name": name, "redirect_host": host})
	})

	// POST /sso/authorize {"client_id","redirect_uri","state"} — kirgan
	// foydalanuvchi ruxsat berdi. Qaytish manzili (kod bilan) qaytariladi.
	//
	// Istalgan kirgan rol chaqira oladi: bu yerda faqat KIM EKANI
	// tasdiqlanadi. Klient (masalan Merchant Panel) o'zi hal qiladi,
	// shu odamga ruxsat beriladimi yoki yo'q.
	mux.HandleFunc("POST /sso/authorize", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			if !s.ssoReady(w) {
				return
			}
			userID := claimsFrom(r).Subject
			if rateLimited(w, ssoAuthorizeLimiter, userID) {
				return
			}
			var req struct {
				ClientID    string `json:"client_id"`
				RedirectURI string `json:"redirect_uri"`
				State       string `json:"state"`
			}
			if !decodeJSON(w, r, &req) {
				return
			}
			location, err := s.SSO.Authorize(userID, req.ClientID, req.RedirectURI, req.State)
			if err != nil {
				httpError(w, ssoStatus(err), err)
				return
			}
			// Kodning O'ZI logga yozilmaydi.
			slog.Info("sso: kod berildi", "user", userID, "client", req.ClientID)
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusOK, map[string]any{"redirect_to": location})
		}))

	// POST /sso/token {"client_id","client_secret","code","redirect_uri"} —
	// klient SERVERI kodni kimlikka almashtiradi. JWT talab qilinmaydi:
	// klient o'zining SIRI bilan tasdiqlanadi. Brauzerdan chaqirilmaydi.
	mux.HandleFunc("POST /sso/token", func(w http.ResponseWriter, r *http.Request) {
		if !s.ssoReady(w) {
			return
		}
		if rateLimited(w, ssoTokenLimiter, clientIP(r)) {
			return
		}
		var req struct {
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
			Code         string `json:"code"`
			RedirectURI  string `json:"redirect_uri"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		id, err := s.SSO.Exchange(r.Context(), req.ClientID, req.ClientSecret, req.Code, req.RedirectURI)
		if err != nil {
			if ssoStatus(err) != http.StatusInternalServerError {
				// Sabab loga yoziladi, javobda esa YO'Q (sir/kod oshkor bo'lmasin).
				slog.Warn("sso: almashtirish rad etildi",
					"client", req.ClientID, "ip", clientIP(r), "err", err)
			}
			httpError(w, ssoStatus(err), err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		writeJSON(w, http.StatusOK, id)
	})

	// POST /sso/identity {"client_id","client_secret","user_id"} — klient SERVERI
	// foydalanuvchi kimligini KODSIZ qayta o'qiydi (kategoriya o'zgarishi
	// sessiya davomida ham yetib borishi uchun). Klient SIRI bilan tasdiqlanadi.
	// Brauzerdan chaqirilmaydi.
	mux.HandleFunc("POST /sso/identity", func(w http.ResponseWriter, r *http.Request) {
		if !s.ssoReady(w) {
			return
		}
		if rateLimited(w, ssoTokenLimiter, clientIP(r)) {
			return
		}
		var req struct {
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
			UserID       string `json:"user_id"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		id, err := s.SSO.Lookup(r.Context(), req.ClientID, req.ClientSecret, req.UserID)
		if err != nil {
			if ssoStatus(err) != http.StatusInternalServerError {
				slog.Warn("sso: identity rad etildi", "client", req.ClientID, "ip", clientIP(r), "err", err)
			}
			httpError(w, ssoStatus(err), err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		writeJSON(w, http.StatusOK, id)
	})
}
