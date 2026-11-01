package qurtorders_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"chustapp/internal/qurt"
	"chustapp/internal/qurtorders"
	"chustapp/internal/storage"
)

type fixture struct {
	svc      *qurtorders.Service
	products *storage.MemoryQurtRepo
	events   []string
	a, b     *qurt.Product // a: do'kon S1, b: do'kon S2
}

func newFixture(t *testing.T, stockA, stockB int) *fixture {
	t.Helper()
	f := &fixture{products: storage.NewMemoryQurtRepo()}
	put := func(store, ext string, price int64, stock int) *qurt.Product {
		p, err := f.products.Upsert(context.Background(), &qurt.Product{
			StoreID: store, ExternalID: ext, Name: "Qurt " + ext, Category: "Sho'r",
			PriceTiyin: price, WeightGrams: 200, Available: true, Stock: stock, StockSet: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	f.a = put("S1", "a", 1_000_000, stockA)
	f.b = put("S2", "b", 2_000_000, stockB)
	n := 0
	f.svc = &qurtorders.Service{
		Repo:     storage.NewMemoryQurtOrderRepo(),
		Products: f.products,
		NewID:    func() string { n++; return "id" + strconv.Itoa(1000+n) },
		Now:      func() time.Time { return time.Date(2026, 11, 2, 10, 0, 0, 0, qurtorders.Tashkent) }, // dushanba
		OnEvent:  func(_ context.Context, e string, _ *qurtorders.Order) { f.events = append(f.events, e) },
	}
	return f
}

func (f *fixture) input(lines ...qurtorders.Line) qurtorders.CreateInput {
	return qurtorders.CreateInput{
		CustomerID: "C1", CustomerName: "Ali", CustomerPhone: "+998901112233",
		Address:      qurtorders.Address{Text: "Chust, Navoiy 1"},
		Lines:        lines,
		ActiveStores: map[string]string{"S1": "Birinchi", "S2": "Ikkinchi"},
	}
}

func (f *fixture) left(t *testing.T, id string) int {
	t.Helper()
	got, _ := f.products.GetMany(context.Background(), []string{id})
	if len(got) != 1 {
		t.Fatalf("mahsulot yo'q: %s", id)
	}
	return got[0].Left()
}

func TestCreateSplitsByStoreAndReserves(t *testing.T) {
	f := newFixture(t, 10, 10)
	out, err := f.svc.Create(context.Background(), f.input(
		qurtorders.Line{ProductID: f.a.ID, Qty: 3}, qurtorders.Line{ProductID: f.b.ID, Qty: 2},
		qurtorders.Line{ProductID: f.a.ID, Qty: 1})) // takror — birlashadi
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0].GroupID != out[1].GroupID || out[0].StoreID == out[1].StoreID {
		t.Fatalf("ikki do'konga bo'linishi kerak edi: %+v", out)
	}
	for _, o := range out {
		switch o.StoreID {
		case "S1":
			if o.Items[0].Qty != 4 || o.TotalTiyin != 4_000_000 {
				t.Errorf("S1: %+v", o)
			}
		case "S2":
			if o.TotalTiyin != 4_000_000 {
				t.Errorf("S2 jami: %d", o.TotalTiyin)
			}
		}
		if o.Status != qurtorders.StatusNew || o.PaymentMethod != "cash" || o.DeliveryDate != "2026-11-03" {
			t.Errorf("boshlang'ich holat/reys: %+v", o)
		}
	}
	if f.left(t, f.a.ID) != 6 || f.left(t, f.b.ID) != 8 {
		t.Fatalf("qoldiq band qilinmadi: a=%d b=%d", f.left(t, f.a.ID), f.left(t, f.b.ID))
	}
}

func TestCreateInsufficientStockCreatesNothing(t *testing.T) {
	f := newFixture(t, 5, 1)
	_, err := f.svc.Create(context.Background(), f.input(
		qurtorders.Line{ProductID: f.a.ID, Qty: 2}, qurtorders.Line{ProductID: f.b.ID, Qty: 2}))
	var se *qurtorders.StockError
	if !errors.As(err, &se) || se.Name != "Qurt b" {
		t.Fatalf("StockError kutilgan: %v", err)
	}
	if f.left(t, f.a.ID) != 5 || f.left(t, f.b.ID) != 1 {
		t.Fatal("yarim band qolib ketdi")
	}
	list, _ := f.svc.Repo.List(context.Background(), qurtorders.Filter{})
	if len(list) != 0 {
		t.Fatalf("buyurtma yaratilmasligi kerak: %d", len(list))
	}
}

func TestCreateValidation(t *testing.T) {
	f := newFixture(t, 5, 5)
	ctx := context.Background()
	bad := map[string]qurtorders.CreateInput{
		"bo'sh savat": f.input(),
		"nol miqdor":  f.input(qurtorders.Line{ProductID: f.a.ID, Qty: 0}),
		"katta":       f.input(qurtorders.Line{ProductID: f.a.ID, Qty: 101}),
		"noma'lum":    f.input(qurtorders.Line{ProductID: "yoq", Qty: 1}),
	}
	noAddr := f.input(qurtorders.Line{ProductID: f.a.ID, Qty: 1})
	noAddr.Address.Text = " "
	bad["manzilsiz"] = noAddr
	inactive := f.input(qurtorders.Line{ProductID: f.a.ID, Qty: 1})
	inactive.ActiveStores = map[string]string{"S2": "x"}
	bad["do'kon qurt emas"] = inactive
	for name, in := range bad {
		if _, err := f.svc.Create(ctx, in); err == nil {
			t.Errorf("%s: rad etilishi kerak edi", name)
		}
	}
	if f.left(t, f.a.ID) != 5 {
		t.Fatal("rad etilgan buyurtma qoldiqni band qildi")
	}
}

func TestCreateIdempotent(t *testing.T) {
	f := newFixture(t, 10, 10)
	in := f.input(qurtorders.Line{ProductID: f.a.ID, Qty: 2})
	in.IdempotencyKey = "k1"
	first, err := f.svc.Create(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.svc.Create(context.Background(), in)
	if err != nil || len(second) != 1 || second[0].ID != first[0].ID {
		t.Fatalf("takroriy so'rov avvalgisini qaytarishi kerak: %v %+v", err, second)
	}
	if f.left(t, f.a.ID) != 8 {
		t.Fatalf("ikki marta band qilindi: %d", f.left(t, f.a.ID))
	}
}

func TestLifecycleAndReservationRelease(t *testing.T) {
	f := newFixture(t, 10, 10)
	ctx := context.Background()
	out, _ := f.svc.Create(ctx, f.input(qurtorders.Line{ProductID: f.a.ID, Qty: 4}))
	id := out[0].ID
	step := func(to qurtorders.Status, by qurtorders.Actor, reason string) error {
		_, err := f.svc.Transition(ctx, id, to, by, "u", reason, nil)
		return err
	}

	// Noto'g'ri aktor / o'tish rad etiladi.
	var te *qurtorders.TransitionError
	if err := step(qurtorders.StatusConfirmed, qurtorders.ActorCustomer, ""); !errors.As(err, &te) {
		t.Fatalf("mijoz tasdiqlay olmaydi: %v", err)
	}
	if err := step(qurtorders.StatusCollected, qurtorders.ActorLogistics, ""); !errors.As(err, &te) {
		t.Fatalf("new -> collected mumkin emas: %v", err)
	}
	if f.left(t, f.a.ID) != 6 {
		t.Fatal("rad etilgan o'tish bandga tegdi")
	}

	// Do'kon tasdiqlaydi: band qo'yib yuboriladi (qoldiqni POS kamaytiradi).
	if err := step(qurtorders.StatusConfirmed, qurtorders.ActorStore, ""); err != nil {
		t.Fatal(err)
	}
	if f.left(t, f.a.ID) != 10 {
		t.Fatalf("tasdiqlangach band qaytishi kerak: %d", f.left(t, f.a.ID))
	}
	// Mijoz tasdiqlangandan keyin bekor qila olmaydi.
	if err := step(qurtorders.StatusCancelled, qurtorders.ActorCustomer, ""); !errors.As(err, &te) {
		t.Fatalf("tasdiqlangan buyurtmani mijoz bekor qila olmaydi: %v", err)
	}
	for _, s := range []struct {
		to qurtorders.Status
		by qurtorders.Actor
	}{
		{qurtorders.StatusPacked, qurtorders.ActorStore},
		{qurtorders.StatusCollected, qurtorders.ActorLogistics},
		{qurtorders.StatusOutForDelivery, qurtorders.ActorLogistics},
	} {
		if err := step(s.to, s.by, ""); err != nil {
			t.Fatalf("%s: %v", s.to, err)
		}
	}
	// Yetkazilmadi — sabab shart; keyin qayta urinish va yetkazildi.
	if err := step(qurtorders.StatusFailed, qurtorders.ActorLogistics, ""); err == nil {
		t.Fatal("sababsiz failed rad etilishi kerak")
	}
	if err := step(qurtorders.StatusFailed, qurtorders.ActorLogistics, "uyda yo'q"); err != nil {
		t.Fatal(err)
	}
	if err := step(qurtorders.StatusOutForDelivery, qurtorders.ActorLogistics, ""); err != nil {
		t.Fatal(err)
	}
	if err := step(qurtorders.StatusDelivered, qurtorders.ActorLogistics, ""); err != nil {
		t.Fatal(err)
	}
	// Terminal holatdan chiqib bo'lmaydi.
	if err := step(qurtorders.StatusCancelled, qurtorders.ActorAdmin, "xato"); !errors.As(err, &te) {
		t.Fatalf("terminaldan chiqish: %v", err)
	}
	got, _ := f.svc.Repo.Get(ctx, id)
	if got.Version != 8 || len(got.History) != 8 {
		t.Fatalf("tarix/versiya: v=%d h=%d", got.Version, len(got.History))
	}
}

func TestRejectAndCustomerCancelReleaseStock(t *testing.T) {
	f := newFixture(t, 10, 10)
	ctx := context.Background()
	o1, _ := f.svc.Create(ctx, f.input(qurtorders.Line{ProductID: f.a.ID, Qty: 3}))
	o2, _ := f.svc.Create(ctx, f.input(qurtorders.Line{ProductID: f.a.ID, Qty: 2}))
	if f.left(t, f.a.ID) != 5 {
		t.Fatalf("band: %d", f.left(t, f.a.ID))
	}
	if _, err := f.svc.Transition(ctx, o1[0].ID, qurtorders.StatusRejected, qurtorders.ActorStore, "s", "", nil); err == nil {
		t.Fatal("rad etish sababsiz bo'lmaydi")
	}
	if _, err := f.svc.Transition(ctx, o1[0].ID, qurtorders.StatusRejected, qurtorders.ActorStore, "s", "tugagan", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Transition(ctx, o2[0].ID, qurtorders.StatusCancelled, qurtorders.ActorCustomer, "C1", "", nil); err != nil {
		t.Fatal(err)
	}
	if f.left(t, f.a.ID) != 10 {
		t.Fatalf("band qaytishi kerak: %d", f.left(t, f.a.ID))
	}
	// Ikki marta bekor qilib bo'lmaydi => band 0 dan pastga tushmaydi.
	_, _ = f.svc.Transition(ctx, o2[0].ID, qurtorders.StatusCancelled, qurtorders.ActorCustomer, "C1", "", nil)
	if f.left(t, f.a.ID) != 10 {
		t.Fatal("takroriy bekor bandni buzdi")
	}
}

func TestTransitionOwnershipCheck(t *testing.T) {
	f := newFixture(t, 10, 10)
	ctx := context.Background()
	out, _ := f.svc.Create(ctx, f.input(qurtorders.Line{ProductID: f.a.ID, Qty: 1}))
	deny := func(*qurtorders.Order) error { return qurtorders.ErrNotFound }
	if _, err := f.svc.Transition(ctx, out[0].ID, qurtorders.StatusConfirmed, qurtorders.ActorStore, "s", "", deny); !errors.Is(err, qurtorders.ErrNotFound) {
		t.Fatalf("egalik tekshiruvi: %v", err)
	}
	got, _ := f.svc.Repo.Get(ctx, out[0].ID)
	if got.Status != qurtorders.StatusNew {
		t.Fatal("rad etilgan tekshiruvda holat o'zgardi")
	}
}

func TestTransitionDate(t *testing.T) {
	f := newFixture(t, 10, 10)
	ctx := context.Background()
	out, _ := f.svc.Create(ctx, f.input(qurtorders.Line{ProductID: f.a.ID, Qty: 1}, qurtorders.Line{ProductID: f.b.ID, Qty: 1}))
	for _, o := range out {
		_, _ = f.svc.Transition(ctx, o.ID, qurtorders.StatusConfirmed, qurtorders.ActorStore, "s", "", nil)
		_, _ = f.svc.Transition(ctx, o.ID, qurtorders.StatusPacked, qurtorders.ActorStore, "s", "", nil)
	}
	done, skipped, err := f.svc.TransitionDate(ctx, "2026-11-03", qurtorders.StatusPacked, qurtorders.StatusCollected, qurtorders.ActorLogistics, "adm")
	if err != nil || done != 2 || skipped != 0 {
		t.Fatalf("done=%d skipped=%d err=%v", done, skipped, err)
	}
	// Boshqa sana — hech narsa.
	done, _, _ = f.svc.TransitionDate(ctx, "2026-11-06", qurtorders.StatusCollected, qurtorders.StatusOutForDelivery, qurtorders.ActorLogistics, "adm")
	if done != 0 {
		t.Fatalf("boshqa sana tegilmasligi kerak: %d", done)
	}
}

func TestScheduleNextDelivery(t *testing.T) {
	s := qurtorders.DefaultSchedule() // seshanba, juma; 18:00 gacha (bir kun oldin)
	at := func(y int, m time.Month, d, h int) time.Time {
		return time.Date(y, m, d, h, 0, 0, 0, qurtorders.Tashkent)
	}
	cases := []struct {
		now  time.Time
		want string
	}{
		{at(2026, 11, 2, 10), "2026-11-03"}, // dushanba 10:00 -> seshanba (muddat dushanba 18:00 emas — bugun 17:00? yo'q: 2-noyabr kuni o'tmagan)
		{at(2026, 11, 2, 19), "2026-11-06"}, // dushanba 19:00 -> muddat o'tdi -> juma
		{at(2026, 11, 3, 9), "2026-11-06"},  // seshanba -> juma
		{at(2026, 11, 5, 17), "2026-11-06"}, // payshanba 17:00 -> juma (muddat 18:00)
		{at(2026, 11, 5, 18), "2026-11-10"}, // payshanba 18:00 -> keyingi seshanba
		{at(2026, 11, 7, 12), "2026-11-10"}, // shanba -> seshanba
	}
	for _, c := range cases {
		if got, _ := s.NextDelivery(c.now); got != c.want {
			t.Errorf("%s -> %s, kutilgan %s", c.now.Format("Mon 2006-01-02 15:04"), got, c.want)
		}
	}
	// Bo'sh sozlama — ertangi kun, buyurtma yo'qolmaydi.
	if got, _ := (qurtorders.Schedule{}).NextDelivery(at(2026, 11, 2, 10)); got != "2026-11-03" {
		t.Errorf("bo'sh jadval: %s", got)
	}
}
