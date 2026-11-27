package shop

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newTestSvc(t *testing.T, base ...BaseProduct) (*Service, *clock) {
	t.Helper()
	tz, _ := time.LoadLocation("Asia/Tashkent")
	n := 0
	c := &clock{t: time.Date(2026, 11, 28, 10, 0, 0, 0, tz)}
	s := NewService(NewMemRepo(base...), func() string { n++; return fmt.Sprintf("id%04d", n) }, tz).WithClock(c.now)
	return s, c
}

var (
	ctx    = context.Background()
	storeA = Store{ID: "store-a", Category: CategoryStore}
	storeB = Store{ID: "store-b", Category: CategoryStore}
	qurt   = Store{ID: "qurt-1", Category: CategoryQurt}
	pharm  = Store{ID: "pharm-1", Category: CategoryPharmacy}
)

func TestUpsertAndStoreIsolation(t *testing.T) {
	s, _ := newTestSvc(t)
	if _, err := s.Upsert(ctx, storeA, UpsertInput{Barcode: "4780001", Name: "Non", Category: "Non", PriceTiyin: 500000, Quantity: 10}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Upsert(ctx, storeA, UpsertInput{Barcode: "bad barcode!", Name: "x"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("noto'g'ri shtrix-kod qabul qilindi: %v", err)
	}
	b, _ := s.Products(ctx, storeB)
	if len(b) != 0 {
		t.Fatal("B do'kon A do'konning mahsulotini ko'rdi")
	}
	if err := s.Delete(ctx, storeB, "4780001"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("B do'kon A mahsulotini o'chira oldi: %v", err)
	}
	a, _ := s.Products(ctx, storeA)
	if len(a) != 1 || a[0].Quantity != 10 || a[0].PriceTiyin != 500000 {
		t.Fatalf("A: %+v", a)
	}
}

func TestQurtWeightRequiredAndKept(t *testing.T) {
	s, _ := newTestSvc(t)
	var changed []string
	s.OnQurtChange = func(_ context.Context, store string, p *Product, deleted bool) {
		changed = append(changed, fmt.Sprintf("%s/%s/%v", store, p.Barcode, deleted))
	}
	if _, err := s.Upsert(ctx, qurt, UpsertInput{Barcode: "q1", Name: "Qurt"}); !errors.Is(err, errQurtWeight) {
		t.Fatalf("yangi qurt mahsulotida gramm majburiy: %v", err)
	}
	w := 250
	if _, err := s.Upsert(ctx, qurt, UpsertInput{Barcode: "q1", Name: "Qurt", WeightGrams: &w, PriceTiyin: 1500000}); err != nil {
		t.Fatal(err)
	}
	// Narx tahriri gramm yubormaydi — mavjud qiymat saqlanadi.
	p, err := s.Upsert(ctx, qurt, UpsertInput{Barcode: "q1", Name: "Qurt", PriceTiyin: 1600000})
	if err != nil || p.WeightGrams != 250 {
		t.Fatalf("gramm saqlanmadi: %+v %v", p, err)
	}
	bad := "javascript:alert(1)"
	if _, err := s.Upsert(ctx, qurt, UpsertInput{Barcode: "q1", Name: "Qurt", ImageURL: &bad}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("xavfli rasm manzili qabul qilindi: %v", err)
	}
	if len(changed) != 2 || changed[0] != "qurt-1/q1/false" {
		t.Fatalf("qurt katalogi xabari: %v", changed)
	}
	// Sotuv ham katalogni yangilaydi (qoldiq/available).
	if _, err := s.Sell(ctx, qurt, []SellLine{{Barcode: "q1", Name: "Qurt", Quantity: 1, PriceTiyin: 1600000}}, "cash"); err != nil {
		t.Fatal(err)
	}
	if len(changed) != 3 || changed[2] != "qurt-1/q1/false" {
		t.Fatalf("sotuvdan keyin qurt katalogi yangilanmadi: %v", changed)
	}
	changed = changed[:2]
	// Oddiy do'konda qurt maydonlari umuman yozilmaydi.
	p, _ = s.Upsert(ctx, storeA, UpsertInput{Barcode: "s1", Name: "X", WeightGrams: &w})
	if p.WeightGrams != 0 || len(changed) != 2 {
		t.Fatal("oddiy do'kon qurt maydonini yozdi yoki qurt katalogiga tegdi")
	}
}

func TestSellOneToOne(t *testing.T) {
	s, c := newTestSvc(t)
	_, _ = s.Upsert(ctx, storeA, UpsertInput{Barcode: "p1", Name: "Sut", Category: "Sut", PriceTiyin: 100000, BasePriceTiyin: 80000, Quantity: 3})
	// 1) qoldiqdan ko'p sotilsa — 0 gacha (kassa to'xtamaydi), sold oshadi.
	if _, err := s.Sell(ctx, storeA, []SellLine{{Barcode: "p1", Name: "Sut", Quantity: 5, PriceTiyin: 100000, Category: "Sut"}}, "cash"); err != nil {
		t.Fatal(err)
	}
	p, _ := s.repo.GetProduct(ctx, storeA.ID, "p1")
	if p.Quantity != 0 || p.Sold != 5 {
		t.Fatalf("qoldiq/sotilgan: %+v", p)
	}
	// 2) shu kuni shu to'lov turi — bitta yig'indi, jami = miqdor * OXIRGI narx.
	c.t = c.t.Add(time.Hour)
	_, _ = s.Sell(ctx, storeA, []SellLine{{Barcode: "p1", Name: "Sut", Quantity: 2, PriceTiyin: 110000}}, "cash")
	// 3) boshqa to'lov turi — alohida yozuv.
	_, _ = s.Sell(ctx, storeA, []SellLine{{Barcode: "p1", Name: "Sut", Quantity: 1, PriceTiyin: 110000}}, "card")
	sales, _ := s.Sales(ctx, storeA, nil, nil)
	if len(sales) != 2 {
		t.Fatalf("kunlik yig'indilar: %+v", sales)
	}
	var cash Sale
	for _, x := range sales {
		if x.PaymentMethod == "cash" {
			cash = x
		}
	}
	if cash.Quantity != 7 || cash.TotalTiyin != 7*110000 || cash.BasePriceTiyin != 80000 {
		t.Fatalf("naqd yig'indi: %+v", cash)
	}
	// 4) ertasiga — yangi yig'indi.
	c.t = c.t.Add(24 * time.Hour)
	_, _ = s.Sell(ctx, storeA, []SellLine{{Barcode: "p1", Name: "Sut", Quantity: 1, PriceTiyin: 110000}}, "cash")
	sales, _ = s.Sales(ctx, storeA, nil, nil)
	if len(sales) != 3 {
		t.Fatalf("ertangi kun alohida bo'lishi kerak: %d", len(sales))
	}
	// 5) yo'q mahsulot sotilsa — 0 qoldiq bilan yaratiladi.
	_, _ = s.Sell(ctx, storeA, []SellLine{{Barcode: "new1", Name: "Yangi", Quantity: 2, PriceTiyin: 5000}}, "")
	n, err := s.repo.GetProduct(ctx, storeA.ID, "new1")
	if err != nil || n.Quantity != 0 || n.Sold != 2 {
		t.Fatalf("yangi mahsulot: %+v %v", n, err)
	}
	// 6) har qator "Sotilganlar"da; 30 kundan eskisi ko'rinmaydi.
	lines, _ := s.SoldLines(ctx, storeA, nil, nil)
	if len(lines) != 5 {
		t.Fatalf("sotuv qatorlari: %d", len(lines))
	}
	c.t = c.t.Add(31 * 24 * time.Hour)
	if lines, _ = s.SoldLines(ctx, storeA, nil, nil); len(lines) != 0 {
		t.Fatalf("30 kundan eski qatorlar ko'rinmasligi kerak: %d", len(lines))
	}
	if _, err := s.Sell(ctx, storeA, nil, "cash"); !errors.Is(err, ErrInvalid) {
		t.Fatal("bo'sh sotuv qabul qilindi")
	}
	if _, err := s.Sell(ctx, storeA, []SellLine{{Barcode: "p1", Name: "Sut", Quantity: -1}}, "cash"); !errors.Is(err, ErrInvalid) {
		t.Fatal("manfiy miqdor qabul qilindi")
	}
}

func TestDebts(t *testing.T) {
	s, _ := newTestSvc(t)
	d, err := s.AddDebt(ctx, storeA, "Ali", "+998901112233", 1000000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddDebt(ctx, storeA, "", "1", 1); !errors.Is(err, ErrInvalid) {
		t.Fatal("ismsiz qarz qabul qilindi")
	}
	full, cur, err := s.PayDebt(ctx, storeA, d.ID, 400000)
	if err != nil || full || cur.AmountTiyin != 600000 {
		t.Fatalf("qisman to'lov: %v %+v %v", full, cur, err)
	}
	if _, _, err := s.PayDebt(ctx, storeB, d.ID, 1); !errors.Is(err, ErrNotFound) {
		t.Fatal("begona do'kon qarzni to'ladi")
	}
	full, _, err = s.PayDebt(ctx, storeA, d.ID, 600000)
	if err != nil || !full {
		t.Fatalf("to'liq to'lov: %v %v", full, err)
	}
	if list, _ := s.Debts(ctx, storeA); len(list) != 0 {
		t.Fatal("to'liq to'langan qarz qoldi")
	}
}

func TestSearchMergesGlobalWithoutDuplicates(t *testing.T) {
	s, _ := newTestSvc(t, BaseProduct{Barcode: "g1", Name: "Coca-Cola 1L"}, BaseProduct{Barcode: "p1", Name: "Coca-Cola 0.5L (global)"})
	_, _ = s.Upsert(ctx, storeA, UpsertInput{Barcode: "p1", Name: "Coca-Cola 0.5L", PriceTiyin: 700000})
	hits, err := s.Search(ctx, storeA, "coca", "")
	if err != nil || len(hits) != 2 || hits[0].IsGlobal || !hits[1].IsGlobal || hits[1].Barcode != "g1" {
		t.Fatalf("qidiruv: %+v %v", hits, err)
	}
	// B do'kon A mahsulotini ko'rmaydi — faqat global.
	hits, _ = s.Search(ctx, storeB, "coca", "")
	for _, h := range hits {
		if !h.IsGlobal {
			t.Fatalf("B qidiruvida A mahsuloti: %+v", h)
		}
	}
}

func TestPharmacyAtomicSellAndAccess(t *testing.T) {
	s, _ := newTestSvc(t)
	if _, err := s.PharmProducts(ctx, storeA); !errors.Is(err, ErrForbidden) {
		t.Fatal("oddiy do'kon apteka bo'limiga kirdi")
	}
	if _, err := s.AddPharm(ctx, pharm, PharmAddInput{Barcode: "d1", Name: "Paratsetamol", Category: "Og'riq", PriceTiyin: 300000, Quantity: 5}); err != nil {
		t.Fatal(err)
	}
	p, _ := s.AddPharm(ctx, pharm, PharmAddInput{Barcode: "d1", Name: "Paratsetamol", Category: "Og'riq", PriceTiyin: 300000, Quantity: 3})
	if p.Quantity != 8 || p.IncomingQuantity != 8 {
		t.Fatalf("qayta qo'shish qoldiq va kirimni oshirishi kerak: %+v", p)
	}
	_, _ = s.AddPharm(ctx, pharm, PharmAddInput{Barcode: "d2", Name: "Vitamin C", Category: "Vitamin", PriceTiyin: 100000, Quantity: 1})
	// d2 yetmaydi -> butun chek bekor, d1 ham kamaymaydi.
	_, err := s.SellPharm(ctx, pharm, []PharmSellLine{{Barcode: "d1", Quantity: 2, PriceTiyin: 300000}, {Barcode: "d2", Quantity: 5, PriceTiyin: 100000}}, "naqd")
	if !errors.Is(err, ErrInsufficient) {
		t.Fatalf("yetmagan dori: %v", err)
	}
	d1, _ := s.repo.GetPharm(ctx, pharm.ID, "d1")
	if d1.Quantity != 8 {
		t.Fatalf("muvaffaqiyatsiz chek qoldiqni o'zgartirdi: %v", d1.Quantity)
	}
	sale, err := s.SellPharm(ctx, pharm, []PharmSellLine{{Barcode: "d1", Quantity: 2, PriceTiyin: 300000}}, "plastik")
	if err != nil || sale.TotalTiyin != 600000 || len(sale.Items) != 1 {
		t.Fatalf("chek: %+v %v", sale, err)
	}
	if _, err := s.SellPharm(ctx, pharm, []PharmSellLine{{Barcode: "d1", Quantity: 1, PriceTiyin: 1}}, "bitcoin"); !errors.Is(err, ErrInvalid) {
		t.Fatal("noma'lum to'lov turi qabul qilindi")
	}
	d1, _ = s.repo.GetPharm(ctx, pharm.ID, "d1")
	if d1.Quantity != 6 || d1.Sold != 2 {
		t.Fatalf("sotuvdan keyin: %+v", d1)
	}
	if list, _ := s.PharmSales(ctx, pharm); len(list) != 1 {
		t.Fatalf("cheklar: %d", len(list))
	}
	found, err := s.PharmInfo(ctx, pharm, "parats")
	if err != nil || found.Barcode != "d1" {
		t.Fatalf("pharm-info: %+v %v", found, err)
	}
}
