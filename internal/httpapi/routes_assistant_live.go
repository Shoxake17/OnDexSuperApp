package httpapi

// routes_assistant_live.go — `GET /ai/live`: ovozli rejimning
// WebSocket kanali (mikrofon → Gemini Live → ovozli javob).
//
// ┌─ PROTOKOL ─────────────────────────────────────────────────────────┐
// Mijozdan serverga:
//   * BINAR kadr — xom PCM16 @16kHz mono mikrofon bo'lagi;
//   * MATN kadr  — JSON boshqaruv: {"type":"end"} (mikrofon o'chdi).
//
// Serverdan mijozga: har doim JSON (`assistant.LiveEvent`) —
//   ready | audio | text | proposal | interrupted | turn_end | error.
//
// Audio yuqoriga BINAR ketadi (base64 33% ortiqcha trafik bo'lardi va
// mikrofon oqimi uzluksiz), pastga esa JSON ichida base64 — chunki
// audio va holat AYNAN bitta tartibda kelishi kerak. Ikki alohida
// kanalda "gapirib bo'ldi" signali audio tugashidan oldin kelib
// qolardi.
// └────────────────────────────────────────────────────────────────────┘

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"chustapp/internal/assistant"
	"chustapp/internal/safego"
	"chustapp/internal/ws"
)

const (
	// liveMaxSession — bitta seansning eng uzun muddati.
	//
	// Gemini Live daqiqa bo'yicha hisoblanadi, ya'ni unutilgan ochiq
	// ulanish TO'G'RIDAN-TO'G'RI pul. Ilova ekranni yopganda o'zi
	// uziladi; bu chegara esa uzilmagan holat uchun.
	liveMaxSession = 10 * time.Minute

	// liveWriteWait — bitta kadrni yozish muddati.
	liveWriteWait = 10 * time.Second

	// livePongWait / livePingPeriod — o'lik ulanishni sezish. Audio
	// oqimi to'xtasa ham (foydalanuvchi jim) ulanish tirik qolishi
	// kerak, shuning uchun ping bor.
	livePongWait   = 60 * time.Second
	livePingPeriod = 25 * time.Second

	// liveReadLimit — bitta kadrning chegarasi.
	liveReadLimit = 64 << 10
)

// liveSessions — foydalanuvchi bo'yicha ochiq seanslar.
//
// ┌─ NEGA BITTA ────────────────────────────────────────────────────────┐
// Ovozli seans — uzluksiz oqim va daqiqalik hisob. Chegara bo'lmasa
// bitta akkaunt o'nlab seans ochib hisobni ko'tarishi mumkin edi
// (`aiChatLimiter` bilan bir xil mantiq, faqat bu yerda narx ancha
// qimmat). Ikkinchi qurilmadan ulanish eskisini ALMASHTIRMAYDI —
// foydalanuvchi eskisini yopishi kerak, aks holda "kim gapiryapti"
// noaniq bo'lardi.
// └─────────────────────────────────────────────────────────────────────┘
var liveSessions = struct {
	mu   sync.Mutex
	open map[string]struct{}
}{open: make(map[string]struct{})}

func liveAcquire(userID string) bool {
	liveSessions.mu.Lock()
	defer liveSessions.mu.Unlock()
	if _, busy := liveSessions.open[userID]; busy {
		return false
	}
	liveSessions.open[userID] = struct{}{}
	return true
}

func liveRelease(userID string) {
	liveSessions.mu.Lock()
	delete(liveSessions.open, userID)
	liveSessions.mu.Unlock()
}

func (s *Server) registerAssistantLiveRoutes(mux *http.ServeMux, allowedOrigins []string) {
	// Sozlanmagan bo'lsa marshrut UMUMAN ro'yxatga olinmaydi:
	// ilova `GET /ai/status` dagi `voice: false` ni ko'rib tugmani
	// chizmaydi, bosilgan taqdirda ham 404 aniq javob beradi.
	if s.AssistantLive == nil || s.Assistant == nil {
		return
	}
	cfg := *s.AssistantLive
	upgrader := ws.NewUpgrader(allowedOrigins)

	mux.HandleFunc("GET /ai/live", func(w http.ResponseWriter, r *http.Request) {
		// Auth — `/ws` bilan AYNAN bir xil bilet mexanizmi: WebSocket
		// handshake'da `Authorization` header qo'yib bo'lmaydi
		// (`internal/ws/tickets.go` izohiga qarang).
		ticket := r.URL.Query().Get("ticket")
		if ticket == "" {
			httpError(w, http.StatusUnauthorized,
				errors.New("?ticket= talab qilinadi (avval POST /ws/ticket)"))
			return
		}
		claims, ok := s.WsTickets.Consume(ticket)
		if !ok {
			httpError(w, http.StatusUnauthorized,
				errors.New("bilet yaroqsiz yoki muddati tugagan"))
			return
		}
		userID := claims.Subject
		if userID == "" {
			httpError(w, http.StatusUnauthorized, errors.New("bilet egasi noma'lum"))
			return
		}

		if !liveAcquire(userID) {
			httpError(w, http.StatusConflict,
				errors.New("ovozli suhbat allaqachon ochiq (boshqa qurilmada?)"))
			return
		}
		defer liveRelease(userID)

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return // Upgrade o'zi javob yozgan
		}
		defer conn.Close()

		ctx, cancel := context.WithTimeout(r.Context(), liveMaxSession)
		defer cancel()

		// ★ RUXSATLAR SERVERDAN — mijoz ro'yxatni bera olmaydi.
		disabled, err := s.UserRepo.DisabledAITools(ctx, userID)
		if err != nil {
			slog.Warn("ai live: ruxsatlarni o'qib bo'lmadi",
				"user", userID, "err", err)
			writeLiveJSON(conn, assistant.LiveEvent{
				Type:  assistant.LiveEventError,
				Error: "Ruxsatlarni o'qib bo'lmadi. Qayta urinib ko'ring.",
			})
			return
		}

		sess, err := s.Assistant.StartLive(ctx, cfg, userID, disabled)
		if err != nil {
			slog.Warn("ai live: seans ochilmadi", "user", userID, "err", err)
			writeLiveJSON(conn, assistant.LiveEvent{
				Type:  assistant.LiveEventError,
				Error: "Ovozli yordamchi hozir mavjud emas.",
			})
			return
		}
		defer sess.Close()

		slog.Info("ai live: seans ochildi", "user", userID)
		defer slog.Info("ai live: seans yopildi", "user", userID)

		// Yozuvchi goroutine — hodisalar va ping AYNAN bitta joydan
		// yoziladi. `gorilla/websocket` bir vaqtda ikki yozuvchini
		// QO'LLAMAYDI (panic beradi), shuning uchun ping ham shu
		// yerda.
		var wg sync.WaitGroup
		wg.Add(1)
		// ┌─ RECOVER BILAN (bug.md 44-band) ───────────────────────┐
		// `wg.Done()` va `cancel()` ATAYLAB tashqi `defer` da:
		// ular panic bo'lganda ham bajarilishi SHART, aks holda
		// handler `wg.Wait()` da abadiy osilib qolardi.
		//
		// Sikl `liveWriteLoop` ga ajratildi — `safego.Run` ga
		// uzatish uchun va o'qish oson bo'lsin deb.
		// └─────────────────────────────────────────────────────────┘
		go func() {
			defer wg.Done()
			defer cancel() // yozuv tugadi — o'qishni ham to'xtatamiz
			safego.Run("assistant.live.writer", func() {
				liveWriteLoop(ctx, conn, sess)
			})
		}()

		// O'qish sikli — shu goroutine'da (handler ulanish yopilguncha
		// tirik turishi kerak).
		conn.SetReadLimit(liveReadLimit)
		_ = conn.SetReadDeadline(time.Now().Add(livePongWait))
		conn.SetPongHandler(func(string) error {
			return conn.SetReadDeadline(time.Now().Add(livePongWait))
		})

		for {
			typ, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			// Har kadr muddatni uzaytiradi: uzluksiz gapirayotgan
			// foydalanuvchi pong kutmasdan ham tirik hisoblanadi.
			_ = conn.SetReadDeadline(time.Now().Add(livePongWait))

			switch typ {
			case websocket.BinaryMessage:
				if err := sess.SendAudio(data); err != nil {
					slog.Warn("ai live: audio yuborilmadi",
						"user", userID, "err", err)
					goto done
				}
			case websocket.TextMessage:
				// ┌─ BOSHQARUV KADRLARI QAT'IY RO'YXAT ───────────────┐
				// Mijoz modelga IXTIYORIY matn yubora olmaydi: bu
				// yerda faqat sanab o'tilgan turlar qabul qilinadi
				// va gapni server o'zi tuzadi. Aks holda ilovani
				// o'zgartirgan odam modelning ko'rsatmasini qayta
				// yozib yuborishi mumkin edi.
				// └───────────────────────────────────────────────────┘
				var ctrl struct {
					Type       string `json:"type"`
					Restaurant string `json:"restaurant"`
					TotalTiyin int64  `json:"total_tiyin"`
				}
				if json.Unmarshal(data, &ctrl) != nil {
					continue // buzilgan boshqaruv kadri — e'tiborsiz
				}
				switch ctrl.Type {
				case "end":
					// Mikrofon o'chdi. Busiz model jimlik kutib
					// qolardi va javob umuman kelmasdi.
					_ = sess.AudioStreamEnd()
				case "checkout_ready":
					// Ilova rasmiylashtirish ekraniga yetib bordi —
					// endi Shaddiy to'lovni so'raydi.
					if err := sess.NotifyCheckoutReady(
						ctrl.Restaurant, ctrl.TotalTiyin); err != nil {
						slog.Warn("ai live: checkout xabari yuborilmadi",
							"user", userID, "err", err)
					}
				}
			}
		}
	done:
		cancel()
		sess.Close()
		wg.Wait()
	})
}

// writeLiveJSON — bitta hodisani yozadi (faqat yozuvchi goroutine'dan).
// liveWriteLoop — ovozli seansning YAGONA yozuvchisi: hodisalar ham,
// ping ham shu yerdan yoziladi.
//
// `gorilla/websocket` bir vaqtda ikki yozuvchini QO'LLAMAYDI (panic
// beradi), shuning uchun ping ham aynan shu siklda.
//
// Alohida funksiyaga ajratildi (bug.md 44-band): `safego.Run` ga
// uzatish uchun — bu goroutine HTTP handlerining panic tutuvchisidan
// tashqarida ishlaydi va undagi panic butun jarayonni yiqitardi.
func liveWriteLoop(ctx context.Context, conn *websocket.Conn, sess *assistant.LiveSession) {
	ticker := time.NewTicker(livePingPeriod)
	defer ticker.Stop()
	for {
		select {
		case ev, open := <-sess.Events():
			if !open {
				return
			}
			if err := writeLiveJSON(conn, ev); err != nil {
				return
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(liveWriteWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func writeLiveJSON(conn *websocket.Conn, ev assistant.LiveEvent) error {
	_ = conn.SetWriteDeadline(time.Now().Add(liveWriteWait))
	return conn.WriteJSON(ev)
}
