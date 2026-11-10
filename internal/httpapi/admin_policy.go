package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"chustapp/internal/identity"
)

// Amal siyosati (CC-0.4): xavfli admin amali = ruxsat (role=admin, `s.auth`
// ichida) + SABAB + qayta tasdiq (Authenticator yoki tiklash kodi; admin
// TOTP ulagan bo'lsa) + audit. Klient (admin paneli) server talab qilganda
// oyna ochib, so'rovni sarlavhalar bilan qayta yuboradi:
//
//	X-OnDex-Reason: <sabab>        (UTF-8, URL-kodlangan)
//	X-OnDex-StepUp: <kod>
//
// Javoblar: 428 {reason_required} | 428 {step_up_required} (RFC 6585 —
// rol rad etilishi 403 bilan aralashmasin) | 403 {step_up_invalid} | 429 —
// hech biri 401 EMAS (klient 401 ni "sessiya
// tugadi" deb tushunardi).

const (
	reasonHeader = "X-OnDex-Reason"
	stepUpHeader = "X-OnDex-StepUp"
)

// adminAction — xavfli admin amali o'rami. `s.auth([]users.Role{admin}, ...)`
// ICHIDA ishlatiladi (claims tayyor bo'ladi).
func (s *Server) adminAction(action string, needReason bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := claimsFrom(r)
		reason := strings.TrimSpace(decodeHeaderValue(r.Header.Get(reasonHeader)))
		if utf8.RuneCountInString(reason) > 500 {
			httpError(w, http.StatusBadRequest, errors.New("sabab juda uzun"))
			return
		}
		if needReason && utf8.RuneCountInString(reason) < 3 {
			writeJSON(w, http.StatusPreconditionRequired, map[string]any{"error": "bu amal uchun sabab yozing", "reason_required": true})
			return
		}
		meta := identity.Meta{IP: clientIP(r), UserAgent: r.UserAgent()}
		if s.MerchantAuth != nil {
			code := strings.TrimSpace(r.Header.Get(stepUpHeader))
			if len(code) > 32 {
				code = ""
			}
			if err := s.MerchantAuth.StepUp(r.Context(), c.Subject, code, "admin."+action, meta); err != nil {
				s.MerchantAuth.RecordAdminAction(r.Context(), c.Subject, action, r.URL.Path, reason, false, meta)
				switch {
				case errors.Is(err, identity.ErrStepUpRequired):
					writeJSON(w, http.StatusPreconditionRequired, map[string]any{"error": err.Error(), "step_up_required": true})
				case errors.Is(err, identity.ErrMFAInvalid):
					writeJSON(w, http.StatusForbidden, map[string]any{"error": "Authenticator kodi noto'g'ri", "step_up_invalid": true})
				case errors.Is(err, identity.ErrMFATooMany):
					tooManyRequests(w, mfaRetryAfter)
				default:
					httpError(w, http.StatusInternalServerError, err)
				}
				return
			}
			s.MerchantAuth.RecordAdminAction(r.Context(), c.Subject, action, r.URL.Path, reason, true, meta)
		}
		next(w, r)
	}
}

// decodeHeaderValue — sabab URL-kodlangan keladi (sarlavhada UTF-8 xavfsiz
// bo'lishi uchun); kodlanmagan bo'lsa o'zi qaytadi.
func decodeHeaderValue(v string) string {
	if d, err := url.QueryUnescape(v); err == nil {
		return d
	}
	return v
}
