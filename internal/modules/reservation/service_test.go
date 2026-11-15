package reservation

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeTables []Table

func (f fakeTables) ListTables(context.Context, string) ([]Table, error) { return f, nil }

type fakePay struct {
	mu       sync.Mutex
	started  []string
	refunded []string
	err      error
}

func (f *fakePay) StartDeposit(_ context.Context, id, _, _, _, _ string) (*PayLink, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	f.started = append(f.started, id)
	return &PayLink{PaymentID: "p-" + id, PayURL: "https://pay/" + id}, nil
}
func (f *fakePay) RefundDeposit(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refunded = append(f.refunded, id)
	return nil
}

type fakeWallet struct {
	balance  int64
	debited  map[string]int64
	refunded map[string]int64
}

func (w *fakeWallet) Debit(_ context.Context, ref, _ string, amt int64) error {
	if w.balance < amt {
		return errors.New("balans yetarli emas")
	}
	w.balance -= amt
	w.debited[ref] += amt
	return nil
}
func (w *fakeWallet) Refund(_ context.Context, ref, _ string, amt int64) error {
	w.balance += amt
	w.refunded[ref] += amt
	return nil
}

type fakeNotify struct {
	mu    sync.Mutex
	types []string
}

func (n *fakeNotify) Notify(_ context.Context, e Event) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.types = append(n.types, e.Type)
}

type fix struct {
	svc    *Service
	store  *MemStore
	pay    *fakePay
	wallet *fakeWallet
	note   *fakeNotify
	now    time.Time
}

const rid = "rest1"

func newFix(t *testing.T, tables ...Table) *fix {
	t.Helper()
	if len(tables) == 0 {
		tables = []Table{
			{ID: "t2", Label: "2", Capacity: 2, Active: true},
			{ID: "t4", Label: "4", Capacity: 4, Active: true},
			{ID: "v6", Label: "VIP-1", Zone: "VIP", Capacity: 6, Active: true},
			{ID: "off", Label: "X", Capacity: 10, Active: false},
		}
	}
	f := &fix{store: NewMemStore(), pay: &fakePay{}, note: &fakeNotify{},
		wallet: &fakeWallet{balance: 100_000_000_00, debited: map[string]int64{}, refunded: map[string]int64{}},
		now: time.Date(2026, 11, 15, 6, 0, 0, 0, time.UTC)} // 11:00 Toshkent
	var seq int64
	svc, err := New(Deps{Store: f.store, Tables: fakeTables(tables), Payments: f.pay, Wallet: f.wallet,
		Notifier: f.note, NewID: func() string { return fmt.Sprintf("r%03d", atomic.AddInt64(&seq, 1)) },
		Now: func() time.Time { return f.now }})
	if err != nil {
		t.Fatal(err)
	}
	f.svc = svc
	if _, err := svc.SaveSettings(context.Background(), Settings{RestaurantID: rid, Enabled: true,
		DepositPerPersonTiyin: 50_000_00, ZoneDeposits: map[string]int64{"VIP": 100_000_00},
		SlotMinutes: 120, MinAdvanceMinutes: 60, MaxDaysAhead: 30}); err != nil {
		t.Fatal(err)
	}
	return f
}

// 19:00 Toshkent = 14:00 UTC.
var evening = time.Date(2026, 11, 15, 14, 0, 0, 0, time.UTC)

func (f *fix) create(t *testing.T, cust string, party int, at time.Time, m Method) *CreateResult {
	t.Helper()
	res, err := f.svc.Create(context.Background(), CreateInput{RestaurantID: rid, CustomerID: cust,
		StartsAt: at, PartySize: party, Method: m})
	if err != nil {
		t.Fatalf("Create(%s, %d): %v", cust, party, err)
	}
	return res
}

func TestCreateCardPicksSmallestTableAndPays(t *testing.T) {
	f := newFix(t)
	ctx := context.Background()
	res := f.create(t, "c1", 3, evening, MethodCard)
	r := res.Reservation
	if r.TableID != "t4" || r.Status != StatusHeld || r.DepositTiyin != 3*50_000_00 || res.Pay == nil {
		t.Fatalf("bron: %+v pay=%v", r, res.Pay)
	}
	if r.EndsAt.Sub(r.StartsAt) != 2*time.Hour || r.HoldExpiresAt == nil {
		t.Fatalf("vaqt: %+v", r)
	}
	if err := f.svc.OnDepositPaid(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := f.svc.Get(ctx, r.ID)
	if got.Status != StatusConfirmed || got.DepositState != DepositPaid {
		t.Fatalf("tasdiq: %+v", got)
	}
	// Takroriy callback — o'zgarishsiz.
	if err := f.svc.OnDepositPaid(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
}

func TestNoDoubleBookingAndZoneDeposit(t *testing.T) {
	f := newFix(t)
	ctx := context.Background()
	a := f.create(t, "c1", 4, evening, MethodWallet).Reservation // t4
	b := f.create(t, "c2", 4, evening, MethodWallet).Reservation // t4 band -> VIP (6)
	if a.TableID != "t4" || b.TableID != "v6" || b.DepositTiyin != 4*100_000_00 {
		t.Fatalf("a=%s b=%s dep=%d", a.TableID, b.TableID, b.DepositTiyin)
	}
	// 4 kishilik joy qolmadi (t2 kichik, off faol emas).
	if _, err := f.svc.Create(ctx, CreateInput{RestaurantID: rid, CustomerID: "c3", StartsAt: evening,
		PartySize: 4, Method: MethodWallet}); !errors.Is(err, ErrNoTable) {
		t.Fatalf("uchinchisi: %v", err)
	}
	// Kesishmaydigan vaqt (2 soatdan keyin) — o'sha stol bo'sh.
	c := f.create(t, "c3", 4, evening.Add(2*time.Hour), MethodWallet).Reservation
	if c.TableID != "t4" {
		t.Fatalf("keyingi slot: %s", c.TableID)
	}
	// Bo'sh vaqtlar: 19:00 da 4 kishilik joy yo'q, 21:00 da ham (c3), 18:00 da ham (kesishadi).
	slots, err := f.svc.Availability(ctx, rid, evening, 4)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range slots {
		if s.StartsAt.Equal(evening) || s.StartsAt.Equal(evening.Add(-time.Hour)) {
			t.Fatalf("band vaqt bo'sh ko'rindi: %v", s.StartsAt)
		}
	}
}

func TestParallelBookingsNeverOverlap(t *testing.T) {
	f := newFix(t, Table{ID: "only", Label: "1", Capacity: 4, Active: true})
	ctx := context.Background()
	var ok int64
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := f.svc.Create(ctx, CreateInput{RestaurantID: rid, CustomerID: fmt.Sprintf("p%d", i),
				StartsAt: evening, PartySize: 2, Method: MethodWallet})
			if err == nil {
				atomic.AddInt64(&ok, 1)
			} else if !errors.Is(err, ErrNoTable) {
				t.Errorf("kutilmagan xato: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if ok != 1 {
		t.Fatalf("bitta stolga %d ta bron tushdi", ok)
	}
}

func TestCancelRules(t *testing.T) {
	ctx := context.Background()

	// 2+ soat oldin — wallet zakladi to'liq qaytadi.
	f := newFix(t)
	r := f.create(t, "c1", 2, evening, MethodWallet).Reservation
	got, err := f.svc.CancelByCustomer(ctx, "c1", r.ID)
	if err != nil || got.DepositState != DepositRefunded || f.wallet.refunded[walletRef(r.ID)] != 2*50_000_00 {
		t.Fatalf("erta bekor: %+v %v %v", got, err, f.wallet.refunded)
	}

	// 2 soatdan kam qolganda — restoranda qoladi.
	f = newFix(t)
	r = f.create(t, "c1", 2, evening, MethodWallet).Reservation
	f.now = evening.Add(-90 * time.Minute)
	got, err = f.svc.CancelByCustomer(ctx, "c1", r.ID)
	if err != nil || got.DepositState != DepositKept || len(f.wallet.refunded) != 0 {
		t.Fatalf("kech bekor: %+v %v", got, err)
	}

	// Begona mijoz bekor qila olmaydi.
	f = newFix(t)
	r = f.create(t, "c1", 2, evening, MethodCard).Reservation
	_ = f.svc.OnDepositPaid(ctx, r.ID)
	if _, err := f.svc.CancelByCustomer(ctx, "boshqa", r.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("begona: %v", err)
	}
	// Restoran bekor qilsa (kech bo'lsa ham) — karta zakladi qaytadi.
	f.now = evening.Add(-30 * time.Minute)
	if _, err := f.svc.CancelByRestaurant(ctx, rid, r.ID, ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("sababsiz: %v", err)
	}
	got, err = f.svc.CancelByRestaurant(ctx, rid, r.ID, "zal ta'mirda")
	if err != nil || got.DepositState != DepositRefunded || len(f.pay.refunded) != 1 {
		t.Fatalf("restoran bekor: %+v %v %v", got, err, f.pay.refunded)
	}
	if _, err := f.svc.CancelByRestaurant(ctx, "boshqa-restoran", r.ID, "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("boshqa restoran: %v", err)
	}
}

func TestSeatNoShowSweep(t *testing.T) {
	ctx := context.Background()
	f := newFix(t)
	a := f.create(t, "c1", 2, evening, MethodWallet).Reservation
	b := f.create(t, "c2", 2, evening, MethodWallet).Reservation // t4
	h := f.create(t, "c3", 2, evening.Add(4*time.Hour), MethodCard).Reservation

	// Eslatma: 2 soat qolganda bir marta.
	f.now = evening.Add(-2 * time.Hour)
	f.svc.Sweep(ctx)
	f.svc.Sweep(ctx)
	reminders := 0
	for _, x := range f.note.types {
		if x == "reservation_reminder" {
			reminders++
		}
	}
	if reminders != 2 {
		t.Fatalf("eslatmalar: %v", f.note.types)
	}
	// Karta to'lanmagan — 15 daqiqadan keyin muddati o'tadi (eslatma oqimidan oldin ham).
	got, _ := f.svc.Get(ctx, h.ID)
	if got.Status != StatusExpired || len(f.pay.refunded) != 1 {
		t.Fatalf("held muddati: %+v", got)
	}

	f.now = evening.Add(5 * time.Minute)
	if _, err := f.svc.MarkNoShow(ctx, rid, b.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("erta kelmadi: %v", err)
	}
	seated, err := f.svc.Seat(ctx, rid, a.ID)
	if err != nil || seated.DepositState != DepositApplied {
		t.Fatalf("keldi: %+v %v", seated, err)
	}
	f.now = evening.Add(20 * time.Minute)
	f.svc.Sweep(ctx)
	gb, _ := f.svc.Get(ctx, b.ID)
	if gb.Status != StatusNoShow || gb.DepositState != DepositKept {
		t.Fatalf("kelmadi: %+v", gb)
	}
	f.now = evening.Add(5 * time.Hour)
	f.svc.Sweep(ctx)
	ga, _ := f.svc.Get(ctx, a.ID)
	if ga.Status != StatusCompleted {
		t.Fatalf("yakun: %+v", ga)
	}
}

func TestLatePaymentAfterExpiry(t *testing.T) {
	ctx := context.Background()
	f := newFix(t, Table{ID: "only", Label: "1", Capacity: 4, Active: true})
	r := f.create(t, "c1", 2, evening, MethodCard).Reservation
	f.now = f.now.Add(20 * time.Minute)
	f.svc.Sweep(ctx) // expired
	// Stol bo'sh — kech to'lov bronni tiklaydi.
	if err := f.svc.OnDepositPaid(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := f.svc.Get(ctx, r.ID)
	if got.Status != StatusConfirmed {
		t.Fatalf("tiklanmadi: %+v", got)
	}

	// Ikkinchi holat: muddat o'tgach stolni boshqa mijoz oldi — pul qaytadi.
	f = newFix(t, Table{ID: "only", Label: "1", Capacity: 4, Active: true})
	r = f.create(t, "c1", 2, evening, MethodCard).Reservation
	f.now = f.now.Add(20 * time.Minute)
	f.svc.Sweep(ctx)
	f.create(t, "c2", 2, evening, MethodWallet)
	if err := f.svc.OnDepositPaid(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	got, _ = f.svc.Get(ctx, r.ID)
	if got.Status != StatusExpired || got.DepositState != DepositRefunded {
		t.Fatalf("kech to'lov qaytmadi: %+v", got)
	}
}

func TestValidation(t *testing.T) {
	ctx := context.Background()
	f := newFix(t)
	cases := map[string]CreateInput{
		"qadamga mos emas": {StartsAt: evening.Add(10 * time.Minute), PartySize: 2, Method: MethodCard},
		"juda yaqin":       {StartsAt: f.now.Add(30 * time.Minute), PartySize: 2, Method: MethodCard},
		"juda uzoq":        {StartsAt: f.now.Add(40 * 24 * time.Hour), PartySize: 2, Method: MethodCard},
		"kishi soni":       {StartsAt: evening, PartySize: 0, Method: MethodCard},
		"usulsiz zaklad":   {StartsAt: evening, PartySize: 2},
	}
	for name, in := range cases {
		in.RestaurantID, in.CustomerID = rid, "c1"
		if _, err := f.svc.Create(ctx, in); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// Faol bronlar chegarasi (3) va bir vaqtga ikki bron taqiqi.
	f.create(t, "c9", 2, evening, MethodWallet)
	if _, err := f.svc.Create(ctx, CreateInput{RestaurantID: rid, CustomerID: "c9", StartsAt: evening.Add(time.Hour),
		PartySize: 2, Method: MethodWallet}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("kesishuvchi ikkinchi bron: %v", err)
	}
	f.create(t, "c9", 2, evening.Add(24*time.Hour), MethodWallet)
	f.create(t, "c9", 2, evening.Add(48*time.Hour), MethodWallet)
	if _, err := f.svc.Create(ctx, CreateInput{RestaurantID: rid, CustomerID: "c9", StartsAt: evening.Add(72 * time.Hour),
		PartySize: 2, Method: MethodWallet}); !errors.Is(err, ErrTooMany) {
		t.Fatalf("chegara: %v", err)
	}
	// Wallet balansi yetmasa — stol bo'shaydi.
	f.wallet.balance = 0
	if _, err := f.svc.Create(ctx, CreateInput{RestaurantID: rid, CustomerID: "c8", StartsAt: evening.Add(4 * time.Hour),
		PartySize: 6, Method: MethodWallet}); !errors.Is(err, ErrWalletFunds) {
		t.Fatalf("balans: %v", err)
	}
	slots, _ := f.svc.Availability(ctx, rid, evening, 6)
	found := false
	for _, s := range slots {
		found = found || s.StartsAt.Equal(evening.Add(4*time.Hour))
	}
	if !found {
		t.Fatal("muvaffaqiyatsiz wallet broni stolni band qilib qoldi")
	}
	// Bron o'chiq restoran.
	if _, err := f.svc.SaveSettings(ctx, Settings{RestaurantID: rid, SlotMinutes: 120, MinAdvanceMinutes: 60, MaxDaysAhead: 30}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Availability(ctx, rid, evening, 2); !errors.Is(err, ErrDisabled) {
		t.Fatalf("o'chiq: %v", err)
	}
}

func TestZeroDepositConfirmsImmediately(t *testing.T) {
	f := newFix(t)
	ctx := context.Background()
	if _, err := f.svc.SaveSettings(ctx, Settings{RestaurantID: rid, Enabled: true, SlotMinutes: 90,
		MinAdvanceMinutes: 0, MaxDaysAhead: 7}); err != nil {
		t.Fatal(err)
	}
	r := f.create(t, "c1", 2, evening, MethodNone).Reservation
	if r.Status != StatusConfirmed || r.DepositState != DepositNone || r.EndsAt.Sub(r.StartsAt) != 90*time.Minute {
		t.Fatalf("zakladsiz: %+v", r)
	}
}
