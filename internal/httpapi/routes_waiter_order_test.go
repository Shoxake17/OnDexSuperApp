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
	"chustapp/internal/storage"
	"chustapp/internal/tables"
	"chustapp/internal/users"
	"chustapp/internal/ws"
)

// Affitsiant ilovasining stollar ro'yxati va buyurtma kiritish oqimi.
// Testlar asosan "ishlamasligi kerak" holatlarni tekshiradi: begona
// restoran stoli, begona taom, yopiq joy, takroriy so'rov, noto'g'ri rol.

type waiterFixture struct {
	h        http.Handler
	tableSvc *tables.Service
	orders   *storage.MemoryOrderRepo
	jwt      map[string]string
}

func newWaiterFixture(t *testing.T) *waiterFixture {
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
	mk("waiter-a", "u-wa1", users.RoleWaiter, testRestA, "+998900000291")
	mk("waiter-b", "u-wb1", users.RoleWaiter, testRestB, "+998900000292")
	mk("customer", "u-c1", users.RoleCustomer, "", "+998900000293")
	mk("restaurant-a", "u-ra1", users.RoleRestaurant, testRestA, "+998900000294")

	catalogRepo := storage.NewMemoryCatalogRepo(
		[]catalog.Restaurant{
			{ID: testRestA, Name: "Restoran A", Open: true},
			{ID: testRestB, Name: "Restoran B", Open: true},
		},
		[]catalog.Product{
			{ID: "prod-a", RestaurantID: testRestA, Name: "Osh", PriceTiyin: 3_500_000, Available: true},
			{ID: "prod-a-off", RestaurantID: testRestA, Name: "Somsa", PriceTiyin: 800_000, Available: false},
			{ID: "prod-b", RestaurantID: testRestB, Name: "Lag'mon", PriceTiyin: 3_000_000, Available: true},
		},
	)
	orderRepo := storage.NewMemoryOrderRepo()
	promos := storage.NewMemoryPromotionsRepo()
	tableSvc := tables.NewService(storage.NewMemoryTableRepo())
	deps := Deps{
		UserRepo:       userRepo,
		OrderRepo:      orderRepo,
		CatalogRepo:    catalogRepo,
		CourierRepo:    storage.NewMemoryCourierRepo(),
		PromotionsRepo: promos,
		Tokens:         tokens,
		Hub:            ws.NewHub(nil),
		CatalogSvc:     catalog.NewService(catalogRepo),
		OrderSvc:       orders.NewService(orderRepo, nil, NewID, promos),
		TableSvc:       tableSvc,
		TableOrders:    orderRepo,
		DevMode:        true,
	}
	return &waiterFixture{h: New(deps).Routes(nil), tableSvc: tableSvc, orders: orderRepo, jwt: jwt}
}

func waiterOrderBody(tableID, key string, items string) string {
	return `{"table_id":"` + tableID + `","party_size":3,"items":` + items +
		`,"idempotency_key":"` + key + `"}`
}

// ★ Stollar ro'yxatida QR token HECH QACHON bo'lmaydi va faqat o'z
// restorani joylari qaytadi.
func TestWaiterTables_NoSecretsOwnRestaurantOnly(t *testing.T) {
	f := newWaiterFixture(t)
	ctx := context.Background()
	tableA, err := f.tableSvc.Create(ctx, testRestA, "5")
	if err != nil {
		t.Fatal(err)
	}
	tableB, err := f.tableSvc.Create(ctx, testRestB, "7")
	if err != nil {
		t.Fatal(err)
	}

	w := do(t, f.h, "GET", "/waiter/tables", f.jwt["waiter-a"], "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /waiter/tables: %d %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, secret := range []string{"qr_token", "qr_link", tableA.QRToken, tableB.QRToken, tableB.ID} {
		if strings.Contains(body, secret) {
			t.Fatalf("XAVFSIZLIK: javobda bo'lmasligi kerak bo'lgan qiymat bor: %q", secret)
		}
	}
	var list []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0]["id"] != tableA.ID || list[0]["status"] != "available" {
		t.Fatalf("kutilgan bitta bo'sh joy, keldi: %v", list)
	}
}

// ★ Asosiy oqim: affitsiant stolga buyurtma kiritadi, narx serverda
// hisoblanadi, buyurtma oshxonaga (restoran paneliga) tushadi.
func TestWaiterPlacesOrder(t *testing.T) {
	f := newWaiterFixture(t)
	ctx := context.Background()
	table, err := f.tableSvc.Create(ctx, testRestA, "5")
	if err != nil {
		t.Fatal(err)
	}

	w := do(t, f.h, "POST", "/waiter/orders", f.jwt["waiter-a"],
		waiterOrderBody(table.ID, "key-1", `[{"product_id":"prod-a","qty":2}]`))
	if w.Code != http.StatusCreated {
		t.Fatalf("buyurtma yaratilmadi: %d %s", w.Code, w.Body.String())
	}
	var view map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view["table_label"] != "Asosiy zal · 5" || view["placed_by_waiter"] != true ||
		view["status"] != "created" || view["total_tiyin"] != float64(7_000_000) {
		t.Fatalf("javob noto'g'ri: %v", view)
	}
	if strings.Contains(w.Body.String(), "u-wa1") || strings.Contains(w.Body.String(), "customer_id") {
		t.Fatal("javobda xodim ID'si yoki mijoz maydoni bo'lmasligi kerak")
	}

	saved, err := f.orders.GetByID(ctx, view["id"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if saved.CustomerID != "" || saved.PlacedBy != "u-wa1" || !saved.IsDineIn() ||
		saved.TableID != table.ID || saved.PaymentMethod != orders.PaymentCash || saved.PartySize != 3 {
		t.Fatalf("saqlangan buyurtma noto'g'ri: %+v", saved)
	}

	// Restoran paneli buyurtmani ko'radi.
	w = do(t, f.h, "GET", "/restaurants/"+testRestA+"/orders", f.jwt["restaurant-a"], "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), saved.ID) {
		t.Fatalf("restoran paneli buyurtmani ko'rmadi: %d %s", w.Code, w.Body.String())
	}
	// Joy endi band.
	w = do(t, f.h, "GET", "/waiter/tables", f.jwt["waiter-a"], "")
	var list []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0]["status"] != "occupied" || list[0]["active_orders"] != float64(1) {
		t.Fatalf("joy band ko'rinmadi: %v", list)
	}
}

// Zaif tarmoqda qayta bosish oshxonaga IKKINCHI buyurtma yubormaydi.
func TestWaiterOrderIdempotent(t *testing.T) {
	f := newWaiterFixture(t)
	ctx := context.Background()
	table, _ := f.tableSvc.Create(ctx, testRestA, "5")
	other, _ := f.tableSvc.Create(ctx, testRestA, "6")
	body := waiterOrderBody(table.ID, "same-key", `[{"product_id":"prod-a","qty":1}]`)

	first := do(t, f.h, "POST", "/waiter/orders", f.jwt["waiter-a"], body)
	second := do(t, f.h, "POST", "/waiter/orders", f.jwt["waiter-a"], body)
	if first.Code != http.StatusCreated || second.Code != http.StatusCreated {
		t.Fatalf("kodlar: %d, %d", first.Code, second.Code)
	}
	var a, b map[string]any
	_ = json.Unmarshal(first.Body.Bytes(), &a)
	_ = json.Unmarshal(second.Body.Bytes(), &b)
	if a["id"] != b["id"] {
		t.Fatalf("takroriy so'rov yangi buyurtma yaratdi: %v != %v", a["id"], b["id"])
	}
	list, _ := f.orders.ListByRestaurant(ctx, testRestA, 10)
	if len(list) != 1 {
		t.Fatalf("oshxonada %d ta buyurtma, 1 ta kutilgan", len(list))
	}

	// Xuddi shu kalit BOSHQA stol bilan — eski buyurtma boshqa stolga
	// yozilib ketmaydi.
	w := do(t, f.h, "POST", "/waiter/orders", f.jwt["waiter-a"],
		waiterOrderBody(other.ID, "same-key", `[{"product_id":"prod-a","qty":1}]`))
	if w.Code != http.StatusConflict {
		t.Fatalf("boshqa stol bilan qayta ishlatilgan kalit: %d %s", w.Code, w.Body.String())
	}
}

// ★★ XAVFSIZLIK: begona restoran, begona taom, yopiq joy va buzuq so'rov.
func TestWaiterOrderRejectsInvalid(t *testing.T) {
	f := newWaiterFixture(t)
	ctx := context.Background()
	tableA, _ := f.tableSvc.Create(ctx, testRestA, "5")
	tableB, _ := f.tableSvc.Create(ctx, testRestB, "5")
	closed, _ := f.tableSvc.Create(ctx, testRestA, "9")
	if _, err := f.tableSvc.SetActive(ctx, closed.ID, false); err != nil {
		t.Fatal(err)
	}
	item := `[{"product_id":"prod-a","qty":1}]`

	cases := []struct {
		nom  string
		body string
		want int
	}{
		{"begona restoran stoli", waiterOrderBody(tableB.ID, "k1", item), http.StatusNotFound},
		{"mavjud bo'lmagan stol", waiterOrderBody("yo-q", "k2", item), http.StatusNotFound},
		{"begona restoran taomi", waiterOrderBody(tableA.ID, "k3", `[{"product_id":"prod-b","qty":1}]`), http.StatusBadRequest},
		{"yopiq joy", waiterOrderBody(closed.ID, "k4", item), http.StatusBadRequest},
		{"mavjud bo'lmagan taom", waiterOrderBody(tableA.ID, "k5", `[{"product_id":"prod-a-off","qty":1}]`), http.StatusBadRequest},
		{"bo'sh savat", waiterOrderBody(tableA.ID, "k6", `[]`), http.StatusBadRequest},
		{"kalitsiz", waiterOrderBody(tableA.ID, "", item), http.StatusBadRequest},
		{"juda ko'p mehmon", `{"table_id":"` + tableA.ID + `","party_size":51,"items":` + item + `,"idempotency_key":"k7"}`, http.StatusBadRequest},
		{"buzuq JSON", `{"table_id":`, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.nom, func(t *testing.T) {
			w := do(t, f.h, "POST", "/waiter/orders", f.jwt["waiter-a"], c.body)
			if w.Code != c.want {
				t.Fatalf("kutilgan %d, keldi %d: %s", c.want, w.Code, w.Body.String())
			}
		})
	}
	list, _ := f.orders.ListByRestaurant(ctx, testRestA, 10)
	listB, _ := f.orders.ListByRestaurant(ctx, testRestB, 10)
	if len(list)+len(listB) != 0 {
		t.Fatalf("rad etilgan so'rovlardan buyurtma yaratildi: %d", len(list)+len(listB))
	}
}

// Affitsiantdan boshqa rol buyurtma kirita olmaydi va stollarni ko'ra olmaydi.
func TestWaiterRoutesRequireWaiterRole(t *testing.T) {
	f := newWaiterFixture(t)
	table, _ := f.tableSvc.Create(context.Background(), testRestA, "5")
	body := waiterOrderBody(table.ID, "k", `[{"product_id":"prod-a","qty":1}]`)
	for _, role := range []string{"customer", "restaurant-a"} {
		if w := do(t, f.h, "POST", "/waiter/orders", f.jwt[role], body); w.Code != http.StatusForbidden {
			t.Fatalf("%s buyurtma kiritdi: %d", role, w.Code)
		}
		if w := do(t, f.h, "GET", "/waiter/tables", f.jwt[role], ""); w.Code != http.StatusForbidden {
			t.Fatalf("%s affitsiant stollarini ko'rdi: %d", role, w.Code)
		}
	}
	if w := do(t, f.h, "POST", "/waiter/orders", "", body); w.Code != http.StatusUnauthorized {
		t.Fatalf("tokensiz so'rov: %d", w.Code)
	}
}
