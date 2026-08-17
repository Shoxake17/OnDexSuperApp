package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"chustapp/internal/couriers"
	"chustapp/internal/orders"
	"chustapp/internal/revoke"
	"chustapp/internal/storage"
	"chustapp/internal/users"
	"chustapp/internal/ws"
)

// Superadmin panelidagi "odamlar" bo'limi testlari.
//
// ┌─ NEGA HTTP DARAJASIDA ────────────────────────────────────────────┐
// Bu yerdagi qoidalarning hammasi handler ichida: kim so'ray oladi,
// kimni o'chirib bo'lmaydi, o'chirishdan oldin nima tekshiriladi va
// o'chirishdan KEYIN token darhol yaroqsiz bo'ladimi. Ularning
// birortasi ham repozitoriy testida ko'rinmaydi.
//
// Eng muhimi — oxirgisi: akkauntni bazadan o'chirish YETARLI EMAS,
// chunki `auth()` bazaga umuman qaramaydi. Bu aynan shu loyihada
// bir marta yo'l qo'yilgan xato (`internal/revoke` paketining paydo
// bo'lish sababi), shuning uchun u test bilan qulflab qo'yilgan.
// └───────────────────────────────────────────────────────────────────┘

type adminFixture struct {
	h           http.Handler
	userRepo    *storage.MemoryUserRepo
	orderRepo   orders.Repository
	courierRepo *storage.MemoryCourierRepo
	devices     *storage.MemoryDeviceStore
	revoked     *revoke.Store

	tickets     *ws.TicketStore
	adminJWT    string
	customerJWT string

	adminID    string
	customerID string
	waiterID   string
	courierID  string // kuryerning AKKAUNT id'si
	courierEnt string // kuryer yozuvi id'si
}

func adminServer(t *testing.T) *adminFixture {
	t.Helper()
	ctx := context.Background()

	userRepo := storage.NewMemoryUserRepo()
	courierRepo := storage.NewMemoryCourierRepo(
		couriers.Courier{ID: "cour-ent-1", Name: "Kuryer", Approved: true})
	orderRepo := storage.NewMemoryOrderRepo()
	devices := storage.NewMemoryDeviceStore()
	revoked := revoke.New(nil, time.Hour)
	tokens := users.NewTokenIssuer("test-secret", time.Hour)

	mk := func(id string, role users.Role, entityID, phone, name string) *users.User {
		u := &users.User{ID: id, Phone: phone, Name: name, Role: role,
			EntityID: entityID, PhoneVerified: true, CreatedAt: time.Now()}
		if err := userRepo.Create(ctx, u); err != nil {
			t.Fatal(err)
		}
		return u
	}
	admin := mk("admin-1", users.RoleAdmin, "", "+998900000001", "Superadmin")
	customer := mk("cust-1", users.RoleCustomer, "", "+998900000002", "Ali Valiyev")
	mk("waiter-1", users.RoleWaiter, "rest-a", "+998900000003", "Olim Karimov")
	mk("cour-1", users.RoleCourier, "cour-ent-1", "+998900000004", "Kuryer")

	adminJWT, err := tokens.Issue(admin)
	if err != nil {
		t.Fatal(err)
	}
	customerJWT, err := tokens.Issue(customer)
	if err != nil {
		t.Fatal(err)
	}

	deps := Deps{
		UserRepo:    userRepo,
		OrderRepo:   orderRepo,
		CourierRepo: courierRepo,
		CatalogRepo: storage.NewMemoryCatalogRepo(nil, nil),
		Devices:     devices,
		Tokens:      tokens,
		Revoked:     revoked,
		// Hub `nil` bo'lsa o'chirish handleri panika qilardi — bu
		// yerda haqiqiy hub ishlatiladi (ulanish yo'q, `Send` jim
		// o'tib ketadi).
		Hub:       ws.NewHub(nil),
		WsTickets: ws.NewTicketStore(),
		DevMode:   true,
	}
	return &adminFixture{
		h: New(deps).Routes(nil), userRepo: userRepo, orderRepo: orderRepo,
		courierRepo: courierRepo, devices: devices, revoked: revoked,
		tickets:  deps.WsTickets,
		adminJWT: adminJWT, customerJWT: customerJWT,
		adminID: "admin-1", customerID: "cust-1", waiterID: "waiter-1",
		courierID: "cour-1", courierEnt: "cour-ent-1",
	}
}

func decodeList(t *testing.T, body []byte) (int, []map[string]any) {
	t.Helper()
	var resp struct {
		Count int              `json:"count"`
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("javobni o'qib bo'lmadi: %v (%s)", err, body)
	}
	return resp.Count, resp.Items
}

// ★ ASOSIY OQIM: mijozlar ro'yxati soni, ismi, telefoni va QAYSI
// ilovadan kirgani bilan qaytadi.
func TestAdminCustomersList(t *testing.T) {
	f := adminServer(t)
	// Foydalanuvchi bir vaqtda TMA'da ham, Android ilovada ham bo'lishi
	// mumkin — ikkalasi ham ko'rinishi kerak.
	if err := f.devices.Touch(context.Background(), f.customerID, users.PlatformTMA, "1.4.0"); err != nil {
		t.Fatal(err)
	}
	if err := f.devices.Touch(context.Background(), f.customerID, users.PlatformAndroid, "1.3.9"); err != nil {
		t.Fatal(err)
	}

	w := do(t, f.h, "GET", "/admin/customers", f.adminJWT, "")
	if w.Code != http.StatusOK {
		t.Fatalf("kutilgan 200, olindi %d: %s", w.Code, w.Body)
	}
	count, items := decodeList(t, w.Body.Bytes())
	if count != 1 || len(items) != 1 {
		t.Fatalf("bitta mijoz kutilgandi, olindi count=%d items=%d", count, len(items))
	}
	row := items[0]
	if row["name"] != "Ali Valiyev" || row["phone"] != "+998900000002" {
		t.Errorf("ism/telefon noto'g'ri: %v", row)
	}
	devices, _ := row["devices"].([]any)
	if len(devices) != 2 {
		t.Fatalf("ikkita qurilma kutilgandi, olindi %v", row["devices"])
	}
	// Faqat mijozlar: affitsiant/kuryer/admin bu ro'yxatga tushmasligi
	// kerak (roli bo'yicha filtr).
	for _, it := range items {
		if it["id"] == f.waiterID || it["id"] == f.courierID {
			t.Error("mijozlar ro'yxatiga boshqa rol tushib qolgan")
		}
	}
}

// ★ Affitsiantlar ro'yxati — ismi, telefoni va restorani.
func TestAdminWaitersList(t *testing.T) {
	f := adminServer(t)
	w := do(t, f.h, "GET", "/admin/waiters", f.adminJWT, "")
	if w.Code != http.StatusOK {
		t.Fatalf("kutilgan 200, olindi %d: %s", w.Code, w.Body)
	}
	count, items := decodeList(t, w.Body.Bytes())
	if count != 1 {
		t.Fatalf("bitta affitsiant kutilgandi, olindi %d", count)
	}
	if items[0]["phone"] != "+998900000003" {
		t.Errorf("affitsiant telefoni noto'g'ri: %v", items[0])
	}
	if items[0]["restaurant_id"] != "rest-a" {
		t.Errorf("restoran bog'lanishi yo'qolgan: %v", items[0])
	}
}

// ★ MAXFIYLIK: bu ro'yxatlar shaxsiy ma'lumot — faqat superadmin.
func TestAdminListsRequireAdminRole(t *testing.T) {
	f := adminServer(t)
	for _, path := range []string{"/admin/customers", "/admin/waiters"} {
		if w := do(t, f.h, "GET", path, f.customerJWT, ""); w.Code != http.StatusForbidden {
			t.Errorf("%s: mijoz uchun 403 kutilgandi, olindi %d", path, w.Code)
		}
		if w := do(t, f.h, "GET", path, "", ""); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: tokensiz 401 kutilgandi, olindi %d", path, w.Code)
		}
	}
}

// ★ ASOSIY OQIM: akkaunt o'chiriladi VA qo'lidagi token DARHOL
// yaroqsiz bo'ladi.
//
// Ikkinchi qismi eng muhimi: bazadan o'chirishning o'zi tokenni
// to'xtatmaydi (`auth()` faqat imzoni tekshiradi).
func TestAdminDeleteUserRevokesSession(t *testing.T) {
	f := adminServer(t)

	// O'chirishdan OLDIN token ishlaydi.
	if w := do(t, f.h, "GET", "/me", f.customerJWT, ""); w.Code == http.StatusUnauthorized {
		t.Fatal("test boshida token yaroqli bo'lishi kerak edi")
	}

	w := do(t, f.h, "DELETE", "/admin/users/"+f.customerID, f.adminJWT, "")
	if w.Code != http.StatusOK {
		t.Fatalf("kutilgan 200, olindi %d: %s", w.Code, w.Body)
	}
	if _, err := f.userRepo.GetByID(context.Background(), f.customerID); err == nil {
		t.Error("akkaunt bazada qolib ketdi")
	}
	if w := do(t, f.h, "GET", "/me", f.customerJWT, ""); w.Code == http.StatusOK {
		t.Error("o'chirilgan akkaunt hali ham ma'lumot qaytaryapti")
	}

	// Bekor qilish belgisi QO'YILGANINI alohida tekshiramiz.
	//
	// NEGA HTTP orqali emas: `revoke` soniyagacha yaxlitlaydi va AYNAN
	// bekor qilish soniyasida chiqarilgan tokenni ataylab yaroqli
	// qoldiradi (`revoke.go` dagi izoh — halol qayta-login ishlashi
	// uchun). Testda token va o'chirish bir xil soniyaga tushadi,
	// ya'ni HTTP javobi bu qoidani ko'rsata olmaydi. Haqiqiy hayotda
	// esa token kirish paytida, o'chirish esa keyinroq bo'ladi —
	// quyidagi tekshiruv aynan shu holatni modellashtiradi.
	if !f.revoked.IsRevoked(f.customerID, time.Now().Add(-time.Minute)) {
		t.Error("sessiya bekor qilinmagan — eski token 30 kun ishlayverardi")
	}
}

// ★ Kuryer akkaunti o'chirilganda kuryer yozuvi ham ro'yxatdan chiqadi
// — aks holda egasiz "arvoh" kuryer qolardi.
func TestAdminDeleteCourierRemovesCourierRecord(t *testing.T) {
	f := adminServer(t)

	w := do(t, f.h, "DELETE", "/admin/users/"+f.courierID, f.adminJWT, "")
	if w.Code != http.StatusOK {
		t.Fatalf("kutilgan 200, olindi %d: %s", w.Code, w.Body)
	}
	list, err := f.courierRepo.ListAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range list {
		if c.ID == f.courierEnt {
			t.Error("kuryer yozuvi ro'yxatda qolib ketdi")
		}
	}
}

// ★ HIMOYA: o'z akkauntini va boshqa superadminni o'chirib bo'lmaydi.
func TestAdminCannotDeleteAdmins(t *testing.T) {
	f := adminServer(t)

	if w := do(t, f.h, "DELETE", "/admin/users/"+f.adminID, f.adminJWT, ""); w.Code != http.StatusBadRequest {
		t.Errorf("o'zini o'chirishda 400 kutilgandi, olindi %d", w.Code)
	}
	// Ikkinchi superadmin — panel orqali o'chirilmaydi.
	other := &users.User{ID: "admin-2", Phone: "+998900000009",
		Role: users.RoleAdmin, PhoneVerified: true, CreatedAt: time.Now()}
	if err := f.userRepo.Create(context.Background(), other); err != nil {
		t.Fatal(err)
	}
	if w := do(t, f.h, "DELETE", "/admin/users/admin-2", f.adminJWT, ""); w.Code != http.StatusForbidden {
		t.Errorf("boshqa superadmin uchun 403 kutilgandi, olindi %d", w.Code)
	}
}

// ★ HIMOYA: mijoz uchun faqat superadmin, va faqat mavjud akkaunt.
func TestAdminDeleteAuthorization(t *testing.T) {
	f := adminServer(t)

	if w := do(t, f.h, "DELETE", "/admin/users/"+f.waiterID, f.customerJWT, ""); w.Code != http.StatusForbidden {
		t.Errorf("mijoz o'chira olmasligi kerak edi, olindi %d", w.Code)
	}
	if w := do(t, f.h, "DELETE", "/admin/users/yo-q-akkaunt", f.adminJWT, ""); w.Code != http.StatusNotFound {
		t.Errorf("mavjud bo'lmagan akkaunt uchun 404 kutilgandi, olindi %d", w.Code)
	}
}

// ★ XAVFSIZLIK: WebSocket bileti ichidagi ROL tokendan olinadi.
//
// Ma'muriyat kanali (`notify.Admin()` — platformadagi BARCHA
// buyurtmalar oqimi) aynan shu rol bo'yicha ochiladi. Agar rolni
// mijoz ayta olsa yoki bilet uni saqlamasa, kanal begonaga ochilib
// ketardi — shuning uchun bu yerda biletning O'ZI tekshiriladi.
func TestWsTicketCarriesRoleFromToken(t *testing.T) {
	f := adminServer(t)

	check := func(jwt, wantRole, wantSubject string) {
		t.Helper()
		w := do(t, f.h, "POST", "/ws/ticket", jwt, "")
		if w.Code != http.StatusCreated {
			t.Fatalf("bilet olinmadi (%d): %s", w.Code, w.Body)
		}
		var resp struct {
			Ticket string `json:"ticket"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		claims, ok := f.tickets.Consume(resp.Ticket)
		if !ok {
			t.Fatal("bilet do'konda topilmadi")
		}
		if claims.Role != wantRole {
			t.Errorf("rol: kutilgan %q, olindi %q", wantRole, claims.Role)
		}
		if claims.Subject != wantSubject {
			t.Errorf("subject: kutilgan %q, olindi %q", wantSubject, claims.Subject)
		}
	}

	check(f.adminJWT, string(users.RoleAdmin), f.adminID)
	// Mijoz uchun rol HECH QACHON "admin" bo'lib qolmasligi kerak.
	check(f.customerJWT, string(users.RoleCustomer), f.customerID)
}

// ★ Yakunlanmagan buyurtmasi bor mijoz o'chirilmaydi (409).
//
// Sabab: o'rtada o'chirish jonli yetkazishni buzadi — kuryer manzilsiz
// qoladi, buyurtma esa hech kim yopa olmaydigan holatda osiladi.
func TestAdminDeleteBlockedByActiveOrder(t *testing.T) {
	f := adminServer(t)
	ctx := context.Background()

	active := &orders.Order{
		ID: "ord-1", CustomerID: f.customerID, RestaurantID: "rest-a",
		Status: orders.StatusPreparing, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := f.orderRepo.Save(ctx, active); err != nil {
		t.Fatal(err)
	}

	w := do(t, f.h, "DELETE", "/admin/users/"+f.customerID, f.adminJWT, "")
	if w.Code != http.StatusConflict {
		t.Fatalf("409 kutilgandi, olindi %d: %s", w.Code, w.Body)
	}
	// Akkaunt TEGILMAGAN bo'lishi kerak.
	if _, err := f.userRepo.GetByID(ctx, f.customerID); err != nil {
		t.Error("rad etilgan o'chirish akkauntga tegib ketdi")
	}

	// Buyurtma yakunlangach — o'chirish ishlaydi.
	active.Status = orders.StatusDelivered
	if err := f.orderRepo.Save(ctx, active); err != nil {
		t.Fatal(err)
	}
	if w := do(t, f.h, "DELETE", "/admin/users/"+f.customerID, f.adminJWT, ""); w.Code != http.StatusOK {
		t.Errorf("yakunlangan buyurtmadan keyin o'chirish ishlashi kerak edi, olindi %d: %s", w.Code, w.Body)
	}
}
