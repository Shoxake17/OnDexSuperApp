package httpapi

import (
	"embed"
	"encoding/base64"
	"errors"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"

	"rsc.io/qr"

	"chustapp/internal/identity"
	"chustapp/internal/identity/authz"
	"chustapp/internal/ratelimit"
	"chustapp/internal/users"
)

// OnDex OIDC provayderi — HTTP qatlami (A4c, Plane ONDEXECOSY-199/200).
// Mantiq `identity.Provider` da; bu yerda faqat protokol va cookie.
//
// ┌─ KIRISH SAHIFASI NEGA API SERVERIDA ──────────────────────────────────┐
// Parol, Telegram kodi va TOTP faqat OnDex'ga (`api.ondex.uz`) yuboriladi —
// hech qaysi ilova (ShoxPos ham) ularni ko'rmaydi va proksi qilmaydi
// (AUTH-PHASE-A.md 0-bo'lim). Sahifa shu domendan beriladi, shuning uchun
// SSO cookie (`__Host-ondex_sso`) ham faqat shu yerda: boshqa ilova uni
// o'qiy olmaydi, JavaScript ham (HttpOnly).
// └───────────────────────────────────────────────────────────────────────┘

//go:embed assets/oidc_login.html assets/oidc_login.js assets/oidc_login.css assets/oidc_bg.jpg assets/oidc_logo.png assets/oidc_account.html assets/oidc_account.js
var oidcLoginFS embed.FS

// ssoCookieName — `__Host-`: faqat Secure, Path=/, Domain'siz (subdomen
// o'rnatolmaydi, boshqa subdomenga ketmaydi).
const ssoCookieName = "__Host-ondex_sso"

var (
	oidcTokenLimiter     = ratelimit.New(1, 30)  // /token: IP bo'yicha
	oidcAuthorizeLimiter = ratelimit.New(2, 60)  // /authorize va /login/complete
	oidcUserinfoLimiter  = ratelimit.New(5, 100) // /userinfo
)

func (s *Server) oidcReady(w http.ResponseWriter) bool {
	if s.OIDC == nil || s.Identity == nil {
		httpError(w, http.StatusServiceUnavailable, errors.New("OnDex orqali kirish (OIDC) hali yoqilmagan"))
		return false
	}
	return true
}

// oidcPageCSP — kirish sahifasi: faqat o'z skripti/uslubi, QR — data: rasm.
const oidcPageCSP = "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; " +
	"connect-src 'self'; form-action 'none'; frame-ancestors 'none'; base-uri 'none'"

func (s *Server) registerOIDCRoutes(mux *http.ServeMux) {
	page := mustEmbed("assets/oidc_login.html")
	js := mustEmbed("assets/oidc_login.js")
	css := mustEmbed("assets/oidc_login.css")
	bg := mustEmbed("assets/oidc_bg.jpg")
	logo := mustEmbed("assets/oidc_logo.png")

	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		if !s.oidcReady(w) {
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=3600")
		writeJSON(w, http.StatusOK, s.OIDC.Discovery())
	})

	mux.HandleFunc("GET /v2/oauth/authorize", func(w http.ResponseWriter, r *http.Request) {
		if !s.oidcReady(w) || rateLimited(w, oidcAuthorizeLimiter, clientIP(r)) {
			return
		}
		noStore(w)
		req, redirectable, err := s.OIDC.ParseAuthorize(r.URL.Query())
		if err != nil {
			var oe *identity.OAuthError
			if redirectable && errors.As(err, &oe) {
				// redirect_uri ro'yxatdagi bilan ANIQ teng (ParseAuthorize) — ochiq yo'naltirgich emas.
				http.Redirect(w, r, s.OIDC.ErrorRedirect(req, oe), http.StatusFound) //nolint:gosec // G710: manzil oq ro'yxatdan
				return
			}
			oidcErrorPage(w, err)
			return
		}
		var sso *identity.Session
		if c, err := r.Cookie(ssoCookieName); err == nil {
			sso, _ = s.OIDC.SSOSession(r.Context(), c.Value)
		}
		if !s.OIDC.NeedsLogin(req, sso) {
			loc, err := s.OIDC.IssueCode(r.Context(), req, sso)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			http.Redirect(w, r, loc, http.StatusFound) //nolint:gosec // G710: redirect_uri oq ro'yxatdan (ParseAuthorize)
			return
		}
		if req.Prompt == "none" {
			//nolint:gosec // G710: redirect_uri oq ro'yxatdan (ParseAuthorize)
			http.Redirect(w, r, s.OIDC.ErrorRedirect(req,
				&identity.OAuthError{Code: "login_required", Description: "kirish talab qilinadi"}), http.StatusFound)
			return
		}
		signed, err := s.OIDC.SignRequest(req)
		if err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		http.Redirect(w, r, "/v2/oauth/login?req="+url.QueryEscape(signed), http.StatusFound)
	})

	// Kirish sahifasi (statik; so'rov imzosi `info` va `complete` da tekshiriladi).
	mux.HandleFunc("GET /v2/oauth/login", func(w http.ResponseWriter, r *http.Request) {
		if !s.oidcReady(w) {
			return
		}
		noStore(w)
		w.Header().Set("Content-Security-Policy", oidcPageCSP)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	})
	asset := func(body []byte, ctype string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", ctype)
			w.Header().Set("Cache-Control", "no-cache")
			_, _ = w.Write(body)
		}
	}
	mux.HandleFunc("GET /v2/oauth/assets/login.js", asset(js, "text/javascript; charset=utf-8"))
	mux.HandleFunc("GET /v2/oauth/assets/login.css", asset(css, "text/css; charset=utf-8"))
	// Rasmlar o'zgarmaydi (embed) — uzoq kesh; yangilanganda fayl nomi o'zgartiriladi.
	image := func(body []byte, ctype string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", ctype)
			w.Header().Set("Cache-Control", "public, max-age=86400")
			_, _ = w.Write(body)
		}
	}
	mux.HandleFunc("GET /v2/oauth/assets/bg.jpg", image(bg, "image/jpeg"))
	mux.HandleFunc("GET /v2/oauth/assets/logo.png", image(logo, "image/png"))

	// info — sahifada "<ilova> ga kirish" deb ko'rsatish uchun.
	mux.HandleFunc("GET /v2/oauth/login/info", func(w http.ResponseWriter, r *http.Request) {
		if !s.oidcReady(w) {
			return
		}
		noStore(w)
		raw := r.URL.Query().Get("req")
		req, err := s.OIDC.OpenRequest(raw)
		if err != nil {
			// Eskirgan (imzosi to'g'ri) so'rov — sahifa ilovadan qayta boshlaydi.
			if u := s.OIDC.RestartURL(raw); u != "" {
				writeJSON(w, http.StatusConflict, map[string]string{"error": "request_expired", "restart_url": u})
				return
			}
			writeOAuthJSONError(w, err)
			return
		}
		c, _ := s.OIDC.Client(req.ClientID)
		writeJSON(w, http.StatusOK, map[string]any{"client_name": c.Name, "sso_client_id": identity.SSOClientID})
	})

	// complete — sahifa `ondex-sso` sessiyasini ochdi (merchant kirish
	// oqimi, client_id=ondex-sso); uning access tokeni bilan keladi. Shu
	// sessiya SSO cookie'ga aylanadi va ilovaga kod beriladi. Bearer
	// sarlavhasi — cookie emas, shuning uchun CSRF imkonsiz.
	mux.HandleFunc("POST /v2/oauth/login/complete", func(w http.ResponseWriter, r *http.Request) {
		if !s.oidcReady(w) || rateLimited(w, oidcAuthorizeLimiter, clientIP(r)) {
			return
		}
		noStore(w)
		var body struct {
			Req     string `json:"req"`
			Account bool   `json:"account"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		// Akkaunt sahifasiga kirish (passport.ondex.uz): ilova yo'q, kod
		// berilmaydi — faqat SSO cookie va o'z sahifamizga qaytish.
		if body.Account && body.Req == "" {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				httpError(w, http.StatusUnauthorized, identity.ErrTokenInvalid)
				return
			}
			sso, err := s.OIDC.SSOSessionFromAccess(r.Context(), strings.TrimPrefix(h, "Bearer "))
			if err != nil {
				httpError(w, http.StatusUnauthorized, identity.ErrTokenInvalid)
				return
			}
			if c, cerr := r.Cookie(ssoCookieName); cerr == nil {
				if old, oerr := s.OIDC.SSOSession(r.Context(), c.Value); oerr == nil && old.ID != sso.ID {
					_ = s.OIDC.EndSSO(r.Context(), old, "sso_replaced")
				}
			}
			setSSOCookie(w, s.OIDC.SSOCookie(sso.ID), sso.AbsExpiresAt)
			writeJSON(w, http.StatusOK, map[string]string{"redirect_to": "/v2/oauth/account"})
			return
		}
		req, err := s.OIDC.OpenRequest(body.Req)
		if err != nil {
			// Kirish shu yerda TUGAGAN (masalan TOTP ulashga 15 daqiqadan ko'p ketdi),
			// so'rov esa eskirgan: SSO cookie baribir qo'yiladi va ilova qaytadan
			// boshlaydi — SSO tufayli darhol kiradi, foydalanuvchi qayta kiritmaydi.
			restart := s.OIDC.RestartURL(body.Req)
			h := r.Header.Get("Authorization")
			if restart == "" || !strings.HasPrefix(h, "Bearer ") {
				writeOAuthJSONError(w, err)
				return
			}
			sso, aerr := s.OIDC.SSOSessionFromAccess(r.Context(), strings.TrimPrefix(h, "Bearer "))
			if aerr != nil {
				httpError(w, http.StatusUnauthorized, identity.ErrTokenInvalid)
				return
			}
			setSSOCookie(w, s.OIDC.SSOCookie(sso.ID), sso.AbsExpiresAt)
			writeJSON(w, http.StatusOK, map[string]string{"redirect_to": restart})
			return
		}
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			httpError(w, http.StatusUnauthorized, identity.ErrTokenInvalid)
			return
		}
		sso, err := s.OIDC.SSOSessionFromAccess(r.Context(), strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			httpError(w, http.StatusUnauthorized, identity.ErrTokenInvalid)
			return
		}
		// Shu brauzerdagi OLDINGI SSO sessiyasi (boshqa akkaunt yoki
		// prompt=login) — yopiladi, aks holda yetim sessiya qolardi.
		if c, cerr := r.Cookie(ssoCookieName); cerr == nil {
			if old, oerr := s.OIDC.SSOSession(r.Context(), c.Value); oerr == nil && old.ID != sso.ID {
				_ = s.OIDC.EndSSO(r.Context(), old, "sso_replaced")
			}
		}
		setSSOCookie(w, s.OIDC.SSOCookie(sso.ID), sso.AbsExpiresAt)
		loc, err := s.OIDC.IssueCode(r.Context(), req, sso)
		if err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"redirect_to": loc})
	})

	mux.HandleFunc("POST /v2/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		if !s.oidcReady(w) || rateLimited(w, oidcTokenLimiter, clientIP(r)) {
			return
		}
		noStore(w)
		w.Header().Set("Pragma", "no-cache")
		r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
		if err := r.ParseForm(); err != nil {
			writeOAuthJSONError(w, &identity.OAuthError{Code: "invalid_request", Description: "forma o'qilmadi", Status: 400})
			return
		}
		f := r.PostForm
		clientID, secret, basic := r.BasicAuth()
		if basic {
			clientID, _ = url.QueryUnescape(clientID)
			secret, _ = url.QueryUnescape(secret)
		} else {
			clientID, secret = f.Get("client_id"), f.Get("client_secret")
		}
		meta := identity.Meta{IP: clientIP(r), UserAgent: r.UserAgent(), DeviceName: "OnDex Merchant (web)"}
		var (
			out *identity.TokenResponse
			err error
		)
		switch f.Get("grant_type") {
		case "authorization_code":
			out, err = s.OIDC.ExchangeCode(r.Context(), clientID, secret, f.Get("code"), f.Get("redirect_uri"),
				f.Get("code_verifier"), meta)
		case "refresh_token":
			out, err = s.OIDC.RefreshGrant(r.Context(), clientID, secret, f.Get("refresh_token"), meta)
		default:
			err = &identity.OAuthError{Code: "unsupported_grant_type", Description: "grant_type qo'llab-quvvatlanmaydi", Status: 400}
		}
		if err != nil {
			if basic {
				w.Header().Set("WWW-Authenticate", `Basic realm="ondex"`)
			}
			writeOAuthJSONError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})

	userinfo := func(w http.ResponseWriter, r *http.Request) {
		if !s.oidcReady(w) || rateLimited(w, oidcUserinfoLimiter, clientIP(r)) {
			return
		}
		noStore(w)
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
			httpError(w, http.StatusUnauthorized, identity.ErrTokenInvalid)
			return
		}
		c, err := s.Identity.Authenticate(r.Context(), strings.TrimPrefix(h, "Bearer "), users.RealmMerchant)
		if err != nil {
			w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
			httpError(w, http.StatusUnauthorized, identity.ErrTokenInvalid)
			return
		}
		info, err := s.oidcUserinfo(r, c)
		if err != nil {
			httpError(w, http.StatusUnauthorized, identity.ErrTokenInvalid)
			return
		}
		writeJSON(w, http.StatusOK, info)
	}
	mux.HandleFunc("GET /v2/oauth/userinfo", userinfo)
	mux.HandleFunc("POST /v2/oauth/userinfo", userinfo)

	// end_session (OIDC RP-Initiated Logout 1.0): SSO sessiyasi + undan
	// ochilgan barcha ilova sessiyalari yopiladi (har biriga back-channel
	// logout ketadi), cookie o'chadi. Qaytish faqat ro'yxatdagi manzilga.
	logout := func(w http.ResponseWriter, r *http.Request) {
		if !s.oidcReady(w) || rateLimited(w, oidcAuthorizeLimiter, clientIP(r)) {
			return
		}
		noStore(w)
		if r.Method == http.MethodPost {
			r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
			_ = r.ParseForm()
		}
		q := r.Form
		if r.Method == http.MethodGet {
			q = r.URL.Query()
		}
		var hint *identity.Hint
		if raw := q.Get("id_token_hint"); raw != "" {
			hint, _ = s.OIDC.ParseIDTokenHint(raw)
		}
		var sso *identity.Session
		if c, err := r.Cookie(ssoCookieName); err == nil {
			sso, _ = s.OIDC.SSOSession(r.Context(), c.Value)
		}
		// Logout CSRF: begona sahifa foydalanuvchini chiqarib yubormasin.
		// GET (havola, <img>) — faqat imzolangan id_token_hint SHU brauzer
		// sessiyasi egasiniki bo'lsa darhol; aks holda tasdiqlash sahifasi.
		// Hint egasi tekshirilmasa, hujumchi O'ZINING (muddati o'tgan ham)
		// id_token'i bilan qurbonni chiqarib yuborardi (OIDC RP-Initiated
		// Logout 2: hint joriy foydalanuvchiga mos kelmasa — so'rash).
		// POST'ni SameSite=Lax cookie himoya qiladi (begona saytdan kelgan
		// POST'ga SSO cookie qo'shilmaydi).
		if r.Method == http.MethodGet && (hint == nil || (sso != nil && sso.UserID != hint.UserID)) {
			logoutConfirmPage(w, q)
			return
		}
		if sso != nil {
			_ = s.OIDC.EndSSO(r.Context(), sso, "logout")
		}
		s.OIDC.EndClientSession(r.Context(), hint)
		clearSSOCookie(w)

		clientID := q.Get("client_id")
		if hint != nil {
			clientID = hint.ClientID
		}
		if loc := s.OIDC.PostLogoutRedirect(clientID, q.Get("post_logout_redirect_uri"), q.Get("state")); loc != "" {
			http.Redirect(w, r, loc, http.StatusFound) //nolint:gosec // G710: faqat klientning ro'yxatdagi manzili
			return
		}
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">` +
			`<title>OnDex — chiqish</title><body style="font-family:system-ui;max-width:480px;margin:15vh auto;padding:0 16px">` +
			`<h1 style="font-size:20px">Siz OnDex'dan chiqdingiz</h1><p>Bu brauzerdagi barcha OnDex ilovalari sessiyasi yopildi.</p></body>`))
	}
	mux.HandleFunc("GET /v2/oauth/logout", logout)
	mux.HandleFunc("POST /v2/oauth/logout", logout)
}

// oidcUserinfo — standart da'volar + OnDex bizneslari (a'zolik bo'yicha).
// Rol/ruxsat tokenga yozilmaydi; ilova har kirishda shu yerdan oladi.
func (s *Server) oidcUserinfo(r *http.Request, c *identity.AccessClaims) (map[string]any, error) {
	u, err := s.UserRepo.GetByID(r.Context(), c.Subject)
	if err != nil || u.IsDeleted() {
		return nil, identity.ErrTokenInvalid
	}
	out := map[string]any{
		"sub": u.ID, "name": u.Name, "phone_number": u.Phone, "phone_number_verified": u.PhoneVerified,
		"sid": c.SessionID, "org": c.OrgID,
	}
	// Eski hamkor akkaunti (rol restaurant/store, A1 dan oldingi model):
	// ShoxPos `resolveUser` shu maydonlarga tayanadi. To'g'ridan-to'g'ri
	// akkauntdan quriladi — eski `internal/sso` sozlanganiga BOG'LIQ EMAS
	// (2026-11-08 ikki serverli sinovda topilgan: SSO o'chiq bo'lsa hamkor
	// ma'lumoti kelmay, kirish `not_linked` bo'lib qolardi).
	if s.Avatars != nil {
		if pic, err := s.Avatars.Get(r.Context(), u.ID); err == nil && pic != "" {
			out["picture"] = pic // OIDC standart da'vosi (barcha ilovalar bir xil rasmni ko'radi)
		}
	}
	if p := s.partnerOf(r, u); p != nil {
		out["partner"] = p
	}
	if s.Authz != nil {
		list, err := s.Authz.List(r.Context(), u.ID)
		if err != nil {
			return nil, err
		}
		bs := make([]map[string]any, 0, len(list))
		for _, a := range list {
			if !a.AllowsApp(authz.AppPanel) {
				continue
			}
			bs = append(bs, map[string]any{"id": a.Business.ID, "type": a.Business.Type, "name": a.Business.Name,
				"legacy_entity_id": a.Business.LegacyEntityID, "is_owner": a.IsOwner, "role": a.RoleKey,
				"org_status": a.OrgStatus, "permissions": a.Permissions()})
		}
		out["businesses"] = bs
	}
	return out, nil
}

// partnerOf — restoran/do'kon hamkor akkaunti ma'lumoti (`internal/sso`
// Identity bilan bir xil qoidalar), aks holda nil.
func (s *Server) partnerOf(r *http.Request, u *users.User) map[string]any {
	if (u.Role != users.RoleRestaurant && u.Role != users.RoleStore) || u.EntityID == "" {
		return nil
	}
	p := map[string]any{"role": string(u.Role), "entity_id": u.EntityID, "entity_name": u.Name, "store_category": ""}
	switch u.Role {
	case users.RoleStore:
		// Kategoriya yozilmagan (eski) do'kon — oddiy do'kon; cheklangan panel
		// faqat ANIQ "qurt" qiymatida.
		cat := string(u.StoreCategory)
		if cat == "" {
			cat = string(users.StoreCategoryStore)
		}
		p["store_category"] = cat
	case users.RoleRestaurant:
		if s.CatalogRepo != nil {
			if rest, err := s.CatalogRepo.GetRestaurant(r.Context(), u.EntityID); err == nil && rest != nil && rest.Name != "" {
				p["entity_name"] = rest.Name
			}
		}
	}
	return p
}

func setSSOCookie(w http.ResponseWriter, value string, until time.Time) {
	http.SetCookie(w, &http.Cookie{Name: ssoCookieName, Value: value, Path: "/", Expires: until,
		MaxAge: int(time.Until(until).Seconds()), Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

func clearSSOCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: ssoCookieName, Value: "", Path: "/", MaxAge: -1,
		Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

// writeOAuthJSONError — RFC 6749 5.2 format.
func writeOAuthJSONError(w http.ResponseWriter, err error) {
	var oe *identity.OAuthError
	if !errors.As(err, &oe) {
		oe = &identity.OAuthError{Code: "server_error", Description: "ichki xato", Status: 500}
	}
	st := oe.Status
	if st == 0 {
		st = 400
	}
	writeJSON(w, st, map[string]string{"error": oe.Code, "error_description": oe.Description})
}

// logoutConfirmPage — chiqishni foydalanuvchi o'zi tasdiqlaydi (POST, SSO
// cookie SameSite=Lax). Klient parametrlari forma ichida saqlanadi: tasdiqdan
// keyin ilova sessiyasi ham yopiladi va ro'yxatdagi manzilga qaytiladi.
func logoutConfirmPage(w http.ResponseWriter, q url.Values) {
	var hidden strings.Builder
	for _, k := range []string{"id_token_hint", "client_id", "post_logout_redirect_uri", "state"} {
		if v := q.Get(k); v != "" && len(v) <= 4096 {
			hidden.WriteString(`<input type="hidden" name="` + k + `" value="` + html.EscapeString(v) + `">`)
		}
	}
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">` +
		`<title>OnDex — chiqish</title><body style="font-family:system-ui;max-width:480px;margin:15vh auto;padding:0 16px">` +
		`<h1 style="font-size:20px">OnDex'dan chiqasizmi?</h1><p>Bu brauzerdagi barcha OnDex ilovalari sessiyasi yopiladi.</p>` +
		`<form method="post" action="/v2/oauth/logout">` + hidden.String() +
		`<button style="padding:10px 18px;font:inherit">Chiqish</button></form></body>`))
}

// oidcErrorPage — klientga qaytarib bo'lmaydigan xato (noma'lum ilova yoki
// qaytish manzili). Ochiq yo'naltirgich bo'lmasin: hech qayerga yuborilmaydi.
func oidcErrorPage(w http.ResponseWriter, err error) {
	msg := "So'rov yaroqsiz"
	var oe *identity.OAuthError
	if errors.As(err, &oe) {
		msg = oe.Description
	}
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write([]byte(`<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">` +
		`<title>OnDex — kirish</title><body style="font-family:system-ui;max-width:480px;margin:15vh auto;padding:0 16px">` +
		`<h1 style="font-size:20px">Kirib bo'lmadi</h1><p>` + html.EscapeString(msg) +
		`</p><p style="color:#666">Ilovaga qaytib, qaytadan urinib ko'ring.</p></body>`))
}

func mustEmbed(name string) []byte {
	b, err := oidcLoginFS.ReadFile(name)
	if err != nil {
		panic("embed: " + name + ": " + err.Error())
	}
	return b
}

// qrDataURI — otpauth URI'ni PNG QR'ga (TOTP ulash; kalit tarmoqdan
// boshqa joyga — uchinchi tomon QR xizmatiga — ketmaydi).
func qrDataURI(text string) string {
	code, err := qr.Encode(text, qr.M)
	if err != nil {
		return ""
	}
	code.Scale = 6
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(code.PNG())
}
