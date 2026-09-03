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
)

// `POST /orders` dagi `expected_total_tiyin` tekshiruvining HTTP
// darajasidagi testlari.
//
// ┌─ NEGA AYNAN SHU DARAJADA ─────────────────────────────────────────┐
// `orders` paketidagi test `CreateExpecting` ning O'ZINI tekshiradi.
// Lekin ilova uchun muhimi — handler o'sha xatoni 409 GA va YANGI
// SUMMA bilan aylantirishi: shu bo'g'in uzilsa, ilova "narx o'zgardi"
// deb ayta oladi-yu, yangi raqamni ko'rsata olmaydi va foydalanuvchi
// nima qilishni bilmay qoladi.
// └───────────────────────────────────────────────────────────────────┘

// totalCheckServer — bitta restoran, bitta taom (30 000 so'm) va
// manzili Chust ichida bo'lgan mijoz.
func totalCheckServer(t *testing.T) (http.Handler, string) {
	t.Helper()
	ctx := context.Background()

	userRepo := storage.NewMemoryUserRepo()
	tokens := users.NewTokenIssuer("test-secret", time.Hour)

	customer := &users.User{
		ID: "cust-1", Phone: "+998901234567",
		Role: users.RoleCustomer, PhoneVerified: true,
		// Chust markazi — `delivery.Covered` shu nuqtani qabul qiladi.
		Address: users.AddressDetails{
			Lat: 40.99454866260851, Lng: 71.24035120010376,
			Text: "Ipak Yo'li ko'chasi",
		},
	}
	if err := userRepo.Create(ctx, customer); err != nil {
		t.Fatal(err)
	}
	jwt, err := tokens.Issue(customer)
	if err != nil {
		t.Fatal(err)
	}

	catalogRepo := storage.NewMemoryCatalogRepo(
		[]catalog.Restaurant{{ID: "rest-1", Name: "Avigo", Open: true}},
		[]catalog.Product{{
			ID: "prod-1", RestaurantID: "rest-1", Name: "Osh",
			PriceTiyin: 3000000, Available: true,
		}},
	)
	orderRepo := storage.NewMemoryOrderRepo()

	deps := Deps{
		UserRepo:    userRepo,
		OrderRepo:   orderRepo,
		CatalogRepo: catalogRepo,
		Tokens:      tokens,
		CatalogSvc:  catalog.NewService(catalogRepo),
		OrderSvc:    orders.NewService(orderRepo, nil, NewID, storage.NewMemoryPromotionsRepo()),
		DevMode:     true,
	}
	return New(deps).Routes(nil), jwt
}

// ★ Ekranda ko'rsatilgan summa eskirgan bo'lsa buyurtma YARATILMAYDI.
//
// Bu AI yordamchisi uchun asosiy himoya: savatni til modeli tuzadi va
// foydalanuvchi faqat yakuniy raqamga qarab tasdiqlaydi.
func TestCreateOrder_StaleExpectedTotal_Conflict(t *testing.T) {
	h, jwt := totalCheckServer(t)

	// Haqiqiy jami — 2 × 30 000 = 60 000 so'm. So'rovda esa mijoz
	// "50 000" ko'rgan deb yuboriladi (eskirgan taklif).
	const realTotal = 6000000

	body := `{"items":[{"product_id":"prod-1","qty":2}],` +
		`"idempotency_key":"k1","expected_total_tiyin":5000000}`
	w := do(t, h, "POST", "/orders", jwt, body)

	if w.Code != http.StatusConflict {
		t.Fatalf("eskirgan summa bilan buyurtma YARATILDI (%d) — %s",
			w.Code, w.Body.String())
	}

	var resp struct {
		Code       string `json:"code"`
		TotalTiyin int64  `json:"total_tiyin"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("javobni o'qib bo'lmadi: %v — %s", err, w.Body.String())
	}
	if resp.Code != "total_changed" {
		t.Errorf("code = %q, kutilgan \"total_changed\"", resp.Code)
	}
	// Ilova aynan shu raqamni ko'rsatadi — usiz 409 foydasiz.
	if resp.TotalTiyin != realTotal {
		t.Errorf("javobdagi yangi summa %d, kutilgan %d",
			resp.TotalTiyin, realTotal)
	}
}

// Mos summa — buyurtma odatdagidek yaratiladi.
func TestCreateOrder_MatchingExpectedTotal_Created(t *testing.T) {
	h, jwt := totalCheckServer(t)

	body := `{"items":[{"product_id":"prod-1","qty":2}],` +
		`"idempotency_key":"k2","expected_total_tiyin":6000000}`
	w := do(t, h, "POST", "/orders", jwt, body)
	if w.Code != http.StatusCreated {
		t.Fatalf("mos summa bilan buyurtma yaratilmadi: %d — %s",
			w.Code, w.Body.String())
	}
}

// Maydon berilmasa — eski klientlar uchun hech narsa o'zgarmaydi.
func TestCreateOrder_NoExpectedTotal_StillWorks(t *testing.T) {
	h, jwt := totalCheckServer(t)

	body := `{"items":[{"product_id":"prod-1","qty":2}],"idempotency_key":"k3"}`
	w := do(t, h, "POST", "/orders", jwt, body)
	if w.Code != http.StatusCreated {
		t.Fatalf("tekshiruvsiz buyurtma yaratilmadi: %d — %s",
			w.Code, w.Body.String())
	}
}
