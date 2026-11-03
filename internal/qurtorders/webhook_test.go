package qurtorders

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func testOrder() *Order {
	return &Order{
		ID: "o1", Number: "Q-AAAA0001", StoreID: "S1", Status: StatusNew, TotalTiyin: 2400000, Version: 1,
		CustomerName: "Ali Valiyev", CustomerPhone: "+998901112233",
		Address: Address{Text: "Chust, Navoiy 5"},
	}
}

func TestNewPOSNotifierValidation(t *testing.T) {
	bad := map[string][2]string{
		"http (localhost emas)": {"http://pos.example.com/hook", testSecret},
		"qisqa sir":             {"https://pos.example.com/hook", "qisqa"},
		"manzilsiz":             {"", testSecret},
		"sxemasiz":              {"pos.example.com/hook", testSecret},
		"ftp":                   {"ftp://pos.example.com/hook", testSecret},
	}
	for name, c := range bad {
		if _, err := NewPOSNotifier(c[0], c[1]); err == nil {
			t.Errorf("%s: rad etilishi kerak edi", name)
		}
	}
	for _, u := range []string{"https://pos.example.com/hook", "http://localhost:3300/hook", "http://127.0.0.1:3300/hook"} {
		if _, err := NewPOSNotifier(u, testSecret); err != nil {
			t.Errorf("%s: qabul qilinishi kerak edi: %v", u, err)
		}
	}
}

func TestNotifySignsAndHidesPII(t *testing.T) {
	got := make(chan *http.Request, 1)
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		got <- r
		w.WriteHeader(204)
	}))
	defer srv.Close()
	n, err := NewPOSNotifier(srv.URL, testSecret)
	if err != nil {
		t.Fatal(err)
	}
	n.Notify(nil, "created", testOrder())

	select {
	case r := <-got:
		ts := r.Header.Get("X-Ondex-Timestamp")
		if want := "sha256=" + Sign(testSecret, ts, body); r.Header.Get("X-Ondex-Signature") != want {
			t.Fatalf("imzo noto'g'ri: %q != %q", r.Header.Get("X-Ondex-Signature"), want)
		}
		// Boshqa sir bilan imzo mos kelmaydi.
		if Sign("boshqa-sir-boshqa-sir-boshqa-sir-1", ts, body) == Sign(testSecret, ts, body) {
			t.Fatal("imzo siqa bog'liq emas")
		}
		var p WebhookPayload
		if err := json.Unmarshal(body, &p); err != nil {
			t.Fatal(err)
		}
		if p.Event != "created" || p.OrderID != "o1" || p.StoreID != "S1" || p.EventID != "o1:created:1" || p.TotalTiyin != 2400000 {
			t.Fatalf("tana: %+v", p)
		}
		for _, secret := range []string{"Ali", "Valiyev", "+99890", "Navoiy", "phone", "address", "customer"} {
			if strings.Contains(string(body), secret) {
				t.Fatalf("xabarga shaxsiy ma'lumot sizdi (%q): %s", secret, body)
			}
		}
	case <-time.After(3 * time.Second):
		t.Fatal("xabar kelmadi")
	}
}

func TestNotifyOnlyStoreRelevantEvents(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(204)
	}))
	defer srv.Close()
	n, _ := NewPOSNotifier(srv.URL, testSecret)
	for _, ev := range []string{"confirmed", "packed", "collected", "out_for_delivery", "delivered", "rejected", "failed"} {
		n.Notify(nil, ev, testOrder())
	}
	time.Sleep(300 * time.Millisecond)
	if atomic.LoadInt32(&hits) != 0 {
		t.Fatalf("do'konga keraksiz voqealar yuborildi: %d", hits)
	}
	n.Notify(nil, "cancelled", testOrder())
	time.Sleep(300 * time.Millisecond)
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("bekor qilish yuborilishi kerak edi: %d", hits)
	}
}

func TestNotifyRetriesThenSucceeds(t *testing.T) {
	var hits int32
	done := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&hits, 1) < 3 {
			w.WriteHeader(502)
			return
		}
		w.WriteHeader(204)
		done <- struct{}{}
	}))
	defer srv.Close()
	n, _ := NewPOSNotifier(srv.URL, testSecret)
	n.backoff = []time.Duration{10 * time.Millisecond, 10 * time.Millisecond}
	n.Notify(nil, "created", testOrder())
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("qayta urinish ishlamadi (urinishlar: %d)", atomic.LoadInt32(&hits))
	}
}

func TestNotifyDoesNotFollowRedirects(t *testing.T) {
	var evil int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { atomic.AddInt32(&evil, 1) }))
	defer target.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer srv.Close()
	n, _ := NewPOSNotifier(srv.URL, testSecret)
	n.backoff = nil
	if err := n.post([]byte(`{}`)); err == nil {
		t.Fatal("yo'naltirish xato bo'lishi kerak edi")
	}
	if atomic.LoadInt32(&evil) != 0 {
		t.Fatal("imzolangan xabar boshqa manzilga yo'naltirildi")
	}
}
