package alerts

import (
	"context"
	"errors"
	"hash/fnv"
	"log/slog"
	"sort"
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

	// shards — bitta restoranning bildirishnomalari QAT'IY tartibda
	// yoziladi va yuboriladi: har shard'da BITTA yozuvchi goroutine.
	//
	// ┌─ NEGA TARTIB KERAK ───────────────────────────────────────────┐
	// Ikki parallel hodisa: A seq=10 ni yozadi, B seq=11 ni yozadi va
	// A dan OLDIN yuboradi. Panel 11 ni ko'radi, keyin tarmoq uziladi
	// va A (10) yetib bormaydi. Qayta ulanganda panel "11 dan
	// keyingilarini ber" deydi — 10 abadiy tushib qoladi.
	// └───────────────────────────────────────────────────────────────┘
	//
	// ┌─ NEGA NAVBAT + PAKET (2026-11-14) ────────────────────────────┐
	// Avval mutex ostida har biri alohida INSERT + UnreadCount edi:
	// bitta restoranga sekundiga yuzlab hodisa kelsa (yuklama testi)
	// navbat bazani kutib 15 s dan oshdi -> 19 331 "yozilmadi".
	// Endi yozuvchi navbatdagilarning hammasini (maxBatch gacha) BITTA
	// INSERT bilan yozadi va restoran bo'yicha BITTA unread so'raydi:
	// yuk oshgan sari paket kattalashadi, navbat esa o'smaydi.
	// └───────────────────────────────────────────────────────────────┘
	shards [64]pubShard
}

const (
	maxBatch     = 200              // bitta INSERT'dagi eng ko'p yozuv
	maxQueue     = 10000            // shard navbati; oshsa ErrQueueFull
	batchTimeout = 10 * time.Second // bitta paketni yozish + unread
)

// ErrQueueFull — navbat to'la (baza uzoq vaqt javob bermayapti).
var ErrQueueFull = errors.New("bildirishnoma navbati to'la")

var errWriterCrashed = errors.New("bildirishnoma yozuvchisi yiqildi")

type pubRes struct {
	inserted bool
	err      error
}

type pubReq struct {
	ctx  context.Context
	n    *Notification
	done chan pubRes // bufer 1: yozuvchi hech qachon bloklanmaydi
}

func (q *pubReq) reply(r pubRes) {
	select {
	case q.done <- r:
	default: // allaqachon javob berilgan
	}
}

type pubShard struct {
	mu      sync.Mutex
	queue   []*pubReq
	running bool

	// sendMu — unread sanash + jonli yuborish (yozuvchi) va o'qish
	// (MarkRead/MarkAllRead) bir-birining orasiga tushmasin: belgi soni
	// eskisi bilan almashib qolmaydi. INSERT bu qulfdan TASHQARIDA.
	sendMu sync.Mutex
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

func (s *Service) shardFor(restaurantID string) *pubShard {
	h := fnv.New32a()
	_, _ = h.Write([]byte(restaurantID))
	return &s.shards[h.Sum32()%uint32(len(s.shards))]
}

// enqueue — navbatga qo'yadi, yozuvchi yo'q bo'lsa ishga tushiradi.
func (s *Service) enqueue(sh *pubShard, req *pubReq) error {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	if len(sh.queue) >= maxQueue {
		return ErrQueueFull
	}
	sh.queue = append(sh.queue, req)
	if !sh.running {
		sh.running = true
		safego.Go("alerts.writer", func() { s.drain(sh) })
	}
	return nil
}

// drain — shard yozuvchisi: navbat bo'shaguncha paketlab yozadi.
func (s *Service) drain(sh *pubShard) {
	var batch []*pubReq
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		// Yiqilsa ham navbat osilib qolmasin: kutayotganlarga xato,
		// keyingi Publish yangi yozuvchi ochadi.
		sh.mu.Lock()
		pending := append(batch, sh.queue...)
		sh.queue, sh.running = nil, false
		sh.mu.Unlock()
		for _, q := range pending {
			q.reply(pubRes{err: errWriterCrashed})
		}
		slog.Error("bildirishnoma yozuvchisi yiqildi", "panic", r)
	}()
	for {
		sh.mu.Lock()
		if len(sh.queue) == 0 {
			sh.running = false
			sh.mu.Unlock()
			return
		}
		k := min(len(sh.queue), maxBatch)
		batch = append(batch[:0], sh.queue[:k]...)
		rest := make([]*pubReq, len(sh.queue)-k)
		copy(rest, sh.queue[k:])
		sh.queue = rest
		sh.mu.Unlock()
		s.writeBatch(sh, batch)
		batch = batch[:0]
	}
}

// writeBatch — bitta INSERT, restoran bo'yicha bitta unread, keyin
// jonli hodisalar seq tartibida va oxirida chaqiruvchilarga javob.
func (s *Service) writeBatch(sh *pubShard, batch []*pubReq) {
	reqs := make([]*pubReq, 0, len(batch))
	ns := make([]*Notification, 0, len(batch))
	for _, q := range batch {
		if err := q.ctx.Err(); err != nil { // chaqiruvchi kutishdan voz kechgan
			q.reply(pubRes{err: err})
			continue
		}
		reqs = append(reqs, q)
		ns = append(ns, q.n)
	}
	if len(ns) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), batchTimeout)
	defer cancel()
	inserted, err := s.store.InsertBatch(ctx, ns)
	if err != nil {
		for _, q := range reqs {
			q.reply(pubRes{err: err})
		}
		return
	}
	if s.hub != nil {
		sh.sendMu.Lock()
		defer sh.sendMu.Unlock()
		// Paketdagi barcha hodisalar restoranning YAKUNIY unread sonini
		// oladi (belgi baribir oxirgi qiymatni ko'rsatadi).
		unread := map[string]int{}
		var order []int
		for i, n := range ns {
			if !inserted[i] {
				continue
			}
			order = append(order, i)
			if _, ok := unread[n.RestaurantID]; !ok {
				u, err := s.store.UnreadCount(ctx, n.RestaurantID)
				if err != nil {
					u = -1
				}
				unread[n.RestaurantID] = u
			}
		}
		sort.Slice(order, func(a, b int) bool { return ns[order[a]].Seq < ns[order[b]].Seq })
		for _, i := range order {
			n := ns[i]
			event := map[string]any{"type": EventNew, "notification": ToView(n)}
			if u := unread[n.RestaurantID]; u >= 0 {
				event["unread"] = u
			}
			s.hub.Send(Topic(n.RestaurantID), event)
		}
	}
	for i, q := range reqs {
		q.reply(pubRes{inserted: inserted[i]})
	}
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

	req := &pubReq{ctx: ctx, n: n, done: make(chan pubRes, 1)}
	if err := s.enqueue(s.shardFor(restaurantID), req); err != nil {
		return nil, err
	}
	select {
	case r := <-req.done:
		if r.err != nil || !r.inserted {
			return nil, r.err
		}
		return n, nil
	case <-ctx.Done():
		// Navbatda qolgan bo'lsa yozuvchi uni o'tkazib yuboradi; allaqachon
		// paketda bo'lsa yoziladi (takroriy urinishni DedupeKey ushlaydi).
		return nil, ctx.Err()
	}
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
	mu := &s.shardFor(restaurantID).sendMu
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
	mu := &s.shardFor(restaurantID).sendMu
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
