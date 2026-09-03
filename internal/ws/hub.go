// Package ws — WebSocket hub: har bir ulangan foydalanuvchiga jonli
// eventlar yuboradi.
//
// ── KALITLAR ───────────────────────────────────────────────────────
// Kalit sxemasi `notify/topic.go` da BIR JOYDA belgilangan
// (`u:<id>`, `e:<module>:<id>`, `o:<id>`). Bu yerda kalit shunchaki
// satr — hub uning ma'nosini bilmaydi.
package ws

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ┌─ VAQT CHEGARALARI ────────────────────────────────────────────────┐
// `pingPeriod` < `pongWait` bo'lishi SHART: server ping yuboradi,
// klient pong qaytaradi va o'qish muddati uzaytiriladi. Busiz
// "yarim ochiq" ulanish (mijoz tunnelga kirdi, TCP hali yopilmagan)
// soatlab tirik ko'rinib turardi va unga yuborilgan har bir xabar
// yo'qolardi.
// └───────────────────────────────────────────────────────────────────┘
const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 50 * time.Second

	// outBuffer — bitta ulanish uchun navbatdagi xabarlar chegarasi.
	//
	// Sekin klient (zaif tarmoq) butun serverni ushlab turmasligi
	// kerak. Navbat to'lsa xabar TASHLANADI va ulanish yopiladi —
	// klient qayta ulanib, o'qilmagan bildirishnomalarni
	// `GET /notifications` orqali oladi (shuning uchun ular DB'ga
	// yoziladi).
	outBuffer = 64
)

// client — bitta WebSocket ulanish.
//
// ┌─ NEGA YOZUVCHI GOROUTINE ─────────────────────────────────────────┐
// Avval `Send` to'g'ridan-to'g'ri `conn.WriteJSON` chaqirardi, ya'ni
// HTTP handler o'lik soket uchun 10 SONIYAGACHA bloklanardi. Bir
// necha o'lik ulanish bo'lsa, buyurtma holatini o'zgartirish
// handleri o'nlab soniya qotardi.
//
// Endi `Send` faqat navbatga qo'yadi (bloklanmaydi), yozishni esa
// har bir ulanishning O'Z goroutine'i bajaradi.
// └───────────────────────────────────────────────────────────────────┘
type client struct {
	conn *websocket.Conn
	out  chan []byte
	// closeOnce — `close(out)` faqat bir marta bajarilsin.
	closeOnce sync.Once
}

func (c *client) stop() {
	c.closeOnce.Do(func() { close(c.out) })
}

// enqueue — xabarni navbatga qo'yadi. Navbat to'lgan bo'lsa `false`
// qaytaradi (chaqiruvchi ulanishni yopadi).
func (c *client) enqueue(msg []byte) bool {
	select {
	case c.out <- msg:
		return true
	default:
		return false
	}
}

// writeLoop — navbatdagi xabarlarni yozadi va davriy ping yuboradi.
func (c *client) writeLoop() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.out:
			if !ok {
				// Navbat yopildi — ulanishni odob bilan yopamiz.
				c.conn.SetWriteDeadline(time.Now().Add(writeWait))
				_ = c.conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

type Hub struct {
	mu       sync.RWMutex
	conns    map[string]map[*client]struct{} // kalit -> shu kalit ostidagi ulanishlar
	upgrader websocket.Upgrader
}

// NewHub — allowedOrigins bo'sh bo'lsa (masalan lokal dev, ALLOWED_ORIGINS
// sozlanmagan) HAMMA origin qabul qilinadi (ogohlantirish bilan) — aks
// holda faqat ro'yxatdagi origin'lardan kelgan brauzer so'rovlariga ruxsat
// beriladi. Origin header UMUMAN yo'q so'rovlar (native Android/iOS
// ilovalar — faqat brauzerlar Origin yuboradi) HAR DOIM qabul qilinadi,
// aks holda mobil ilovalar butunlay ishlamay qolardi.
func NewHub(allowedOrigins []string) *Hub {
	return &Hub{
		conns:    make(map[string]map[*client]struct{}),
		upgrader: NewUpgrader(allowedOrigins),
	}
}

// NewUpgrader — origin siyosati BIR JOYDA.
//
// Hub'dan tashqari ovozli rejim ham (`GET /ai/live`) o'z ulanishini
// ko'taradi. Ikki joyda ikki nusxa `CheckOrigin` tursa, ulardan biri
// o'zgarib qolib bitta kanal ochiq, ikkinchisi yopiq bo'lardi — va bu
// AYNAN xavfsizlik qarori, shuning uchun nusxalanmasligi kerak.
func NewUpgrader(allowedOrigins []string) websocket.Upgrader {
	allowedSet := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		o = strings.TrimSpace(o)
		if o != "" {
			allowedSet[o] = struct{}{}
		}
	}
	if len(allowedSet) == 0 {
		slog.Warn("ws: ALLOWED_ORIGINS sozlanmagan — istalgan brauzer origin'idan WebSocket ulanishga ruxsat berilyapti (faqat dev uchun, production'dan oldin sozlanishi SHART)")
	}
	return websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true // native mobil ilova — Origin yubormaydi, bu me'yor
			}
			if len(allowedSet) == 0 {
				return true // sozlanmagan — dev fallback
			}
			_, ok := allowedSet[origin]
			return ok
		},
	}
}

// Serve — HTTP so'rovni WebSocket'ga ko'taradi va berilgan kalitlar ostida
// ro'yxatga oladi. Ulanish uzilguncha bloklaydi.
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, keys ...string) {
	// Bo'sh kalitlarni OLDINDAN tashlaymiz. Avval ular ro'yxatga
	// tushardi: mijozning `EntityID` si bo'sh bo'lgani uchun HAMMA
	// mijoz "" kaliti ostida to'planardi (xabar yetmasdi, chunki
	// `Send("")` no-op edi, lekin xarita bekorga shishardi).
	clean := make([]string, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		if k == "" {
			continue
		}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		clean = append(clean, k)
	}
	if len(clean) == 0 {
		http.Error(w, "obuna kaliti yo'q", http.StatusBadRequest)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &client{conn: conn, out: make(chan []byte, outBuffer)}

	h.mu.Lock()
	for _, k := range clean {
		if h.conns[k] == nil {
			h.conns[k] = make(map[*client]struct{})
		}
		h.conns[k][c] = struct{}{}
	}
	h.mu.Unlock()
	slog.Info("ws: ulandi", "keys", clean)

	go c.writeLoop()

	defer func() {
		h.mu.Lock()
		for _, k := range clean {
			delete(h.conns[k], c)
			if len(h.conns[k]) == 0 {
				delete(h.conns, k)
			}
		}
		h.mu.Unlock()
		c.stop() // writeLoop tugaydi va ulanishni yopadi
		slog.Info("ws: uzildi", "keys", clean)
	}()

	// Klientdan kelgan xabarlar hozircha ishlatilmaydi — o'qish sikli
	// uzilishni sezish VA pong'ni qabul qilish uchun.
	conn.SetReadLimit(4 << 10)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

// Send — kalit ostidagi barcha ulanishlarga JSON event yuboradi.
//
// BLOKLAMAYDI: xabar har bir ulanishning navbatiga qo'yiladi.
// Ulanish yo'q bo'lsa jimgina o'tib ketadi (foydalanuvchi offline).
func (h *Hub) Send(key string, v any) {
	if key == "" {
		return
	}
	// JSON BIR MARTA kodlanadi — avval har ulanish uchun alohida
	// kodlanardi (bir xil xabar 3 ta ulanishga ketsa, 3 marta).
	msg, err := json.Marshal(v)
	if err != nil {
		slog.Warn("ws: xabarni kodlab bo'lmadi", "key", key, "err", err)
		return
	}

	h.mu.RLock()
	clients := make([]*client, 0, len(h.conns[key]))
	for c := range h.conns[key] {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		if !c.enqueue(msg) {
			// Navbat to'ldi — klient juda sekin. Ulanishni yopamiz;
			// u qayta ulanib, o'qilmaganlarni HTTP orqali oladi.
			slog.Warn("ws: navbat to'ldi, ulanish yopildi", "key", key)
			c.stop()
		}
	}
}

// Online — shu kalit ostida ochiq ulanish bormi.
//
// Push (FCM) qatlami shuni tekshiradi: foydalanuvchi ilovani ochiq
// ushlab turgan bo'lsa, unga qo'shimcha push yuborish keraksiz
// bezovtalik bo'lardi.
func (h *Hub) Online(key string) bool {
	if key == "" {
		return false
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns[key]) > 0
}
