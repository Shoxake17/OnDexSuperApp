// "Ulangan ilovalar" — foydalanuvchi tomoni.
//
// ┌─ BU FAYL NIMA UCHUN MAVJUD ────────────────────────────────────────┐
// Agent integratsiyasining butun xavfsizligi bitta shartga tayanadi:
// foydalanuvchi (a) nima bo'layotganini KO'RADI va (b) uni istalgan
// vaqtda TO'XTATA oladi. Texnik himoya qanchalik kuchli bo'lmasin,
// odam ruxsat berganini ko'rmasa va bekor qila olmasa — bu ishonchli
// tizim emas.
//
// Shu sababli bu yerdagi endpointlar «qo'shimcha qulaylik» emas,
// modelning teng huquqli qismi:
//
//	rozilik ekrani   — nimaga ruxsat berayotganini KO'RSATADI;
//	chegara qo'yish  — pulning yuqori chegarasini ODAM belgilaydi;
//	tasdiq so'rovi   — chegaradan oshgani unga keladi;
//	ulanganlar ro'yxati + uzish — bir tugmali to'xtatish;
//	tarix (audit)    — agent nima qilganini keyin ham ko'rish.
//
// └────────────────────────────────────────────────────────────────────┘
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"chustapp/internal/agentapi"
)

func (s *Server) registerMeAgentRoutes(mux *http.ServeMux) {
	// GET /me/agent/link/{code} — rozilik ekrani uchun ma'lumot.
	//
	// Kod deep link'dan (`ondex://agent-link?code=...`) yoki qo'lda
	// terishdan keladi. IP bo'yicha cheklangan: kod qisqa, uni
	// taxmin qilishga urinish bo'lishi mumkin.
	mux.HandleFunc("GET /me/agent/link/{code}", s.auth(nil,
		s.agentLinkLimited(func(w http.ResponseWriter, r *http.Request) {
			link, partner, err := s.AgentSvc.ResolveUserCode(r.Context(), r.PathValue("code"))
			if err != nil {
				httpError(w, agentStatus(err), err)
				return
			}
			// Ruxsatlar MATN bilan birga qaytariladi. Faqat
			// `orders:create` deb ko'rsatish — foydalanuvchidan
			// texnik atamani tushunishni talab qilish, ya'ni
			// aslida roziligini SO'RAMASLIK.
			perms := make([]map[string]string, 0, len(link.Scopes))
			for _, sc := range link.Scopes {
				perms = append(perms, map[string]string{
					"scope": sc, "label": agentapi.ScopeLabel(sc),
				})
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"partner":     partner.Name,
				"environment": partner.Environment,
				"permissions": perms,
				"expires_at":  link.ExpiresAt,
			})
		})))

	// POST /me/agent/link/{code}/approve — "Ruxsat beraman".
	mux.HandleFunc("POST /me/agent/link/{code}/approve", s.auth(nil,
		s.agentLinkLimited(func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				// Scopes — foydalanuvchi TANLAGANLARI. Bo'sh
				// bo'lsa sherik so'raganlarning hammasi.
				Scopes []string `json:"scopes"`
				// PerOrderLimitTiyin — 0 (standart) = HAR BIR
				// buyurtma alohida tasdiqlanadi.
				PerOrderLimitTiyin int64 `json:"per_order_limit_tiyin"`
				DailyLimitTiyin    int64 `json:"daily_limit_tiyin"`
			}
			// Bo'sh tana ham to'g'ri so'rov: "hamma so'ralgan
			// ruxsat, chegarasiz rejim" degani.
			if r.Body != nil {
				_ = json.NewDecoder(r.Body).Decode(&req)
			}
			code := r.PathValue("code")
			if len(req.Scopes) == 0 {
				link, _, err := s.AgentSvc.ResolveUserCode(r.Context(), code)
				if err != nil {
					httpError(w, agentStatus(err), err)
					return
				}
				req.Scopes = link.Scopes
			}
			g, err := s.AgentSvc.ApproveLink(r.Context(), claimsFrom(r).Subject, code,
				req.Scopes, req.PerOrderLimitTiyin, req.DailyLimitTiyin)
			if err != nil {
				httpError(w, agentStatus(err), err)
				return
			}
			writeJSON(w, http.StatusOK, g)
		})))

	// POST /me/agent/link/{code}/deny — "Rad etaman".
	mux.HandleFunc("POST /me/agent/link/{code}/deny", s.auth(nil,
		s.agentLinkLimited(func(w http.ResponseWriter, r *http.Request) {
			if err := s.AgentSvc.DenyLink(r.Context(), claimsFrom(r).Subject,
				r.PathValue("code")); err != nil {
				httpError(w, agentStatus(err), err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		})))

	// GET /me/agent/grants — "Ulangan ilovalar" ro'yxati.
	mux.HandleFunc("GET /me/agent/grants", s.auth(nil,
		s.agentEnabled(func(w http.ResponseWriter, r *http.Request) {
			list, err := s.AgentSvc.ListGrants(r.Context(), claimsFrom(r).Subject)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			if list == nil {
				list = []*agentapi.Grant{}
			}
			writeJSON(w, http.StatusOK, list)
		})))

	// DELETE /me/agent/grants/{id} — ulanishni uzish.
	//
	// Ta'sir DARHOL: grant tokenlari JWT emas, har so'rovda bazadan
	// tekshiriladi. Ya'ni "bekor qilingan, lekin hali amal
	// qiladigan" oyna umuman yo'q.
	mux.HandleFunc("DELETE /me/agent/grants/{id}", s.auth(nil,
		s.agentEnabled(func(w http.ResponseWriter, r *http.Request) {
			if err := s.AgentSvc.RevokeGrant(r.Context(), claimsFrom(r).Subject,
				r.PathValue("id")); err != nil {
				httpError(w, agentStatus(err), err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		})))

	// GET /me/agent/activity — agent nima qilgani.
	mux.HandleFunc("GET /me/agent/activity", s.auth(nil,
		s.agentEnabled(func(w http.ResponseWriter, r *http.Request) {
			list, err := s.AgentSvc.ListAudit(r.Context(), claimsFrom(r).Subject, 50)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			if list == nil {
				list = []*agentapi.AuditEntry{}
			}
			writeJSON(w, http.StatusOK, list)
		})))

	// GET /me/agent/drafts — tasdiq kutayotgan buyurtmalar.
	mux.HandleFunc("GET /me/agent/drafts", s.auth(nil,
		s.agentEnabled(func(w http.ResponseWriter, r *http.Request) {
			list, err := s.AgentSvc.ListOpenDrafts(r.Context(), claimsFrom(r).Subject)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			if list == nil {
				list = []*agentapi.Draft{}
			}
			writeJSON(w, http.StatusOK, list)
		})))

	// POST /me/agent/drafts/{id}/approve — chegaradan oshgan
	// buyurtmani tasdiqlash. Buyurtma AYNAN shu yerda yaratiladi.
	mux.HandleFunc("POST /me/agent/drafts/{id}/approve", s.auth(nil,
		s.agentEnabled(func(w http.ResponseWriter, r *http.Request) {
			d, err := s.AgentSvc.UserApproveDraft(r.Context(), claimsFrom(r).Subject,
				r.PathValue("id"), s.placeAgentOrder)
			if err != nil {
				httpError(w, agentStatus(err), err)
				return
			}
			writeJSON(w, http.StatusCreated, d)
		})))

	// POST /me/agent/drafts/{id}/reject — rad etish.
	mux.HandleFunc("POST /me/agent/drafts/{id}/reject", s.auth(nil,
		s.agentEnabled(func(w http.ResponseWriter, r *http.Request) {
			if err := s.AgentSvc.UserRejectDraft(r.Context(), claimsFrom(r).Subject,
				r.PathValue("id")); err != nil {
				httpError(w, agentStatus(err), err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		})))
}

// agentEnabled — integratsiya umuman yoqilganmi.
//
// `AgentSvc` `nil` bo'lishi NORMAL holat: bazasiz (xotira) dev
// rejimida yoki funksiya ataylab o'chirilganda. Bunday paytda
// endpointlar 503 qaytaradi — panic bermaydi va qolgan API
// o'zgarishsiz ishlayveradi (`TableSvc`/`Payments` bilan bir xil
// falsafa).
func (s *Server) agentEnabled(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.AgentSvc == nil {
			httpError(w, http.StatusServiceUnavailable,
				errors.New("agent integratsiyasi sozlanmagan"))
			return
		}
		next(w, r)
	}
}

// agentLinkLimited — ulanish kodi bilan ishlaydigan endpointlar uchun
// IP chegarasi.
//
// Kod 8 belgi (32^8) va 10 daqiqa yashaydi. Chegarasiz qoldirilsa,
// sekundiga minglab urinish bilan kutayotgan rozilik so'rovini
// o'g'irlab olish nazariy jihatdan mumkin bo'lardi.
func (s *Server) agentLinkLimited(next http.HandlerFunc) http.HandlerFunc {
	return s.agentEnabled(func(w http.ResponseWriter, r *http.Request) {
		if ok, wait := agentLinkLimiter.AllowWithWait(clientIP(r)); !ok {
			tooManyRequests(w, wait)
			return
		}
		next(w, r)
	})
}
