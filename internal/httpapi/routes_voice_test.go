package httpapi

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"chustapp/internal/orders"
)

type fakeTTS struct {
	mu    sync.Mutex
	texts []string
}

func (f *fakeTTS) Synthesize(_ context.Context, text string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.texts = append(f.texts, text)
	return []byte("RIFF" + text), nil
}

func (f *fakeTTS) VoiceID() string { return "test/Kore" }

func (f *fakeTTS) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.texts)
}

func TestCourierArrivalVoice(t *testing.T) {
	f := trackingServer(t)
	f.saveOrder(t, "o-voice", orders.StatusReady, false)
	path := "/couriers/k-track/voice/arrival?order_id=o-voice"

	w := do(t, f.h, "GET", path, f.jwt["courier"], "")
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "audio/wav" ||
		!strings.Contains(w.Body.String(), "Siz Book Cafe restoraniga yetib keldingiz.") {
		t.Fatalf("ovoz: %d %q %s", w.Code, w.Header().Get("Content-Type"), w.Body.String())
	}
	// Ikkinchi so'rov — keshdan, TTS'ga qayta bormaydi.
	if w := do(t, f.h, "GET", path, f.jwt["courier"], ""); w.Code != http.StatusOK || f.tts.count() != 1 {
		t.Fatalf("kesh ishlamadi: %d, sintez %d marta", w.Code, f.tts.count())
	}

	// Begona buyurtma va yakunlangan buyurtma — 404 (matnni kuryer tanlay olmaydi).
	if err := f.orders.Save(context.Background(), &orders.Order{
		ID: "o-begona", CustomerID: "u-track-c", RestaurantID: testRestA, CourierID: "k-boshqa",
		Status: orders.StatusReady, CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	f.saveOrder(t, "o-done", orders.StatusDelivered, false)
	for _, id := range []string{"o-begona", "o-done", ""} {
		if w := do(t, f.h, "GET", "/couriers/k-track/voice/arrival?order_id="+id, f.jwt["courier"], ""); w.Code != http.StatusNotFound {
			t.Errorf("%q: 404 kutilgan, keldi %d", id, w.Code)
		}
	}
	if f.tts.count() != 1 {
		t.Fatalf("rad etilgan so'rovlar TTS'ga borib qoldi: %d", f.tts.count())
	}

	for tok, want := range map[string]int{
		f.jwt["customer"]: http.StatusForbidden,
		"":                http.StatusUnauthorized,
	} {
		if w := do(t, f.h, "GET", path, tok, ""); w.Code != want {
			t.Errorf("kutilgan %d, keldi %d", want, w.Code)
		}
	}
	if w := do(t, f.h, "GET", "/couriers/k-boshqa/voice/arrival?order_id=o-begona", f.jwt["courier"], ""); w.Code != http.StatusForbidden {
		t.Fatalf("boshqa kuryer nomidan: 403 kutilgan, keldi %d", w.Code)
	}
}

func TestCourierArrivalVoiceDisabled(t *testing.T) {
	f := staffServer(t)
	_, _, c, tok := staffCourier(t, f, "+998901112260")
	if w := do(t, f.h, "GET", "/couriers/"+c.ID+"/voice/arrival?order_id=x", tok, ""); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("sozlanmagan: 503 kutilgan, keldi %d", w.Code)
	}
}
