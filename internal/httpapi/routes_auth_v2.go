package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"chustapp/internal/identity"
	"chustapp/internal/ratelimit"
	"chustapp/internal/users"
)

// Markaziy sessiya API'si (A2, docs/architecture/AUTH-PHASE-A.md).
//
// ┌─ BU YERDA NIMA BOR ───────────────────────────────────────────────────┐
//   GET    /.well-known/jwks.json   — ochiq kalitlar (servislar tokenni o'zi tekshiradi)
//   POST   /v2/auth/token/refresh   — refresh rotatsiyasi
//   POST   /v2/auth/logout          — joriy sessiyani yopish
//   GET    /v2/auth/sessions        — mening qurilmalarim
//   DELETE /v2/auth/sessions/{id}   — boshqa qurilmani yopish
// Kirish (sessiya OCHISH) — A3 da (merchant OTP/parol/TOTP oqimlari).
// └───────────────────────────────────────────────────────────────────────┘
//
// Eski `/auth/*` va HS256 tokenlar o'zgarishsiz ishlaydi; bu marshrutlar
// FAQAT yangi (EdDSA) tokenni qabul qiladi.

// refreshIPLimiter — refresh endpointi: halol klient 10 daqiqada ~1 marta
// chaqiradi; o'g'irlangan tokenlarni ko'r-ko'rona sinashga qarshi.
var refreshIPLimiter = ratelimit.New(0.5, 30)

const claimsV2Key ctxKey = 1

// identityReady — markaziy sessiya xizmati ulanganmi.
func (s *Server) identityReady(w http.ResponseWriter) bool {
	if s.Identity == nil {
		httpError(w, http.StatusServiceUnavailable, errors.New("yangi kirish tizimi hali yoqilmagan"))
		return false
	}
	return true
}

// authV2 — EdDSA access tokenni tekshiradi VA sessiya markazda faolligini
// tasdiqlaydi. `realms` bo'sh bo'lmasa token shu realmlardan biriga
// tegishli bo'lishi shart.
func (s *Server) authV2(realms []users.Realm, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.identityReady(w) {
			return
		}
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			httpError(w, http.StatusUnauthorized, errors.New("Authorization: Bearer <token> talab qilinadi"))
			return
		}
		c, err := s.Identity.Authenticate(r.Context(), strings.TrimPrefix(h, "Bearer "), realms...)
		if err != nil {
			if errors.Is(err, identity.ErrTokenInvalid) {
				httpError(w, http.StatusUnauthorized, errors.New("sessiya tugagan — qaytadan kiring"))
				return
			}
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), claimsV2Key, c)))
	}
}

func claimsV2From(r *http.Request) *identity.AccessClaims {
	c, _ := r.Context().Value(claimsV2Key).(*identity.AccessClaims)
	return c
}

// noStore — token javoblari hech qayerda keshlanmasin (RFC 6749 5.1).
func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
}

type sessionView struct {
	ID         string    `json:"id"`
	ClientID   string    `json:"client_id"`
	DeviceName string    `json:"device_name,omitempty"`
	UserAgent  string    `json:"user_agent,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	Current    bool      `json:"current"`
}

func (s *Server) registerAuthV2Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		if !s.identityReady(w) {
			return
		}
		body, err := s.Identity.Issuer().Keys().JWKS()
		if err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		// Ochiq ma'lumot, lekin kalit rotatsiyasi tez yetib borishi uchun qisqa kesh.
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Header().Set("Content-Type", "application/jwk-set+json")
		_, _ = w.Write(body)
	})

	mux.HandleFunc("POST /v2/auth/token/refresh", func(w http.ResponseWriter, r *http.Request) {
		if !s.identityReady(w) {
			return
		}
		if rateLimited(w, refreshIPLimiter, clientIP(r)) {
			return
		}
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		noStore(w)
		tok, err := s.Identity.Refresh(r.Context(), req.RefreshToken,
			identity.Meta{IP: clientIP(r), UserAgent: r.UserAgent()})
		switch {
		case err == nil:
			writeJSON(w, http.StatusOK, tok)
		case errors.Is(err, identity.ErrRefreshInvalid), errors.Is(err, identity.ErrRefreshReuse):
			httpError(w, http.StatusUnauthorized, err)
		default:
			httpError(w, http.StatusInternalServerError, err)
		}
	})

	mux.HandleFunc("POST /v2/auth/logout", s.authV2(nil, func(w http.ResponseWriter, r *http.Request) {
		if err := s.Identity.Logout(r.Context(), claimsV2From(r)); err != nil && !errors.Is(err, identity.ErrNotFound) {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	mux.HandleFunc("GET /v2/auth/sessions", s.authV2(nil, func(w http.ResponseWriter, r *http.Request) {
		c := claimsV2From(r)
		list, err := s.Identity.ListSessions(r.Context(), c.Subject)
		if err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		out := make([]sessionView, 0, len(list))
		for _, x := range list {
			out = append(out, sessionView{ID: x.ID, ClientID: x.ClientID, DeviceName: x.DeviceName,
				UserAgent: x.UserAgent, CreatedAt: x.CreatedAt, LastUsedAt: x.LastUsedAt, Current: x.ID == c.SessionID})
		}
		writeJSON(w, http.StatusOK, map[string]any{"sessions": out})
	}))

	mux.HandleFunc("DELETE /v2/auth/sessions/{id}", s.authV2(nil, func(w http.ResponseWriter, r *http.Request) {
		c := claimsV2From(r)
		err := s.Identity.RevokeOwn(r.Context(), c.Subject, r.PathValue("id"))
		switch {
		case err == nil:
			w.WriteHeader(http.StatusNoContent)
		case errors.Is(err, identity.ErrNotFound):
			// Begona yoki mavjud bo'lmagan sessiya — bir xil javob.
			httpError(w, http.StatusNotFound, errors.New("sessiya topilmadi"))
		default:
			httpError(w, http.StatusInternalServerError, err)
		}
	}))
}
