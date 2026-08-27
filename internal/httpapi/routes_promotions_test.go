package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/orders"
	"chustapp/internal/promotions"
	"chustapp/internal/storage"
	"chustapp/internal/users"
	"chustapp/internal/ws"
)

// Aksiya endpointlarining HTTP darajasidagi testlari.
//
// ┌─ NEGA AYNAN SHU DARAJADA ─────────────────────────────────────────┐
// Menyudagi chegirma xatosining ILDIZI aynan shu qatlamda edi:
// `/active-promotions` javobida `max_discount_amount_tiyin` UMUMAN
// yo'q edi, ya'ni klient cheklovni bilolmasdi va menyuda serverning
// narxidan boshqa narx chizardi. Hisoblash paketining testlari bunday
// yetishmayotgan MAYDONNI hech qachon ushlay olmaydi — javobni aynan
// shu yerda tekshirish kerak.
// └───────────────────────────────────────────────────────────────────┘

const promoTestRest = "rest-p"

func promoServer(t *testing.T, promos ...*promotions.Promotion) (http.Handler, string, string) {
	t.Helper()
	ctx := context.Background()

	userRepo := storage.NewMemoryUserRepo()
	tokens := users.NewTokenIssuer("test-secret", time.Hour)

	mk := func(id string, role users.Role, entityID, phone string) string {
		u := &users.User{ID: id, Phone: phone, Role: role, EntityID: entityID,
			PhoneVerified: true, CreatedAt: time.Now()}
		if err := userRepo.Create(ctx, u); err != nil {
			t.Fatal(err)
		}
		jwt, err := tokens.Issue(u)
		if err != nil {
			t.Fatal(err)
		}
		return jwt
	}
	customerJWT := mk("u-cust", users.RoleCustomer, "", "+998900000011")
	restaurantJWT := mk("u-rest", users.RoleRestaurant, promoTestRest, "+998900000012")

	catalogRepo := storage.NewMemoryCatalogRepo(
		[]catalog.Restaurant{{ID: promoTestRest, Name: "Feel Food", Open: true}},
		[]catalog.Product{
			// Ichimliklar: biri o'z chegirma narxi bilan.
			{ID: "cola", RestaurantID: promoTestRest, Name: "Coca Cola",
				Category: "Ichimliklar", PriceTiyin: 1500000,
				DiscountPriceTiyin: 500000, Available: true},
			{ID: "moxito", RestaurantID: promoTestRest, Name: "Moxito",
				Category: "Ichimliklar", PriceTiyin: 1900000, Available: true},
		},
	)
	promoRepo := storage.NewMemoryPromotionsRepo()
	for _, p := range promos {
		p.RestaurantID = promoTestRest
		if err := promoRepo.Save(ctx, p); err != nil {
			t.Fatal(err)
		}
	}

	deps := Deps{
		UserRepo:       userRepo,
		CatalogRepo:    catalogRepo,
		PromotionsRepo: promoRepo,
		OrderRepo:      storage.NewMemoryOrderRepo(),
		Tokens:         tokens,
		CatalogSvc:     catalog.NewService(catalogRepo),
		OrderSvc: orders.NewService(storage.NewMemoryOrderRepo(), nil, NewID,
			promoRepo),
		Hub:     ws.NewHub(nil),
		DevMode: true,
	}
	return New(deps).Routes(nil), customerJWT, restaurantJWT
}

func activeTestPromo(p *promotions.Promotion) *promotions.Promotion {
	p.ID = "promo-1"
	p.Active = true
	p.StartAt = time.Now().Add(-time.Hour)
	p.Indefinite = true
	return p
}

// Klient chegirmani TO'G'RI chiza olishi uchun kerak bo'lgan barcha
// cheklovlar javobda bo'lishi shart.
func TestActivePromotionsIncludeLimits(t *testing.T) {
	h, customerJWT, _ := promoServer(t, activeTestPromo(&promotions.Promotion{
		Name: "30% chegirma", Type: promotions.TypePercent,
		DiscountUnit: promotions.DiscountUnitPercent, DiscountValue: 30,
		MinOrderAmountTiyin:    5000000,
		MaxDiscountAmountTiyin: 200000,
		AppliesToCategories:    true,
		TargetCategories:       []string{"Ichimliklar"},
	}))

	w := do(t, h, "GET", "/restaurants/"+promoTestRest+"/active-promotions", customerJWT, "")
	if w.Code != http.StatusOK {
		t.Fatalf("kutilgan 200, keldi %d — %s", w.Code, w.Body.String())
	}
	var list []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("1 ta aksiya kutilgan, keldi %d", len(list))
	}
	got := list[0]
	if v, ok := got["max_discount_amount_tiyin"]; !ok || v.(float64) != 200000 {
		t.Errorf("max_discount_amount_tiyin javobda yo'q yoki noto'g'ri: %v — "+
			"busiz menyu cheklovni bilmay to'liq chegirma ko'rsatadi", v)
	}
	if v := got["min_order_amount_tiyin"]; v.(float64) != 5000000 {
		t.Errorf("min_order_amount_tiyin: %v, kutilgan 5000000", v)
	}
}

// Turga zid birlik saqlangan bo'lsa ham klientga TUR bo'yicha to'g'ri
// birlik boradi (`Promotion.EffectiveUnit`).
func TestActivePromotionsNormalizeDiscountUnit(t *testing.T) {
	h, customerJWT, _ := promoServer(t, activeTestPromo(&promotions.Promotion{
		Name: "Summa", Type: promotions.TypeFixedAmount,
		DiscountUnit:        promotions.DiscountUnitPercent, // zid qiymat
		DiscountValue:       500000,
		AppliesToCategories: true,
		TargetCategories:    []string{"Ichimliklar"},
	}))

	w := do(t, h, "GET", "/restaurants/"+promoTestRest+"/active-promotions", customerJWT, "")
	var list []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if got := list[0]["discount_unit"]; got != "amount" {
		t.Errorf("discount_unit: %v, kutilgan \"amount\" (tur — summa)", got)
	}
}

// Zid juftlik UMUMAN saqlanmaydi — server bilan klient uni har xil
// o'qishi mumkin bo'lgan yozuv yaratilmasin.
func TestSavePromotionRejectsTypeUnitMismatch(t *testing.T) {
	h, _, restaurantJWT := promoServer(t)

	body := `{"name":"Summa orqali","type":"fixed_amount","discount_unit":"percent",` +
		`"discount_value":20,"start_at":"` + time.Now().UTC().Format(time.RFC3339) +
		`","indefinite":true,"active":true,"applies_to_categories":true,` +
		`"target_categories":["Ichimliklar"]}`
	w := do(t, h, "POST", "/restaurants/"+promoTestRest+"/promotions", restaurantJWT, body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("zid tur/birlik juftligi QABUL QILINDI (%d) — %s", w.Code, w.Body.String())
	}

	// To'g'ri juftlik esa saqlanadi.
	ok := `{"name":"Summa orqali","type":"fixed_amount","discount_unit":"amount",` +
		`"discount_value":500000,"start_at":"` + time.Now().UTC().Format(time.RFC3339) +
		`","indefinite":true,"active":true,"applies_to_categories":true,` +
		`"target_categories":["Ichimliklar"]}`
	if w := do(t, h, "POST", "/restaurants/"+promoTestRest+"/promotions", restaurantJWT, ok); w.Code != http.StatusCreated {
		t.Fatalf("to'g'ri aksiya saqlanmadi: %d — %s", w.Code, w.Body.String())
	}
}

// ★ XAVFSIZLIK: ulgurji narx OCHIQ menyuda hech qachon ko'rinmasligi
// kerak — bu restoranning ichki biznes ma'lumoti. Panel esa uni
// avtorizatsiyalangan endpoint'dan oladi.
func TestWholesalePriceHiddenFromPublicMenu(t *testing.T) {
	h, customerJWT, restaurantJWT := promoServer(t)

	// Panel mahsulotga ulgurji narx belgilaydi.
	body := `{"id":"cola","name":"Coca Cola","category":"Ichimliklar",` +
		`"price_tiyin":1500000,"wholesale_price_tiyin":900000,"available":true}`
	if w := do(t, h, "POST", "/restaurants/"+promoTestRest+"/products", restaurantJWT, body); w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("mahsulot saqlanmadi: %d — %s", w.Code, w.Body.String())
	}

	// Ochiq menyu — maydon UMUMAN bo'lmasligi kerak.
	w := do(t, h, "GET", "/restaurants/"+promoTestRest+"/menu", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("menyu olinmadi: %d", w.Code)
	}
	var pub []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &pub); err != nil {
		t.Fatal(err)
	}
	for _, p := range pub {
		if v, ok := p["wholesale_price_tiyin"]; ok && v.(float64) != 0 {
			t.Errorf("ULGURJI NARX OCHIQ MENYUDA SIZIB CHIQDI: %v (%v)", v, p["name"])
		}
	}

	// Panel endpoint'ida esa qiymat BOR.
	w = do(t, h, "GET", "/restaurants/"+promoTestRest+"/products", restaurantJWT, "")
	if w.Code != http.StatusOK {
		t.Fatalf("panel ro'yxati olinmadi: %d — %s", w.Code, w.Body.String())
	}
	var priv []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &priv); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, p := range priv {
		if p["id"] == "cola" && p["wholesale_price_tiyin"].(float64) == 900000 {
			found = true
		}
	}
	if !found {
		t.Error("panel ro'yxatida ulgurji narx yo'q — forma uni ko'rsata olmaydi")
	}

	// Mijoz roli bu endpoint'ga KIRA OLMAYDI.
	if w := do(t, h, "GET", "/restaurants/"+promoTestRest+"/products", customerJWT, ""); w.Code != http.StatusForbidden {
		t.Errorf("mijozga panel ro'yxati berildi: %d", w.Code)
	}
}

// ★ ASOSIY: quote javobidagi QATOR narxlari jamiga aniq qo'shiladi.
//
// Aynan shu yerda mijoz savatda "19 000", pastda esa "24 000 so'm"
// ko'rgan edi: qatorlarni klient o'zi taxmin qilardi.
func TestQuoteLinesSumToTotal(t *testing.T) {
	h, customerJWT, _ := promoServer(t, activeTestPromo(&promotions.Promotion{
		Name: "Mustaqillik kuni", Type: promotions.TypeFixedAmount,
		DiscountUnit: promotions.DiscountUnitAmount, DiscountValue: 500000,
		AppliesToCategories: true,
		TargetCategories:    []string{"Ichimliklar"},
	}))

	body := `{"items":[{"product_id":"cola","qty":1},{"product_id":"moxito","qty":1}]}`
	w := do(t, h, "POST", "/restaurants/"+promoTestRest+"/quote", customerJWT, body)
	if w.Code != http.StatusOK {
		t.Fatalf("kutilgan 200, keldi %d — %s", w.Code, w.Body.String())
	}
	var q orders.QuoteResult
	if err := json.Unmarshal(w.Body.Bytes(), &q); err != nil {
		t.Fatal(err)
	}
	if len(q.Lines) != 2 {
		t.Fatalf("2 ta qator kutilgan, keldi %d", len(q.Lines))
	}
	var lineTotals, lineDiscounts int64
	for _, l := range q.Lines {
		lineTotals += l.TotalTiyin
		lineDiscounts += l.DiscountTiyin
	}
	if lineTotals != q.TotalTiyin {
		t.Errorf("qatorlar jami %d, quote jami %d — mijoz ikki xil raqam ko'radi",
			lineTotals, q.TotalTiyin)
	}
	if lineDiscounts != q.DiscountTiyin {
		t.Errorf("qator chegirmalari %d, jami chegirma %d", lineDiscounts, q.DiscountTiyin)
	}
	// cola: o'z chegirma narxi -10 000 (aksiyaning -5 000 idan foydali),
	// moxito: aksiya -5 000. Har qator o'zining eng yaxshisini oladi.
	if q.TotalTiyin != 1900000 {
		t.Errorf("jami: %d, kutilgan 1900000 (34 000 - 15 000 so'm)", q.TotalTiyin)
	}
	if q.PromotionDiscountTiyin != 500000 {
		t.Errorf("aksiya hissasi: %d, kutilgan 500000 (qolgani mahsulot chegirmasi)",
			q.PromotionDiscountTiyin)
	}
}
