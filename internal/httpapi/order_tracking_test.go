package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/couriers"
	"chustapp/internal/orders"
	"chustapp/internal/revoke"
	"chustapp/internal/storage"
	"chustapp/internal/tracking"
	"chustapp/internal/users"
	"chustapp/internal/voice"
	"chustapp/internal/ws"
)

// fakeDirections — Google o'rniga: har chaqiruvni sanaydi.
type fakeDirections struct {
	mu       sync.Mutex
	calls    int
	fail     bool
	duration int
}

func (f *fakeDirections) fn(_ context.Context, from, to tracking.Point, _ string) (DirectionsResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.fail {
		return DirectionsResult{}, errNoRoute
	}
	mid := tracking.Point{Lat: (from.Lat + to.Lat) / 2, Lng: (from.Lng + to.Lng) / 2}
	return DirectionsResult{
		Points:         []tracking.Point{from, mid, to},
		DistanceMeters: 2400, DurationSeconds: f.duration,
	}, nil
}

func (f *fakeDirections) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeDirections) setFail(v bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fail = v
}

type trackingFixture struct {
	h        http.Handler
	jwt      map[string]string
	orders   *storage.MemoryOrderRepo
	couriers *storage.MemoryCourierRepo
	routes   *storage.MemoryRouteRepo
	dir      *fakeDirections
	tts      *fakeTTS
}

var (
	trackRestaurant = tracking.Point{Lat: 41.0000, Lng: 71.2300}
	trackCustomer   = tracking.Point{Lat: 41.0200, Lng: 71.2500}
	trackCourier    = tracking.Point{Lat: 41.0050, Lng: 71.2350}
)

func trackingServer(t *testing.T) trackingFixture {
	t.Helper()
	ctx := context.Background()
	userRepo := storage.NewMemoryUserRepo()
	tokens := users.NewTokenIssuer("test-secret", time.Hour)
	jwt := map[string]string{}
	mk := func(key, id string, role users.Role, entityID, phone string) {
		u := &users.User{ID: id, Phone: phone, Role: role, EntityID: entityID, PhoneVerified: true, CreatedAt: time.Now()}
		if err := userRepo.Create(ctx, u); err != nil {
			t.Fatal(err)
		}
		tok, err := tokens.Issue(u)
		if err != nil {
			t.Fatal(err)
		}
		jwt[key] = tok
	}
	mk("customer", "u-track-c", users.RoleCustomer, "", "+998900000181")
	mk("other", "u-track-o", users.RoleCustomer, "", "+998900000182")
	mk("a", "u-track-a", users.RoleRestaurant, testRestA, "+998900000183")
	mk("b", "u-track-b", users.RoleRestaurant, testRestB, "+998900000184")
	mk("admin", "u-track-admin", users.RoleAdmin, "", "+998900000185")
	mk("courier", "u-track-k", users.RoleCourier, "k-track", "+998900000186")

	catalogRepo := storage.NewMemoryCatalogRepo([]catalog.Restaurant{
		{ID: testRestA, Name: "Book Cafe", Open: true, Lat: trackRestaurant.Lat, Lng: trackRestaurant.Lng, LogoURL: "/uploads/book-cafe.png"},
		{ID: testRestB, Name: "B", Open: true},
	}, nil)
	courierRepo := storage.NewMemoryCourierRepo(couriers.Courier{
		ID: "k-track", Name: "Jasur Karimov", RestaurantID: testRestA, Approved: true,
		Lat: trackCourier.Lat, Lng: trackCourier.Lng, VehicleType: couriers.VehicleMoped, Rating: 5,
	})
	orderRepo := storage.NewMemoryOrderRepo()
	routes := storage.NewMemoryRouteRepo()
	dir := &fakeDirections{duration: 540}
	tts := &fakeTTS{}
	h := New(Deps{
		UserRepo:       userRepo,
		CatalogRepo:    catalogRepo,
		PromotionsRepo: storage.NewMemoryPromotionsRepo(),
		OrderRepo:      orderRepo,
		OrderSvc:       orders.NewService(orderRepo, nil, NewID, storage.NewMemoryPromotionsRepo()),
		CourierRepo:    courierRepo,
		RouteRepo:      routes,
		Directions:     dir.fn,
		Voice:          voice.NewService(tts, storage.NewMemoryClipStore()),
		Tokens:         tokens,
		Revoked:        revoke.New(nil, time.Hour),
		Hub:            ws.NewHub(nil),
		DevMode:        true,
	}).Routes(nil)
	return trackingFixture{h: h, jwt: jwt, orders: orderRepo, couriers: courierRepo, routes: routes, dir: dir, tts: tts}
}

func (f trackingFixture) saveOrder(t *testing.T, id string, status orders.Status, dineIn bool) {
	t.Helper()
	now := time.Now().UTC()
	o := &orders.Order{
		ID: id, CustomerID: "u-track-c", RestaurantID: testRestA, CourierID: "k-track", Status: status,
		DeliveryLat: trackCustomer.Lat, DeliveryLng: trackCustomer.Lng, CreatedAt: now.Add(-30 * time.Minute),
	}
	if dineIn {
		o.Type = orders.TypeDineIn
	}
	if status == orders.StatusPickedUp || status == orders.StatusDelivered {
		o.History = append(o.History, orders.StatusChange{From: orders.StatusReady, To: orders.StatusPickedUp, At: now.Add(-12 * time.Minute)})
	}
	if status == orders.StatusDelivered {
		o.History = append(o.History, orders.StatusChange{From: orders.StatusPickedUp, To: orders.StatusDelivered, At: now})
	}
	if err := f.orders.Save(context.Background(), o); err != nil {
		t.Fatal(err)
	}
}

func (f trackingFixture) get(t *testing.T, tok, id string) (int, deliveryTrackingView) {
	t.Helper()
	w := do(t, f.h, "GET", "/orders/"+id+"/tracking", tok, "")
	var v deliveryTrackingView
	if w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatalf("javob: %v — %s", err, w.Body.String())
		}
	}
	return w.Code, v
}

// Taom olinmaguncha yo'l chizilmaydi va Google'ga so'rov ketmaydi.
func TestTrackingWaitingPhase(t *testing.T) {
	f := trackingServer(t)
	f.saveOrder(t, "o-wait", orders.StatusPreparing, false)
	code, v := f.get(t, f.jwt["customer"], "o-wait")
	if code != http.StatusOK || v.Phase != trackingWaiting || v.PlannedRoute != nil || v.Courier != nil {
		t.Fatalf("kutish bosqichi: %d %+v", code, v)
	}
	if f.dir.count() != 0 {
		t.Fatalf("taom olinmasdan yo'l so'raldi: %d", f.dir.count())
	}
}

func TestTrackingLiveThenDelivered(t *testing.T) {
	f := trackingServer(t)
	ctx := context.Background()
	f.saveOrder(t, "o-live", orders.StatusPickedUp, false)

	code, v := f.get(t, f.jwt["customer"], "o-live")
	if code != http.StatusOK || v.Phase != trackingLive {
		t.Fatalf("jonli bosqich: %d %+v", code, v)
	}
	if v.Origin == nil || *v.Origin != trackRestaurant || v.Destination == nil || *v.Destination != trackCustomer {
		t.Fatalf("A/B nuqtalar noto'g'ri: %+v %+v", v.Origin, v.Destination)
	}
	if v.Courier == nil || *v.Courier != trackCourier {
		t.Fatalf("kuryer joylashuvi: %+v", v.Courier)
	}
	if v.RestaurantName != "Book Cafe" || v.RestaurantLogoURL != "/uploads/book-cafe.png" {
		t.Fatalf("A nuqta uchun restoran logosi: %q %q", v.RestaurantName, v.RestaurantLogoURL)
	}
	if v.PlannedRoute == nil || len(v.PlannedRoute.Points) != 3 || v.PlannedRoute.Points[0] != trackRestaurant {
		t.Fatalf("A→B yo'li: %+v", v.PlannedRoute)
	}
	if v.RemainingRoute == nil || v.RemainingRoute.Points[0] != trackCourier ||
		v.ETASeconds == nil || *v.ETASeconds != 540 || v.ComputedAt == nil || v.PickedUpAt == nil {
		t.Fatalf("qolgan yo'l/vaqt: %+v eta=%v", v.RemainingRoute, v.ETASeconds)
	}
	if f.dir.count() != 2 {
		t.Fatalf("A→B va kuryer→B — 2 ta so'rov kutilgan, keldi %d", f.dir.count())
	}

	// Takroriy so'rovlar (mijoz, restoran, admin) Google'ga qayta bormaydi.
	for _, tok := range []string{f.jwt["customer"], f.jwt["a"], f.jwt["admin"]} {
		if code, _ := f.get(t, tok, "o-live"); code != http.StatusOK {
			t.Fatalf("ruxsatli tomon: %d", code)
		}
	}
	if f.dir.count() != 2 {
		t.Fatalf("kesh ishlamadi: %d ta so'rov", f.dir.count())
	}
	// `?planned=0` — A→B nuqtalari qayta yuborilmaydi, qolgani bor.
	w := do(t, f.h, "GET", "/orders/o-live/tracking?planned=0", f.jwt["customer"], "")
	var lite deliveryTrackingView
	if err := json.Unmarshal(w.Body.Bytes(), &lite); err != nil || w.Code != http.StatusOK ||
		lite.PlannedRoute != nil || lite.RestaurantLogoURL != "" ||
		lite.Origin == nil || lite.RemainingRoute == nil || lite.ETASeconds == nil {
		t.Fatalf("planned=0: %d %s", w.Code, w.Body.String())
	}
	if len(w.Body.Bytes()) >= len(do(t, f.h, "GET", "/orders/o-live/tracking", f.jwt["customer"], "").Body.Bytes()) {
		t.Fatal("planned=0 javobi kichikroq bo'lishi kerak")
	}
	if saved, err := f.routes.GetRoute(ctx, "o-live"); err != nil || saved.Origin != trackRestaurant {
		t.Fatalf("A→B yo'li buyurtmaga saqlanmadi: %+v %v", saved, err)
	}

	// Begonalar ko'rmaydi; kuryer roli bu endpointga kirmaydi.
	for tok, want := range map[string]int{
		f.jwt["other"]:   http.StatusNotFound,
		f.jwt["b"]:       http.StatusNotFound,
		f.jwt["courier"]: http.StatusForbidden,
		"":               http.StatusUnauthorized,
	} {
		if code, _ := f.get(t, tok, "o-live"); code != want {
			t.Errorf("kutilgan %d, keldi %d", want, code)
		}
	}

	// Kuryer eshik oldida — qolgan vaqt 0, Google'ga so'rov yo'q.
	if err := f.couriers.UpdateLocation(ctx, "k-track", trackCustomer.Lat+0.0002, trackCustomer.Lng); err != nil {
		t.Fatal(err)
	}
	if _, v := f.get(t, f.jwt["customer"], "o-live"); v.ETASeconds == nil || *v.ETASeconds != 0 {
		t.Fatalf("yetib kelganda qolgan vaqt 0 bo'lishi kerak: %v", v.ETASeconds)
	}

	// Yetkazildi: yo'l buyurtmada qoladi, kuryer joylashuvi YOPILADI.
	f.saveOrder(t, "o-live", orders.StatusDelivered, false)
	code, v = f.get(t, f.jwt["customer"], "o-live")
	if code != http.StatusOK || v.Phase != trackingDelivered {
		t.Fatalf("yetkazilgan: %d %+v", code, v)
	}
	if v.Courier != nil || v.RemainingRoute != nil || v.ETASeconds != nil {
		t.Fatalf("yetkazilgandan keyin kuryer ma'lumoti ochiq qoldi: %+v", v)
	}
	if v.PlannedRoute == nil || len(v.PlannedRoute.Points) != 3 || v.DeliveredAt == nil || v.PickedUpAt == nil {
		t.Fatalf("yetkazish yo'li saqlanib ko'rinishi kerak: %+v", v)
	}
	if f.dir.count() != 2 {
		t.Fatalf("saqlangan yo'l qayta hisoblandi: %d", f.dir.count())
	}
}

func TestTrackingDineInAndCancelled(t *testing.T) {
	f := trackingServer(t)
	f.saveOrder(t, "o-dine", orders.StatusReady, true)
	if code, _ := f.get(t, f.jwt["customer"], "o-dine"); code != http.StatusNotFound {
		t.Fatalf("stol buyurtmasi: 404 kutilgan, keldi %d", code)
	}
	f.saveOrder(t, "o-cancel", orders.StatusCancelled, false)
	if code, v := f.get(t, f.jwt["customer"], "o-cancel"); code != http.StatusOK || v.Phase != trackingNone || v.Destination != nil {
		t.Fatalf("bekor qilingan: %d %+v", code, v)
	}
}

// Google ishlamasa javob buzilmaydi va har so'rovda qayta urinilmaydi.
func TestTrackingDirectionsFailure(t *testing.T) {
	f := trackingServer(t)
	f.dir.setFail(true)
	f.saveOrder(t, "o-fail", orders.StatusPickedUp, false)

	code, v := f.get(t, f.jwt["customer"], "o-fail")
	if code != http.StatusOK || v.Phase != trackingLive || v.Courier == nil || v.Origin == nil || v.Destination == nil {
		t.Fatalf("xatoda ham nuqtalar qaytishi kerak: %d %+v", code, v)
	}
	if v.PlannedRoute != nil || v.RemainingRoute != nil || v.ETASeconds != nil {
		t.Fatalf("yo'l yo'q bo'lsa to'qib chiqarilmasligi kerak: %+v", v)
	}
	calls := f.dir.count()
	f.get(t, f.jwt["customer"], "o-fail")
	if f.dir.count() != calls {
		t.Fatalf("xatodan keyin darhol qayta urinildi: %d -> %d", calls, f.dir.count())
	}
	if _, err := f.routes.GetRoute(context.Background(), "o-fail"); !errors.Is(err, tracking.ErrNotFound) {
		t.Fatalf("muvaffaqiyatsiz yo'l saqlanmasligi kerak: %v", err)
	}
}

func TestLiveRouteCacheThrottle(t *testing.T) {
	c := newLiveRouteCache()
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return now }
	calls, fail := 0, false
	fetch := func(context.Context) (DirectionsResult, error) {
		calls++
		if fail {
			return DirectionsResult{}, errors.New("google")
		}
		return DirectionsResult{DurationSeconds: 600 - calls}, nil
	}
	p1 := tracking.Point{Lat: 41.0, Lng: 71.23}
	far := tracking.Point{Lat: 41.01, Lng: 71.23} // ~1.1 km
	near := tracking.Point{Lat: 41.0103, Lng: 71.23}
	ctx := context.Background()

	step := func(d time.Duration, from tracking.Point, wantCalls int) (DirectionsResult, bool) {
		t.Helper()
		now = now.Add(d)
		res, _, ok := c.get(ctx, "o1", from, fetch)
		if calls != wantCalls {
			t.Fatalf("+%v: kutilgan %d ta so'rov, keldi %d", d, wantCalls, calls)
		}
		return res, ok
	}
	step(0, p1, 1)
	step(10*time.Second, far, 1)  // 30 s ichida — joy o'zgarsa ham kesh
	step(40*time.Second, far, 2)  // eskirdi va kuryer uzoqlashdi
	step(40*time.Second, near, 2) // deyarli joyida — 3 daqiqagacha kesh
	step(3*time.Minute, near, 3)  // 3 daqiqadan eski — qayta hisob
	fail = true
	res, ok := step(3*time.Minute, near, 4) // Google xatosi
	if !ok || res.DurationSeconds != 597 {
		t.Fatalf("xatoda oldingi natija qolishi kerak: %+v %v", res, ok)
	}
	step(time.Second, near, 4) // xatodan keyin 15 s qayta urinilmaydi

	if !c.allowPlanned("o1") || c.allowPlanned("o1") {
		t.Fatal("A→B urinishi daqiqasiga bir bo'lishi kerak")
	}
	now = now.Add(2 * time.Hour)
	c.get(ctx, "o2", p1, fetch)
	if _, has := c.entries["o1"]; has {
		t.Fatal("eski yozuv xotiradan o'chmadi")
	}
}

func TestDownsampleKeepsEnds(t *testing.T) {
	pts := make([]tracking.Point, tracking.MaxRoutePoints*3+7)
	for i := range pts {
		pts[i] = tracking.Point{Lat: float64(i)}
	}
	got := tracking.Downsample(pts)
	if len(got) > tracking.MaxRoutePoints || got[0] != pts[0] || got[len(got)-1] != pts[len(pts)-1] {
		t.Fatalf("siyraklashtirish: %d nuqta, uchlar %v..%v", len(got), got[0], got[len(got)-1])
	}
}
