package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// `internal/ws` testlari (bug.md 4, 34, 104-bandlar).
//
// ┌─ NEGA BU PAKETDA TEST BO'LISHI SHART ──────────────────────────────┐
// Bu paket butun jonli qatlamning yuragi: bir nechta goroutine bitta
// ulanishga bir vaqtda yozadi. 34-banddagi nosozlik (yopilgan kanalga
// yozish → panic → BUTUN jarayon qulaydi) aynan shu yerda, testsiz
// joyda yashagan. Quyidagi testlar ulanishni HAQIQATAN ko'taradi —
// mock emas, chunki nosozlik mock qilib bo'lmaydigan qismda edi.
// └────────────────────────────────────────────────────────────────────┘

// dialHub — hub'ga haqiqiy WebSocket ulanishini ochadi.
func dialHub(t *testing.T, h *Hub, keys ...string) (*websocket.Conn, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.Serve(w, r, keys...)
	}))
	conn, _, err := websocket.DefaultDialer.Dial(
		"ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		srv.Close()
		t.Fatalf("ulanib bo'lmadi: %v", err)
	}
	t.Cleanup(func() {
		conn.Close()
		srv.Close()
	})
	// Ro'yxatga olinishini kutamiz — `Serve` uni goroutine ichida
	// bajaradi, ulanish o'rnatilishi bilan bir vaqtda emas.
	waitOnline(t, h, keys[0])
	return conn, srv
}

func waitOnline(t *testing.T, h *Hub, key string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if h.Online(key) {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("ulanish %q kaliti ostida ro'yxatga olinmadi", key)
}

// Asosiy oqim: kalit ostida yuborilgan xabar shu kalitga obuna
// ulanishga yetib boradi, boshqasiga bormaydi.
func TestHubSendReachesOnlySubscribers(t *testing.T) {
	h := NewHub(nil)
	conn, _ := dialHub(t, h, "u:alice")

	h.Send("u:bob", map[string]any{"type": "boshqaga"})
	h.Send("u:alice", map[string]any{"type": "salom"})

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("xabar o'qilmadi: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(msg, &got); err != nil {
		t.Fatal(err)
	}
	// Birinchi kelgan xabar "salom" bo'lishi kerak: "boshqaga" hech
	// qachon bu ulanishga yozilmaydi.
	if got["type"] != "salom" {
		t.Fatalf("begona kalitning xabari keldi: %v", got)
	}
}

// Bo'sh kalitlar tashlanadi va hech qanday kalit qolmasa ulanish
// ko'tarilmaydi (avval hamma mijoz "" kaliti ostida to'planardi).
func TestHubRejectsEmptyKeys(t *testing.T) {
	h := NewHub(nil)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.Serve(w, r, "", "")
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bo'sh kalit bilan ulanish ko'tarildi: status %d", resp.StatusCode)
	}
	if h.Online("") {
		t.Fatal(`"" kaliti ostida ulanish ro'yxatga olindi`)
	}
}

// 34-BANDNING ASOSIY REGRESSIYASI.
//
// Bitta ulanish bir necha kalitga obuna (`routes_ws.go` shunday
// qiladi). Ikki goroutine ikki xil kalitga bir vaqtda `Send` qiladi;
// navbat to'lgach biri ulanishni yopadi. Avval bu `close(out)` edi va
// ikkinchi goroutine YOPILGAN kanalga yozib panic berardi — panic esa
// `notify` ning recover'siz goroutine'ida BUTUN jarayonni yiqitardi.
//
// ┌─ NEGA HUB DARAJASIDA EMAS, `client` DARAJASIDA ────────────────────┐
// Bu poygani haqiqiy ulanish orqali chaqirish ISHONCHSIZ: ulanish
// uzilgani sezilishi bilan klient hub xaritasidan chiqariladi va
// keyingi `Send` uni umuman topmaydi — oyna juda tor. Shuning uchun
// invariant AYNAN o'zi turgan joyda o'lchanadi: `stop()` bilan
// `enqueue()` bir vaqtda ishlaganda panic BO'LMASLIGI kerak.
//
// Tekshirildi: `stop()` ni eski holiga (`close(c.out)`) qaytarilsa,
// bu test darhol panic bilan yiqiladi. Pastdagi hub darajasidagi
// test esa o'sha holatda ham o'tib ketardi.
// └────────────────────────────────────────────────────────────────────┘
//
// Panic test jarayonini o'ldiradi, ya'ni testning muvaffaqiyatli
// tugashi — tuzatishning isboti.
func TestClientEnqueueDuringStopNeverPanics(t *testing.T) {
	// Ko'p takrorlash: poyga bir urinishda tug'ilmasligi mumkin.
	for round := 0; round < 500; round++ {
		c := &client{out: make(chan []byte, 4), done: make(chan struct{})}

		var wg sync.WaitGroup
		start := make(chan struct{})
		// Ikki "Send" goroutine'i — ikki xil kalitdan kelgan xabarlar.
		for range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				for range 50 {
					c.enqueue([]byte("xabar"))
				}
			}()
		}
		// Uchinchi goroutine — navbat to'lganini ko'rib ulanishni yopadi.
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			c.stop()
		}()

		close(start)
		wg.Wait()
	}
}

// Hub darajasidagi tekshiruv: uzilgan ulanishga ko'p kalitdan bir
// vaqtda yuborish serverni yiqitmaydi va `Send` bloklanmaydi.
func TestHubConcurrentSendAfterCloseDoesNotPanic(t *testing.T) {
	h := NewHub(nil)
	conn, _ := dialHub(t, h, "u:alice", "e:food:rest-a", "o:ord-1")

	// Klient O'QIMAYDI — navbat (outBuffer=64) to'lib, `Send` ulanishni
	// yopishga majbur bo'ladi. Aynan shu nuqtada poyga tug'ilardi.
	conn.UnderlyingConn().Close()

	var wg sync.WaitGroup
	for _, key := range []string{"u:alice", "e:food:rest-a", "o:ord-1"} {
		wg.Add(1)
		go func(k string) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				h.Send(k, map[string]any{"type": "spam", "i": i})
			}
		}(key)
	}
	wg.Wait()
}

// `stop()` bir necha marta chaqirilsa ham panic bermasligi kerak
// (`Send` uni har bir kalit uchun alohida chaqirishi mumkin).
func TestClientStopIsIdempotent(t *testing.T) {
	c := &client{out: make(chan []byte, 1), done: make(chan struct{})}
	c.stop()
	c.stop()
	c.stop()

	// Yopilgandan keyin navbatga qo'yish xato emas — xabar shunchaki
	// tashlanadi (ulanish baribir yopilgan).
	if !c.enqueue([]byte("x")) {
		t.Fatal("yopilgan ulanishga enqueue false qaytardi — chaqiruvchi qayta stop() qiladi")
	}
}

// Navbat to'lganda `enqueue` false qaytaradi — chaqiruvchi shunda
// ulanishni yopadi. Bu xulq saqlanib qolishi kerak, aks holda sekin
// klient butun serverni ushlab turadi.
func TestEnqueueReportsFullQueue(t *testing.T) {
	c := &client{out: make(chan []byte, 2), done: make(chan struct{})}
	if !c.enqueue([]byte("1")) || !c.enqueue([]byte("2")) {
		t.Fatal("bo'sh navbatga qo'yib bo'lmadi")
	}
	if c.enqueue([]byte("3")) {
		t.Fatal("to'lgan navbat true qaytardi")
	}
}

// Origin siyosati: brauzer origin'i ro'yxatda bo'lmasa rad etiladi,
// Origin header UMUMAN bo'lmasa (native mobil ilova) qabul qilinadi.
func TestUpgraderOriginPolicy(t *testing.T) {
	up := NewUpgrader([]string{"https://ondex.uz"})

	cases := []struct {
		origin string
		want   bool
	}{
		{"", true},                      // native ilova
		{"https://ondex.uz", true},      // ruxsat etilgan
		{"https://evil.example", false}, // begona sayt
		{"http://ondex.uz", false},      // sxema ham muhim
	}
	for _, tc := range cases {
		r := httptest.NewRequest("GET", "/ws", nil)
		if tc.origin != "" {
			r.Header.Set("Origin", tc.origin)
		}
		if got := up.CheckOrigin(r); got != tc.want {
			t.Fatalf("origin %q: kutilgan %v, olingan %v", tc.origin, tc.want, got)
		}
	}
}

// Sozlanmagan holat (dev): hamma origin qabul qilinadi.
func TestUpgraderWithoutAllowlistAcceptsAll(t *testing.T) {
	up := NewUpgrader(nil)
	r := httptest.NewRequest("GET", "/ws", nil)
	r.Header.Set("Origin", "https://evil.example")
	if !up.CheckOrigin(r) {
		t.Fatal("dev fallback ishlamadi")
	}
}
