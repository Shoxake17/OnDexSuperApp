package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/orders"
	"chustapp/internal/storage"
	"chustapp/internal/tables"
	"chustapp/internal/users"
	"chustapp/internal/ws"
)

// tablesServer — ikki restoran; tokenlar: "a", "b" (restoran xodimlari),
// "waiter" (rest-a), "customer".
func tablesServer(t *testing.T) (http.Handler, *storage.MemoryOrderRepo, *tables.Service, map[string]string) {
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
	mk("a", "u-a", users.RoleRestaurant, testRestA, "+998900000051")
	mk("b", "u-b", users.RoleRestaurant, testRestB, "+998900000052")
	mk("waiter", "u-w", users.RoleWaiter, testRestA, "+998900000053")
	mk("customer", "u-c", users.RoleCustomer, "", "+998900000054")

	catalogRepo := storage.NewMemoryCatalogRepo(
		[]catalog.Restaurant{{ID: testRestA, Name: "A", Open: true}, {ID: testRestB, Name: "B", Open: true}}, nil)
	orderRepo := storage.NewMemoryOrderRepo()
	tableSvc := tables.NewService(storage.NewMemoryTableRepo())
	h := New(Deps{
		UserRepo:       userRepo,
		CatalogRepo:    catalogRepo,
		PromotionsRepo: storage.NewMemoryPromotionsRepo(),
		OrderRepo:      orderRepo,
		TableSvc:       tableSvc,
		TableOrders:    orderRepo,
		Tokens:         tokens,
		CatalogSvc:     catalog.NewService(catalogRepo),
		OrderSvc:       orders.NewService(orderRepo, nil, NewID, storage.NewMemoryPromotionsRepo()),
		Hub:            ws.NewHub(nil),
		DevMode:        true,
	}).Routes(nil)
	return h, orderRepo, tableSvc, jwt
}

type tableView struct {
	ID            string                 `json:"id"`
	Kind          string                 `json:"kind"`
	KindTitle     string                 `json:"kind_title"`
	Label         string                 `json:"label"`
	DisplayLabel  string                 `json:"display_label"`
	Capacity      *int                   `json:"capacity"`
	QRToken       string                 `json:"qr_token"`
	Status        string                 `json:"status"`
	CleaningSince *time.Time             `json:"cleaning_since"`
	LastScannedAt *time.Time             `json:"last_scanned_at"`
	ActiveOrders  []tables.OrderSnapshot `json:"active_orders"`
	LastOrder     *tables.OrderSnapshot  `json:"last_order"`
}

func listTables(t *testing.T, h http.Handler, token string) map[string]tableView {
	t.Helper()
	w := do(t, h, "GET", "/restaurants/"+testRestA+"/tables", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("ro'yxat: %d — %s", w.Code, w.Body.String())
	}
	var list []tableView
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	out := map[string]tableView{}
	for _, v := range list {
		out[v.Label] = v
	}
	return out
}

// Holatlar buyurtmalardan HISOBLANADI: band — yakunlanmagan stol
// buyurtmasi bor; yakunlangan yoki to'lanmagan karta buyurtmasi joyni
// band qilmaydi.
func TestTablesListShowsLiveStatus(t *testing.T) {
	h, repo, svc, jwt := tablesServer(t)
	ctx := context.Background()
	busy, _ := svc.Create(ctx, testRestA, "1")
	free, _ := svc.Create(ctx, testRestA, "2")
	closed, _ := svc.Create(ctx, testRestA, "3")
	cleaning, _ := svc.Create(ctx, testRestA, "4")
	if _, err := svc.SetActive(ctx, closed.ID, false); err != nil {
		t.Fatal(err)
	}
	w := do(t, h, "PATCH", "/tables/"+cleaning.ID, jwt["a"], `{"cleaning":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("tozalash belgisi: %d — %s", w.Code, w.Body.String())
	}

	now := time.Now()
	for _, o := range []orders.Order{
		{ID: "o1", OrderNumber: "140926-1024", RestaurantID: testRestA, Type: orders.TypeDineIn, TableID: busy.ID,
			Status: orders.StatusPreparing, Items: []orders.Item{{Qty: 2}, {Qty: 1}}, CreatedAt: now.Add(-5 * time.Minute)},
		{ID: "o2", RestaurantID: testRestA, Type: orders.TypeDineIn, TableID: free.ID,
			Status: orders.StatusServed, CreatedAt: now.Add(-time.Hour)},
		{ID: "o3", RestaurantID: testRestA, Type: orders.TypeDineIn, TableID: free.ID, Status: orders.StatusCreated,
			CreatedAt: now, PaymentMethod: orders.PaymentCard, PaymentState: orders.PaymentAwaiting},
		// Begona restoranning shu stol ID'li buyurtmasi hisobga olinmaydi.
		{ID: "o4", RestaurantID: testRestB, Type: orders.TypeDineIn, TableID: closed.ID,
			Status: orders.StatusPreparing, CreatedAt: now},
	} {
		o := o
		if err := repo.Save(ctx, &o); err != nil {
			t.Fatal(err)
		}
	}

	got := listTables(t, h, jwt["a"])
	if got["1"].Status != "occupied" || len(got["1"].ActiveOrders) != 1 || got["1"].ActiveOrders[0].Items != 3 ||
		got["1"].ActiveOrders[0].OrderNumber != "140926-1024" {
		t.Errorf("band joy: %+v", got["1"])
	}
	if got["2"].Status != "available" || len(got["2"].ActiveOrders) != 0 {
		t.Errorf("bo'sh joy (yakunlangan va to'lanmagan buyurtma band qilmaydi): %+v", got["2"])
	}
	if got["3"].Status != "inactive" {
		t.Errorf("yopiq joy: %+v", got["3"])
	}
	if got["4"].Status != "cleaning" || got["4"].CleaningSince == nil {
		t.Errorf("tozalanmoqda: %+v", got["4"])
	}
	if got["1"].QRToken == "" || got["1"].Kind != "table" || got["1"].KindTitle != "Stol" {
		t.Errorf("maydonlar: %+v", got["1"])
	}
}

// "Tozalanmoqda" belgisidan keyin yangi buyurtma kelsa — belgi eskirgan.
func TestTablesCleaningFlagExpiresAfterNewOrder(t *testing.T) {
	h, repo, svc, jwt := tablesServer(t)
	ctx := context.Background()
	x, _ := svc.Create(ctx, testRestA, "1")
	if w := do(t, h, "PATCH", "/tables/"+x.ID, jwt["a"], `{"cleaning":true}`); w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	o := orders.Order{ID: "o1", RestaurantID: testRestA, Type: orders.TypeDineIn, TableID: x.ID,
		Status: orders.StatusServed, CreatedAt: time.Now().Add(time.Minute)}
	if err := repo.Save(ctx, &o); err != nil {
		t.Fatal(err)
	}
	got := listTables(t, h, jwt["a"])["1"]
	if got.Status != "available" || got.LastOrder == nil || got.LastOrder.ID != "o1" {
		t.Fatalf("eskirgan belgi: %+v", got)
	}
}

func TestTablesCreateBatchAndKinds(t *testing.T) {
	h, _, _, jwt := tablesServer(t)
	path := "/restaurants/" + testRestA + "/tables"

	w := do(t, h, "POST", path, jwt["a"], `{"label":"1","kind":"vip_room","zone":"2-qavat","capacity":12}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("yaratish: %d — %s", w.Code, w.Body.String())
	}
	var one tableView
	_ = json.Unmarshal(w.Body.Bytes(), &one)
	if one.Kind != "vip_room" || one.Capacity == nil || *one.Capacity != 12 || one.DisplayLabel != "2-qavat · VIP xona 1" {
		t.Fatalf("VIP xona: %+v", one)
	}

	w = do(t, h, "POST", path+"/batch", jwt["a"], `{"kind":"cabin","prefix":"","from":1,"count":3,"capacity":4}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("partiya: %d — %s", w.Code, w.Body.String())
	}
	var batch []tableView
	_ = json.Unmarshal(w.Body.Bytes(), &batch)
	if len(batch) != 3 || batch[0].DisplayLabel != "Asosiy zal · Kabina 1" {
		t.Fatalf("partiya natijasi: %+v", batch)
	}

	cases := []struct {
		body string
		code int
	}{
		{`{"kind":"cabin","from":3,"count":2}`, http.StatusConflict},
		{`{"kind":"cabin","from":1,"count":101}`, http.StatusBadRequest},
		{`{"kind":"sauna","from":1,"count":1}`, http.StatusBadRequest},
		{`{"kind":"cabin","from":50,"count":1,"capacity":0}`, http.StatusBadRequest},
		{`{"kind":`, http.StatusBadRequest},
	}
	for _, c := range cases {
		if w := do(t, h, "POST", path+"/batch", jwt["a"], c.body); w.Code != c.code {
			t.Errorf("%s: kutilgan %d, keldi %d — %s", c.body, c.code, w.Code, w.Body.String())
		}
	}
	// Chetdagi bo'shliq (yangi qator ham) kesiladi — bu xato emas; nom ICHIDAGI
	// boshqaruv belgisi esa rad etiladi.
	if w := do(t, h, "POST", path, jwt["a"], `{"label":"5A","kind":"table"}`); w.Code != http.StatusBadRequest {
		t.Errorf("boshqaruv belgili nom: %d", w.Code)
	}
	// Ro'yxat xom holda sanaladi: `listTables` nom bo'yicha xarita quradi,
	// "VIP xona 1" va "Kabina 1" esa bir xil ("1") nomli.
	w = do(t, h, "GET", path, jwt["a"], "")
	var all []tableView
	if err := json.Unmarshal(w.Body.Bytes(), &all); err != nil {
		t.Fatal(err)
	}
	if len(all) != 4 {
		t.Fatalf("yiqilgan so'rovlardan joylar qolib ketdi yoki yo'qoldi: %d", len(all))
	}

	w = do(t, h, "GET", "/tables/kinds", jwt["a"], "")
	var kinds []tables.KindInfo
	_ = json.Unmarshal(w.Body.Bytes(), &kinds)
	if w.Code != http.StatusOK || len(kinds) != len(tables.Kinds()) {
		t.Fatalf("turlar: %d %+v", w.Code, kinds)
	}
}

// ★★ QR token hech qanday tahrirda o'zgarmaydi — HTTP orqali ham.
func TestTablesPatchKeepsQRToken(t *testing.T) {
	h, _, svc, jwt := tablesServer(t)
	x, _ := svc.Create(context.Background(), testRestA, "1")
	w := do(t, h, "PATCH", "/tables/"+x.ID, jwt["a"],
		`{"label":"7","zone":"Ayvon","kind":"tapchan","capacity":8,"cleaning":true,"active":false}`)
	if w.Code != http.StatusOK {
		t.Fatalf("tahrir: %d — %s", w.Code, w.Body.String())
	}
	var v tableView
	_ = json.Unmarshal(w.Body.Bytes(), &v)
	if v.QRToken != x.QRToken || v.Kind != "tapchan" || *v.Capacity != 8 || v.DisplayLabel != "Ayvon · Topchan 7" {
		t.Fatalf("tahrir natijasi: %+v", v)
	}
	if w := do(t, h, "PATCH", "/tables/"+x.ID, jwt["a"], `{"capacity":null}`); w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	if w := do(t, h, "PATCH", "/tables/"+x.ID, jwt["a"], `{"capacity":"ko'p"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("noto'g'ri sig'im: %d", w.Code)
	}
	got, _ := svc.Get(context.Background(), x.ID)
	if got.Capacity != nil || got.QRToken != x.QRToken {
		t.Fatalf("sig'im tozalanmadi yoki token o'zgardi: %+v", got)
	}
}

func TestTablesOwnershipAndRoles(t *testing.T) {
	h, _, svc, jwt := tablesServer(t)
	x, _ := svc.Create(context.Background(), testRestA, "1")
	pathA := "/restaurants/" + testRestA + "/tables"

	for _, c := range []struct {
		method, path, token, body string
		want                      int
	}{
		{"GET", pathA, jwt["b"], "", http.StatusNotFound},
		{"POST", pathA + "/batch", jwt["b"], `{"from":1,"count":1}`, http.StatusNotFound},
		{"PATCH", "/tables/" + x.ID, jwt["b"], `{"cleaning":true}`, http.StatusNotFound},
		{"DELETE", "/tables/" + x.ID, jwt["b"], "", http.StatusNotFound},
		{"GET", pathA, jwt["waiter"], "", http.StatusForbidden},
		{"GET", "/tables/kinds", jwt["customer"], "", http.StatusForbidden},
		{"GET", pathA, "", "", http.StatusUnauthorized},
	} {
		if w := do(t, h, c.method, c.path, c.token, c.body); w.Code != c.want {
			t.Errorf("%s %s: kutilgan %d, keldi %d", c.method, c.path, c.want, w.Code)
		}
	}
	got, _ := svc.Get(context.Background(), x.ID)
	if got.CleaningSince != nil {
		t.Fatal("XAVFSIZLIK: begona restoran joy holatini o'zgartirdi")
	}
}

// Mijoz QR'ni skanerlaganda vaqt yoziladi, soxta token hech narsa qoldirmaydi.
func TestTableResolveRecordsScan(t *testing.T) {
	h, _, svc, jwt := tablesServer(t)
	x, _ := svc.Create(context.Background(), testRestA, "1")

	fake := "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	if w := do(t, h, "GET", "/tables/resolve?token="+fake, jwt["customer"], ""); w.Code != http.StatusNotFound {
		t.Fatalf("soxta token: %d", w.Code)
	}
	if v := listTables(t, h, jwt["a"])["1"]; v.LastScannedAt != nil {
		t.Fatal("soxta token skanerlash sifatida yozildi")
	}

	w := do(t, h, "GET", "/tables/resolve?token="+x.QRToken, jwt["customer"], "")
	if w.Code != http.StatusOK {
		t.Fatalf("resolve: %d — %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if _, leaked := resp["qr_token"]; leaked {
		t.Fatal("resolve javobida token qaytdi")
	}
	if v := listTables(t, h, jwt["a"])["1"]; v.LastScannedAt == nil {
		t.Fatal("skanerlash vaqti yozilmadi")
	}
}
