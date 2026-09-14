package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/orders"
	"chustapp/internal/stats"
	"chustapp/internal/storage"
	"chustapp/internal/users"
	"chustapp/internal/ws"
)

const (
	statsRestA = "rest-a"
	statsRestB = "rest-b"
)

// statsServer — ikki restoran; tokenlar: "a", "b" (restoran xodimlari),
// "admin", "waiter" (rest-a), "customer".
func statsServer(t *testing.T) (http.Handler, *storage.MemoryOrderRepo, map[string]string) {
	t.Helper()
	ctx := context.Background()
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
	mk("a", "u-a", users.RoleRestaurant, statsRestA, "+998900000031")
	mk("b", "u-b", users.RoleRestaurant, statsRestB, "+998900000032")
	mk("admin", "u-admin", users.RoleAdmin, "", "+998900000033")
	mk("waiter", "u-waiter", users.RoleWaiter, statsRestA, "+998900000034")
	mk("customer", "u-cust", users.RoleCustomer, "", "+998900000035")

	catalogRepo := storage.NewMemoryCatalogRepo(
		[]catalog.Restaurant{
			{ID: statsRestA, Name: "A restoran", Open: true},
			{ID: statsRestB, Name: "B restoran", Open: true},
		}, nil)
	orderRepo := storage.NewMemoryOrderRepo()

	h := New(Deps{
		UserRepo:       userRepo,
		CatalogRepo:    catalogRepo,
		PromotionsRepo: storage.NewMemoryPromotionsRepo(),
		OrderRepo:      orderRepo,
		StatsSource:    orderRepo,
		HistorySource:  orderRepo,
		Tokens:         tokens,
		CatalogSvc:     catalog.NewService(catalogRepo),
		OrderSvc: orders.NewService(orderRepo, nil, NewID,
			storage.NewMemoryPromotionsRepo()),
		Hub:     ws.NewHub(nil),
		DevMode: true,
	}).Routes(nil)
	return h, orderRepo, jwt
}

func statsPath(restaurantID string, daysBack int) string {
	now := time.Now().In(stats.Location)
	return "/restaurants/" + restaurantID + "/stats?from=" +
		now.AddDate(0, 0, -daysBack).Format("2006-01-02") + "&to=" + now.Format("2006-01-02")
}

func TestStatsOwnershipAndRoles(t *testing.T) {
	h, _, jwt := statsServer(t)

	if w := do(t, h, "GET", statsPath(statsRestA, 6), jwt["b"], ""); w.Code != http.StatusForbidden {
		t.Fatalf("XAVFSIZLIK: B restoran A statistikasini ko'rdi: %d", w.Code)
	}
	for _, key := range []string{"waiter", "customer"} {
		if w := do(t, h, "GET", statsPath(statsRestA, 6), jwt[key], ""); w.Code != http.StatusForbidden {
			t.Fatalf("XAVFSIZLIK: %s roli statistikani ko'rdi: %d", key, w.Code)
		}
	}
	if w := do(t, h, "GET", statsPath(statsRestA, 6), "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("tokensiz so'rov: %d", w.Code)
	}
	if w := do(t, h, "GET", statsPath(statsRestB, 6), jwt["admin"], ""); w.Code != http.StatusOK {
		t.Fatalf("admin istalgan restoranni ko'rishi kerak: %d — %s", w.Code, w.Body.String())
	}
	if w := do(t, h, "GET", statsPath("yoq-restoran", 6), jwt["admin"], ""); w.Code != http.StatusNotFound {
		t.Fatalf("mavjud bo'lmagan restoran: %d", w.Code)
	}
}

func TestStatsValidatesQuery(t *testing.T) {
	h, _, jwt := statsServer(t)
	base := "/restaurants/" + statsRestA + "/stats"
	for _, q := range []string{
		"",
		"?from=2026-01-01",
		"?from=bugun&to=2026-01-02",
		"?from=2020-01-01&to=2022-01-01",
		"?from=2026-01-01&to=2026-01-02&granularity=year",
	} {
		if w := do(t, h, "GET", base+q, jwt["a"], ""); w.Code != http.StatusBadRequest {
			t.Errorf("%q uchun 400 kutilgan edi, keldi %d", q, w.Code)
		}
	}
}

// Asosiy tekshiruv: faqat O'Z restorani, faqat KO'RINADIGAN buyurtmalar,
// tushum faqat bajarilganlardan.
func TestStatsCountsOnlyVisibleOrdersOfOwnRestaurant(t *testing.T) {
	h, repo, jwt := statsServer(t)
	createdAt := time.Now().Add(-time.Minute)
	list := []orders.Order{
		{ID: "o1", RestaurantID: statsRestA, CustomerID: "c1", Status: orders.StatusDelivered,
			TotalTiyin: 5_000_000, CreatedAt: createdAt},
		{ID: "o2", RestaurantID: statsRestA, CustomerID: "c2", Status: orders.StatusCancelled,
			TotalTiyin: 2_000_000, CreatedAt: createdAt},
		{ID: "o3", RestaurantID: statsRestA, CustomerID: "c3", Status: orders.StatusCreated,
			TotalTiyin: 9_000_000, CreatedAt: createdAt,
			PaymentMethod: orders.PaymentCard, PaymentState: orders.PaymentAwaiting},
		{ID: "o4", RestaurantID: statsRestB, CustomerID: "c1", Status: orders.StatusDelivered,
			TotalTiyin: 7_000_000, CreatedAt: createdAt},
	}
	for i := range list {
		if err := repo.Save(context.Background(), &list[i]); err != nil {
			t.Fatal(err)
		}
	}

	w := do(t, h, "GET", statsPath(statsRestA, 6), jwt["a"], "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d — %s", w.Code, w.Body.String())
	}
	var res stats.Result
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	c := res.Current
	if c.Orders != 2 || c.Completed != 1 || c.Cancelled != 1 || c.InProgress != 0 {
		t.Fatalf("sonlar noto'g'ri (to'lanmagan yoki begona buyurtma kirib qolgan?): %+v", c)
	}
	if c.RevenueTiyin != 5_000_000 {
		t.Fatalf("tushum: %d", c.RevenueTiyin)
	}
	if c.NewCustomers != 1 {
		t.Fatalf("yangi mijozlar: %d", c.NewCustomers)
	}
	if len(res.Series) != 7 {
		t.Fatalf("7 kunlik nuqta kutilgan edi: %d", len(res.Series))
	}
}

type historyPage struct {
	Orders []struct {
		ID            string `json:"id"`
		CustomerPhone string `json:"customer_phone"`
	} `json:"orders"`
	NextCursor string          `json:"next_cursor"`
	Summary    *stats.Lifetime `json:"summary"`
}

// "Barcha buyurtmalar": egalik, sahifalash (bo'shliqsiz va takrorsiz),
// ko'rinish qoidasi va butun tarix xulosasi.
func TestOrderHistoryOwnershipPagingAndSummary(t *testing.T) {
	h, repo, jwt := statsServer(t)
	// Ikki Toshkent kuni: h1-h2 — D kuni, qolganlari — D+1.
	today := time.Now().In(stats.Location)
	base := time.Date(today.Year(), today.Month(), today.Day()-10, 10, 0, 0, 0, stats.Location)
	list := []orders.Order{
		{ID: "h1", RestaurantID: statsRestA, CustomerID: "u-cust", Status: orders.StatusDelivered,
			TotalTiyin: 5_000_000, CreatedAt: base},
		{ID: "h2", RestaurantID: statsRestA, CustomerID: "c2", Status: orders.StatusCancelled,
			TotalTiyin: 2_000_000, CreatedAt: base.Add(time.Hour)},
		{ID: "h3", RestaurantID: statsRestA, CustomerID: "u-cust", Status: orders.StatusServed,
			TotalTiyin: 3_000_000, CreatedAt: base.Add(24 * time.Hour)},
		{ID: "h4", RestaurantID: statsRestA, CustomerID: "c3", Status: orders.StatusPreparing,
			TotalTiyin: 1_000_000, CreatedAt: base.Add(25 * time.Hour)},
		// Ko'rinmasligi kerak: to'lanmagan karta va begona restoran.
		{ID: "h5", RestaurantID: statsRestA, CustomerID: "c4", Status: orders.StatusCreated,
			TotalTiyin: 9_000_000, CreatedAt: base.Add(26 * time.Hour),
			PaymentMethod: orders.PaymentCard, PaymentState: orders.PaymentAwaiting},
		{ID: "h6", RestaurantID: statsRestB, CustomerID: "c1", Status: orders.StatusDelivered,
			TotalTiyin: 7_000_000, CreatedAt: base.Add(27 * time.Hour)},
	}
	for i := range list {
		if err := repo.Save(context.Background(), &list[i]); err != nil {
			t.Fatal(err)
		}
	}
	path := "/restaurants/" + statsRestA + "/orders/history"

	if w := do(t, h, "GET", path, jwt["b"], ""); w.Code != http.StatusForbidden {
		t.Fatalf("XAVFSIZLIK: B restoran A buyurtmalar tarixini ko'rdi: %d", w.Code)
	}
	for _, key := range []string{"waiter", "customer"} {
		if w := do(t, h, "GET", path, jwt[key], ""); w.Code != http.StatusForbidden {
			t.Fatalf("XAVFSIZLIK: %s roli buyurtmalar tarixini ko'rdi: %d", key, w.Code)
		}
	}
	if w := do(t, h, "GET", path, "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("tokensiz so'rov: %d", w.Code)
	}
	if w := do(t, h, "GET", "/restaurants/yoq-restoran/orders/history", jwt["admin"], ""); w.Code != http.StatusNotFound {
		t.Fatalf("mavjud bo'lmagan restoran: %d", w.Code)
	}
	for _, q := range []string{
		"?status=delivered", "?limit=0", "?limit=51", "?cursor=!!!",
		"?from=2026-01-01", "?from=2026-01-02&to=2026-01-01", "?from=bugun&to=2026-01-01",
	} {
		if w := do(t, h, "GET", path+q, jwt["a"], ""); w.Code != http.StatusBadRequest {
			t.Errorf("%q uchun 400 kutilgan edi, keldi %d", q, w.Code)
		}
	}

	get := func(q string) historyPage {
		t.Helper()
		w := do(t, h, "GET", path+q, jwt["a"], "")
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status %d — %s", q, w.Code, w.Body.String())
		}
		var p historyPage
		if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}

	first := get("?limit=2")
	s := first.Summary
	if s == nil {
		t.Fatal("birinchi sahifada xulosa yo'q")
	}
	if s.Orders != 4 || s.Completed != 2 || s.Cancelled != 1 || s.InProgress != 1 {
		t.Fatalf("xulosa sonlari (to'lanmagan yoki begona buyurtma kirib qolgan?): %+v", s)
	}
	if s.RevenueTiyin != 8_000_000 || s.InProgressTiyin != 1_000_000 ||
		s.AvgOrderTiyin == nil || *s.AvgOrderTiyin != 4_000_000 {
		t.Fatalf("xulosa summalari: %+v", s)
	}

	var ids []string
	phones := map[string]string{}
	page := first
	for i := 0; ; i++ {
		if i > 0 && page.Summary != nil {
			t.Fatal("xulosa keyingi sahifalarda qayta hisoblanmasligi kerak")
		}
		for _, o := range page.Orders {
			ids = append(ids, o.ID)
			phones[o.ID] = o.CustomerPhone
		}
		if page.NextCursor == "" {
			break
		}
		if i > 5 {
			t.Fatal("sahifalash tugamadi")
		}
		page = get("?limit=2&cursor=" + page.NextCursor)
	}
	if want := "h4 h3 h2 h1"; strings.Join(ids, " ") != want {
		t.Fatalf("buyurtmalar: %v, kutilgan %s", ids, want)
	}
	if phones["h1"] != "+998900000035" || phones["h2"] != "" {
		t.Fatalf("mijoz telefoni: %v", phones)
	}

	done := get("?status=completed&limit=50")
	if len(done.Orders) != 2 || done.NextCursor != "" {
		t.Fatalf("bajarilganlar filtri: %+v", done)
	}

	// Davr filtri: ro'yxat ham, xulosa ham faqat shu kunlar bo'yicha.
	day := base.Format("2006-01-02")
	firstDay := get("?from=" + day + "&to=" + day)
	if len(firstDay.Orders) != 2 || firstDay.Orders[0].ID != "h2" || firstDay.Orders[1].ID != "h1" {
		t.Fatalf("davr filtri: %+v", firstDay.Orders)
	}
	if fs := firstDay.Summary; fs == nil || fs.Orders != 2 || fs.Completed != 1 || fs.Cancelled != 1 ||
		fs.RevenueTiyin != 5_000_000 {
		t.Fatalf("davr xulosasi: %+v", firstDay.Summary)
	}
	next := base.AddDate(0, 0, 1).Format("2006-01-02")
	secondDay := get("?status=in_progress&from=" + next + "&to=" + next)
	if len(secondDay.Orders) != 1 || secondDay.Orders[0].ID != "h4" {
		t.Fatalf("davr + holat filtri: %+v", secondDay.Orders)
	}
}
