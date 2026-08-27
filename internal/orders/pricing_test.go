package orders

import (
	"context"
	"testing"
	"time"

	"chustapp/internal/promotions"
)

// fakePromoRepo — priceCart'ni HAQIQIY aksiyalar bilan sinash uchun.
// Avvalgi barcha buyurtma testlari `promotionsRepo` ga `nil` uzatardi,
// ya'ni pul yo'lining aksiyalar bilan kesishgan qismi UMUMAN
// sinalmasdi — aynan shu joyda 0 so'mlik buyurtma bug'i yashiringan edi.
type fakePromoRepo struct{ promos []*promotions.Promotion }

func (r *fakePromoRepo) ListByRestaurant(context.Context, string) ([]*promotions.Promotion, error) {
	return r.promos, nil
}
func (r *fakePromoRepo) GetByID(context.Context, string) (*promotions.Promotion, error) {
	return nil, promotions.ErrNotFound
}
func (r *fakePromoRepo) Save(context.Context, *promotions.Promotion) error { return nil }
func (r *fakePromoRepo) Delete(context.Context, string) error              { return nil }
func (r *fakePromoRepo) DeleteByRestaurant(context.Context, string) (int, error) {
	return 0, nil
}
func (r *fakePromoRepo) IncrementUsage(context.Context, string, int64) error { return nil }

func activePromo(p *promotions.Promotion) *promotions.Promotion {
	p.Active = true
	p.StartAt = time.Now().Add(-time.Hour)
	p.Indefinite = true
	return p
}

func newPricingService(promos ...*promotions.Promotion) *Service {
	return NewService(newFakeOCCRepo(), nil, func() string { return "id" },
		&fakePromoRepo{promos: promos})
}

// HAQIQIY REGRESSIYA (2026-08-07, qurilmada ko'rilgan): mahsulotning o'z
// chegirma narxi VA aksiya bitta mahsulotga to'g'ri kelganda ular
// QO'SHILARDI va jami NOLGA tushardi — 105 850 so'mlik savat "0 so'm"
// bo'lib ko'rinardi, ya'ni bepul buyurtma.
//
// To'g'ri xatti-harakat: ikkalasidan ENG YAXSHISI qo'llanadi, hech
// qachon ikkalasi birga emas.
func TestProductDiscountAndPromotionDoNotStack(t *testing.T) {
	// Mahsulot: 15 000 so'm, chegirma narxi 5 000 so'm (10 000 foyda).
	// Aksiya: "Ichimliklar" turkumiga har donaga -5 000 so'm.
	promo := activePromo(&promotions.Promotion{
		ID: "pr1", Name: "Mustaqillik kuni",
		Type: promotions.TypeFixedAmount, DiscountUnit: promotions.DiscountUnitAmount,
		DiscountValue:       500000, // 5 000 so'm
		AppliesToCategories: true,
		TargetCategories:    []string{"Ichimliklar"},
	})
	svc := newPricingService(promo)

	items := []Item{{
		ProductID: "p1", Name: "Coca Cola", Category: "Ichimliklar", Qty: 2,
		PriceTiyin: 1500000, DiscountPriceTiyin: 500000,
	}}
	subtotal, discount, applied, lineDiscounts, err := svc.priceCart(context.Background(), "r1", items, "")
	if err != nil {
		t.Fatalf("kutilmagan xato: %v", err)
	}

	// Subtotal HAR DOIM ro'yxat narxida — klientdagi "chizilgan" summa
	// bilan mos kelishi uchun.
	if subtotal != 3000000 {
		t.Errorf("subtotal: %d, kutilgan 3000000 (2 x 15 000 so'm)", subtotal)
	}
	// Mahsulot chegirmasi: 2 x 10 000 = 20 000 so'm.
	// Aksiya:              2 x  5 000 = 10 000 so'm.
	// Eng yaxshisi — mahsulot chegirmasi.
	if discount != 2000000 {
		t.Errorf("chegirma: %d, kutilgan 2000000 (mahsulot chegirmasi yutishi kerak)", discount)
	}
	if len(applied.Promotions) != 0 {
		t.Errorf("mahsulot chegirmasi yutganda aksiya qo'llanmasligi kerak: %+v", applied.Promotions)
	}
	if total := subtotal - discount; total != 1000000 {
		t.Errorf("jami: %d, kutilgan 1000000 (2 x 5 000 so'm) — NOL BO'LMASLIGI kerak", total)
	}
	// Qator taqsimoti ham to'liq: bitta qator bor, unga BUTUN chegirma
	// tegishli bo'lishi kerak (klient savatda aynan shu raqamni chizadi).
	if len(lineDiscounts) != 1 || lineDiscounts[0] != discount {
		t.Errorf("qator taqsimoti: %v, kutilgan [%d]", lineDiscounts, discount)
	}
}

// Teskari holat: aksiya mahsulot chegirmasidan foydaliroq bo'lsa, AYNAN
// u qo'llanadi va chekda nomi ko'rinadi.
func TestPromotionWinsWhenBetterThanProductDiscount(t *testing.T) {
	promo := activePromo(&promotions.Promotion{
		ID: "pr1", Name: "50% chegirma",
		Type: promotions.TypePercent, DiscountUnit: promotions.DiscountUnitPercent,
		DiscountValue: 50, AppliesToOrders: true,
	})
	svc := newPricingService(promo)

	// Mahsulot chegirmasi atigi 1 000 so'm, aksiya esa 50% = 7 000 so'm.
	items := []Item{{
		ProductID: "p1", Name: "Osh", Qty: 1,
		PriceTiyin: 1400000, DiscountPriceTiyin: 1300000,
	}}
	subtotal, discount, applied, _, err := svc.priceCart(context.Background(), "r1", items, "")
	if err != nil {
		t.Fatalf("kutilmagan xato: %v", err)
	}
	if subtotal != 1400000 {
		t.Errorf("subtotal: %d, kutilgan 1400000", subtotal)
	}
	if discount != 700000 {
		t.Errorf("chegirma: %d, kutilgan 700000 (aksiya yutishi kerak)", discount)
	}
	if p := applied.Primary(); p == nil || p.Promotion.Name != "50% chegirma" {
		t.Errorf("aksiya qo'llanishi va chekda ko'rinishi kerak edi: %+v", applied.Promotions)
	}
}

// ★ HAQIQIY REGRESSIYA (2026-08-18, "Feel Food", telefonda o'lchandi):
// menyuda 12 000 so'mlik Cola aksiya bilan 7 000, 15 000 so'mlik Cola
// o'z chegirma narxi bilan 5 000 ko'rinardi — ya'ni jami 12 000. Savat
// esa 17 000 so'm derdi.
//
// Sabab: aksiya (5 000 + 5 000 = 10 000) va mahsulot chegirmasi
// (10 000) BUTUN savat uchun raqobatlashib, faqat bittasi
// qo'llanardi. To'g'ri qoida — HAR QATOR o'zining eng yaxshisini oladi
// (bitta qatorda ikkalasi qo'shilmaydi).
func TestPriceCartTakesBestDiscountPerLine(t *testing.T) {
	promo := activePromo(&promotions.Promotion{
		ID: "pr1", Name: "Mustaqillik kuni",
		Type: promotions.TypeFixedAmount, DiscountUnit: promotions.DiscountUnitAmount,
		DiscountValue:       500000, // 5 000 so'm har donaga
		AppliesToCategories: true,
		TargetCategories:    []string{"Ichimliklar"},
	})
	svc := newPricingService(promo)

	items := []Item{
		// 1 L — faqat aksiya: 12 000 -> 7 000.
		{ProductID: "cola-1l", Name: "Coca Cola 1 L", Category: "Ichimliklar", Qty: 1,
			PriceTiyin: 1200000},
		// 1.5 L — o'z chegirma narxi foydaliroq: 15 000 -> 5 000.
		{ProductID: "cola-15l", Name: "Coca Cola 1.5 L", Category: "Ichimliklar", Qty: 1,
			PriceTiyin: 1500000, DiscountPriceTiyin: 500000},
	}
	subtotal, discount, applied, lineDiscounts, err := svc.priceCart(context.Background(), "r1", items, "")
	if err != nil {
		t.Fatalf("kutilmagan xato: %v", err)
	}
	if subtotal != 2700000 {
		t.Errorf("subtotal: %d, kutilgan 2700000", subtotal)
	}
	if discount != 1500000 {
		t.Errorf("chegirma: %d, kutilgan 1500000 (5 000 + 10 000 so'm)", discount)
	}
	if total := subtotal - discount; total != 1200000 {
		t.Errorf("jami: %d, kutilgan 1200000 (12 000 so'm) — menyudagi 7 000 + 5 000", total)
	}
	if lineDiscounts[0] != 500000 || lineDiscounts[1] != 1000000 {
		t.Errorf("taqsimot: %v, kutilgan [500000 1000000]", lineDiscounts)
	}
	if applied.PromotionTiyin != 500000 {
		t.Errorf("aksiya hissasi: %d, kutilgan 500000", applied.PromotionTiyin)
	}
	var sum int64
	for _, d := range lineDiscounts {
		sum += d
	}
	if sum != discount {
		t.Errorf("qatorlar yig'indisi %d, jami chegirma %d — teng emas", sum, discount)
	}
}

// Aksiya hech bir qatorda mahsulot chegirmasidan foydali bo'lmasa —
// UMUMAN qo'llanmaydi: chekda nomi chiqmaydi, statistikasi oshmaydi.
func TestPriceCartPromotionNotAppliedWhenNeverBetter(t *testing.T) {
	promo := activePromo(&promotions.Promotion{
		ID: "pr1", Name: "Kichik aksiya",
		Type: promotions.TypeFixedAmount, DiscountUnit: promotions.DiscountUnitAmount,
		DiscountValue:       100000, // 1 000 so'm
		AppliesToCategories: true,
		TargetCategories:    []string{"Ichimliklar"},
	})
	svc := newPricingService(promo)

	items := []Item{
		{ProductID: "cola", Name: "Coca Cola", Category: "Ichimliklar", Qty: 2,
			PriceTiyin: 1500000, DiscountPriceTiyin: 500000}, // -10 000 har donaga
	}
	_, discount, applied, lineDiscounts, err := svc.priceCart(context.Background(), "r1", items, "")
	if err != nil {
		t.Fatalf("kutilmagan xato: %v", err)
	}
	if discount != 2000000 {
		t.Errorf("chegirma: %d, kutilgan 2000000 (2 x 10 000 so'm)", discount)
	}
	if len(applied.Promotions) != 0 {
		t.Errorf("aksiya qo'llanmasligi kerak edi: %+v", applied.Promotions)
	}
	if lineDiscounts[0] != 2000000 {
		t.Errorf("taqsimot: %v, kutilgan [2000000]", lineDiscounts)
	}
}

// ★ HAQIQIY REGRESSIYA (2026-08-18, telefonda o'lchandi): savatda IKKI
// XIL aksiya bo'lganda ham har mahsulot o'zining chegirmasini olishi
// kerak. 12 000 so'mlik Cola (-5 000) + 10 000 so'mlik kokteyl (-20%)
// = 15 000 so'm. Avval server butun savatga faqat BITTA aksiya
// qo'llardi va 17 000 so'm chiqardi.
func TestPriceCartAppliesTwoPromotionsOnDifferentLines(t *testing.T) {
	drinks := activePromo(&promotions.Promotion{
		ID: "pr1", Name: "Ichimliklarga -5 000",
		Type: promotions.TypeFixedAmount, DiscountUnit: promotions.DiscountUnitAmount,
		DiscountValue:       500000,
		AppliesToCategories: true,
		TargetCategories:    []string{"Ichimliklar"},
	})
	kokteyl := activePromo(&promotions.Promotion{
		ID: "pr2", Name: "Kokteylga 20%",
		Type: promotions.TypePercent, DiscountUnit: promotions.DiscountUnitPercent,
		DiscountValue:     20,
		AppliesToProducts: true,
		TargetProductIDs:  []string{"kokteyl"},
	})
	svc := newPricingService(drinks, kokteyl)

	items := []Item{
		{ProductID: "cola", Name: "Coca Cola", Category: "Ichimliklar", Qty: 1,
			PriceTiyin: 1200000},
		{ProductID: "kokteyl", Name: "Kokteyl", Category: "Kokteyllar", Qty: 1,
			PriceTiyin: 1000000},
	}
	subtotal, discount, applied, lineDiscounts, err := svc.priceCart(context.Background(), "r1", items, "")
	if err != nil {
		t.Fatalf("kutilmagan xato: %v", err)
	}
	if subtotal != 2200000 {
		t.Errorf("subtotal: %d, kutilgan 2200000", subtotal)
	}
	if total := subtotal - discount; total != 1500000 {
		t.Errorf("jami: %d, kutilgan 1500000 (7 000 + 8 000 so'm)", total)
	}
	if lineDiscounts[0] != 500000 || lineDiscounts[1] != 200000 {
		t.Errorf("taqsimot: %v, kutilgan [500000 200000]", lineDiscounts)
	}
	// Ikkala aksiya ham statistikaga o'z hissasi bilan tushishi kerak.
	if len(applied.Promotions) != 2 {
		t.Fatalf("ikkala aksiya ham qo'llanishi kerak edi: %+v", applied.Promotions)
	}
	if p := applied.Primary(); p == nil || p.Promotion.ID != "pr1" || p.DiscountTiyin != 500000 {
		t.Errorf("asosiy aksiya: %+v, kutilgan pr1 / 500000", p)
	}
}

// Quote klientga qator narxlarini ham beradi va ular jamiga ANIQ
// qo'shiladi — savat ekrani aynan shu raqamlarni chizadi.
func TestQuoteReturnsConsistentLines(t *testing.T) {
	promo := activePromo(&promotions.Promotion{
		ID: "pr1", Name: "30%",
		Type: promotions.TypePercent, DiscountUnit: promotions.DiscountUnitPercent,
		DiscountValue: 30, AppliesToOrders: true,
	})
	svc := newPricingService(promo)

	items := []Item{
		{ProductID: "a", Name: "Osh", Qty: 2, PriceTiyin: 3500001},
		{ProductID: "b", Name: "Choy", Qty: 1, PriceTiyin: 500001},
	}
	q, err := svc.Quote(context.Background(), "r1", items, "")
	if err != nil {
		t.Fatalf("kutilmagan xato: %v", err)
	}
	if len(q.Lines) != 2 {
		t.Fatalf("qator soni: %d, kutilgan 2", len(q.Lines))
	}
	var lineTotals, lineDiscounts int64
	for _, l := range q.Lines {
		if l.SubtotalTiyin != l.UnitPriceTiyin*int64(l.Qty) {
			t.Errorf("qator %s: subtotal %d, kutilgan %d", l.ProductID, l.SubtotalTiyin, l.UnitPriceTiyin*int64(l.Qty))
		}
		if l.TotalTiyin != l.SubtotalTiyin-l.DiscountTiyin {
			t.Errorf("qator %s: jami %d, kutilgan %d", l.ProductID, l.TotalTiyin, l.SubtotalTiyin-l.DiscountTiyin)
		}
		lineTotals += l.TotalTiyin
		lineDiscounts += l.DiscountTiyin
	}
	if lineTotals != q.TotalTiyin {
		t.Errorf("qatorlar jami %d, quote jami %d — teng bo'lishi shart", lineTotals, q.TotalTiyin)
	}
	if lineDiscounts != q.DiscountTiyin {
		t.Errorf("qator chegirmalari %d, quote chegirmasi %d", lineDiscounts, q.DiscountTiyin)
	}
}

// Chegirma hech qachon jamidan oshmaydi — manfiy jami mumkin emas.
func TestDiscountNeverExceedsSubtotal(t *testing.T) {
	promo := activePromo(&promotions.Promotion{
		ID: "pr1", Name: "Ulkan chegirma",
		Type: promotions.TypeFixedAmount, DiscountUnit: promotions.DiscountUnitAmount,
		DiscountValue: 99999999, AppliesToOrders: true,
	})
	svc := newPricingService(promo)
	items := []Item{{ProductID: "p1", Name: "Choy", Qty: 1, PriceTiyin: 500000}}

	subtotal, discount, _, _, err := svc.priceCart(context.Background(), "r1", items, "")
	if err != nil {
		t.Fatalf("kutilmagan xato: %v", err)
	}
	if discount > subtotal {
		t.Errorf("chegirma (%d) jamidan (%d) oshib ketdi", discount, subtotal)
	}
}

// Jami nolga tushsa, buyurtma YARATILMAYDI — bepul buyurtma jimgina
// o'tib ketmasligi kerak (noto'g'ri kiritilgan chegirma narxiga qarshi
// oxirgi himoya).
func TestCreateRejectsZeroTotal(t *testing.T) {
	promo := activePromo(&promotions.Promotion{
		ID: "pr1", Name: "100%",
		Type: promotions.TypePercent, DiscountUnit: promotions.DiscountUnitPercent,
		DiscountValue: 100, AppliesToOrders: true,
	})
	svc := newPricingService(promo)

	_, err := svc.Create(context.Background(), &Order{
		CustomerID: "c1", RestaurantID: "r1",
		Items: []Item{{ProductID: "p1", Name: "Choy", Qty: 1, PriceTiyin: 500000}},
	})
	if err == nil {
		t.Fatal("jami 0 bo'lgan buyurtma rad etilishi kerak edi")
	}
}
