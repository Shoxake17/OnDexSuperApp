// Package ws — WebSocket hub: har bir ulangan foydalanuvchiga jonli eventlar
// yuboradi. Kalit sifatida user ID va entity ID (kuryer/restoran) ishlatiladi.
package ws

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	// MVP: hamma origin qabul qilinadi; production'da o'z domenlarimizga cheklanadi.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// client — bitta WebSocket ulanish. gorilla/websocket bir vaqtda bitta yozuvchini
// talab qiladi, shuning uchun yozish mutex ostida.
type client struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (c *client) send(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.conn.WriteJSON(v)
}

type Hub struct {
	mu    sync.RWMutex
	conns map[string]map[*client]struct{} // kalit -> shu kalit ostidagi ulanishlar
}

func NewHub() *Hub {
	return &Hub{conns: make(map[string]map[*client]struct{})}
}

// Serve — HTTP so'rovni WebSocket'ga ko'taradi va berilgan kalitlar ostida
// ro'yxatga oladi. Ulanish uzilguncha bloklaydi.
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, keys ...string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &client{conn: conn}

	h.mu.Lock()
	for _, k := range keys {
		if k == "" {
			continue
		}
		if h.conns[k] == nil {
			h.conns[k] = make(map[*client]struct{})
		}
		h.conns[k][c] = struct{}{}
	}
	h.mu.Unlock()
	slog.Info("ws: ulandi", "keys", keys)

	defer func() {
		h.mu.Lock()
		for _, k := range keys {
			delete(h.conns[k], c)
			if len(h.conns[k]) == 0 {
				delete(h.conns, k)
			}
		}
		h.mu.Unlock()
		conn.Close()
		slog.Info("ws: uzildi", "keys", keys)
	}()

	// Klientdan kelgan xabarlar hozircha ishlatilmaydi — o'qish sikli faqat
	// uzilishni sezish uchun.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

// Send — kalit ostidagi barcha ulanishlarga JSON event yuboradi.
// Ulanish yo'q bo'lsa jimgina o'tib ketadi (foydalanuvchi offline).
func (h *Hub) Send(key string, v any) {
	if key == "" {
		return
	}
	h.mu.RLock()
	clients := make([]*client, 0, len(h.conns[key]))
	for c := range h.conns[key] {
		clients = append(clients, c)
	}
	h.mu.RUnlock()
	for _, c := range clients {
		if err := c.send(v); err != nil {
			c.conn.Close() // o'lik ulanish — read loop o'zi ro'yxatdan chiqaradi
		}
	}
}
