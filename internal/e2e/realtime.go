package e2e

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"chustapp/internal/e2e/report"
	"chustapp/internal/notify"
)

// Feed — bitta ishtirokchining JONLI (WebSocket) xabarlar oqimi.
//
// ┌─ NEGA KERAK ──────────────────────────────────────────────────────┐
// Panel va ilovalar yangilanishni WebSocket orqali oladi. Bilet
// prefiksi nosozligi (brauzer klientlari jonli xabar olmay qolgan edi)
// aynan shu qatlamda yashirinib yotgan: HTTP testlari hammasi yashil
// edi. Bu yerda xabar HAQIQATDAN yetib kelishi tekshiriladi.
// └───────────────────────────────────────────────────────────────────┘
type Feed struct {
	name string
	conn *websocket.Conn
	mu   sync.Mutex
	msgs []map[string]any
	done chan struct{}
}

// Listen — ishtirokchi nomidan WebSocket ulaydi (bir martalik bilet bilan).
func (w *World) Listen(t *testing.T, a *Actor, query string) *Feed {
	t.Helper()
	tk := w.Do(t, a, "POST", "/ws/ticket", nil).Expect(t, 201).Map(t)
	ticket, _ := tk["ticket"].(string)
	if ticket == "" {
		t.Fatalf("bilet yo'q: %v", tk)
	}
	url := "ws" + strings.TrimPrefix(w.Srv.URL, "http") + "/ws?ticket=" + ticket
	if query != "" {
		url += "&" + query
	}
	start := time.Now()
	conn, resp, err := websocket.DefaultDialer.Dial(url, http.Header{})
	ms := float64(time.Since(start).Microseconds()) / 1000
	status := 101
	if resp != nil {
		status = resp.StatusCode
	}
	Rec.AddCall(report.Call{Scenario: w.scenario, Step: w.step, Method: "GET", Path: "/ws",
		Route: MatchRoute(Routes, "GET", "/ws"), Status: status, Ms: ms, Actor: a.Name})
	if err != nil {
		t.Fatalf("%s: WebSocket ulanmadi: %v", a.Name, err)
	}
	f := &Feed{name: a.Name, conn: conn, done: make(chan struct{})}
	go func() {
		defer close(f.done)
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var m map[string]any
			if json.Unmarshal(raw, &m) == nil {
				f.mu.Lock()
				f.msgs = append(f.msgs, m)
				f.mu.Unlock()
			}
		}
	}()
	// Yopish SSENARIY oxirida (qadam subtesti emas): aks holda ulanish
	// Listen chaqirilgan qadam tugashi bilan yopilib qolardi.
	w.mu.Lock()
	w.closers = append(w.closers, conn)
	w.mu.Unlock()
	// Ulanish serverda RO'YXATGA olinguncha kutamiz: aks holda darhol
	// keyingi hodisa yo'qolardi (haqiqiy ilova buni HTTP orqali qayta
	// yuklab oladi — bu yerda esa jonli yetkazishni sinaymiz).
	w.Eventually(t, a.Name+" WebSocket ro'yxatga olindi", 2*time.Second, func() bool {
		return w.Hub.Online(notify.User(a.UserID))
	})
	return f
}

// Wait — shart bajariladigan xabarni kutadi (allaqachon kelganlari ham hisobga olinadi).
func (f *Feed) Wait(t *testing.T, what string, timeout time.Duration, pred func(map[string]any) bool) map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		f.mu.Lock()
		for _, m := range f.msgs {
			if pred(m) {
				f.mu.Unlock()
				return m
			}
		}
		f.mu.Unlock()
		if time.Now().After(deadline) {
			closed := ""
			select {
			case <-f.done:
				closed = " — ULANISH YOPILGAN"
			default:
			}
			t.Fatalf("%s: jonli xabar %v ichida kelmadi (%s qabul qilganlari: %v)%s", what, timeout, f.name, f.types(), closed)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// Never — shu xabar KELMAGANINI tasdiqlaydi (maxfiylik/izolyatsiya).
func (f *Feed) Never(t *testing.T, what string, wait time.Duration, pred func(map[string]any) bool) {
	t.Helper()
	time.Sleep(wait)
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, m := range f.msgs {
		if pred(m) {
			t.Fatalf("%s: %s begona xabarni oldi: %v", what, f.name, m)
		}
	}
}

func (f *Feed) types() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.msgs))
	for _, m := range f.msgs {
		s, _ := m["type"].(string)
		out = append(out, s)
	}
	return out
}

// Dump — qabul qilingan xabarlar (nosozlikni topish uchun).
func (f *Feed) Dump() []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]map[string]any(nil), f.msgs...)
}
