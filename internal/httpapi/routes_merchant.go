package httpapi

import (
	"errors"
	"net/http"

	"chustapp/internal/identity"
	"chustapp/internal/identity/authz"
	"chustapp/internal/ratelimit"
	"chustapp/internal/users"
)

// Merchant katalogi (A3c, AUTH-PHASE-A.md 7-bo'lim).
//
// ┌─ MARSHRUTLAR ─────────────────────────────────────────────────────────┐
//   POST   /v2/merchant/signup                         (ochiq: telefon kodi + parol)
//   GET    /v2/merchant/me
//   GET    /v2/merchant/businesses/{id}/staff           staff.view
//   POST   /v2/merchant/businesses/{id}/staff           staff.manage (taklif)
//   PATCH  /v2/merchant/businesses/{id}/staff/{mid}     staff.manage
//   POST   /v2/merchant/businesses/{id}/staff/{mid}/dismiss
//   POST   /v2/merchant/businesses/{id}/staff/{mid}/invite   (qayta taklif)
//   GET    /v2/merchant/businesses/{id}/roles           staff.view
//   POST   /v2/merchant/businesses/{id}/roles           staff.manage
// └───────────────────────────────────────────────────────────────────────┘
//
// Faqat merchant realmi EdDSA tokeni. Biznes amali faqat a'zolik ruxsat
// bergan ILOVA sessiyasidan (masalan "faqat OnDexPro" xodimi panel API'sini
// chaqira olmaydi). Begona biznes — 404.

// signupIPLimiter — ro'yxatdan o'tish: Argon2 (64 MB) + kod tekshiruvi; halol
// foydalanuvchi bir necha marta chaqiradi.
var signupIPLimiter = ratelimit.New(0.05, 10)

func respondDirectoryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, authz.ErrNotFound):
		httpError(w, http.StatusNotFound, errors.New("topilmadi"))
	case errors.Is(err, authz.ErrForbidden), errors.Is(err, authz.ErrSuspended):
		httpError(w, http.StatusForbidden, err)
	case errors.Is(err, authz.ErrInvalid):
		httpError(w, http.StatusBadRequest, err)
	case errors.Is(err, authz.ErrConflict), errors.Is(err, identity.ErrAccountExists):
		httpError(w, http.StatusConflict, err)
	default:
		respondMerchantAuthError(w, err)
	}
}

// businessApp — sessiya ilovasi shu biznesdagi a'zolikka ruxsat etilganmi.
// Rad etilsa javobni o'zi yozadi.
func (s *Server) businessApp(w http.ResponseWriter, r *http.Request, bizID string) bool {
	c := claimsV2From(r)
	acc, err := s.MerchantAuth.Access(r.Context(), c.Subject, bizID)
	if err != nil {
		respondDirectoryError(w, err)
		return false
	}
	if !acc.AllowsApp(authz.AppForClient(c.ClientID)) {
		httpError(w, http.StatusForbidden, identity.ErrAppNotAllowed)
		return false
	}
	return true
}

func (s *Server) merchantV2(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.merchantAuthReady(w) {
			return
		}
		s.authV2([]users.Realm{users.RealmMerchant}, next)(w, r)
	}
}

// bizV2 — merchant tokeni + biznes/ilova tekshiruvi; handler (actor, biznes) oladi.
func (s *Server) bizV2(next func(w http.ResponseWriter, r *http.Request, actor, biz string)) http.HandlerFunc {
	return s.merchantV2(func(w http.ResponseWriter, r *http.Request) {
		biz := r.PathValue("id")
		if !s.businessApp(w, r, biz) {
			return
		}
		next(w, r, claimsV2From(r).Subject, biz)
	})
}

func (s *Server) registerMerchantRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v2/merchant/signup", func(w http.ResponseWriter, r *http.Request) {
		if !s.merchantAuthReady(w) {
			return
		}
		var req struct {
			Phone           string `json:"phone"`
			Code            string `json:"code"`
			Password        string `json:"password"`
			PasswordConfirm string `json:"password_confirm"`
			FirstName       string `json:"first_name"`
			LastName        string `json:"last_name"`
			Org             struct {
				Name      string `json:"name"`
				LegalForm string `json:"legal_form"`
				TIN       string `json:"tin"`
			} `json:"org"`
			Business struct {
				Type string `json:"type"`
				Name string `json:"name"`
			} `json:"business"`
			ClientID   string `json:"client_id"`
			DeviceName string `json:"device_name"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		if len(req.Password) > users.MaxPasswordLength || len(req.PasswordConfirm) > users.MaxPasswordLength {
			httpError(w, http.StatusBadRequest, users.ErrInvalidCredentials)
			return
		}
		// Argon2 va kod tekshiruvi oldidan (forma xatosi `Signup` ichida bepul).
		if rateLimited(w, signupIPLimiter, clientIP(r)) {
			return
		}
		out, err := s.MerchantAuth.Signup(r.Context(), identity.SignupInput{Phone: req.Phone, Code: req.Code,
			Password: req.Password, PasswordConfirm: req.PasswordConfirm, FirstName: req.FirstName, LastName: req.LastName,
			OrgName: req.Org.Name, LegalForm: req.Org.LegalForm, TIN: req.Org.TIN,
			BusinessType: authz.BusinessType(req.Business.Type), BusinessName: req.Business.Name,
			ClientID: req.ClientID}, merchantMeta(r, req.DeviceName))
		if err != nil {
			respondDirectoryError(w, err)
			return
		}
		writeOutcome(w, out)
	})

	mux.HandleFunc("GET /v2/merchant/me", s.merchantV2(func(w http.ResponseWriter, r *http.Request) {
		me, err := s.MerchantAuth.Me(r.Context(), claimsV2From(r).Subject)
		if err != nil {
			respondDirectoryError(w, err)
			return
		}
		noStore(w)
		writeJSON(w, http.StatusOK, me)
	}))

	mux.HandleFunc("GET /v2/merchant/businesses/{id}/staff", s.bizV2(func(w http.ResponseWriter, r *http.Request, actor, biz string) {
		list, err := s.MerchantAuth.ListStaff(r.Context(), actor, biz)
		if err != nil {
			respondDirectoryError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"staff": list})
	}))

	mux.HandleFunc("POST /v2/merchant/businesses/{id}/staff", s.bizV2(func(w http.ResponseWriter, r *http.Request, actor, biz string) {
		var req struct {
			Phone     string   `json:"phone"`
			FirstName string   `json:"first_name"`
			LastName  string   `json:"last_name"`
			RoleID    string   `json:"role_id"`
			Apps      []string `json:"apps"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		v, err := s.MerchantAuth.AddStaff(r.Context(), actor, biz, identity.AddStaffInput{Phone: req.Phone,
			FirstName: req.FirstName, LastName: req.LastName, RoleID: req.RoleID, Apps: req.Apps}, merchantMeta(r, ""))
		if err != nil {
			respondDirectoryError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, v)
	}))

	mux.HandleFunc("PATCH /v2/merchant/businesses/{id}/staff/{mid}", s.bizV2(func(w http.ResponseWriter, r *http.Request, actor, biz string) {
		var req struct {
			RoleID string   `json:"role_id"`
			Apps   []string `json:"apps"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		v, err := s.MerchantAuth.UpdateStaff(r.Context(), actor, biz, r.PathValue("mid"),
			identity.UpdateStaffInput{RoleID: req.RoleID, Apps: req.Apps}, merchantMeta(r, ""))
		if err != nil {
			respondDirectoryError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
	}))

	mux.HandleFunc("POST /v2/merchant/businesses/{id}/staff/{mid}/dismiss", s.bizV2(func(w http.ResponseWriter, r *http.Request, actor, biz string) {
		if err := s.MerchantAuth.DismissStaff(r.Context(), actor, biz, r.PathValue("mid"), merchantMeta(r, "")); err != nil {
			respondDirectoryError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	mux.HandleFunc("POST /v2/merchant/businesses/{id}/staff/{mid}/invite", s.bizV2(func(w http.ResponseWriter, r *http.Request, actor, biz string) {
		v, err := s.MerchantAuth.ReinviteStaff(r.Context(), actor, biz, r.PathValue("mid"), merchantMeta(r, ""))
		if err != nil {
			respondDirectoryError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
	}))

	mux.HandleFunc("GET /v2/merchant/businesses/{id}/roles", s.bizV2(func(w http.ResponseWriter, r *http.Request, actor, biz string) {
		v, err := s.MerchantAuth.Roles(r.Context(), actor, biz)
		if err != nil {
			respondDirectoryError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
	}))

	mux.HandleFunc("POST /v2/merchant/businesses/{id}/roles", s.bizV2(func(w http.ResponseWriter, r *http.Request, actor, biz string) {
		var req struct {
			Key         string             `json:"key"`
			Name        string             `json:"name"`
			Permissions []authz.Permission `json:"permissions"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		role, err := s.MerchantAuth.CreateRole(r.Context(), actor, biz,
			identity.CreateRoleInput{Key: req.Key, Name: req.Name, Permissions: req.Permissions}, merchantMeta(r, ""))
		if err != nil {
			respondDirectoryError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, role)
	}))
}
