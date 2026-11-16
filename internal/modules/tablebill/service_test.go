package tablebill

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeOrders struct {
	mu     sync.Mutex
	orders []Order
}

func (f *fakeOrders) TableOrders(context.Context, string, string, time.Time) ([]Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Order(nil), f.orders...), nil
}

type fakePay struct{ started int64 }

func (f *fakePay) StartShare(_ context.Context, id, _, _, _ string) (*PayLink, error) {
	atomic.AddInt64(&f.started, 1)
	return &PayLink{PaymentID: "p-" + id, PayURL: "https://pay/" + id}, nil
}

type fakeDeposit struct{ amount int64 }

func (f fakeDeposit) SeatedDeposit(context.Context, string, string) (string, int64, bool) {
	return "rsv1", f.amount, f.amount > 0
}

type fakeNote struct{ events []string }

func (n *fakeNote) BillChanged(_ context.Context, _ *Bill, e string) { n.events = append(n.events, e) }

type fix struct {
	svc    *Service
	orders *fakeOrders
	pay    *fakePay
	note   *fakeNote
	now    time.Time
}

// Stol: 2 buyurtma. o1: 2x Osh (45 000) + 1x Choy (5 000) = 95 000;
// o2: 3x Somsa (8 000) = 24 000. Jami 119 000 so'm.
func newFix(t *testing.T, deposit int64) *fix {
	t.Helper()
	f := &fix{orders: &fakeOrders{}, pay: &fakePay{}, note: &fakeNote{}, now: time.Date(2026, 11, 16, 15, 0, 0, 0, time.UTC)}
	f.orders.orders = []Order{
		{ID: "o1", Number: "161126-1", Status: "served", TotalTiyin: 9_500_000, WaiterID: "w1", WaiterName: "Vali",
			Lines: []Line{{Name: "Osh", Qty: 2, UnitTiyin: 4_500_000, AmountTiyin: 9_000_000}, {Name: "Choy", Qty: 1, UnitTiyin: 500_000, AmountTiyin: 500_000}}},
		{ID: "o2", Number: "161126-2", Status: "ready", TotalTiyin: 2_400_000,
			Lines: []Line{{Name: "Somsa", Qty: 3, UnitTiyin: 800_000, AmountTiyin: 2_400_000}}},
	}
	var seq int64
	svc, err := New(Deps{Store: NewMemStore(), Orders: f.orders, Deposits: fakeDeposit{deposit}, Payments: f.pay,
		Notifier: f.note, NewID: func() string { return fmt.Sprintf("id%03d", atomic.AddInt64(&seq, 1)) },
		Now: func() time.Time { return f.now }})
	if err != nil {
		t.Fatal(err)
	}
	f.svc = svc
	return f
}

func TestEqualSplitTipAndClose(t *testing.T) {
	f := newFix(t, 0)
	ctx := context.Background()
	v, err := f.svc.Current(ctx, "r1", "t1", "5")
	if err != nil || v.TotalTiyin != 11_900_000 || v.RemainingTiyin != 11_900_000 || len(v.Orders) != 2 {
		t.Fatalf("hisob: %+v %v", v, err)
	}
	// Ikkinchi chaqiriq — o'sha hisob.
	v2, _ := f.svc.Current(ctx, "r1", "t1", "5")
	if v2.Bill.ID != v.Bill.ID {
		t.Fatal("ikkinchi ochiq hisob yaratildi")
	}
	// 4 ga teng: ceil(119 000 / 4) = 29 750; 10% choy puli.
	r, err := f.svc.CreateShare(ctx, "r1", ShareInput{BillID: v.Bill.ID, PayerID: "c1", Kind: KindEqual, Parts: 4, TipPercent: 10})
	if err != nil || r.Share.AmountTiyin != 2_975_000 || r.Share.TipTiyin != 297_500 || r.Pay == nil {
		t.Fatalf("ulush: %+v %v", r, err)
	}
	if amt, _ := f.svc.ShareAmountTiyin(ctx, r.Share.ID); amt != 2_975_000+297_500 {
		t.Fatalf("to'lov summasi: %d", amt)
	}
	if err := f.svc.OnSharePaid(ctx, r.Share.ID); err != nil {
		t.Fatal(err)
	}
	v, _ = f.svc.ViewOf(ctx, v.Bill.ID)
	if v.PaidTiyin != 2_975_000 || v.TipsTiyin != 297_500 || v.RemainingTiyin != 11_900_000-2_975_000 {
		t.Fatalf("to'langandan keyin: %+v", v)
	}
	// Qolganini ofitsiant naqd yopadi -> hisob yopiladi.
	v, err = f.svc.CloseByStaff(ctx, "r1", v.Bill.ID, MethodCash, "w1")
	if err != nil || v.Bill.Status != StatusPaid || v.RemainingTiyin != 0 {
		t.Fatalf("naqd yopish: %+v %v", v, err)
	}
	if _, err := f.svc.CreateShare(ctx, "r1", ShareInput{BillID: v.Bill.ID, PayerID: "c2", Kind: KindEqual, Parts: 2}); !errors.Is(err, ErrNothingDue) {
		t.Fatalf("yopilgan hisob: %v", err)
	}
	// Yopilgandan keyin stolda yangi hisob ochiladi (yangi buyurtma bo'lsa).
	f.orders.orders = append(f.orders.orders, Order{ID: "o3", Number: "161126-3", Status: "created", TotalTiyin: 500_000,
		Lines: []Line{{Name: "Choy", Qty: 1, UnitTiyin: 500_000, AmountTiyin: 500_000}}})
	nv, _ := f.svc.Current(ctx, "r1", "t1", "5")
	if nv.Bill.ID == v.Bill.ID || nv.TotalTiyin != 500_000 {
		t.Fatalf("yangi seans: %+v", nv)
	}
}

func TestItemsCannotBePaidTwice(t *testing.T) {
	f := newFix(t, 0)
	ctx := context.Background()
	v, _ := f.svc.Current(ctx, "r1", "t1", "5")
	// Mehmon A: 1 Osh + Choy.
	a, err := f.svc.CreateShare(ctx, "r1", ShareInput{BillID: v.Bill.ID, PayerID: "a", Kind: KindItems,
		Items: []ItemRef{{OrderID: "o1", Line: 0, Qty: 1}, {OrderID: "o1", Line: 1, Qty: 1}}})
	if err != nil || a.Share.AmountTiyin != 5_000_000 {
		t.Fatalf("A: %+v %v", a, err)
	}
	// Mehmon B Choyni ham tanlasa — rad (A to'layapti).
	if _, err := f.svc.CreateShare(ctx, "r1", ShareInput{BillID: v.Bill.ID, PayerID: "b", Kind: KindItems,
		Items: []ItemRef{{OrderID: "o1", Line: 1, Qty: 1}}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ikki marta: %v", err)
	}
	// B: qolgan 1 Osh + 3 Somsa.
	b, err := f.svc.CreateShare(ctx, "r1", ShareInput{BillID: v.Bill.ID, PayerID: "b", Kind: KindItems,
		Items: []ItemRef{{OrderID: "o1", Line: 0, Qty: 1}, {OrderID: "o2", Line: 0, Qty: 3}}})
	if err != nil || b.Share.AmountTiyin != 6_900_000 {
		t.Fatalf("B: %+v %v", b, err)
	}
	// Ikkalasi to'lanmagan paytda ofitsiant yopa olmaydi.
	if _, err := f.svc.CloseByStaff(ctx, "r1", v.Bill.ID, MethodCash, "w1"); !errors.Is(err, ErrPendingPayments) {
		t.Fatalf("pending: %v", err)
	}
	_ = f.svc.OnSharePaid(ctx, a.Share.ID)
	_ = f.svc.OnSharePaid(ctx, b.Share.ID)
	v, _ = f.svc.ViewOf(ctx, v.Bill.ID)
	if v.Bill.Status != StatusPaid || v.PaidTiyin != 11_900_000 {
		t.Fatalf("to'liq: %+v", v)
	}
	last := f.note.events[len(f.note.events)-1]
	if last != "bill_paid" {
		t.Fatalf("hodisa: %v", f.note.events)
	}
}

func TestPendingExpiresAndFailedReleases(t *testing.T) {
	f := newFix(t, 0)
	ctx := context.Background()
	v, _ := f.svc.Current(ctx, "r1", "t1", "5")
	a, _ := f.svc.CreateShare(ctx, "r1", ShareInput{BillID: v.Bill.ID, PayerID: "a", Kind: KindItems,
		Items: []ItemRef{{OrderID: "o1", Line: 1, Qty: 1}}})
	// To'lov amalga oshmadi — choy yana bo'sh.
	_ = f.svc.OnShareFailed(ctx, a.Share.ID)
	if _, err := f.svc.CreateShare(ctx, "r1", ShareInput{BillID: v.Bill.ID, PayerID: "b", Kind: KindItems,
		Items: []ItemRef{{OrderID: "o1", Line: 1, Qty: 1}}}); err != nil {
		t.Fatalf("bo'shagan qator: %v", err)
	}
	// 15 daqiqadan keyin to'lanmagan ulush band qilmaydi.
	f.now = f.now.Add(16 * time.Minute)
	v, _ = f.svc.ViewOf(ctx, v.Bill.ID)
	if v.PendingTiyin != 0 || v.RemainingTiyin != 11_900_000 {
		t.Fatalf("muddat: %+v", v)
	}
}

func TestDepositReducesDueAndValidation(t *testing.T) {
	f := newFix(t, 4_000_000) // bron zakladi 40 000
	ctx := context.Background()
	v, _ := f.svc.Current(ctx, "r1", "t1", "5")
	if v.Bill.DepositTiyin != 4_000_000 || v.DueTiyin != 7_900_000 {
		t.Fatalf("zaklad: %+v", v)
	}
	if _, err := f.svc.CreateShare(ctx, "r1", ShareInput{BillID: v.Bill.ID, Kind: KindEqual, Parts: 2, TipPercent: 7}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("tip: %v", err)
	}
	if _, err := f.svc.CreateShare(ctx, "boshqa", ShareInput{BillID: v.Bill.ID, Kind: KindEqual, Parts: 2}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("begona restoran: %v", err)
	}
	if _, err := f.svc.CreateShare(ctx, "r1", ShareInput{BillID: v.Bill.ID, Kind: KindEqual, Parts: 0}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("parts: %v", err)
	}
	if _, err := f.svc.CloseByStaff(ctx, "r1", v.Bill.ID, "bitcoin", "w1"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("usul: %v", err)
	}
	// Teng bo'lish zakladdan keyingi summa bo'yicha: 79 000 / 2 = 39 500.
	r, err := f.svc.CreateShare(ctx, "r1", ShareInput{BillID: v.Bill.ID, Kind: KindEqual, Parts: 2})
	if err != nil || r.Share.AmountTiyin != 3_950_000 {
		t.Fatalf("teng: %+v %v", r, err)
	}
}

func TestParallelSharesNeverExceedDue(t *testing.T) {
	f := newFix(t, 0)
	ctx := context.Background()
	v, _ := f.svc.Current(ctx, "r1", "t1", "5")
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = f.svc.CreateShare(ctx, "r1", ShareInput{BillID: v.Bill.ID, PayerID: fmt.Sprint(i), Kind: KindEqual, Parts: 2})
		}(i)
	}
	wg.Wait()
	v, _ = f.svc.ViewOf(ctx, v.Bill.ID)
	if v.PendingTiyin > v.DueTiyin {
		t.Fatalf("ortiqcha band: pending %d > due %d", v.PendingTiyin, v.DueTiyin)
	}
}
