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
	"chustapp/internal/users"
	"chustapp/internal/ws"
)

// `POST /restaurants/{id}/products` testlari (bug.md 19 va 77-bandlar).
//
// ┌─ NEGA IKKALASI BITTA FAYLDA ───────────────────────────────────────┐
// Ikki xato bir joyda yashagan: handler tanadan kelgan `id` ni hech
// tekshirmasdan yozuvni TO'LIQ almashtirardi. Bundan ikki oqibat
// chiqadi — begona restoranning taomini bosib yozish (19) va
// yuborilmagan, server boshqaradigan maydonlarning (3D model)
// o'chishi (77). Tuzatish ham bitta: saqlashdan oldin mavjud yozuvni
// o'qish.
// └────────────────────────────────────────────────────────────────────┘

const (
	catTestRestA = "rest-a"
	catTestRestB = "rest-b"
)

// catalogServer — ikki restoran, har birida bitta taom. `rest-a` ning
// taomida TAYYOR 3D model bor (uni panel hech qachon yubormaydi).
func catalogServer(t *testing.T) (http.Handler, string, string) {
	t.Helper()
	ctx := context.Background()

	userRepo := storage.NewMemoryUserRepo()
	tokens := users.NewTokenIssuer("test-secret", time.Hour)

	mk := func(id, entityID, phone string) string {
		u := &users.User{ID: id, Phone: phone, Role: users.RoleRestaurant,
			EntityID: entityID, PhoneVerified: true, CreatedAt: time.Now()}
		if err := userRepo.Create(ctx, u); err != nil {
			t.Fatal(err)
		}
		jwt, err := tokens.Issue(u)
		if err != nil {
			t.Fatal(err)
		}
		return jwt
	}
	jwtA := mk("u-a", catTestRestA, "+998900000021")
	jwtB := mk("u-b", catTestRestB, "+998900000022")

	catalogRepo := storage.NewMemoryCatalogRepo(
		[]catalog.Restaurant{
			{ID: catTestRestA, Name: "A restoran", Open: true},
			{ID: catTestRestB, Name: "B restoran", Open: true},
		},
		[]catalog.Product{
			{ID: "somsa-a", RestaurantID: catTestRestA, Name: "Somsa",
				Category: "Taomlar", PriceTiyin: 1200000, Available: true,
				DiscountPriceTiyin: 900000,
				// Server boshqaradigan maydonlar — pullik xizmat natijasi.
				Model3DURL:    "https://cdn.example/models/somsa.glb",
				Model3DStatus: catalog.Model3DReady},
			{ID: "lagmon-b", RestaurantID: catTestRestB, Name: "Lag'mon",
				Category: "Taomlar", PriceTiyin: 3000000, Available: true},
		},
	)
	deps := Deps{
		UserRepo:       userRepo,
		CatalogRepo:    catalogRepo,
		PromotionsRepo: storage.NewMemoryPromotionsRepo(),
		OrderRepo:      storage.NewMemoryOrderRepo(),
		Tokens:         tokens,
		CatalogSvc:     catalog.NewService(catalogRepo),
		OrderSvc: orders.NewService(storage.NewMemoryOrderRepo(), nil, NewID,
			storage.NewMemoryPromotionsRepo()),
		Hub:     ws.NewHub(nil),
		DevMode: true,
	}
	return New(deps).Routes(nil), jwtA, jwtB
}

// panelProduct — panel ro'yxatidan bitta taomni oladi.
func panelProduct(t *testing.T, h http.Handler, jwt, restaurantID, productID string) map[string]any {
	t.Helper()
	w := do(t, h, "GET", "/restaurants/"+restaurantID+"/products", jwt, "")
	if w.Code != http.StatusOK {
		t.Fatalf("panel ro'yxati olinmadi: %d — %s", w.Code, w.Body.String())
	}
	var list []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	for _, p := range list {
		if p["id"] == productID {
			return p
		}
	}
	t.Fatalf("%q taomi ro'yxatda yo'q", productID)
	return nil
}

// 19-BANDNING REGRESSIYASI.
//
// B restorani O'Z yo'liga (`/restaurants/rest-b/products`) so'rov
// yuboradi — ya'ni yo'l bo'yicha egalik tekshiruvi o'tadi — lekin
// TANADA A restoranining taom ID sini beradi. Avval bu taom butunlay
// bosib yozilardi: raqibning menyusini o'chirish yoki narxini buzish
// mumkin edi.
func TestSaveProduct_RejectsForeignProductID(t *testing.T) {
	h, jwtA, jwtB := catalogServer(t)

	body := `{"id":"somsa-a","name":"BOSIB YOZILDI","category":"Taomlar",` +
		`"price_tiyin":1,"available":false}`
	w := do(t, h, "POST", "/restaurants/"+catTestRestB+"/products", jwtB, body)
	if w.Code != http.StatusForbidden {
		t.Fatalf("begona taom bosib yozildi: status %d — %s", w.Code, w.Body.String())
	}

	// A ning taomi tegilmagan bo'lishi kerak.
	p := panelProduct(t, h, jwtA, catTestRestA, "somsa-a")
	if p["name"] != "Somsa" {
		t.Fatalf("begona restoran taom nomini o'zgartirdi: %v", p["name"])
	}
	if p["price_tiyin"].(float64) != 1200000 {
		t.Fatalf("begona restoran narxni o'zgartirdi: %v", p["price_tiyin"])
	}
}

// 77-BANDNING REGRESSIYASI.
//
// Panel `model_3d_*` maydonlarini YUBORMAYDI (`api.dart` ro'yxatiga
// qarang). Avval to'liq almashtirish semantikasi tufayli har saqlash
// tayyor 3D modelni jimgina o'chirardi — hatto "Faol/Nofaol" tugmasi
// ham. Model pullik tashqi xizmatda yaratilgani uchun panel keyin uni
// QAYTA yaratishni taklif qilardi: har tahrir = yangi to'lov.
func TestSaveProduct_KeepsServerManagedFields(t *testing.T) {
	h, jwtA, _ := catalogServer(t)

	// Panel aynan shu maydonlarni yuboradi — model maydonlari YO'Q.
	body := `{"id":"somsa-a","name":"Somsa","category":"Taomlar",` +
		`"price_tiyin":1200000,"available":false}`
	w := do(t, h, "POST", "/restaurants/"+catTestRestA+"/products", jwtA, body)
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("saqlanmadi: %d — %s", w.Code, w.Body.String())
	}

	// 1. POST javobining O'ZIDA model bo'lishi kerak — panel keyingi
	//    qadamda aynan shu javobga qarab "3D yaratamizmi?" deb
	//    so'raydi (`_maybeOfferModel3D`). Javob bo'sh bo'lsa taklif
	//    HAR SAFAR chiqadi.
	var saved map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved["model_3d_url"] != "https://cdn.example/models/somsa.glb" {
		t.Fatalf("POST javobida 3D model yo'q: %v — panel qayta generatsiya taklif qiladi",
			saved["model_3d_url"])
	}
	if saved["model_3d_status"] != catalog.Model3DReady {
		t.Fatalf("POST javobida 3D holati yo'qoldi: %v", saved["model_3d_status"])
	}

	// 2. Bazada ham saqlanib qolgan bo'lishi kerak.
	p := panelProduct(t, h, jwtA, catTestRestA, "somsa-a")
	if p["model_3d_url"] != "https://cdn.example/models/somsa.glb" {
		t.Fatalf("3D model bazada O'CHDI: %v", p["model_3d_url"])
	}
	// Yuborilgan maydon esa yangilanishi kerak — tuzatish tahrirni
	// bloklab qo'ymasin.
	if p["available"] != false {
		t.Fatalf("yuborilgan maydon yangilanmadi: available=%v", p["available"])
	}
}

// Klient `model_3d_url` ni O'ZI yuborsa ham u e'tiborga olinmasligi
// kerak: 3D holati `internal/model3d` ning ishi. Aks holda panel
// (yoki qo'lda yozilgan so'rov) mahsulotni istalgan GLB havolasiga
// ko'rsatib qo'ya olardi.
func TestSaveProduct_IgnoresClientSuppliedModelFields(t *testing.T) {
	h, jwtA, _ := catalogServer(t)

	body := `{"id":"somsa-a","name":"Somsa","category":"Taomlar",` +
		`"price_tiyin":1200000,"available":true,` +
		`"model_3d_url":"https://evil.example/payload.glb",` +
		`"model_3d_status":"ready"}`
	if w := do(t, h, "POST", "/restaurants/"+catTestRestA+"/products", jwtA, body); w.Code >= 400 {
		t.Fatalf("saqlanmadi: %d — %s", w.Code, w.Body.String())
	}

	p := panelProduct(t, h, jwtA, catTestRestA, "somsa-a")
	if p["model_3d_url"] != "https://cdn.example/models/somsa.glb" {
		t.Fatalf("klient 3D havolasini o'zi belgiladi: %v", p["model_3d_url"])
	}
}

// Yangi taom yaratish (ID'siz) ishlashda davom etishi kerak.
func TestSaveProduct_CreateStillWorks(t *testing.T) {
	h, jwtA, _ := catalogServer(t)

	body := `{"name":"Yangi taom","category":"Taomlar","price_tiyin":500000,"available":true}`
	w := do(t, h, "POST", "/restaurants/"+catTestRestA+"/products", jwtA, body)
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("yangi taom yaratilmadi: %d — %s", w.Code, w.Body.String())
	}
	var saved map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if id, _ := saved["id"].(string); id == "" {
		t.Fatal("yangi taomga ID berilmadi")
	}
}

// Mavjud bo'lmagan ID bilan kelgan so'rov — bu HAM yaratish (panel
// oflayn yaratgan yozuvni sinxronlashi mumkin), rad etilmasligi kerak.
func TestSaveProduct_UnknownIDCreates(t *testing.T) {
	h, jwtA, _ := catalogServer(t)

	body := `{"id":"yangi-id","name":"Taom","category":"Taomlar",` +
		`"price_tiyin":500000,"available":true}`
	if w := do(t, h, "POST", "/restaurants/"+catTestRestA+"/products", jwtA, body); w.Code >= 400 {
		t.Fatalf("noma'lum ID rad etildi: %d — %s", w.Code, w.Body.String())
	}
	if p := panelProduct(t, h, jwtA, catTestRestA, "yangi-id"); p["name"] != "Taom" {
		t.Fatalf("yozuv saqlanmadi: %v", p)
	}
}
