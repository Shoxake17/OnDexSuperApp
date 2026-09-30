package orders

import (
	"context"
	"errors"
	"testing"
)

// YETKAZIB BERISH HAQI QUOTE VA BUYURTMADA BIR XIL.
//
// â”Œâ”€ NEGA ENG MUHIM TEKSHIRUV SHU â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
// Mijoz checkout ekranida `Quote` bergan raqamni ko'radi va o'shanga
// rozi bo'lib to'laydi. Buyurtma esa `Create` hisoblagan raqam bilan
// yaratiladi. Ikkalasi ajralsa mijoz bir summani ko'rib, boshqasini
// to'lardi â€” va bu JIMGINA sodir bo'lardi.
// â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜

// stubPricer â€” belgilangan haqni qaytaradi va chaqiruvlarni yozadi.
type stubPricer struct {
	fee    int64
	err    error
	dineIn []bool // har chaqiruvdagi qiymat
	totals []int64
}

func (p *stubPricer) DeliveryFee(_ context.Context, _ string, total int64, dineIn bool) (int64, error) {
	p.dineIn = append(p.dineIn, dineIn)
	p.totals = append(p.totals, total)
	if p.err != nil {
		return 0, p.err
	}
	if dineIn {
		return 0, nil
	}
	return p.fee, nil
}

func feeTestItems() []Item {
	return []Item{{ProductID: "p1", Name: "Lavash", Qty: 2, PriceTiyin: 1_500_000}}
}

const feeTestGoods = 3_000_000 // 2 Ã— 15 000 so'm

func TestQuoteIncludesDeliveryFee(t *testing.T) {
	svc := NewService(newFakeOCCRepo(), nil, func() string { return "o1" }, nil)
	svc.WithDelivery(&stubPricer{fee: 800_000})

	q, err := svc.Quote(context.Background(), "r1", feeTestItems(), "c1", false)
	if err != nil {
		t.Fatalf("kutilmagan xato: %v", err)
	}
	if q.DeliveryFeeTiyin != 800_000 {
		t.Errorf("haq %d, kutilgan 800000", q.DeliveryFeeTiyin)
	}
	if want := int64(feeTestGoods + 800_000); q.TotalTiyin != want {
		t.Errorf("jami %d, kutilgan %d", q.TotalTiyin, want)
	}
	// âš ï¸ INVARIANT: qatorlar yig'indisi + haq == jami. Yetkazish haqi
	// TAOM emas, shuning uchun u qatorlar ichiga KIRMAYDI.
	var lines int64
	for _, l := range q.Lines {
		lines += l.TotalTiyin
	}
	if lines+q.DeliveryFeeTiyin != q.TotalTiyin {
		t.Errorf("qatorlar (%d) + haq (%d) != jami (%d)", lines, q.DeliveryFeeTiyin, q.TotalTiyin)
	}
}

func TestQuoteDineInHasNoFee(t *testing.T) {
	p := &stubPricer{fee: 800_000}
	svc := NewService(newFakeOCCRepo(), nil, func() string { return "o1" }, nil)
	svc.WithDelivery(p)

	q, err := svc.Quote(context.Background(), "r1", feeTestItems(), "c1", true)
	if err != nil {
		t.Fatalf("kutilmagan xato: %v", err)
	}
	if q.DeliveryFeeTiyin != 0 {
		t.Errorf("stol buyurtmasida haq %d, 0 kutilgan", q.DeliveryFeeTiyin)
	}
	if q.TotalTiyin != feeTestGoods {
		t.Errorf("jami %d, kutilgan %d", q.TotalTiyin, feeTestGoods)
	}
	// Narxlovchi umuman bezovta qilinmaydi â€” stol buyurtmasida
	// restoranni bazadan o'qish ham keraksiz.
	if len(p.dineIn) != 0 {
		t.Errorf("stol buyurtmasida narxlovchi chaqirilmasligi kerak")
	}
}

func TestCreateSnapshotsDeliveryFee(t *testing.T) {
	svc := NewService(newFakeOCCRepo(), nil, func() string { return "o1" }, nil)
	svc.WithDelivery(&stubPricer{fee: 800_000})

	o := &Order{CustomerID: "c1", RestaurantID: "r1", Items: feeTestItems()}
	created, err := svc.Create(context.Background(), o)
	if err != nil {
		t.Fatalf("kutilmagan xato: %v", err)
	}
	if created.DeliveryFeeTiyin != 800_000 {
		t.Errorf("buyurtmada haq %d, kutilgan 800000", created.DeliveryFeeTiyin)
	}
	if want := int64(feeTestGoods + 800_000); created.TotalTiyin != want {
		t.Errorf("jami %d, kutilgan %d", created.TotalTiyin, want)
	}
	if created.SubtotalTiyin != feeTestGoods {
		t.Errorf("subtotal %d â€” haq unga QO'SHILMASLIGI kerak", created.SubtotalTiyin)
	}
}

// Quote va Create AYNI raqamni berishi kerak â€” mijoz ko'rgan summa
// bilan yaratilgan buyurtma bir xil bo'lsin.
func TestQuoteAndCreateAgree(t *testing.T) {
	svc := NewService(newFakeOCCRepo(), nil, func() string { return "o1" }, nil)
	svc.WithDelivery(&stubPricer{fee: 800_000})

	q, err := svc.Quote(context.Background(), "r1", feeTestItems(), "c1", false)
	if err != nil {
		t.Fatal(err)
	}
	created, err := svc.Create(context.Background(),
		&Order{CustomerID: "c1", RestaurantID: "r1", Items: feeTestItems()})
	if err != nil {
		t.Fatal(err)
	}
	if q.TotalTiyin != created.TotalTiyin {
		t.Errorf("quote %d != buyurtma %d", q.TotalTiyin, created.TotalTiyin)
	}
	if q.DeliveryFeeTiyin != created.DeliveryFeeTiyin {
		t.Errorf("haq: quote %d != buyurtma %d", q.DeliveryFeeTiyin, created.DeliveryFeeTiyin)
	}
}

// Narxlovchi ulanmagan bo'lsa (eski sozlama, testlar) haq 0 â€” avvalgi
// xatti-harakat saqlanadi.
func TestNoPricerMeansNoFee(t *testing.T) {
	svc := NewService(newFakeOCCRepo(), nil, func() string { return "o1" }, nil)
	q, err := svc.Quote(context.Background(), "r1", feeTestItems(), "c1", false)
	if err != nil {
		t.Fatal(err)
	}
	if q.DeliveryFeeTiyin != 0 || q.TotalTiyin != feeTestGoods {
		t.Errorf("narxlovchisiz: haq %d, jami %d", q.DeliveryFeeTiyin, q.TotalTiyin)
	}
}

// â”Œâ”€ XATO YUTILMAYDI â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
// Haqni hisoblab bo'lmasa (restoran topilmadi, baza javob bermadi)
// buyurtma NOL haq bilan yaratilmasligi kerak â€” restoran pulsiz
// qolardi va sabab ko'rinmasdi.
// â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
func TestPricerErrorStopsOrder(t *testing.T) {
	boom := errors.New("baza javob bermadi")
	svc := NewService(newFakeOCCRepo(), nil, func() string { return "o1" }, nil)
	svc.WithDelivery(&stubPricer{err: boom})

	if _, err := svc.Quote(context.Background(), "r1", feeTestItems(), "c1", false); !errors.Is(err, boom) {
		t.Errorf("quote: %v, xato kutilgan", err)
	}
	_, err := svc.Create(context.Background(),
		&Order{CustomerID: "c1", RestaurantID: "r1", Items: feeTestItems()})
	if !errors.Is(err, boom) {
		t.Errorf("create: %v, xato kutilgan", err)
	}
}
