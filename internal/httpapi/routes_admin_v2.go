package httpapi

import (
	"errors"
	"net/http"
	"slices"

	"chustapp/internal/users"
)

// Admin (platform realmi) markaziy API — A6 / CC-0.1.
//
//	GET  /v2/admin/organizations?status=pending   KYC navbati
//	POST /v2/admin/organizations/{id}/verify      {decision, reason, stepup_code}
//
// Faqat platform kaliti bilan imzolangan, TOTP bilan ochilgan sessiya.
// Har bir qaror — Authenticator kodi (step-up) + audit.

// adminV2 — platform realmi tokeni + sessiya ikkinchi omil bilan ochilgan.
func (s *Server) adminV2(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.merchantAuthReady(w) {
			return
		}
		s.authV2([]users.Realm{users.RealmPlatform}, func(w http.ResponseWriter, r *http.Request) {
			c := claimsV2From(r)
			if !slices.Contains(c.AMR, "totp") && !slices.Contains(c.AMR, "recovery") {
				httpError(w, http.StatusUnauthorized, errors.New("admin sessiyasi ikki bosqichli tasdiqsiz — qaytadan kiring"))
				return
			}
			noStore(w)
			next(w, r)
		})(w, r)
	}
}

func (s *Server) registerAdminV2Routes(mux *http.ServeMux) {
	// whoami — ekotizimning boshqa servislari (OnDexMap admin serveri, 138)
	// admin tokenini SHU YERDA tekshiradi: imzo, platform kaliti, sessiya
	// faolligi (chiqish/bloklash darhol kuchga kiradi), TOTP. Ular kalit
	// saqlamaydi va o'z akkaunt bazasini yuritmaydi — bitta panel, bitta kirish.
	mux.HandleFunc("GET /v2/admin/whoami", s.adminV2(func(w http.ResponseWriter, r *http.Request) {
		c := claimsV2From(r)
		u, err := s.UserRepo.GetByID(r.Context(), c.Subject)
		if err != nil || u.Role != users.RoleAdmin || u.IsDeleted() {
			httpError(w, http.StatusUnauthorized, errors.New("admin emas"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"user_id": u.ID, "name": u.Name, "phone": u.Phone,
			"session_id": c.SessionID, "amr": c.AMR, "expires_at": c.ExpiresAt})
	}))

	mux.HandleFunc("GET /v2/admin/organizations", s.adminV2(func(w http.ResponseWriter, r *http.Request) {
		list, err := s.MerchantAuth.Organizations(r.Context(), claimsV2From(r).Subject, r.URL.Query().Get("status"))
		if err != nil {
			respondDirectoryError(w, err)
			return
		}
		out := make([]map[string]any, 0, len(list))
		for _, o := range list {
			out = append(out, map[string]any{"id": o.ID, "name": o.Name, "legal_form": o.LegalForm, "tin": o.TIN,
				"status": o.Status, "status_reason": o.StatusReason, "owner_user_id": o.OwnerUserID,
				"created_at": o.CreatedAt, "verified_at": o.VerifiedAt})
		}
		writeJSON(w, http.StatusOK, map[string]any{"organizations": out})
	}))

	mux.HandleFunc("POST /v2/admin/organizations/{id}/verify", s.adminV2(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Decision string `json:"decision"`
			Reason   string `json:"reason"`
			StepUp   string `json:"stepup_code"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		if len(req.StepUp) > 32 || len(req.Reason) > 2000 {
			httpError(w, http.StatusBadRequest, errors.New("maydon juda uzun"))
			return
		}
		o, err := s.MerchantAuth.DecideOrganization(r.Context(), claimsV2From(r).Subject, r.PathValue("id"),
			req.Decision, req.Reason, req.StepUp, merchantMeta(r, ""))
		if err != nil {
			respondDirectoryError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": o.ID, "status": o.Status, "status_reason": o.StatusReason,
			"verified_at": o.VerifiedAt})
	}))
}
