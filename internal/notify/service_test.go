package notify

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// Bildirishnoma qatlami testlari.
//
// Eng muhimi: xabar AVVAL SAQLANADI, keyin yuboriladi — soket o'lik
// bo'lsa ham foydalanuvchi uni keyin ko'radi.

type memStore struct {
	mu    sync.Mutex
	items []*Notification
	fail  bool
}

func (s *memStore) Save(_ context.Context, n *Notification) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return errors.New("saqlab bo'lmadi")
	}
	cp := *n
	s.items = append(s.items, &cp)
	return nil
}
func (s *memStore) List(context.Context, string, int) ([]*Notification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*Notification(nil), s.items...), nil
}
func (s *memStore) UnreadCount(context.Context, string) (int, error) { return 0, nil }
func (s *memStore) MarkRead(context.Context, string, string) error   { return nil }
func (s *memStore) MarkAllRead(context.Context, string) error        { return nil }

func (s *memStore) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.items)
}

type fakePusher struct {
	mu     sync.Mutex
	sent   int
	tokens []string
}

func (p *fakePusher) Push(_ context.Context, tokens []string, _ Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sent++
	p.tokens = append(p.tokens, tokens...)
	return nil
}
func (p *fakePusher) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.sent
}

type fakeTokens struct{ tokens []string }

func (f *fakeTokens) SaveToken(context.Context, string, string, string) error { return nil }
func (f *fakeTokens) DeleteToken(context.Context, string) error               { return nil }
func (f *fakeTokens) TokensFor(context.Context, string) ([]string, error) {
	return f.tokens, nil
}

func newSvc(store Store) *Service {
	n := 0
	// Hub `nil` — WebSocket testda kerak emas, `Service` uni
	// tekshiradi (nil-xavfsiz).
	return NewService(store, nil, func() string { n++; return "n" + string(rune('0'+n)) })
}

// ★ ASOSIY TEST: xabar SAQLANADI.
//
// Avval bildirishnoma faqat WebSocket orqali ketardi va soket o'lik
// bo'lsa IZSIZ yo'qolardi.
func TestNotifyPersistsBeforeDelivery(t *testing.T) {
	store := &memStore{}
	svc := newSvc(store)

	svc.Notify(context.Background(), "u1", Event{
		Module: ModuleFood, Kind: "order_status",
		Title: "Buyurtma", Body: "Tayyor",
		Data: map[string]string{"order_id": "o1"},
	})

	if store.count() != 1 {
		t.Fatalf("bildirishnoma saqlanmadi (%d ta)", store.count())
	}
	items, _ := store.List(context.Background(), "u1", 10)
	got := items[0]
	if got.UserID != "u1" || got.Module != ModuleFood || got.Kind != "order_status" {
		t.Fatalf("noto'g'ri yozildi: %+v", got)
	}
	if got.Data["order_id"] != "o1" {
		t.Fatalf("data yo'qoldi: %+v", got.Data)
	}
	if got.CreatedAt.IsZero() {
		t.Error("created_at to'ldirilmagan")
	}
}

// Bo'sh foydalanuvchi ID — jimgina o'tib ketadi (masalan buyurtmada
// hali kuryer biriktirilmagan).
func TestNotifySkipsEmptyUser(t *testing.T) {
	store := &memStore{}
	newSvc(store).Notify(context.Background(), "", Event{Kind: "x"})
	if store.count() != 0 {
		t.Fatal("bo'sh foydalanuvchi uchun yozuv yaratildi")
	}
}

// Saqlash muvaffaqiyatsiz bo'lsa ham panika bo'lmasligi va yuborish
// davom etishi kerak.
func TestNotifySurvivesStoreFailure(t *testing.T) {
	store := &memStore{fail: true}
	svc := newSvc(store)
	svc.Notify(context.Background(), "u1", Event{Kind: "x"})
	// Panika bo'lmadi — test shu bilan o'tdi.
}

// ★ PUSH: ilova YOPIQ bo'lsa yuboriladi.
func TestPushSentWhenOffline(t *testing.T) {
	store := &memStore{}
	pusher := &fakePusher{}
	svc := newSvc(store).WithPush(pusher, &fakeTokens{tokens: []string{"t1", "t2"}})

	svc.Notify(context.Background(), "u1", Event{Kind: "order_status"})

	// Yuborish alohida goroutine'da — kutamiz.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && pusher.count() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if pusher.count() != 1 {
		t.Fatalf("push yuborilmadi (%d)", pusher.count())
	}
	if len(pusher.tokens) != 2 {
		t.Fatalf("ikkala qurilmaga ham yuborilishi kerak edi: %v", pusher.tokens)
	}
}

// Push sozlanmagan bo'lsa (kalit yo'q) hamma narsa baribir ishlaydi.
func TestWorksWithoutPusher(t *testing.T) {
	store := &memStore{}
	svc := newSvc(store) // WithPush CHAQIRILMADI
	svc.Notify(context.Background(), "u1", Event{Kind: "x"})
	if store.count() != 1 {
		t.Fatal("push yo'qligi saqlashni buzdi")
	}
}

// ---------- Kalit sxemasi ----------

// Turli tipdagi kalitlar HECH QACHON kesishmasligi kerak — aks holda
// bir foydalanuvchi boshqasining xabarini olardi.
func TestTopicsNeverCollide(t *testing.T) {
	const id = "abc123"
	keys := []string{
		User(id),
		Entity(ModuleFood, id),
		Entity(ModuleShop, id),
		Order(id),
	}
	seen := map[string]bool{}
	for _, k := range keys {
		if k == "" {
			t.Fatal("kalit bo'sh chiqdi")
		}
		if seen[k] {
			t.Fatalf("kalitlar to'qnashdi: %q (%v)", k, keys)
		}
		seen[k] = true
	}
	// Xom ID hech qachon kalit bo'lmasligi kerak.
	for _, k := range keys {
		if k == id {
			t.Fatal("xom ID kalit sifatida ishlatilyapti")
		}
	}
}

// Bo'sh kirish — bo'sh kalit (hub uni tashlab yuboradi).
func TestTopicsRejectEmpty(t *testing.T) {
	if User("") != "" || Order("") != "" ||
		Entity("", "x") != "" || Entity("food", "") != "" {
		t.Fatal("bo'sh kirish uchun bo'sh bo'lmagan kalit qaytdi")
	}
}
