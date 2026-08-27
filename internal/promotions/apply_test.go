package promotions

import (
	"testing"
	"time"
)

func basePromo(id string, typ Type) *Promotion {
	now := time.Now()
	return &Promotion{
		ID:            id,
		Type:          typ,
		DiscountUnit:  DiscountUnitPercent,
		DiscountValue: 20,
		StartAt:       now.Add(-time.Hour),
		EndAt:         now.Add(time.Hour),
		Active:        true,
	}
}

// amountPromo — "summa orqali chegirma" (tiyinda), berilgan turkumga.
func amountPromo(id string, valueTiyin int64, categories ...string) *Promotion {
	p := basePromo(id, TypeFixedAmount)
	p.DiscountUnit = DiscountUnitAmount
	p.DiscountValue = valueTiyin
	p.AppliesToCategories = true
	p.TargetCategories = categories
	return p
}

// percentPromo — foizli chegirma, berilgan mahsulotlarga.
func percentPromo(id string, percent int64, productIDs ...string) *Promotion {
	p := basePromo(id, TypePercent)
	p.DiscountValue = percent
	p.AppliesToProducts = true
	p.TargetProductIDs = productIDs
	return p
}

func sumLines(values []int64) int64 {
	var total int64
	for _, v := range values {
		total += v
	}
	return total
}

// checkInvariants — HAR BIR natija uchun bajarilishi shart bo'lgan
// qoidalar. Ular buzilsa mijoz savatda bir raqam, pastda boshqasini
// ko'radi (yoki qator "minus pulga" o'tib ketadi).
func checkInvariants(t *testing.T, r Result, lines []CartLine) {
	t.Helper()
	if len(r.LineDiscounts) != len(lines) {
		t.Fatalf("qator soni: %d, kutilgan %d", len(r.LineDiscounts), len(lines))
	}
	if s := sumLines(r.LineDiscounts); s != r.DiscountTiyin {
		t.Errorf("qatorlar yig'indisi %d, jami chegirma %d — TENG bo'lishi shart", s, r.DiscountTiyin)
	}
	for i, d := range r.LineDiscounts {
		if d < 0 {
			t.Errorf("qator %d: manfiy chegirma %d", i, d)
		}
		if s := lines[i].subtotal(); d > s {
			t.Errorf("qator %d: chegirma %d, qator summasi %d — oshib ketdi", i, d, s)
		}
	}
	var promoTotal int64
	for _, ap := range r.Promotions {
		if ap.DiscountTiyin <= 0 {
			t.Errorf("aksiya %q ro'yxatda, lekin hissasi %d", ap.Promotion.ID, ap.DiscountTiyin)
		}
		promoTotal += ap.DiscountTiyin
	}
	if promoTotal != r.PromotionTiyin {
		t.Errorf("aksiyalar hissasi %d, PromotionTiyin %d", promoTotal, r.PromotionTiyin)
	}
	if r.PromotionTiyin > r.DiscountTiyin {
		t.Errorf("aksiyalar hissasi (%d) jami chegirmadan (%d) katta", r.PromotionTiyin, r.DiscountTiyin)
	}
}

func apply(promos []*Promotion, lines []CartLine, baseline []int64) Result {
	return Apply(promos, lines, baseline, 0, time.Now())
}

// ═══════════════════════════════════════════════════════════════════
// ★ ASOSIY QOIDA: HAR QATOR O'ZINING ENG YAXSHISINI OLADI
// ═══════════════════════════════════════════════════════════════════

// HAQIQIY HOLAT (2026-08-18, "Feel Food"): 12 000 so'mlik Cola'ga
// -5 000 so'mlik aksiya, 10 000 so'mlik kokteylga esa BOSHQA, -20%
// aksiya tegadi. Menyu 7 000 + 8 000 = 15 000 ko'rsatardi, savat esa
// 17 000 — chunki server butun savatga FAQAT BITTA aksiya qo'llardi.
func TestApplyDifferentPromotionsOnDifferentLines(t *testing.T) {
	cola := amountPromo("cola-5k", 500000, "Ichimliklar")
	kokteyl := percentPromo("kokteyl-20", 20, "kokteyl")

	lines := []CartLine{
		{ProductID: "cola", Category: "Ichimliklar", UnitPriceTiyin: 1200000, Qty: 1},
		{ProductID: "kokteyl", Category: "Kokteyllar", UnitPriceTiyin: 1000000, Qty: 1},
	}
	got := apply([]*Promotion{cola, kokteyl}, lines, nil)
	checkInvariants(t, got, lines)

	if got.LineDiscounts[0] != 500000 || got.LineDiscounts[1] != 200000 {
		t.Errorf("taqsimot: %v, kutilgan [500000 200000]", got.LineDiscounts)
	}
	if got.DiscountTiyin != 700000 {
		t.Errorf("jami chegirma: %d, kutilgan 700000", got.DiscountTiyin)
	}
	if total := cartSubtotal(lines) - got.DiscountTiyin; total != 1500000 {
		t.Errorf("jami: %d, kutilgan 1500000 (7 000 + 8 000 so'm)", total)
	}
	if len(got.Promotions) != 2 {
		t.Fatalf("ikkala aksiya ham qo'llanishi kerak edi: %+v", got.Promotions)
	}
	// Ro'yxat hissasi bo'yicha kamayish tartibida.
	if got.Promotions[0].Promotion.ID != "cola-5k" || got.Promotions[0].DiscountTiyin != 500000 {
		t.Errorf("birinchi aksiya: %+v", got.Promotions[0])
	}
	if got.Promotions[1].Promotion.ID != "kokteyl-20" || got.Promotions[1].DiscountTiyin != 200000 {
		t.Errorf("ikkinchi aksiya: %+v", got.Promotions[1])
	}
}

// BITTA qatorga ikkita aksiya to'g'ri kelsa — faqat FOYDALIROG'I
// (qo'shilmaydi, aks holda narx nolga tushib ketardi).
func TestApplyNeverStacksTwoPromotionsOnOneLine(t *testing.T) {
	small := amountPromo("small", 200000, "Ichimliklar")
	big := amountPromo("big", 500000, "Ichimliklar")

	lines := []CartLine{{ProductID: "cola", Category: "Ichimliklar", UnitPriceTiyin: 1200000, Qty: 1}}
	got := apply([]*Promotion{small, big}, lines, nil)
	checkInvariants(t, got, lines)

	if got.DiscountTiyin != 500000 {
		t.Errorf("chegirma: %d, kutilgan 500000 (faqat kattasi)", got.DiscountTiyin)
	}
	if len(got.Promotions) != 1 || got.Promotions[0].Promotion.ID != "big" {
		t.Errorf("faqat `big` qo'llanishi kerak edi: %+v", got.Promotions)
	}
}

// Mahsulotning o'z chegirma narxi (baseline) ham qator darajasidagi
// nomzod: aksiya undan foydali bo'lsagina yutadi.
func TestApplyBaselineWinsWhenBetter(t *testing.T) {
	promo := amountPromo("promo", 500000, "Ichimliklar")

	lines := []CartLine{
		{ProductID: "cola-1l", Category: "Ichimliklar", UnitPriceTiyin: 1200000, Qty: 1},
		{ProductID: "cola-15l", Category: "Ichimliklar", UnitPriceTiyin: 1500000, Qty: 1},
	}
	baseline := []int64{0, 1000000} // 1.5 L da o'z chegirma narxi: -10 000

	got := apply([]*Promotion{promo}, lines, baseline)
	checkInvariants(t, got, lines)

	if got.LineDiscounts[0] != 500000 || got.LineDiscounts[1] != 1000000 {
		t.Errorf("taqsimot: %v, kutilgan [500000 1000000]", got.LineDiscounts)
	}
	if got.DiscountTiyin != 1500000 {
		t.Errorf("jami chegirma: %d, kutilgan 1500000", got.DiscountTiyin)
	}
	// Statistikaga faqat aksiya bergan qism.
	if got.PromotionTiyin != 500000 {
		t.Errorf("aksiya hissasi: %d, kutilgan 500000", got.PromotionTiyin)
	}
}

// Aksiya hech bir qatorda mahsulot chegirmasidan foydali bo'lmasa —
// UMUMAN qo'llanmaydi (chekda nomi chiqmaydi, statistikasi oshmaydi).
func TestApplyPromotionIgnoredWhenNeverBetter(t *testing.T) {
	promo := amountPromo("promo", 100000, "Ichimliklar")

	lines := []CartLine{{ProductID: "cola", Category: "Ichimliklar", UnitPriceTiyin: 1500000, Qty: 2}}
	baseline := []int64{2000000} // 2 x 10 000

	got := apply([]*Promotion{promo}, lines, baseline)
	checkInvariants(t, got, lines)

	if got.DiscountTiyin != 2000000 {
		t.Errorf("chegirma: %d, kutilgan 2000000", got.DiscountTiyin)
	}
	if len(got.Promotions) != 0 || got.Primary() != nil {
		t.Errorf("aksiya qo'llanmasligi kerak edi: %+v", got.Promotions)
	}
}

// Butun buyurtmaga tegishli aksiya ham qator darajasida raqobatlashadi:
// mos qatorlarda foydaliroq bo'lsa yutadi, aks holda yo'q.
func TestApplyOrderWidePromotionCompetesPerLine(t *testing.T) {
	orderWide := basePromo("order-10", TypePercent)
	orderWide.DiscountValue = 10
	orderWide.AppliesToOrders = true

	drinks := amountPromo("drinks", 500000, "Ichimliklar")

	lines := []CartLine{
		// 10% = 1 200, aksiya 5 000 — ichimliklar aksiyasi yutadi.
		{ProductID: "cola", Category: "Ichimliklar", UnitPriceTiyin: 1200000, Qty: 1},
		// 10% = 3 500, boshqa aksiya tegmaydi — order-wide yutadi.
		{ProductID: "osh", Category: "Milliy taomlar", UnitPriceTiyin: 3500000, Qty: 1},
	}
	got := apply([]*Promotion{orderWide, drinks}, lines, nil)
	checkInvariants(t, got, lines)

	if got.LineDiscounts[0] != 500000 || got.LineDiscounts[1] != 350000 {
		t.Errorf("taqsimot: %v, kutilgan [500000 350000]", got.LineDiscounts)
	}
	if len(got.Promotions) != 2 {
		t.Errorf("ikkala aksiya ham hissa qo'shishi kerak edi: %+v", got.Promotions)
	}
}

// ═══════════════════════════════════════════════════════════════════
// MEXANIKALAR
// ═══════════════════════════════════════════════════════════════════

func TestApplyPercentOnOrder(t *testing.T) {
	p := basePromo("p1", TypePercent)
	p.AppliesToOrders = true
	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 10000, Qty: 2}}
	got := apply([]*Promotion{p}, lines, nil)
	checkInvariants(t, got, lines)
	if got.DiscountTiyin != 4000 {
		t.Fatalf("kutilgan 4000, olindi %d", got.DiscountTiyin)
	}
}

func TestApplyFixedAmountAppliesPerProductAndQty(t *testing.T) {
	p := basePromo("p1", TypeFixedAmount)
	p.DiscountUnit = DiscountUnitAmount
	p.DiscountValue = 5000
	p.AppliesToProducts = true
	p.TargetProductIDs = []string{"a", "b"}
	lines := []CartLine{
		{ProductID: "a", UnitPriceTiyin: 12000, Qty: 2}, // 2 x 5 000
		{ProductID: "b", UnitPriceTiyin: 15000, Qty: 1}, // 1 x 5 000
		{ProductID: "c", UnitPriceTiyin: 20000, Qty: 1}, // mos emas
	}
	got := apply([]*Promotion{p}, lines, nil)
	checkInvariants(t, got, lines)
	if got.DiscountTiyin != 15000 {
		t.Fatalf("kutilgan 15000, olindi %d", got.DiscountTiyin)
	}
	if got.LineDiscounts[2] != 0 {
		t.Errorf("mos kelmagan qatorga chegirma tushdi: %v", got.LineDiscounts)
	}
}

// Chegirma qatorning o'z summasidan oshmaydi — taom "minus pulga"
// o'tib ketmaydi.
func TestApplyFixedAmountCappedByLineSubtotal(t *testing.T) {
	p := basePromo("p1", TypeFixedAmount)
	p.DiscountUnit = DiscountUnitAmount
	p.DiscountValue = 50000 // taomdan qimmat
	p.AppliesToProducts = true
	p.TargetProductIDs = []string{"a"}
	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 10000, Qty: 1}}
	got := apply([]*Promotion{p}, lines, nil)
	checkInvariants(t, got, lines)
	if got.DiscountTiyin != 10000 {
		t.Fatalf("kutilgan 10000, olindi %d", got.DiscountTiyin)
	}
}

func TestApplyBOGO(t *testing.T) {
	p := basePromo("p1", TypeBOGO)
	p.AppliesToProducts = true
	p.TargetProductIDs = []string{"a"}
	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 5000, Qty: 5}} // 2 dona bepul
	got := apply([]*Promotion{p}, lines, nil)
	checkInvariants(t, got, lines)
	if got.DiscountTiyin != 10000 {
		t.Fatalf("kutilgan 10000, olindi %d", got.DiscountTiyin)
	}
}

func TestApplyBOGOOddQtyNoFreeUnit(t *testing.T) {
	p := basePromo("p1", TypeBOGO)
	p.AppliesToProducts = true
	p.TargetProductIDs = []string{"a"}
	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 5000, Qty: 1}}
	got := apply([]*Promotion{p}, lines, nil)
	if got.DiscountTiyin != 0 || len(got.Promotions) != 0 {
		t.Fatalf("1 dona uchun chegirma bo'lmasligi kerak: %+v", got)
	}
}

func TestApplyBundleRequiresAllProducts(t *testing.T) {
	p := basePromo("p1", TypeBundle)
	p.AppliesToProducts = true
	p.TargetProductIDs = []string{"a", "b"}

	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 10000, Qty: 1}}
	if got := apply([]*Promotion{p}, lines, nil); got.DiscountTiyin != 0 {
		t.Fatalf("`b` yo'q — chegirma bo'lmasligi kerak: %+v", got)
	}

	lines = append(lines, CartLine{ProductID: "b", UnitPriceTiyin: 10000, Qty: 1})
	got := apply([]*Promotion{p}, lines, nil)
	checkInvariants(t, got, lines)
	if got.DiscountTiyin != 4000 {
		t.Fatalf("kutilgan 4000, olindi %d", got.DiscountTiyin)
	}
}

// ═══════════════════════════════════════════════════════════════════
// CHEKLOVLAR
// ═══════════════════════════════════════════════════════════════════

// Maksimal chegirma — aksiyaning O'ZI YUTGAN qatorlari yig'indisiga.
func TestApplyMaxDiscountCap(t *testing.T) {
	p := basePromo("p1", TypePercent)
	p.AppliesToOrders = true
	p.DiscountValue = 50
	p.MaxDiscountAmountTiyin = 3000
	lines := []CartLine{
		{ProductID: "a", UnitPriceTiyin: 10000, Qty: 1},
		{ProductID: "b", UnitPriceTiyin: 10000, Qty: 1},
	}
	got := apply([]*Promotion{p}, lines, nil)
	checkInvariants(t, got, lines)
	if got.DiscountTiyin != 3000 {
		t.Fatalf("chegara 3000 kutilgan edi, olindi %d", got.DiscountTiyin)
	}
	if got.LineDiscounts[0] != 1500 || got.LineDiscounts[1] != 1500 {
		t.Errorf("taqsimot: %v, kutilgan [1500 1500]", got.LineDiscounts)
	}
}

// Chegara ishlaganda ham qator mahsulotning o'z chegirmasidan past
// tushmaydi (mijoz hech qachon yutqazmaydi).
func TestApplyMaxDiscountCapNeverGoesBelowBaseline(t *testing.T) {
	p := basePromo("p1", TypePercent)
	p.AppliesToOrders = true
	p.DiscountValue = 50
	p.MaxDiscountAmountTiyin = 1000
	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 10000, Qty: 1}}
	baseline := []int64{3000}

	got := apply([]*Promotion{p}, lines, baseline)
	checkInvariants(t, got, lines)
	if got.DiscountTiyin != 3000 {
		t.Errorf("chegirma: %d, kutilgan 3000 (mahsulot chegirmasi)", got.DiscountTiyin)
	}
	if len(got.Promotions) != 0 {
		t.Errorf("chegara tufayli aksiya hissasi qolmadi, ro'yxatda bo'lmasligi kerak: %+v", got.Promotions)
	}
}

func TestApplyMinOrderAmountGate(t *testing.T) {
	p := basePromo("p1", TypePercent)
	p.AppliesToOrders = true
	p.MinOrderAmountTiyin = 50000
	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 10000, Qty: 1}}
	if got := apply([]*Promotion{p}, lines, nil); got.DiscountTiyin != 0 {
		t.Fatalf("minimal summa bajarilmadi — chegirma bo'lmasligi kerak: %+v", got)
	}
}

func TestApplyLoyaltyGatedByPreviousOrders(t *testing.T) {
	p := basePromo("p1", TypeLoyalty)
	p.AppliesToOrders = true
	p.MinPreviousOrders = 5
	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 10000, Qty: 1}}

	if got := Apply([]*Promotion{p}, lines, nil, 2, time.Now()); got.DiscountTiyin != 0 {
		t.Fatalf("2 ta buyurtma — hali erta: %+v", got)
	}
	got := Apply([]*Promotion{p}, lines, nil, 5, time.Now())
	checkInvariants(t, got, lines)
	if got.DiscountTiyin != 2000 {
		t.Fatalf("kutilgan 2000, olindi %d", got.DiscountTiyin)
	}
}

func TestApplyFreeDeliveryNeverApplied(t *testing.T) {
	p := basePromo("p1", TypeFreeDelivery)
	p.AppliesToOrders = true
	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 10000, Qty: 1}}
	if got := apply([]*Promotion{p}, lines, nil); got.DiscountTiyin != 0 {
		t.Fatalf("yetkazish chegirmasi narxga tegmasligi kerak: %+v", got)
	}
}

func TestApplyIgnoresInactive(t *testing.T) {
	p := basePromo("p1", TypePercent)
	p.AppliesToOrders = true
	p.Active = false
	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 10000, Qty: 1}}
	if got := apply([]*Promotion{p}, lines, nil); got.DiscountTiyin != 0 {
		t.Fatalf("to'xtatilgan aksiya qo'llandi: %+v", got)
	}
}

func TestApplyIgnoresExpired(t *testing.T) {
	p := basePromo("p1", TypePercent)
	p.AppliesToOrders = true
	p.StartAt = time.Now().Add(-48 * time.Hour)
	p.EndAt = time.Now().Add(-24 * time.Hour)
	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 10000, Qty: 1}}
	if got := apply([]*Promotion{p}, lines, nil); got.DiscountTiyin != 0 {
		t.Fatalf("muddati o'tgan aksiya qo'llandi: %+v", got)
	}
}

// ═══════════════════════════════════════════════════════════════════
// TAQSIMOT VA YAXLITLASH
// ═══════════════════════════════════════════════════════════════════

// Yaxlitlash qoldig'i yo'qolmaydi: 50% x 1001 = 500.5 -> 500, qatorlarga
// 499 + 0 tushadi va qolgan 1 tiyin eng katta qatorga qaytariladi.
func TestApplyLineDiscountsSumExactly(t *testing.T) {
	p := basePromo("p1", TypePercent)
	p.AppliesToOrders = true
	p.DiscountValue = 50
	lines := []CartLine{
		{ProductID: "a", UnitPriceTiyin: 1000, Qty: 1},
		{ProductID: "b", UnitPriceTiyin: 1, Qty: 1},
	}
	got := apply([]*Promotion{p}, lines, nil)
	checkInvariants(t, got, lines)
	if got.LineDiscounts[0] != 500 || got.LineDiscounts[1] != 0 {
		t.Errorf("taqsimot: %v, kutilgan [500 0]", got.LineDiscounts)
	}
}

// ═══════════════════════════════════════════════════════════════════
// TUR ↔ BIRLIK ZIDDIYATI (EffectiveUnit)
// ═══════════════════════════════════════════════════════════════════

// Panelda tur va birlik alohida tanlanadi, ya'ni bazada zid juftlik
// bo'lishi mumkin edi. Endi TUR hal qiladi.
func TestApplyPercentTypeIgnoresAmountUnit(t *testing.T) {
	p := basePromo("p1", TypePercent)
	p.DiscountUnit = DiscountUnitAmount // zid qiymat
	p.DiscountValue = 20
	p.AppliesToOrders = true
	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 10000, Qty: 1}}
	got := apply([]*Promotion{p}, lines, nil)
	if got.DiscountTiyin != 2000 {
		t.Fatalf("20%% = 2000 kutilgan edi (20 tiyin emas), olindi %d", got.DiscountTiyin)
	}
}

func TestApplyFixedAmountTypeIgnoresPercentUnit(t *testing.T) {
	p := basePromo("p1", TypeFixedAmount)
	p.DiscountUnit = DiscountUnitPercent // zid qiymat
	p.DiscountValue = 5000
	p.AppliesToOrders = true
	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 10000, Qty: 1}}
	got := apply([]*Promotion{p}, lines, nil)
	if got.DiscountTiyin != 5000 {
		t.Fatalf("5000 tiyin summa kutilgan edi, olindi %d", got.DiscountTiyin)
	}
}

// Bo'sh birlik (eski yozuvlar) — FOIZ deb qaraladi, klientdagi standart
// bilan bir xil.
func TestEffectiveUnitDefaultsToPercent(t *testing.T) {
	p := &Promotion{Type: TypeBundle}
	if got := p.EffectiveUnit(); got != DiscountUnitPercent {
		t.Errorf("bo'sh birlik: %q, kutilgan %q", got, DiscountUnitPercent)
	}
}
