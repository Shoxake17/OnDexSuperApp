package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"chustapp/internal/alerts"
	"chustapp/internal/catalog"
	"chustapp/internal/storage"
	"chustapp/internal/users"
	"chustapp/internal/ws"
)

func alertsServer(t *testing.T) (http.Handler, *alerts.Service, map[string]string) {
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
	mk("a", "u-a", users.RoleRestaurant, testRestA, "+998900000081")
	mk("b", "u-b", users.RoleRestaurant, testRestB, "+998900000082")
	mk("admin", "u-admin", users.RoleAdmin, "", "+998900000083")
	mk("waiter", "u-w", users.RoleWaiter, testRestA, "+998900000084")
	mk("customer", "u-c", users.RoleCustomer, "", "+998900000085")

	catalogRepo := storage.NewMemoryCatalogRepo(
		[]catalog.Restaurant{{ID: testRestA, Name: "Book Cafe", Open: true}, {ID: testRestB, Name: "B", Open: true}}, nil)
	svc := alerts.NewService(storage.NewMemoryAlertStore(), ws.NewHub(nil), NewID)
	h := New(Deps{
		UserRepo:       userRepo,
		CatalogRepo:    catalogRepo,
		PromotionsRepo: storage.NewMemoryPromotionsRepo(),
		OrderRepo:      storage.NewMemoryOrderRepo(),
		Tokens:         tokens,
		AlertsSvc:      svc,
		Hub:            ws.NewHub(nil),
		DevMode:        true,
	}).Routes(nil)
	return h, svc, jwt
}

type alertListResp struct {
	Items      []alerts.View `json:"items"`
	NextBefore int64         `json:"next_before"`
	Counts     alerts.Counts `json:"counts"`
	Unread     int           `json:"unread"`
	LatestSeq  int64         `json:"latest_seq"`
}

func getAlerts(t *testing.T, h http.Handler, token, query string) alertListResp {
	t.Helper()
	w := do(t, h, "GET", "/restaurants/"+testRestA+"/notifications"+query, token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s: %d — %s", query, w.Code, w.Body.String())
	}
	var r alertListResp
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func seedAlerts(t *testing.T, svc *alerts.Service) []*alerts.Notification {
	t.Helper()
	ctx := context.Background()
	var out []*alerts.Notification
	for _, in := range []alerts.Input{
		{Kind: alerts.KindNewOrder, Category: alerts.CategoryNew, Title: "Yangi buyurtma", Body: "#1 buyurtma tushdi. Jami: 180 000 so'm"},
		{Kind: alerts.KindPaymentReceived, Category: alerts.CategorySuccess, Title: "To'lov qabul qilindi", Body: "Karta orqali 140 000 so'm"},
		{Kind: alerts.KindDispatchFailed, Category: alerts.CategoryImportant, Title: "Kuryer topishda xatolik", Body: "100% tizim xatosi"},
	} {
		n, err := svc.Publish(ctx, testRestA, in)
		if err != nil || n == nil {
			t.Fatal(err)
		}
		out = append(out, n)
	}
	if _, err := svc.Publish(ctx, testRestB, alerts.Input{Kind: "k", Category: alerts.CategoryInfo, Title: "B xabari"}); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestAlertsTenancyAndRoles(t *testing.T) {
	h, svc, jwt := alertsServer(t)
	seedAlerts(t, svc)
	path := "/restaurants/" + testRestA + "/notifications"
	for _, c := range []struct {
		token string
		want  int
	}{
		{jwt["b"], http.StatusNotFound},
		{jwt["waiter"], http.StatusForbidden},
		{jwt["customer"], http.StatusForbidden},
		{"", http.StatusUnauthorized},
		{jwt["admin"], http.StatusOK},
		{jwt["a"], http.StatusOK},
	} {
		if w := do(t, h, "GET", path, c.token, ""); w.Code != c.want {
			t.Errorf("GET: kutilgan %d, keldi %d", c.want, w.Code)
		}
	}
	// Jonli kanal: rahbariyat kanali faqat restoran akkauntiga.
	srv := New(Deps{})
	req := httptest.NewRequest("GET", "/ws", nil)
	if !slices.Contains(srv.wsSubscriptionKeys(req, "u-a", testRestA, string(users.RoleRestaurant)), alerts.Topic(testRestA)) {
		t.Fatal("restoran akkaunti rahbariyat kanalini olmadi")
	}
	if slices.Contains(srv.wsSubscriptionKeys(req, "u-w", testRestA, string(users.RoleWaiter)), alerts.Topic(testRestA)) {
		t.Fatal("XAVFSIZLIK: affitsiant rahbariyat kanaliga obuna bo'ldi")
	}
}

func TestAlertsListFiltersPagingAndRead(t *testing.T) {
	h, svc, jwt := alertsServer(t)
	seeded := seedAlerts(t, svc)

	all := getAlerts(t, h, jwt["a"], "")
	if len(all.Items) != 3 || all.Items[0].Seq <= all.Items[2].Seq || all.Unread != 3 || all.Counts.Total != 3 ||
		all.Counts.ByCategory[alerts.CategorySuccess] != 1 || all.LatestSeq != seeded[2].Seq {
		t.Fatalf("ro'yxat: %+v", all)
	}
	if strings.Contains(do(t, h, "GET", "/restaurants/"+testRestA+"/notifications", jwt["a"], "").Body.String(), "dedupe") {
		t.Fatal("ichki kalit javobda")
	}
	if r := getAlerts(t, h, jwt["a"], "?category=success"); len(r.Items) != 1 || r.Items[0].Kind != alerts.KindPaymentReceived {
		t.Fatalf("tur filtri: %+v", r.Items)
	}
	if r := getAlerts(t, h, jwt["a"], "?q=140%20000"); len(r.Items) != 1 {
		t.Fatalf("qidiruv: %+v", r.Items)
	}
	if r := getAlerts(t, h, jwt["a"], "?q=100%25"); len(r.Items) != 1 {
		t.Fatalf("%% so'zma-so'z qidirilishi kerak: %+v", r.Items)
	}
	if r := getAlerts(t, h, jwt["a"], "?period=today"); len(r.Items) != 3 {
		t.Fatalf("bugun: %+v", r.Items)
	}
	page := getAlerts(t, h, jwt["a"], "?limit=2")
	if len(page.Items) != 2 || page.NextBefore != page.Items[1].Seq {
		t.Fatalf("sahifa: %+v", page)
	}
	rest := getAlerts(t, h, jwt["a"], "?limit=2&before="+itoa(page.NextBefore))
	if len(rest.Items) != 1 || rest.NextBefore != 0 {
		t.Fatalf("keyingi sahifa: %+v", rest)
	}
	after := getAlerts(t, h, jwt["a"], "?after="+itoa(seeded[0].Seq))
	if len(after.Items) != 2 || after.Items[0].Seq != seeded[1].Seq {
		t.Fatalf("catch-up: %+v", after.Items)
	}
	for _, bad := range []string{"?category=hack", "?period=year", "?limit=500", "?before=-1", "?after=abc", "?q=" + strings.Repeat("a", 101)} {
		if w := do(t, h, "GET", "/restaurants/"+testRestA+"/notifications"+bad, jwt["a"], ""); w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", bad, w.Code)
		}
	}

	// O'qish: o'z yozuvi — ha; begona restoranniki — o'zgarmaydi.
	base := "/restaurants/" + testRestA + "/notifications/"
	if w := do(t, h, "POST", base+seeded[0].ID+"/read", jwt["a"], ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"unread":2`) {
		t.Fatalf("read: %d %s", w.Code, w.Body.String())
	}
	bItems, _ := svc.List(context.Background(), testRestB, alerts.Query{Limit: 10})
	do(t, h, "POST", base+bItems[0].ID+"/read", jwt["a"], "")
	if u, _, _ := svc.Summary(context.Background(), testRestB); u != 1 {
		t.Fatal("XAVFSIZLIK: begona restoran bildirishnomasi o'qilgan bo'ldi")
	}
	if w := do(t, h, "POST", base+"..%2F..%2Fx/read", jwt["a"], ""); w.Code == http.StatusOK {
		t.Fatal("noto'g'ri ID qabul qilindi")
	}
	for _, body := range []string{`{}`, `{"up_to_seq":0}`, `{"up_to_seq":5,"all":true}`, `[]`} {
		if w := do(t, h, "POST", base+"read-all", jwt["a"], body); w.Code != http.StatusBadRequest {
			t.Errorf("read-all %s: %d", body, w.Code)
		}
	}
	w := do(t, h, "POST", base+"read-all", jwt["a"], `{"up_to_seq":`+itoa(seeded[1].Seq)+`}`)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"unread":1`) {
		t.Fatalf("read-all: %d %s", w.Code, w.Body.String())
	}
	sum := do(t, h, "GET", base+"summary", jwt["a"], "")
	if sum.Code != http.StatusOK || !strings.Contains(sum.Body.String(), `"unread":1`) {
		t.Fatalf("summary: %s", sum.Body.String())
	}
}

func TestAdminPlatformUpdate(t *testing.T) {
	h, svc, jwt := alertsServer(t)
	path := "/admin/restaurant-notifications"
	for _, key := range []string{"a", "waiter", "customer"} {
		if w := do(t, h, "POST", path, jwt[key], `{"title":"x"}`); w.Code != http.StatusForbidden {
			t.Errorf("%s: %d", key, w.Code)
		}
	}
	for _, body := range []string{`{"title":""}`, `{"title":"a\u0000b"}`, `{"title":"` + strings.Repeat("a", 121) + `"}`,
		`{"title":"ok","extra":1}`} {
		if w := do(t, h, "POST", path, jwt["admin"], body); w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", body, w.Code)
		}
	}
	if w := do(t, h, "POST", path, jwt["admin"], `{"title":"ok","restaurant_id":"yoq"}`); w.Code != http.StatusNotFound {
		t.Fatalf("mavjud bo'lmagan restoran: %d", w.Code)
	}
	w := do(t, h, "POST", path, jwt["admin"], `{"title":"Tizim yangilanishi","body":"Restoran panelida yangi funksiyalar qo'shildi."}`)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"sent":2`) {
		t.Fatalf("yuborish: %d %s", w.Code, w.Body.String())
	}
	items, _ := svc.List(context.Background(), testRestA, alerts.Query{Limit: 10})
	if len(items) != 1 || items[0].Category != alerts.CategoryUpdate {
		t.Fatalf("restoranda: %+v", items)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
