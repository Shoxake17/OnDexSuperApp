package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"time"

	"chustapp/internal/identity"
	"chustapp/internal/ratelimit"
	"chustapp/internal/users"
)

// OnDex ID akkaunt sahifasi — passport.ondex.uz (Plane 270).
//
// ┌─ XAVFSIZLIK ──────────────────────────────────────────────────────────┐
// Sahifa SSO cookie (`__Host-ondex_sso`, HttpOnly, SameSite=Lax) bilan
// ishlaydi — token brauzerga umuman chiqmaydi. Cookie bilan ishlaydigan
// HAR BIR o'zgartiruvchi so'rov uchun ikki qatlam CSRF himoyasi:
//   - `X-OnDex-Account: 1` sarlavhasi — begona sayt uni preflight'siz
//     yubora olmaydi (CORS bu yo'llarga ruxsat bermaydi);
//   - `Origin` (yoki `Sec-Fetch-Site`) aynan shu sayt bo'lishi SHART.
// Parolni almashtirish — joriy parol + Authenticator kodi (step-up), so'ng
// BARCHA sessiyalar yopiladi.
// └───────────────────────────────────────────────────────────────────────┘

var accountLimiter = ratelimit.New(1, 40) // akkaunt API: IP bo'yicha

// clientDisplayName — sessiya ro'yxatida ilova nomi.
var clientDisplayName = map[string]string{
	identity.SSOClientID: "OnDex ID (brauzer)",
	"merchant-desktop":   "OnDex Merchant (Windows)",
	"ondex-pro":          "OnDexPro",
	"ondex-go":           "OnDexGO",
}

func (s *Server) clientName(id string) string {
	if n, ok := clientDisplayName[id]; ok {
		return n
	}
	if s.OIDC != nil {
		if c, ok := s.OIDC.Client(id); ok && c.Name != "" {
			return c.Name
		}
	}
	return id
}

// accountSession — SSO cookie'dagi sessiya. Yo'q/yaroqsiz — nil.
func (s *Server) accountSession(r *http.Request) *identity.Session {
	c, err := r.Cookie(ssoCookieName)
	if err != nil {
		return nil
	}
	sso, err := s.OIDC.SSOSession(r.Context(), c.Value)
	if err != nil {
		return nil
	}
	return sso
}

// sameOriginWrite — CSRF: maxsus sarlavha + shu saytning Origin'i.
func (s *Server) sameOriginWrite(r *http.Request) bool {
	if r.Header.Get("X-OnDex-Account") != "1" {
		return false
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	iss, err := url.Parse(s.OIDC.IssuerURL())
	if err != nil {
		return false
	}
	return origin == iss.Scheme+"://"+iss.Host
}

type accountSessionView struct {
	ID         string    `json:"id"`
	App        string    `json:"app"`
	Device     string    `json:"device"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	Current    bool      `json:"current"`
}

func (s *Server) registerAccountRoutes(mux *http.ServeMux) {
	page := mustEmbed("assets/oidc_account.html")
	js := mustEmbed("assets/oidc_account.js")

	mux.HandleFunc("GET /v2/oauth/account", func(w http.ResponseWriter, r *http.Request) {
		if !s.oidcReady(w) {
			return
		}
		noStore(w)
		if s.accountSession(r) == nil {
			http.Redirect(w, r, "/v2/oauth/login?account=1", http.StatusFound)
			return
		}
		w.Header().Set("Content-Security-Policy", oidcPageCSP)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	})
	mux.HandleFunc("GET /v2/oauth/assets/account.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(js)
	})

	// guard — har bir akkaunt API'si: OIDC yoqilgan, limit, sessiya, (yozuvda) CSRF.
	guard := func(write bool, next func(w http.ResponseWriter, r *http.Request, sso *identity.Session)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !s.oidcReady(w) || s.MerchantAuth == nil {
				httpError(w, http.StatusServiceUnavailable, errors.New("akkaunt xizmati yoqilmagan"))
				return
			}
			if rateLimited(w, accountLimiter, clientIP(r)) {
				return
			}
			noStore(w)
			if write && !s.sameOriginWrite(r) {
				httpError(w, http.StatusForbidden, errors.New("so'rov manbasi tasdiqlanmadi"))
				return
			}
			sso := s.accountSession(r)
			if sso == nil {
				clearSSOCookie(w)
				httpError(w, http.StatusUnauthorized, errors.New("sessiya tugagan — qaytadan kiring"))
				return
			}
			next(w, r, sso)
		}
	}

	mux.HandleFunc("GET /v2/oauth/account/api/me", guard(false, func(w http.ResponseWriter, r *http.Request, sso *identity.Session) {
		me, err := s.MerchantAuth.Me(r.Context(), sso.UserID)
		if err != nil {
			httpError(w, http.StatusUnauthorized, errSessionGone)
			return
		}
		u, err := s.UserRepo.GetByID(r.Context(), sso.UserID)
		if err != nil {
			httpError(w, http.StatusUnauthorized, errSessionGone)
			return
		}
		totp, err := s.MerchantAuth.HasTOTP(r.Context(), sso.UserID)
		if err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		list, err := s.Identity.ListSessions(r.Context(), sso.UserID)
		if err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		sessions := []accountSessionView{}
		apps := map[string]bool{}
		for _, x := range list {
			if x.RevokedAt != nil {
				continue
			}
			sessions = append(sessions, accountSessionView{ID: x.ID, App: s.clientName(x.ClientID), Device: x.DeviceName,
				CreatedAt: x.CreatedAt, LastUsedAt: x.LastUsedAt, Current: x.ID == sso.ID || x.ParentID == sso.ID})
			if x.ClientID != identity.SSOClientID {
				apps[s.clientName(x.ClientID)] = true
			}
		}
		sort.Slice(sessions, func(i, j int) bool { return sessions[i].LastUsedAt.After(sessions[j].LastUsedAt) })
		connected := make([]string, 0, len(apps))
		for a := range apps {
			connected = append(connected, a)
		}
		sort.Strings(connected)
		out := map[string]any{
			"user": me.User, "businesses": me.Businesses, "totp_enabled": totp,
			"password_set": u.PasswordHash != "", "sessions": sessions, "connected_apps": connected,
			"passkeys_enabled": s.MerchantAuth.PasskeysEnabled(),
		}
		if s.Avatars != nil {
			if pic, err := s.Avatars.Get(r.Context(), u.ID); err == nil && pic != "" {
				out["picture"] = pic
			}
		}
		writeJSON(w, http.StatusOK, out)
	}))

	mux.HandleFunc("POST /v2/oauth/account/api/sessions/revoke", guard(true, func(w http.ResponseWriter, r *http.Request, sso *identity.Session) {
		var body struct {
			ID string `json:"id"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		if body.ID == "" || len(body.ID) > 128 {
			httpError(w, http.StatusBadRequest, errors.New("sessiya ko'rsatilmagan"))
			return
		}
		if err := s.Identity.RevokeOwn(r.Context(), sso.UserID, body.ID); err != nil {
			if errors.Is(err, identity.ErrNotFound) {
				httpError(w, http.StatusNotFound, err)
				return
			}
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		ended := body.ID == sso.ID
		if ended {
			clearSSOCookie(w)
		}
		writeJSON(w, http.StatusOK, map[string]bool{"signed_out": ended})
	}))

	mux.HandleFunc("POST /v2/oauth/account/api/logout", guard(true, func(w http.ResponseWriter, r *http.Request, sso *identity.Session) {
		_ = s.OIDC.EndSSO(r.Context(), sso, "logout")
		clearSSOCookie(w)
		writeJSON(w, http.StatusOK, map[string]bool{"signed_out": true})
	}))

	mux.HandleFunc("POST /v2/oauth/account/api/logout-all", guard(true, func(w http.ResponseWriter, r *http.Request, sso *identity.Session) {
		n, err := s.Identity.RevokeAllForUser(r.Context(), sso.UserID, "logout_all")
		if err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		clearSSOCookie(w)
		writeJSON(w, http.StatusOK, map[string]any{"signed_out": true, "sessions": n})
	}))

	// Passkey (AUD-4): ro'yxat; ulash (Authenticator kodi bilan, 2 qadam);
	// o'chirish (kod bilan). Step-up xatolari 400 (401 sahifani chiqarib yuboradi).
	respondAccountPasskey := func(w http.ResponseWriter, err error) {
		switch {
		case errors.Is(err, identity.ErrStepUpRequired):
			httpError(w, http.StatusBadRequest, err)
		case errors.Is(err, identity.ErrMFAInvalid):
			httpError(w, http.StatusBadRequest, errors.New("Authenticator kodi noto'g'ri"))
		default:
			respondPasskeyError(w, err)
		}
	}
	mux.HandleFunc("GET /v2/oauth/account/api/passkeys", guard(false, func(w http.ResponseWriter, r *http.Request, sso *identity.Session) {
		list, err := s.MerchantAuth.ListPasskeys(r.Context(), sso.UserID)
		if err != nil {
			respondAccountPasskey(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"passkeys": list})
	}))
	mux.HandleFunc("POST /v2/oauth/account/api/passkeys/options", guard(true, func(w http.ResponseWriter, r *http.Request, sso *identity.Session) {
		var body struct {
			Code string `json:"code"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		if len(body.Code) > 32 {
			httpError(w, http.StatusBadRequest, errors.New("maydon juda uzun"))
			return
		}
		opts, state, err := s.MerchantAuth.PasskeyRegisterOptions(r.Context(), sso.UserID, body.Code, merchantMeta(r, ""))
		if err != nil {
			respondAccountPasskey(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"public_key": opts.Response, "state": state})
	}))
	mux.HandleFunc("POST /v2/oauth/account/api/passkeys", guard(true, func(w http.ResponseWriter, r *http.Request, sso *identity.Session) {
		var body struct {
			State      string          `json:"state"`
			Credential json.RawMessage `json:"credential"`
			Name       string          `json:"name"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		if len(body.State) > 16<<10 || len(body.Name) > 256 || len(body.Credential) == 0 {
			httpError(w, http.StatusBadRequest, identity.ErrPasskeyState)
			return
		}
		v, err := s.MerchantAuth.PasskeyRegister(r.Context(), sso.UserID, body.State, body.Credential, body.Name, merchantMeta(r, ""))
		if err != nil {
			respondAccountPasskey(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, v)
	}))
	mux.HandleFunc("POST /v2/oauth/account/api/passkeys/delete", guard(true, func(w http.ResponseWriter, r *http.Request, sso *identity.Session) {
		var body struct {
			ID   string `json:"id"`
			Code string `json:"code"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		if body.ID == "" || len(body.ID) > 1400 || len(body.Code) > 32 {
			httpError(w, http.StatusBadRequest, errors.New("passkey ko'rsatilmagan"))
			return
		}
		if err := s.MerchantAuth.DeletePasskey(r.Context(), sso.UserID, body.ID, body.Code, merchantMeta(r, "")); err != nil {
			respondAccountPasskey(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
	}))

	mux.HandleFunc("POST /v2/oauth/account/api/password", guard(true, func(w http.ResponseWriter, r *http.Request, sso *identity.Session) {
		var body struct {
			Current  string `json:"current_password"`
			Password string `json:"password"`
			Confirm  string `json:"password_confirm"`
			Code     string `json:"code"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		if len(body.Current) > users.MaxPasswordLength || len(body.Password) > users.MaxPasswordLength ||
			len(body.Confirm) > users.MaxPasswordLength || len(body.Code) > 32 {
			httpError(w, http.StatusBadRequest, errors.New("maydon juda uzun"))
			return
		}
		err := s.MerchantAuth.ChangePassword(r.Context(), sso.UserID, body.Current, body.Password, body.Confirm,
			body.Code, merchantMeta(r, ""))
		switch {
		case err == nil:
			clearSSOCookie(w) // barcha sessiyalar yopildi
			writeJSON(w, http.StatusOK, map[string]bool{"signed_out": true})
		case errors.Is(err, identity.ErrStepUpRequired):
			httpError(w, http.StatusBadRequest, err)
		case errors.Is(err, identity.ErrMFAInvalid):
			// 401 EMAS: sahifa 401 ni "sessiya tugadi" deb kirishga qaytaradi.
			httpError(w, http.StatusBadRequest, errors.New("Authenticator kodi noto'g'ri"))
		case errors.Is(err, users.ErrCurrentPasswordWrong), errors.Is(err, users.ErrInvalidCredentials):
			httpError(w, http.StatusBadRequest, errors.New("joriy parol noto'g'ri"))
		case users.IsUserFacing(err):
			httpError(w, http.StatusBadRequest, err) // zaif parol, mos kelmaydi
		default:
			respondMerchantAuthError(w, err)
		}
	}))
}
