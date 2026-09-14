package alerts_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"chustapp/internal/alerts"
	"chustapp/internal/orders"
	"chustapp/internal/staff"
	"chustapp/internal/storage"
)

type sent struct {
	key   string
	event map[string]any
}

type fakeHub struct {
	mu   sync.Mutex
	sent []sent
}

func (h *fakeHub) Send(key string, v any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sent = append(h.sent, sent{key, v.(map[string]any)})
}

func (h *fakeHub) snapshot() []sent {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]sent(nil), h.sent...)
}

type innerNotifier struct{ created, changed int }

func (n *innerNotifier) OrderCreated(*orders.Order)                      { n.created++ }
func (n *innerNotifier) OrderStatusChanged(*orders.Order, orders.Status) { n.changed++ }

var tashkent = alerts.Location

func newSvc(now time.Time) (*alerts.Service, *storage.MemoryAlertStore, *fakeHub) {
	store := storage.NewMemoryAlertStore()
	hub := &fakeHub{}
	i := 0
	var mu sync.Mutex
	svc := alerts.NewService(store, hub, func() string {
		mu.Lock()
		defer mu.Unlock()
		i++
		return fmt.Sprintf("n%03d", i)
	}).WithClock(func() time.Time { return now })
	return svc, store, hub
}

func list(t *testing.T, svc *alerts.Service, rid string) []*alerts.Notification {
	t.Helper()
	items, err := svc.List(context.Background(), rid, alerts.Query{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	return items
}

// eventually — asinxron yozuv (hook'lar o'z goroutine'ida ishlaydi).
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("kutilgan holat bo'lmadi: %s", what)
}

func TestPublishDedupeWhitelistAndLiveEvent(t *testing.T) {
	svc, _, hub := newSvc(time.Now())
	ctx := context.Background()
	in := alerts.Input{
		Kind: alerts.KindNewOrder, Category: alerts.CategoryNew,
		Title: "  Yangi\u202e buyurtma ", Body: "Jami: 180 000 so'm",
		Data:      map[string]string{"order_id": "o1", "phone": "+998901234567", "token": "x"},
		DedupeKey: "new_order:o1",
	}
	n, err := svc.Publish(ctx, "r1", in)
	if err != nil || n == nil || n.Seq != 1 {
		t.Fatalf("birinchi: %+v %v", n, err)
	}
	if n.Title != "Yangi buyurtma" {
		t.Fatalf("boshqaruv belgisi tozalanmadi: %q", n.Title)
	}
	if len(n.Data) != 1 || n.Data["order_id"] != "o1" {
		t.Fatalf("data oq ro'yxati: %+v", n.Data)
	}
	if dup, err := svc.Publish(ctx, "r1", in); err != nil || dup != nil {
		t.Fatalf("takror yaratildi: %+v %v", dup, err)
	}
	// Boshqa restoranda xuddi shu kalit — alohida hodisa.
	if other, _ := svc.Publish(ctx, "r2", in); other == nil {
		t.Fatal("boshqa restoranga yozilmadi")
	}
	got := hub.snapshot()
	if len(got) != 2 || got[0].key != "m:food:r1" || got[0].event["type"] != alerts.EventNew || got[0].event["unread"] != 1 {
		t.Fatalf("jonli hodisa: %+v", got)
	}
	if _, err := svc.Publish(ctx, "r1", alerts.Input{Kind: "x", Category: "bogus", Title: "t"}); err == nil {
		t.Fatal("noma'lum tur qabul qilindi")
	}
}

func TestPublishOrderIsSequential(t *testing.T) {
	svc, _, hub := newSvc(time.Now())
	var wg sync.WaitGroup
	for i := 0; i < 60; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = svc.Publish(context.Background(), "r1", alerts.Input{
				Kind: "k", Category: alerts.CategoryInfo, Title: fmt.Sprintf("t%d", i)})
		}(i)
	}
	wg.Wait()
	var last int64
	for _, s := range hub.snapshot() {
		seq := s.event["notification"].(alerts.View).Seq
		if seq <= last {
			t.Fatalf("jonli kanalga tartib buzildi: %d keyin %d", last, seq)
		}
		last = seq
	}
	if last != 60 {
		t.Fatalf("oxirgi seq: %d", last)
	}
}

func TestOrderHooks(t *testing.T) {
	svc, _, _ := newSvc(time.Now())
	inner := &innerNotifier{}
	n := svc.WrapOrders(inner)

	o := &orders.Order{ID: "o1", OrderNumber: "140926-0000123", RestaurantID: "r1", TotalTiyin: 18000000,
		Status: orders.StatusCreated}
	n.OrderCreated(o)
	if inner.created != 1 {
		t.Fatal("ichki notifier chaqirilmadi")
	}
	eventually(t, "yangi buyurtma", func() bool { return len(list(t, svc, "r1")) == 1 })
	got := list(t, svc, "r1")[0]
	if got.Kind != alerts.KindNewOrder || !strings.Contains(got.Body, "#140926-0000123") || !strings.Contains(got.Body, "180 000 so'm") {
		t.Fatalf("yangi buyurtma: %+v", got)
	}
	n.OrderCreated(o) // takror — ikkinchisi yaratilmaydi
	time.Sleep(50 * time.Millisecond)

	// Restoranning o'zi rad etgani/bekor qilgani — xabar yo'q.
	byRestaurant := *o
	byRestaurant.Status = orders.StatusCancelled
	byRestaurant.History = []orders.StatusChange{{From: orders.StatusCreated, To: orders.StatusCancelled, By: orders.ActorRestaurant}}
	n.OrderStatusChanged(&byRestaurant, orders.StatusCreated)
	// Mijoz bekor qildi — muhim.
	o2 := &orders.Order{ID: "o2", OrderNumber: "140926-0000124", RestaurantID: "r1", TotalTiyin: 500000,
		Status:  orders.StatusCancelled,
		History: []orders.StatusChange{{From: orders.StatusCreated, To: orders.StatusCancelled, By: orders.ActorCustomer}}}
	n.OrderStatusChanged(o2, orders.StatusCreated)
	eventually(t, "bekor qilish", func() bool { return len(list(t, svc, "r1")) == 2 })
	time.Sleep(50 * time.Millisecond)
	items := list(t, svc, "r1")
	if len(items) != 2 || items[0].Category != alerts.CategoryImportant || !strings.Contains(items[0].Body, "Mijoz") {
		t.Fatalf("bekor qilish: %+v", items)
	}
	if inner.changed != 2 {
		t.Fatal("ichki notifier holat o'zgarishini olmadi")
	}
}

func TestStaffAndPaymentHooks(t *testing.T) {
	svc, _, _ := newSvc(time.Now())
	svc.WithOrders(func(_ context.Context, id string) (*orders.Order, error) {
		return &orders.Order{ID: id, OrderNumber: "140926-0000777"}, nil
	})
	m := staff.Member{ID: "s1", RestaurantID: "r1", FirstName: "Malika", LastName: "To'xtayeva",
		Position: staff.PositionWaiter, Phone: "+998901234567"}
	svc.StaffEvents(m, []staff.Event{
		{ID: "e1", Kind: staff.EventCreated},
		{ID: "e2", Kind: staff.EventStatusChanged, From: "active", To: "on_leave"},
		{ID: "e3", Kind: staff.EventPositionChanged, From: "cashier", To: "waiter"},
		{ID: "e4", Kind: staff.EventSalaryChanged},
		{ID: "e5", Kind: staff.EventAccessGranted},
	})
	svc.PaymentReceived("r1", "p1", "o7", 14000000)
	svc.PaymentReceived("r1", "p1", "o7", 14000000) // takroriy callback
	eventually(t, "xodim va to'lov", func() bool { return len(list(t, svc, "r1")) == 4 })
	time.Sleep(50 * time.Millisecond)
	items := list(t, svc, "r1")
	if len(items) != 4 {
		t.Fatalf("kutilgan 4 (maosh va kirish yozilmaydi): %d", len(items))
	}
	var payment *alerts.Notification
	for _, it := range items {
		if strings.Contains(it.Body, "+998") || strings.Contains(it.Body, "maosh") {
			t.Fatalf("shaxsiy ma'lumot bildirishnomada: %q", it.Body)
		}
		if it.Kind == alerts.KindPaymentReceived {
			payment = it
		}
	}
	if payment == nil || payment.Category != alerts.CategorySuccess ||
		!strings.Contains(payment.Body, "140 000 so'm") || !strings.Contains(payment.Body, "#140926-0000777") {
		t.Fatalf("to'lov: %+v", payment)
	}
}

func TestWaitingOrdersReminder(t *testing.T) {
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, tashkent)
	svc, _, _ := newSvc(now)
	ordersList := []*orders.Order{
		{ID: "late", OrderNumber: "A1", RestaurantID: "r1", Status: orders.StatusCreated, CreatedAt: now.Add(-6 * time.Minute)},
		{ID: "fresh", RestaurantID: "r1", Status: orders.StatusCreated, CreatedAt: now.Add(-2 * time.Minute)},
		{ID: "accepted", RestaurantID: "r1", Status: orders.StatusAccepted, CreatedAt: now.Add(-20 * time.Minute)},
		{ID: "unpaid", RestaurantID: "r1", Status: orders.StatusCreated, CreatedAt: now.Add(-20 * time.Minute),
			PaymentMethod: orders.PaymentCard},
		{ID: "ancient", RestaurantID: "r1", Status: orders.StatusCreated, CreatedAt: now.Add(-5 * time.Hour)},
	}
	deps := alerts.JobDeps{RecentOrders: func(context.Context) ([]*orders.Order, error) { return ordersList, nil }}
	if n, err := svc.CheckWaitingOrders(context.Background(), deps); err != nil || n != 1 {
		t.Fatalf("eslatma: %d %v", n, err)
	}
	if n, _ := svc.CheckWaitingOrders(context.Background(), deps); n != 0 {
		t.Fatal("eslatma takrorlandi")
	}
	items := list(t, svc, "r1")
	if items[0].Category != alerts.CategoryReminder || !strings.Contains(items[0].Body, "#A1") || !strings.Contains(items[0].Body, "6 daqiqadan") {
		t.Fatalf("eslatma matni: %+v", items[0])
	}
}

func TestDailyReports(t *testing.T) {
	ctx := context.Background()
	deps := alerts.JobDeps{
		Restaurants: func(context.Context) ([]string, error) { return []string{"r1", "r2"}, nil },
		DaySummary: func(_ context.Context, rid string, from, to time.Time) (alerts.DaySummary, error) {
			if !from.Equal(time.Date(2026, 9, 13, 0, 0, 0, 0, tashkent)) || !to.Equal(time.Date(2026, 9, 14, 0, 0, 0, 0, tashkent)) {
				return alerts.DaySummary{}, fmt.Errorf("davr noto'g'ri: %v %v", from, to)
			}
			if rid == "r1" {
				return alerts.DaySummary{Orders: 34, Completed: 31, Cancelled: 3, RevenueTiyin: 456000000}, nil
			}
			return alerts.DaySummary{}, nil
		},
	}
	early, _, _ := newSvc(time.Date(2026, 9, 14, 0, 2, 0, 0, tashkent))
	if n, _ := early.CheckDailyReports(ctx, deps); n != 0 {
		t.Fatal("yarim tundan darhol keyin hisobot yuborilmasligi kerak")
	}
	svc, _, _ := newSvc(time.Date(2026, 9, 14, 10, 0, 0, 0, tashkent))
	if n, err := svc.CheckDailyReports(ctx, deps); err != nil || n != 1 {
		t.Fatalf("hisobot: %d %v", n, err)
	}
	if n, _ := svc.CheckDailyReports(ctx, deps); n != 0 {
		t.Fatal("hisobot takrorlandi")
	}
	r1 := list(t, svc, "r1")
	if len(r1) != 1 || r1[0].Data["date"] != "2026-09-13" || !strings.Contains(r1[0].Body, "13-sentabr: 34 ta buyurtma") ||
		!strings.Contains(r1[0].Body, "4 560 000 so'm") {
		t.Fatalf("hisobot matni: %+v", r1)
	}
	if len(list(t, svc, "r2")) != 0 {
		t.Fatal("buyurtmasiz restoranga hisobot ketdi")
	}
}

func TestReadStateAndCatchUp(t *testing.T) {
	svc, _, hub := newSvc(time.Now())
	ctx := context.Background()
	for i := 1; i <= 3; i++ {
		_, _ = svc.Publish(ctx, "r1", alerts.Input{Kind: "k", Category: alerts.CategoryInfo, Title: fmt.Sprintf("t%d", i)})
	}
	other, _ := svc.Publish(ctx, "r2", alerts.Input{Kind: "k", Category: alerts.CategoryInfo, Title: "begona"})

	// Begona restoran yozuvini o'qilgan qilib bo'lmaydi.
	if unread, _ := svc.MarkRead(ctx, "r1", other.ID); unread != 3 {
		t.Fatalf("begona: %d", unread)
	}
	if u, _, _ := svc.Summary(ctx, "r2"); u != 1 {
		t.Fatal("XAVFSIZLIK: begona restoran bildirishnomasi o'qilgan bo'ldi")
	}
	// "Barchasini o'qish" — ko'rilgan oxirgi raqamgacha; keyin kelgan o'qilmagan qoladi.
	updated, unread, err := svc.MarkAllRead(ctx, "r1", 2)
	if err != nil || updated != 2 || unread != 1 {
		t.Fatalf("read-all: %d %d %v", updated, unread, err)
	}
	last := hub.snapshot()[len(hub.snapshot())-1]
	if last.event["type"] != alerts.EventRead || last.event["unread"] != 1 {
		t.Fatalf("o'qish hodisasi: %+v", last)
	}
	// Qayta ulanish: 1 dan keyingilari, o'sish tartibida.
	items, _ := svc.List(ctx, "r1", alerts.Query{AfterSeq: 1, Limit: 10})
	if len(items) != 2 || items[0].Seq != 2 || items[1].Seq != 3 {
		t.Fatalf("catch-up: %+v", items)
	}
	if _, _, err := svc.MarkAllRead(ctx, "r1", 0); err == nil {
		t.Fatal("up_to_seq=0 qabul qilindi")
	}
}
