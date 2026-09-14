package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/couriers"
	"chustapp/internal/orders"
	"chustapp/internal/storage"
	"chustapp/internal/users"
	"chustapp/internal/ws"
)

// Ikkala haqiqiy ombor ham nazoratchi manbaini qondirishi SHART — aks
// holda `main` startda to'xtaydi. Tekshiruv kompilyatsiya vaqtida.
var (
	_ AwaitingCourierLister = (*storage.PgOrderRepo)(nil)
	_ AwaitingCourierLister = (*storage.MemoryOrderRepo)(nil)
)

type watchClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *watchClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *watchClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

type watchFixture struct {
	h     http.Handler
	s     *Server
	w     *dispatchWatchdog
	svc   *orders.Service
	repo  *storage.MemoryOrderRepo
	clock *watchClock
	jwt   map[string]string
}

func newWatchFixture(t *testing.T, withDispatcher bool) *watchFixture {
	t.Helper()
	ctx := context.Background()
	clock := &watchClock{t: time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)}
	repo := storage.NewMemoryOrderRepo()
	var seqMu sync.Mutex
	seq := 0
	svc := orders.NewService(repo, nil, func() string {
		seqMu.Lock()
		defer seqMu.Unlock()
		seq++
		return fmt.Sprintf("ord-w%d", seq)
	}, nil).WithClock(clock.Now)

	userRepo := storage.NewMemoryUserRepo()
	tokens := users.NewTokenIssuer("test-secret", time.Hour)
	jwt := map[string]string{}
	mk := func(key, id string, role users.Role, entityID, phone string) {
		u := &users.User{ID: id, Phone: phone, Role: role, EntityID: entityID,
			PhoneVerified: true, CreatedAt: time.Now()}
		if err := userRepo.Create(ctx, u); err != nil {
			t.Fatal(err)
		}
		tok, err := tokens.Issue(u)
		if err != nil {
			t.Fatal(err)
		}
		jwt[key] = tok
	}
	mk("rest-a", "u-wa", users.RoleRestaurant, testRestA, "+998900000191")
	mk("rest-b", "u-wb", users.RoleRestaurant, testRestB, "+998900000192")
	mk("customer", "u-wc", users.RoleCustomer, "", "+998900000193")
	mk("admin", "u-wd", users.RoleAdmin, "", "+998900000194")

	courierRepo := storage.NewMemoryCourierRepo()
	deps := Deps{
		UserRepo:    userRepo,
		OrderRepo:   repo,
		OrderSvc:    svc,
		CourierRepo: courierRepo,
		CatalogRepo: storage.NewMemoryCatalogRepo([]catalog.Restaurant{
			{ID: testRestA, Name: "Book Cafe", Open: true, Lat: 41.0, Lng: 71.23}}, nil),
		PromotionsRepo: storage.NewMemoryPromotionsRepo(),
		Tokens:         tokens,
		Hub:            ws.NewHub(nil),
		DevMode:        true,
	}
	if withDispatcher {
		// Onlayn kuryer yo'q — qidiruv tsiklda aylanib turadi: aynan
		// "kuryer topilmayapti" holati.
		deps.Dispatcher = couriers.NewDispatcher(courierRepo, nil, nil, 40*time.Millisecond)
	}
	s := New(deps)
	return &watchFixture{h: s.Routes(nil), s: s, w: newDispatchWatchdog(s, repo),
		svc: svc, repo: repo, clock: clock, jwt: jwt}
}

// readyOrder — yetkazish buyurtmasi, restoran qabul qilgan va taom tayyor.
func (f *watchFixture) readyOrder(t *testing.T) *orders.Order {
	t.Helper()
	ctx := context.Background()
	o, err := f.svc.Create(ctx, &orders.Order{
		CustomerID: "u-wc", RestaurantID: testRestA, Type: orders.TypeDelivery,
		Items: []orders.Item{{ProductID: "p1", Name: "Osh", Qty: 1, PriceTiyin: 3_500_000}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, to := range []orders.Status{orders.StatusAccepted, orders.StatusPreparing, orders.StatusReady} {
		if o, err = f.svc.ChangeStatus(ctx, o.ID, to, orders.ActorRestaurant); err != nil {
			t.Fatalf("-> %s: %v", to, err)
		}
	}
	return o
}

func (f *watchFixture) tick(t *testing.T) {
	t.Helper()
	if err := f.w.tick(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func (f *watchFixture) get(t *testing.T, id string) *orders.Order {
	t.Helper()
	o, err := f.svc.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func waitDispatch(t *testing.T, d *couriers.Dispatcher, orderID string, want bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if d.IsRunning(orderID) == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("qidiruv ishlashi = %v bo'lmadi (%s)", want, orderID)
}

// ★ Asosiy stsenariy: tayyor buyurtma kuryersiz ABADIY qolib ketmaydi.
func TestDispatchWatchdog_NotFoundThenAutoCancel(t *testing.T) {
	f := newWatchFixture(t, false)
	o := f.readyOrder(t)

	f.clock.Advance(orders.CourierSearchWindow - time.Second)
	f.tick(t)
	if got := f.get(t, o.ID); got.DispatchState != orders.DispatchSearching {
		t.Fatalf("muddatdan oldin holat %q", got.DispatchState)
	}

	f.clock.Advance(time.Second)
	f.tick(t)
	got := f.get(t, o.ID)
	if got.DispatchState != orders.DispatchNotFound || got.Status != orders.StatusReady {
		t.Fatalf("10 daqiqadan keyin: holat=%q status=%s", got.DispatchState, got.Status)
	}

	f.clock.Advance(orders.CourierNotFoundCancelAfter - time.Second)
	f.tick(t)
	if got := f.get(t, o.ID); got.Status != orders.StatusReady {
		t.Fatalf("30 daqiqadan OLDIN bekor qilindi: %s", got.Status)
	}

	f.clock.Advance(time.Second)
	f.tick(t)
	got = f.get(t, o.ID)
	if got.Status != orders.StatusCancelled || got.History[len(got.History)-1].By != orders.ActorSystem {
		t.Fatalf("30 daqiqadan keyin: status=%s tarix=%+v", got.Status, got.History)
	}
	list, err := f.repo.ListAwaitingCourier(context.Background(), 10)
	if err != nil || len(list) != 0 {
		t.Fatalf("bekor qilingan buyurtma kuryer kutayotganlar ro'yxatida qoldi: %d, %v", len(list), err)
	}
}

// Restoran tugmalari faqat "kuryer topilmadi" holatida va faqat O'Z
// buyurtmasi uchun ishlaydi.
func TestDispatchRetryAndCancelEndpoints(t *testing.T) {
	f := newWatchFixture(t, false)
	o := f.readyOrder(t)
	path := "/orders/" + o.ID
	cancelBody := `{"to":"cancelled"}`

	if w := do(t, f.h, "POST", path+"/transition", f.jwt["rest-a"], cancelBody); w.Code != http.StatusConflict {
		t.Fatalf("qidiruv davomida bekor qilish: %d %s", w.Code, w.Body.String())
	}
	if w := do(t, f.h, "POST", path+"/dispatch/retry", f.jwt["rest-a"], `{}`); w.Code != http.StatusConflict {
		t.Fatalf("qidiruv davomida qayta qidirish: %d %s", w.Code, w.Body.String())
	}

	f.clock.Advance(orders.CourierSearchWindow)
	f.tick(t)

	if w := do(t, f.h, "POST", path+"/dispatch/retry", f.jwt["rest-b"], `{}`); w.Code != http.StatusNotFound {
		t.Fatalf("XAVFSIZLIK: begona restoran qayta qidirdi: %d", w.Code)
	}
	if w := do(t, f.h, "POST", path+"/dispatch/retry", f.jwt["customer"], `{}`); w.Code != http.StatusForbidden {
		t.Fatalf("XAVFSIZLIK: mijoz qayta qidirdi: %d", w.Code)
	}
	if w := do(t, f.h, "POST", path+"/transition", f.jwt["rest-b"], cancelBody); w.Code != http.StatusNotFound {
		t.Fatalf("XAVFSIZLIK: begona restoran bekor qildi: %d", w.Code)
	}
	// Mijoz — buyurtma egasi, lekin tayyor buyurtmani bekor qila olmaydi:
	// taom allaqachon tayyorlangan, qarorni restoran qiladi.
	if w := do(t, f.h, "POST", path+"/transition", f.jwt["customer"], cancelBody); w.Code != http.StatusConflict {
		t.Fatalf("mijoz tayyor buyurtmani bekor qildi: %d", w.Code)
	}

	w := do(t, f.h, "POST", path+"/dispatch/retry", f.jwt["rest-a"], `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("qayta qidirish: %d %s", w.Code, w.Body.String())
	}
	var got orders.Order
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := f.clock.Now().Add(orders.CourierSearchWindow)
	if got.DispatchState != orders.DispatchSearching || got.DispatchDeadline == nil || !got.DispatchDeadline.Equal(want) {
		t.Fatalf("qayta qidirishdan keyin: holat=%q muddat=%v", got.DispatchState, got.DispatchDeadline)
	}
	if w := do(t, f.h, "POST", path+"/dispatch/retry", f.jwt["rest-a"], `{}`); w.Code != http.StatusConflict {
		t.Fatalf("ikkinchi bosish: %d", w.Code)
	}
	if w := do(t, f.h, "POST", path+"/transition", f.jwt["rest-a"], cancelBody); w.Code != http.StatusConflict {
		t.Fatalf("qayta qidiruv davomida bekor qilish: %d", w.Code)
	}

	f.clock.Advance(orders.CourierSearchWindow)
	f.tick(t)
	w = do(t, f.h, "POST", path+"/transition", f.jwt["rest-a"], cancelBody)
	if w.Code != http.StatusOK {
		t.Fatalf("kuryer topilmaganda bekor qilish: %d %s", w.Code, w.Body.String())
	}
	if got := f.get(t, o.ID); got.Status != orders.StatusCancelled {
		t.Fatalf("status=%s", got.Status)
	}
}

// Qidiruv goroutine'i yo'q (server qayta ishga tushgan) — nazoratchi uni
// tiklaydi; muddat tugaganda to'xtatadi va restoran qaror qilmaguncha
// o'zi qayta boshlamaydi.
func TestDispatchWatchdog_RelaunchesAndStopsSearch(t *testing.T) {
	f := newWatchFixture(t, true)
	o := f.readyOrder(t)
	d := f.s.Dispatcher
	if d.IsRunning(o.ID) {
		t.Fatal("qidiruv oldindan ishlamasligi kerak edi")
	}

	f.tick(t)
	waitDispatch(t, d, o.ID, true)

	f.clock.Advance(orders.CourierSearchWindow)
	f.tick(t)
	if d.IsRunning(o.ID) {
		t.Fatal("kuryer topilmadi holatida qidiruv to'xtatilmadi")
	}
	if got := f.get(t, o.ID); got.DispatchState != orders.DispatchNotFound {
		t.Fatalf("holat %q", got.DispatchState)
	}

	f.clock.Advance(2 * dispatchRelaunchGap)
	f.tick(t)
	time.Sleep(50 * time.Millisecond)
	if d.IsRunning(o.ID) {
		t.Fatal("nazoratchi \"kuryer topilmadi\" holatidagi qidiruvni o'zi qayta boshladi")
	}

	if w := do(t, f.h, "POST", "/orders/"+o.ID+"/dispatch/retry", f.jwt["rest-a"], `{}`); w.Code != http.StatusOK {
		t.Fatalf("qayta qidirish: %d %s", w.Code, w.Body.String())
	}
	waitDispatch(t, d, o.ID, true)

	// Bekor qilish ochiq qidiruvni DARHOL yopadi.
	if w := do(t, f.h, "POST", "/orders/"+o.ID+"/transition", f.jwt["admin"], `{"to":"cancelled"}`); w.Code != http.StatusOK {
		t.Fatalf("admin bekor qilishi: %d %s", w.Code, w.Body.String())
	}
	if d.IsRunning(o.ID) {
		t.Fatal("bekor qilingan buyurtmaning qidiruvi to'xtatilmadi")
	}
}
