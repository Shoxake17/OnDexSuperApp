package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/couriers"
	"chustapp/internal/orders"
	"chustapp/internal/revoke"
	"chustapp/internal/stats"
	"chustapp/internal/storage"
	"chustapp/internal/users"
	"chustapp/internal/ws"
)

// Superadmin statistikasi (`GET /admin/stats?period=`): BARCHA restoranlar
// bo'yicha jami, har restoran jadvali, holatlar va kunlik grafik.

type platformStatsResp struct {
	Period      string         `json:"period"`
	Totals      statsTotals    `json:"totals"`
	Restaurants []statsRow     `json:"restaurants"`
	Statuses    map[string]int `json:"statuses"`
	Daily       []struct {
		Date         string `json:"date"`
		Orders       int    `json:"orders"`
		Completed    int    `json:"completed"`
		RevenueTiyin int64  `json:"revenue_tiyin"`
	} `json:"daily"`
	RestaurantsTotal int `json:"restaurants_total"`
	RestaurantsOpen  int `json:"restaurants_open"`
	CouriersOnline   int `json:"couriers_online"`
	CouriersPending  int `json:"couriers_pending"`
	CouriersTotal    int `json:"couriers_total"`
}

type statsTotals struct {
	Orders         int   `json:"orders"`
	New            int   `json:"new"`
	InProgress     int   `json:"in_progress"`
	Accepted       int   `json:"accepted"`
	Completed      int   `json:"completed"`
	Cancelled      int   `json:"cancelled"`
	RevenueTiyin   int64 `json:"revenue_tiyin"`
	AvgCheckTiyin  int64 `json:"avg_check_tiyin"`
	CompletionRate int   `json:"completion_rate"`
}

type statsRow struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Deleted bool   `json:"deleted"`
	statsTotals
}

func statsFixture(t *testing.T) (h http.Handler, adminJWT, customerJWT string) {
	t.Helper()
	ctx := context.Background()
	userRepo := storage.NewMemoryUserRepo()
	tokens := users.NewTokenIssuer("test-secret", time.Hour)
	admin := &users.User{ID: "admin-1", Phone: "+998900000001", Name: "Admin", Role: users.RoleAdmin,
		PhoneVerified: true, CreatedAt: time.Now()}
	customer := &users.User{ID: "cust-1", Phone: "+998900000002", Name: "Mijoz", Role: users.RoleCustomer,
		PhoneVerified: true, CreatedAt: time.Now()}
	for _, u := range []*users.User{admin, customer} {
		if err := userRepo.Create(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	var err error
	if adminJWT, err = tokens.Issue(admin); err != nil {
		t.Fatal(err)
	}
	if customerJWT, err = tokens.Issue(customer); err != nil {
		t.Fatal(err)
	}

	orderRepo := storage.NewMemoryOrderRepo()
	today := time.Now().In(stats.Location)
	at := func(daysAgo int) time.Time {
		return time.Date(today.Year(), today.Month(), today.Day(), 12, 0, 0, 0, stats.Location).
			AddDate(0, 0, -daysAgo)
	}
	seed := []*orders.Order{
		{ID: "a1", RestaurantID: "rest-a", Status: orders.StatusDelivered, TotalTiyin: 100000, CreatedAt: at(0)},
		{ID: "a2", RestaurantID: "rest-a", Status: orders.StatusServed, TotalTiyin: 50000, CreatedAt: at(0)},
		{ID: "a3", RestaurantID: "rest-a", Status: orders.StatusCreated, TotalTiyin: 70000, CreatedAt: at(0)},
		{ID: "a4", RestaurantID: "rest-a", Status: orders.StatusPreparing, TotalTiyin: 80000, CreatedAt: at(0)},
		{ID: "a5", RestaurantID: "rest-a", Status: orders.StatusCancelled, TotalTiyin: 90000, CreatedAt: at(0)},
		{ID: "a6", RestaurantID: "rest-a", Status: orders.StatusDelivered, TotalTiyin: 200000, CreatedAt: at(10)},
		{ID: "b1", RestaurantID: "rest-b", Status: orders.StatusRejected, TotalTiyin: 40000, CreatedAt: at(0)},
		// O'chirilgan restoranning buyurtmasi — jamiga kiradi.
		{ID: "x1", RestaurantID: "rest-gone", Status: orders.StatusDelivered, TotalTiyin: 30000, CreatedAt: at(0)},
		// To'lanmagan karta buyurtmasi — HECH QAYERDA hisoblanmaydi.
		{ID: "u1", RestaurantID: "rest-a", Status: orders.StatusCreated, TotalTiyin: 999999,
			PaymentMethod: orders.PaymentCard, PaymentState: orders.PaymentAwaiting, CreatedAt: at(0)},
	}
	for _, o := range seed {
		if err := orderRepo.Save(ctx, o); err != nil {
			t.Fatal(err)
		}
	}

	catalogRepo := storage.NewMemoryCatalogRepo([]catalog.Restaurant{
		{ID: "rest-a", Name: "Book Cafe", Open: true},
		{ID: "rest-b", Name: "Osh Markazi", Open: false},
		{ID: "rest-empty", Name: "Yangi Restoran", Open: true},
	}, nil)

	courierRepo := storage.NewMemoryCourierRepo(
		couriers.Courier{ID: "c-online", Name: "Online", Approved: true, Available: true},
		couriers.Courier{ID: "c-pending", Name: "Kutmoqda", Approved: false},
		// Restoranning o'z kuryeri (kirishi yopiq): admin tasdiqlay olmaydi,
		// "tasdiq kutayotgan"ga KIRMASLIGI kerak.
		couriers.Courier{ID: "c-rest", Name: "Restoran kuryeri", RestaurantID: "rest-a", Approved: false},
	)

	deps := Deps{
		UserRepo: userRepo, OrderRepo: orderRepo, CourierRepo: courierRepo, CatalogRepo: catalogRepo,
		Devices: storage.NewMemoryDeviceStore(), Tokens: tokens, Revoked: revoke.New(nil, time.Hour),
		Hub: ws.NewHub(nil), WsTickets: ws.NewTicketStore(), DevMode: true,
	}
	return New(deps).Routes(nil), adminJWT, customerJWT
}

func getStats(t *testing.T, h http.Handler, jwt, query string) platformStatsResp {
	t.Helper()
	w := do(t, h, "GET", "/admin/stats"+query, jwt, "")
	if w.Code != http.StatusOK {
		t.Fatalf("/admin/stats%s: kutilgan 200, keldi %d (%s)", query, w.Code, w.Body.String())
	}
	var resp platformStatsResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("javobni o'qib bo'lmadi: %v (%s)", err, w.Body.String())
	}
	return resp
}

func rowByID(rows []statsRow, id string) *statsRow {
	for i := range rows {
		if rows[i].ID == id {
			return &rows[i]
		}
	}
	return nil
}

// ★ ASOSIY OQIM: bugungi davr — jami, har restoran va o'chirilgan restoran.
func TestAdminStatsTodayTotalsAndPerRestaurant(t *testing.T) {
	h, admin, _ := statsFixture(t)
	r := getStats(t, h, admin, "?period=today")
	if r.Period != "today" {
		t.Fatalf("davr today bo'lishi kerak, keldi %q", r.Period)
	}

	// Bugun: a1..a5 (5) + b1 + x1 = 7; to'lanmagan u1 sanalmaydi.
	want := statsTotals{Orders: 7, New: 1, InProgress: 1, Accepted: 4, Completed: 3, Cancelled: 2,
		RevenueTiyin: 180000, AvgCheckTiyin: 60000, CompletionRate: 60}
	if r.Totals != want {
		t.Fatalf("jami noto'g'ri:\n keldi %+v\n kerak %+v", r.Totals, want)
	}

	a := rowByID(r.Restaurants, "rest-a")
	if a == nil {
		t.Fatal("rest-a qatori yo'q")
	}
	wantA := statsTotals{Orders: 5, New: 1, InProgress: 1, Accepted: 3, Completed: 2, Cancelled: 1,
		RevenueTiyin: 150000, AvgCheckTiyin: 75000, CompletionRate: 66}
	if a.statsTotals != wantA {
		t.Fatalf("rest-a noto'g'ri:\n keldi %+v\n kerak %+v", a.statsTotals, wantA)
	}

	// Buyurtmasiz restoran ham jadvalda (nollar bilan).
	if e := rowByID(r.Restaurants, "rest-empty"); e == nil || e.Orders != 0 {
		t.Fatalf("buyurtmasiz restoran jadvalda nol bilan turishi kerak, keldi %+v", e)
	}

	// O'chirilgan restoran: alohida qator, jami bilan mos.
	var gone *statsRow
	for i := range r.Restaurants {
		if r.Restaurants[i].Deleted {
			gone = &r.Restaurants[i]
		}
	}
	if gone == nil || gone.Orders != 1 || gone.RevenueTiyin != 30000 {
		t.Fatalf("o'chirilgan restoran qatori kerak (1 ta, 30000), keldi %+v", gone)
	}

	// Jadval yig'indisi kartalar bilan AYNAN teng.
	var sum int
	var rev int64
	for _, row := range r.Restaurants {
		sum += row.Orders
		rev += row.RevenueTiyin
	}
	if sum != r.Totals.Orders || rev != r.Totals.RevenueTiyin {
		t.Fatalf("jadval yig'indisi (%d, %d) jami (%d, %d) bilan mos emas", sum, rev, r.Totals.Orders, r.Totals.RevenueTiyin)
	}

	// Tartib: tushum bo'yicha kamayish — eng yuqorida rest-a.
	if r.Restaurants[0].ID != "rest-a" {
		t.Fatalf("eng ko'p tushumli restoran birinchi bo'lishi kerak, keldi %q", r.Restaurants[0].ID)
	}

	// Holatlar (bugun).
	if r.Statuses["delivered"] != 2 || r.Statuses["served"] != 1 || r.Statuses["created"] != 1 {
		t.Fatalf("holatlar noto'g'ri: %v", r.Statuses)
	}

	// Katalog va kuryerlar.
	if r.RestaurantsTotal != 3 || r.RestaurantsOpen != 2 {
		t.Fatalf("restoranlar: jami 3 / ochiq 2 kerak, keldi %d / %d", r.RestaurantsTotal, r.RestaurantsOpen)
	}
	if r.CouriersTotal != 3 || r.CouriersOnline != 1 {
		t.Fatalf("kuryerlar: jami 3 / online 1 kerak, keldi %d / %d", r.CouriersTotal, r.CouriersOnline)
	}
	// Restoranning o'z kuryeri "tasdiq kutayotgan"ga kirmaydi.
	if r.CouriersPending != 1 {
		t.Fatalf("tasdiq kutayotgan 1 ta (faqat platforma kuryeri) bo'lishi kerak, keldi %d", r.CouriersPending)
	}
}

// Davr filtri: 7 kunda 10 kun oldingi buyurtma yo'q, 30 kun va hammasida bor.
func TestAdminStatsPeriods(t *testing.T) {
	h, admin, _ := statsFixture(t)
	cases := []struct {
		query   string
		period  string
		aOrders int
		aRev    int64
	}{
		{"?period=today", "today", 5, 150000},
		{"?period=7d", "7d", 5, 150000},
		{"?period=30d", "30d", 6, 350000},
		{"?period=all", "all", 6, 350000},
		{"", "30d", 6, 350000},             // standart
		{"?period=xato", "30d", 6, 350000}, // noma'lum -> standart
	}
	for _, c := range cases {
		r := getStats(t, h, admin, c.query)
		if r.Period != c.period {
			t.Errorf("%q: davr %q kerak, keldi %q", c.query, c.period, r.Period)
		}
		a := rowByID(r.Restaurants, "rest-a")
		if a == nil || a.Orders != c.aOrders || a.RevenueTiyin != c.aRev {
			t.Errorf("%q: rest-a %d ta / %d bo'lishi kerak, keldi %+v", c.query, c.aOrders, c.aRev, a)
		}
	}
}

// Kunlik grafik: 14 ta kun, oxirgisi — bugun, davrdan mustaqil.
func TestAdminStatsDailyChart(t *testing.T) {
	h, admin, _ := statsFixture(t)
	r := getStats(t, h, admin, "?period=today")
	if len(r.Daily) != 14 {
		t.Fatalf("14 ta kun kerak, keldi %d", len(r.Daily))
	}
	todayStr := time.Now().In(stats.Location).Format("2006-01-02")
	last := r.Daily[13]
	if last.Date != todayStr {
		t.Fatalf("oxirgi kun bugun (%s) bo'lishi kerak, keldi %s", todayStr, last.Date)
	}
	if last.Orders != 7 || last.Completed != 3 || last.RevenueTiyin != 180000 {
		t.Fatalf("bugungi kun: 7 ta / 3 ta / 180000 kerak, keldi %+v", last)
	}
	// 10 kun oldingi buyurtma (a6) grafikda o'z kunida — davr "today" bo'lsa ham.
	tenAgo := r.Daily[13-10]
	if tenAgo.Orders != 1 || tenAgo.RevenueTiyin != 200000 {
		t.Fatalf("10 kun oldin: 1 ta / 200000 kerak, keldi %+v", tenAgo)
	}
}

func TestAdminStatsRequiresAdmin(t *testing.T) {
	h, _, customer := statsFixture(t)
	if w := do(t, h, "GET", "/admin/stats?period=all", customer, ""); w.Code != http.StatusForbidden {
		t.Fatalf("mijoz uchun 403 kutilgan, keldi %d", w.Code)
	}
	if w := do(t, h, "GET", "/admin/stats", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("tokensiz 401 kutilgan, keldi %d", w.Code)
	}
}
