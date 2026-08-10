package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"chustapp/internal/notify"
)

// Bildirishnomalar (migration 0030).
//
// ┌─ NEGA HTTP ENDPOINT KERAK ────────────────────────────────────────┐
// WebSocket xabari FAQAT ilova ochiq bo'lganda yetadi. Ilova yopiq,
// tarmoq uzilgan yoki soket o'lik bo'lsa xabar yo'qolardi. Endi u
// DB'ga yoziladi va foydalanuvchi ilovani ochganda shu endpointlar
// orqali o'qilmaganlarni ko'radi.
// └───────────────────────────────────────────────────────────────────┘
func (s *Server) registerNotificationRoutes(mux *http.ServeMux) {
	// GET /notifications?limit=50 — eng yangilaridan.
	mux.HandleFunc("GET /notifications", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			if s.Notifications == nil {
				httpError(w, http.StatusServiceUnavailable,
					errors.New("bildirishnomalar sozlanmagan"))
				return
			}
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			list, err := s.Notifications.List(r.Context(), claimsFrom(r).Subject, limit)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			// `nil` emas, BO'SH MASSIV qaytariladi: `nil` slice JSON'da
			// `null` bo'lib ketadi va klientda ro'yxatni aylantirish
			// xatoga olib keladi.
			if list == nil {
				list = []*notify.Notification{}
			}
			writeJSON(w, http.StatusOK, list)
		}))

	// GET /notifications/unread-count — qo'ng'iroq belgisidagi raqam.
	mux.HandleFunc("GET /notifications/unread-count", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			if s.Notifications == nil {
				writeJSON(w, http.StatusOK, map[string]int{"count": 0})
				return
			}
			n, err := s.Notifications.UnreadCount(r.Context(), claimsFrom(r).Subject)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]int{"count": n})
		}))

	// POST /notifications/{id}/read
	//
	// XAVFSIZLIK: `user_id` sharti do'kon qatlamida SQL'ning O'ZIDA
	// (`MarkRead` izohiga qarang) — begona bildirishnomani o'qilgan
	// qilib bo'lmaydi.
	mux.HandleFunc("POST /notifications/{id}/read", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			if s.Notifications == nil {
				httpError(w, http.StatusServiceUnavailable,
					errors.New("bildirishnomalar sozlanmagan"))
				return
			}
			id := r.PathValue("id")
			if err := s.Notifications.MarkRead(r.Context(), claimsFrom(r).Subject, id); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		}))

	// POST /notifications/read-all
	mux.HandleFunc("POST /notifications/read-all", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			if s.Notifications == nil {
				httpError(w, http.StatusServiceUnavailable,
					errors.New("bildirishnomalar sozlanmagan"))
				return
			}
			if err := s.Notifications.MarkAllRead(r.Context(), claimsFrom(r).Subject); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		}))

	// ---------- Push tokenlari ----------

	// POST /me/push-token {"token":"...","platform":"android"}
	//
	// Ilova FCM tokenini olgach shu yerga yuboradi. Token BOSHQA
	// foydalanuvchida bo'lsa yangi egasiga o'tadi (bitta telefonda
	// ikki hisob almashgan holat) — busiz chiqib ketgan foydalanuvchi
	// begona buyurtmalar haqida push olishda davom etardi.
	mux.HandleFunc("POST /me/push-token", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			if s.PushTokens == nil {
				writeJSON(w, http.StatusOK, map[string]bool{"saved": false})
				return
			}
			var req struct {
				Token    string `json:"token"`
				Platform string `json:"platform"`
			}
			if !decodeJSON(w, r, &req) {
				return
			}
			req.Token = strings.TrimSpace(req.Token)
			// Uzunlik chegarasi: FCM tokeni ~160-200 belgi. Chegarasiz
			// qoldirilsa bu jadval xotira yeguvchiga aylanardi.
			if req.Token == "" || len(req.Token) > 512 {
				httpError(w, http.StatusBadRequest, errors.New("token noto'g'ri"))
				return
			}
			// `platform` ham cheklanadi. Umumiy tana chegarasi 1MB
			// (`withBodyLimit`), ya'ni DoS emas — lekin usiz har bir
			// qurilma yozuviga megabaytlik axlat saqlash mumkin edi.
			// Bu maydon faqat diagnostika uchun, mantiqqa ta'sir
			// qilmaydi, shuning uchun XATO QAYTARILMAYDI: noto'g'ri
			// qiymat sababli push ro'yxatdan o'tmay qolishi
			// tekshiruvning foydasidan ko'ra zararli bo'lardi.
			req.Platform = strings.TrimSpace(req.Platform)
			if len(req.Platform) > 32 {
				req.Platform = req.Platform[:32]
			}
			if err := s.PushTokens.SaveToken(r.Context(),
				claimsFrom(r).Subject, req.Token, req.Platform); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
		}))

	// DELETE /me/push-token — chiqishda chaqiriladi.
	//
	// NEGA MUHIM: token o'chirilmasa, chiqib ketgan foydalanuvchining
	// telefoni keyingi egasining bildirishnomalarini olishda davom
	// etardi.
	mux.HandleFunc("DELETE /me/push-token", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			if s.PushTokens == nil {
				writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
				return
			}
			token := strings.TrimSpace(r.URL.Query().Get("token"))
			if token == "" {
				httpError(w, http.StatusBadRequest, errors.New("token yo'q"))
				return
			}
			if err := s.PushTokens.DeleteToken(r.Context(), token); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		}))
}
