package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"chustapp/internal/ratelimit"
	"chustapp/internal/sso"
)

// ssoTokenLimiter — `/sso/identity`: chaqiruvchi klient SERVERI (bitta IP),
// chegara kirish tezligidan yuqori, lekin sirni tanlashga yo'l qo'ymaydi.
var ssoTokenLimiter = ratelimit.New(2, 30)

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

// Eski (OIDC'dan oldingi) yagona kirish — FAQAT `/sso/identity` qoldi.
//
// ┌─ OLIB TASHLANDI (SSO xavfsizlik auditi, 2026-11-10) ──────────────────┐
// `/sso/client`, `/sso/authorize`, `/sso/token` — eski HS256 sessiya bilan
// bir martalik kod beradigan brauzer oqimi edi. Merchant web (ShoxPos)
// OnDex OIDC'ga (`/v2/oauth/*`: PKCE, nonce, aniq redirect_uri, SSO cookie,
// back-channel logout) to'liq o'tgach bu yo'lni hech kim ishlatmaydi —
// ishlatilmaydigan kirish yo'li faqat ortiqcha hujum yuzasi. Qolgan
// `/sso/identity` — klient SERVERI siri bilan kimlikni qayta o'qish
// (ShoxPos `merchantAccount.resolveUser`).
// └───────────────────────────────────────────────────────────────────────┘
func (s *Server) registerSSORoutes(mux *http.ServeMux) {	// POST /sso/identity {"client_id","client_secret","user_id"} — klient SERVERI
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
