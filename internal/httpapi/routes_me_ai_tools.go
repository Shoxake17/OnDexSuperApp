package httpapi

// routes_me_ai_tools.go — yordamchi (Shaddiy) qaysi amallarni bajara
// olishini foydalanuvchining O'ZI boshqaradi.
//
// ┌─ NEGA SERVERDA ────────────────────────────────────────────────────┐
// Buni ilovada saqlash mumkin edi, lekin unda u SOZLAMA bo'lardi:
// ikkinchi telefonda boshqacha, ilova qayta o'rnatilganda esa
// tiklangan bo'lardi. Va eng muhimi — tekshiruv MIJOZDA bo'lardi,
// ya'ni so'rovni o'zgartirgan odam o'chirilgan amalni qayta yoqa
// olardi.
//
// Endi ro'yxat akkauntga tegishli va `/ai/chat` ham, `/ai/live` ham
// uni SERVERDAN o'qiydi. O'chirilgan amal modelga umuman e'lon
// qilinmaydi va chaqirilsa ham bajarilmaydi.
// └────────────────────────────────────────────────────────────────────┘

import (
	"encoding/json"
	"errors"
	"net/http"

	"chustapp/internal/assistant"
)

func (s *Server) registerMeAIToolRoutes(mux *http.ServeMux) {
	// GET /me/ai-tools — barcha amallar va ularning holati.
	//
	// Ro'yxatni SERVER beradi: ilova amal nomlarini qo'lda yozsa,
	// backend'da yangi amal qo'shilganda u ilovada ko'rinmasdi va
	// foydalanuvchi uni boshqara olmasdi.
	mux.HandleFunc("GET /me/ai-tools", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			claims := claimsFrom(r)
			disabled, err := s.UserRepo.DisabledAITools(r.Context(), claims.Subject)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			off := make(map[string]bool, len(disabled))
			for _, n := range disabled {
				off[n] = true
			}

			out := make([]map[string]any, 0, len(assistant.CapabilityNames()))
			for _, name := range assistant.CapabilityNames() {
				out = append(out, map[string]any{
					"name":    name,
					"enabled": !off[name],
				})
			}
			writeJSON(w, http.StatusOK, map[string]any{"tools": out})
		}))

	// PUT /me/ai-tools — holatni yangilaydi.
	//
	// Mijoz YOQILGANLARNI yuboradi, server esa o'chirilganlarni
	// hisoblab saqlaydi. Nega shunday: ro'yxat serverda o'sishi
	// mumkin, mijoz esa eski nusxada bo'lishi mumkin — u bilmagan
	// yangi amal jimgina o'chib qolmasligi kerak.
	mux.HandleFunc("PUT /me/ai-tools", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			claims := claimsFrom(r)

			var req struct {
				Enabled []string `json:"enabled"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}

			on := make(map[string]bool, len(req.Enabled))
			for _, n := range req.Enabled {
				on[n] = true
			}

			// ★ FAQAT MA'LUM NOMLAR. Mijoz yuborgan begona nom
			// e'tiborsiz qoldiriladi — aks holda bazaga cheksiz
			// axlat yozish mumkin bo'lardi.
			known := assistant.CapabilityNames()
			disabled := make([]string, 0, len(known))
			for _, name := range known {
				if !on[name] {
					disabled = append(disabled, name)
				}
			}

			// Hammasini o'chirib bo'lmaydi: bunda yordamchi hech
			// narsa qila olmay, "javob bermayapti" bo'lib ko'rinardi.
			// Chegara sifat uchun emas, TUSHUNARLILIK uchun.
			if len(disabled) == len(known) {
				httpError(w, http.StatusBadRequest,
					errors.New("kamida bitta amal yoqilgan bo'lishi kerak"))
				return
			}

			if err := s.UserRepo.SetDisabledAITools(
				r.Context(), claims.Subject, disabled); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}))
}
