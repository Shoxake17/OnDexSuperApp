package notify

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"chustapp/internal/couriers"
)

type eventPusher struct {
	mu     sync.Mutex
	events []Event
}

func (p *eventPusher) Push(_ context.Context, _ []string, e Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, e)
	return nil
}

func (p *eventPusher) all() []Event {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]Event(nil), p.events...)
}

func (p *eventPusher) waitFor(t *testing.T, n int) []Event {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for len(p.all()) < n && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	events := p.all()
	if len(events) != n {
		t.Fatalf("%d ta push kutilgan, keldi %d", n, len(events))
	}
	return events
}

func courierLookup(_ context.Context, courierID string) (string, error) {
	if courierID == "k1" {
		return "u-k1", nil
	}
	return "", errors.New("kuryer akkaunti yo'q")
}

// Ilova yopiq (jonli kanal yo'q) — taklif push bilan boradi, tarixga
// YOZILMAYDI, muddati taklif muddatiga teng; taklif yopilsa signal ham
// o'chiriladi.
func TestCourierOfferPushWhenAppClosed(t *testing.T) {
	store := &memStore{}
	pusher := &eventPusher{}
	svc := newSvc(store).WithPush(pusher, &fakeTokens{tokens: []string{"tok-1"}})
	live := NewLive(svc).WithCourierUserLookup(courierLookup)

	before := time.Now()
	live.SendOffer("k1", couriers.OfferInfo{OrderID: "o1", RestaurantName: "Book Cafe", ExpiresIn: 20 * time.Second})
	e := pusher.waitFor(t, 1)[0]
	if e.Kind != courierOfferKind || e.TTL != 20*time.Second || e.Data["order_id"] != "o1" ||
		!strings.Contains(e.Body, "Book Cafe") {
		t.Fatalf("push mazmuni: %+v", e)
	}
	expires, err := strconv.ParseInt(e.Data["expires_at"], 10, 64)
	if err != nil || expires < before.Add(19*time.Second).UnixMilli() || expires > time.Now().Add(21*time.Second).UnixMilli() {
		t.Fatalf("expires_at noto'g'ri: %q", e.Data["expires_at"])
	}
	if store.count() != 0 {
		t.Fatal("20 soniyalik taklif bildirishnomalar tarixiga yozilmasligi kerak")
	}

	live.CancelOffer("k1", "o1")
	c := pusher.waitFor(t, 2)[1]
	if c.Kind != courierOfferCancelledKind || c.Data["order_id"] != "o1" {
		t.Fatalf("bekor qilish push'i: %+v", c)
	}

	ctx := context.Background()
	if !live.CanPush(ctx, "k1") {
		t.Fatal("tokeni bor kuryer uyg'otilishi kerak")
	}
	if live.CanPush(ctx, "begona") {
		t.Fatal("akkaunti topilmagan kuryer 'yetib boradi' deb hisoblandi")
	}
	if live.Online("k1") {
		t.Fatal("hub yo'q — kuryer onlayn bo'lishi mumkin emas")
	}
}

// FCM sozlanmagan — push ham, "yetib boradi" ham yo'q.
func TestCourierOfferWithoutFCM(t *testing.T) {
	live := NewLive(newSvc(&memStore{})).WithCourierUserLookup(courierLookup)
	live.SendOffer("k1", couriers.OfferInfo{OrderID: "o1", ExpiresIn: 20 * time.Second})
	live.CancelOffer("k1", "o1")
	if live.CanPush(context.Background(), "k1") {
		t.Fatal("FCM yo'q bo'lsa CanPush false bo'lishi kerak")
	}
}

// Taklif — faqat ma'lumot (data-only): Android o'zi qisqa ovoz bilan
// ko'rsatmasin, ilova takrorlanuvchi signal chiqarsin.
func TestCourierOfferFCMMessageIsDataOnly(t *testing.T) {
	msg := fcmMessage("tok", Event{
		Module: ModuleFood, Kind: courierOfferKind, Title: "Yangi buyurtma", Body: "Book Cafe",
		Data: map[string]string{"order_id": "o1", "expires_at": "123"}, TTL: 19500 * time.Millisecond,
	})
	if _, has := msg["notification"]; has {
		t.Fatal("taklif push'ida notification bloki bo'lmasligi kerak")
	}
	android := msg["android"].(map[string]any)
	if android["ttl"] != "20s" || android["priority"] != "high" {
		t.Fatalf("android: %v", android)
	}
	if _, has := android["notification"]; has {
		t.Fatal("android.notification bo'lmasligi kerak")
	}
	data := msg["data"].(map[string]string)
	if data["kind"] != courierOfferKind || data["title"] != "Yangi buyurtma" || data["body"] != "Book Cafe" ||
		data["order_id"] != "o1" || data["expires_at"] != "123" {
		t.Fatalf("data: %v", data)
	}

	other := fcmMessage("tok", Event{Kind: "order_status", Title: "T", Body: "B"})
	if _, has := other["notification"]; !has {
		t.Fatal("oddiy xabarda notification bloki qolishi kerak")
	}
	if _, has := other["android"].(map[string]any)["ttl"]; has {
		t.Fatal("muddatsiz xabarga ttl qo'shilmasligi kerak")
	}
}
