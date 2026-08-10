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
func (r *fakePromoRepo) Delete(context.Context, string) error             { return nil }
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
	subtotal, discount, applied, err := svc.priceCart(context.Background(), "r1", items, "")
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
	if applied != nil {
		t.Errorf("mahsulot chegirmasi yutganda aksiya qo'llanmasligi kerak, olindi %q", applied.Promotion.Name)
	}
	if total := subtotal - discount; total != 1000000 {
		t.Errorf("jami: %d, kutilgan 1000000 (2 x 5 000 so'm) — NOL BO'LMASLIGI kerak", total)
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
	subtotal, discount, applied, err := svc.priceCart(context.Background(), "r1", items, "")
	if err != nil {
		t.Fatalf("kutilmagan xato: %v", err)
	}
	if subtotal != 1400000 {
		t.Errorf("subtotal: %d, kutilgan 1400000", subtotal)
	}
	if discount != 700000 {
		t.Errorf("chegirma: %d, kutilgan 700000 (aksiya yutishi kerak)", discount)
	}
	if applied == nil || applied.Promotion.Name != "50% chegirma" {
		t.Errorf("aksiya qo'llanishi va chekda ko'rinishi kerak edi: %+v", applied)
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

	subtotal, discount, _, err := svc.priceCart(context.Background(), "r1", items, "")
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
