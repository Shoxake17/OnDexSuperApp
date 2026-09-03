// Ilova ichidagi AI yordamchi (`POST /ai/chat`).
//
// ┌─ `/agent/v1/*` DAN FARQI ──────────────────────────────────────────┐
// `/agent/v1/*` — TASHQI integratsiya (boshqa server, grant, pul
// chegarasi). Bu esa OnDex ilovasining O'ZIDAGI yordamchi: oddiy
// foydalanuvchi JWT'si bilan ishlaydi, grant talab qilmaydi va
// BUYURTMA YARATA OLMAYDI — faqat savat taklifini qaytaradi.
//
// Ikkala yo'l bir-biriga tegmaydi.
// └────────────────────────────────────────────────────────────────────┘
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"chustapp/internal/assistant"
	"chustapp/internal/ratelimit"
)

// aiChatLimiter — foydalanuvchi bo'yicha.
//
// HAR BIR so'rov tashqi til modelini chaqiradi, ya'ni PUL sarflaydi.
// Chegarasiz qoldirilsa bitta akkaunt bilan hisobni ko'tarish
// mumkin edi (moliyaviy DoS) — bu `geoUserLimiter` bilan bir xil
// mantiq.
//
// Daqiqasiga ~12 (portlash 6): odam yozib yoki gapirib bunga yetib
// bormaydi, sikldagi skript esa darhol to'xtaydi.
var aiChatLimiter = ratelimit.New(0.2, 6)

func (s *Server) registerAssistantRoutes(mux *http.ServeMux) {
	// GET /ai/status — ilova yordamchi tugmasini ko'rsatsinmi.
	//
	// Busiz ilova tugmani chizib, bosilganda 503 ko'rsatardi —
	// ya'ni bo'lmagan funksiyani va'da qilardi.
	mux.HandleFunc("GET /ai/status", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{
				"enabled": s.Assistant != nil,
				// voice — OVOZLI rejim (Gemini Live) yoqilganmi.
				//
				// Alohida maydon: kalit qo'yilmagan serverda chat
				// ishlaydi, ovoz esa yo'q. Busiz ilova mikrofon
				// tugmasini chizib, bosilganda 404 ko'rsatardi.
				"voice": s.Assistant != nil && s.AssistantLive != nil,
				// voice_input_rate / voice_output_rate — ilova audio
				// qatlamini shu qiymatlar bilan sozlaydi. Server
				// tomonda o'zgarsa ilovani qayta yozish shart
				// bo'lmasin (noto'g'ri chastota = "cho'zilgan" ovoz).
				"voice_input_rate":  16000,
				"voice_output_rate": assistant.LiveOutputRate,
			})
		}))

	// POST /ai/chat — bitta savol.
	mux.HandleFunc("POST /ai/chat", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			if s.Assistant == nil {
				httpError(w, http.StatusServiceUnavailable,
					errors.New("AI yordamchi hozircha mavjud emas"))
				return
			}
			claims := claimsFrom(r)
			if ok, wait := aiChatLimiter.AllowWithWait(claims.Subject); !ok {
				tooManyRequests(w, wait)
				return
			}

			var req struct {
				Message string `json:"message"`
				// History — suhbat tarixi MIJOZDA saqlanadi.
				// Server sessiya ushlab turmaydi; kelgan tarix
				// `trimHistory` da tozalanadi (`system` roli va
				// soxta tool natijalari tashlanadi).
				History []assistant.Message `json:"history"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}

			// ★ RUXSATLAR SERVERDAN. Mijoz ularni YUBORMAYDI —
			// aks holda o'chirilgan amalni qayta yoqish uchun
			// so'rovni o'zgartirish kifoya bo'lardi.
			disabled, err := s.UserRepo.DisabledAITools(r.Context(), claims.Subject)
			if err != nil {
				// Ruxsatlarni o'qib bo'lmasa suhbatni to'xtatmaymiz,
				// lekin XAVFSIZ tomonga og'amiz: hech narsa
				// o'chirilmagan deb hisoblash noto'g'ri bo'lardi.
				httpError(w, http.StatusServiceUnavailable,
					errors.New("ruxsatlarni o'qib bo'lmadi, qayta urining"))
				return
			}

			res, err := s.Assistant.Chat(r.Context(), claims.Subject,
				req.Message, req.History, disabled)
			if err != nil {
				if errors.Is(err, assistant.ErrTooLong) {
					httpError(w, http.StatusBadRequest, err)
					return
				}
				// Provayder xatolari ma'noli matn bilan keladi
				// ("juda ko'p so'rov"), shuning uchun 502 va o'sha
				// matn — foydalanuvchi nima bo'layotganini biladi.
				httpError(w, http.StatusBadGateway, err)
				return
			}
			writeJSON(w, http.StatusOK, res)
		}))
}
