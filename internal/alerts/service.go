package alerts

import (
	"context"
	"hash/fnv"
	"log/slog"
	"strings"
	"sync"
	"time"

	"chustapp/internal/notify"
	"chustapp/internal/orders"
	"chustapp/internal/safego"
)

// WebSocket hodisa turlari (manager kanalida).
const (
	EventNew  = "restaurant_notification"
	EventRead = "restaurant_notifications_read"
)

// Sender — `ws.Hub` (testda soxta).
type Sender interface {
	Send(key string, v any)
}

// Topic — restoran RAHBARIYATI kanali. Affitsiant unga obuna bo'lmaydi
// (`httpapi.managerTopicIfRestaurant`): to'lovlar, xodimlar va hisobot
// faqat restoran akkauntiga.
func Topic(restaurantID string) string { return notify.Manager(notify.ModuleFood, restaurantID) }

type Service struct {
	store Store
	hub   Sender
	idgen func() string
	now   func() time.Time

	// orderLookup — to'lov/kuryer hodisasida buyurtma RAQAMINI topish.
	orderLookup func(ctx context.Context, id string) (*orders.Order, error)

	// locks — bitta restoranning bildirishnomalari QAT'IY tartibda
	// yoziladi va yuboriladi.
	//
	// ┌─ NEGA KERAK ──────────────────────────────────────────────────┐
	// Ikki parallel hodisa: A seq=10 ni yozadi, B seq=11 ni yozadi va
	// A dan OLDIN yuboradi. Panel 11 ni ko'radi, keyin tarmoq uziladi
	// va A (10) yetib bormaydi. Qayta ulanganda panel "11 dan
	// keyingilarini ber" deydi — 10 abadiy tushib qoladi.
	//
	// Yozish+yuborish restoran bo'yicha ketma-ket bo'lsa, ulanishga
	// xabarlar seq tartibida boradi va "oxirgi olingan raqamdan
	// keyingilari" so'rovi hech qachon bo'shliq qoldirmaydi.
	// └───────────────────────────────────────────────────────────────┘
	locks [64]sync.Mutex
}

func NewService(store Store, hub Sender, idgen func() string) *Service {
	return &Service{store: store, hub: hub, idgen: idgen, now: time.Now}
}

// WithClock — vaqt manbai (testlar uchun).
func (s *Service) WithClock(now func() time.Time) *Service {
	s.now = now
	return s
}

// WithOrders — buyurtma raqamini topish uchun (`orders.Repository.GetByID`).
func (s *Service) WithOrders(lookup func(ctx context.Context, id string) (*orders.Order, error)) *Service {
	s.orderLookup = lookup
	return s
}

func (s *Service) lockFor(restaurantID string) *sync.Mutex {
	h := fnv.New32a()
	_, _ = h.Write([]byte(restaurantID))
	return &s.locks[h.Sum32()%uint32(len(s.locks))]
}

// Input — yangi bildirishnoma.
type Input struct {
	Kind      string
	Category  Category
	Title     string
	Body      string
	Data      map[string]string
	DedupeKey string
}

// Publish — yozadi, keyin manager kanaliga yuboradi. Takroriy
// (`DedupeKey` bor) bo'lsa (nil, nil).
func (s *Service) Publish(ctx context.Context, restaurantID string, in Input) (*Notification, error) {
	restaurantID = strings.TrimSpace(restaurantID)
	if restaurantID == "" {
		return nil, invalid("restaurant_id bo'sh")
	}
	if _, err := ParseCategory(string(in.Category)); err != nil || in.Category == "" {
		return nil, invalid("bildirishnoma turi noto'g'ri")
	}
	title := sanitize(in.Title, MaxTitleLen)
	if title == "" {
		return nil, invalid("sarlavha bo'sh")
	}
	data := map[string]string{}
	for k, v := range in.Data {
		if dataKeys[k] && v != "" {
			data[k] = sanitize(v, 80)
		}
	}
	n := &Notification{
		ID: s.idgen(), RestaurantID: restaurantID, Kind: sanitize(in.Kind, 40), Category: in.Category,
		Title: title, Body: sanitize(in.Body, MaxBodyLen), Data: data, CreatedAt: s.now(),
	}
	n.DedupeKey = strings.TrimSpace(in.DedupeKey)
	if n.DedupeKey == "" {
		n.DedupeKey = "id:" + n.ID
	}

	mu := s.lockFor(restaurantID)
	mu.Lock()
	defer mu.Unlock()
	inserted, err := s.store.Insert(ctx, n)
	if err != nil || !inserted {
		return nil, err
	}
	if s.hub != nil {
		event := map[string]any{"type": EventNew, "notification": ToView(n)}
		if unread, err := s.store.UnreadCount(ctx, restaurantID); err == nil {
			event["unread"] = unread
		}
		s.hub.Send(Topic(restaurantID), event)
	}
	return n, nil
}

// publishAsync — hodisa manbai (buyurtma yaratish, to'lov callback'i)
// bildirishnoma yozilishini KUTMAYDI va uning xatosidan yiqilmaydi.
func (s *Service) publishAsync(restaurantID string, build func(ctx context.Context) (Input, bool)) {
	if s == nil || restaurantID == "" {
		return
	}
	safego.Go("alerts.publish", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		in, ok := build(ctx)
		if !ok {
			return
		}
		if _, err := s.Publish(ctx, restaurantID, in); err != nil {
			slog.Error("bildirishnoma yozilmadi", "restaurant", restaurantID, "kind", in.Kind, "err", err)
		}
	})
}

func clampLimit(n int) int {
	if n <= 0 {
		return 30
	}
	if n > MaxLimit {
		return MaxLimit
	}
	return n
}

func (s *Service) List(ctx context.Context, restaurantID string, q Query) ([]*Notification, error) {
	q.Limit = clampLimit(q.Limit)
	q.Search = strings.TrimSpace(q.Search)
	if len([]rune(q.Search)) > MaxSearchLen {
		return nil, invalid("qidiruv matni juda uzun")
	}
	return s.store.List(ctx, restaurantID, q)
}

func (s *Service) Counts(ctx context.Context, restaurantID string, since time.Time) (Counts, error) {
	return s.store.Counts(ctx, restaurantID, since)
}

// Summary — qo'ng'iroq belgisi va qayta ulanish uchun.
func (s *Service) Summary(ctx context.Context, restaurantID string) (unread int, latestSeq int64, err error) {
	if unread, err = s.store.UnreadCount(ctx, restaurantID); err != nil {
		return 0, 0, err
	}
	latestSeq, err = s.store.LatestSeq(ctx, restaurantID)
	return unread, latestSeq, err
}

// MarkRead — boshqa ochiq panellar ham (bir nechta kompyuter) belgini
// darhol yangilaydi.
func (s *Service) MarkRead(ctx context.Context, restaurantID, id string) (int, error) {
	mu := s.lockFor(restaurantID)
	mu.Lock()
	defer mu.Unlock()
	changed, err := s.store.MarkRead(ctx, restaurantID, id, s.now())
	if err != nil {
		return 0, err
	}
	unread, err := s.store.UnreadCount(ctx, restaurantID)
	if err != nil {
		return 0, err
	}
	if changed && s.hub != nil {
		s.hub.Send(Topic(restaurantID), map[string]any{"type": EventRead, "id": id, "unread": unread})
	}
	return unread, nil
}

func (s *Service) MarkAllRead(ctx context.Context, restaurantID string, upToSeq int64) (int, int, error) {
	if upToSeq <= 0 {
		return 0, 0, invalid("up_to_seq musbat bo'lishi kerak")
	}
	mu := s.lockFor(restaurantID)
	mu.Lock()
	defer mu.Unlock()
	updated, err := s.store.MarkAllRead(ctx, restaurantID, upToSeq, s.now())
	if err != nil {
		return 0, 0, err
	}
	unread, err := s.store.UnreadCount(ctx, restaurantID)
	if err != nil {
		return 0, 0, err
	}
	if updated > 0 && s.hub != nil {
		s.hub.Send(Topic(restaurantID), map[string]any{"type": EventRead, "up_to_seq": upToSeq, "unread": unread})
	}
	return updated, unread, nil
}
