package qurtorders_test

import (
	"context"
	"testing"

	"chustapp/internal/qurtorders"
)

// packedOrder — S1 do'konidan `packed` holatigacha olib borilgan buyurtma.
func (f *fixture) packedOrder(t *testing.T) *qurtorders.Order {
	t.Helper()
	ctx := context.Background()
	out, err := f.svc.Create(ctx, f.input(qurtorders.Line{ProductID: f.a.ID, Qty: 2}))
	if err != nil {
		t.Fatal(err)
	}
	o := out[0]
	for _, to := range []qurtorders.Status{qurtorders.StatusConfirmed, qurtorders.StatusPacked} {
		if _, err := f.svc.Transition(ctx, o.ID, to, qurtorders.ActorStore, "S1", "", nil); err != nil {
			t.Fatal(err)
		}
	}
	return o
}

func (f *fixture) courierStep(t *testing.T, id string, to qurtorders.Status, courier, reason string) error {
	t.Helper()
	_, err := f.svc.Transition(context.Background(), id, to, qurtorders.ActorLogistics, courier, reason, nil)
	return err
}

func TestAssignByDateOnlyAssignableAndUnassigned(t *testing.T) {
	f := newFixture(t, 20, 20)
	ctx := context.Background()
	packed := f.packedOrder(t)
	fresh, err := f.svc.Create(ctx, f.input(qurtorders.Line{ProductID: f.b.ID, Qty: 1})) // hali `new`
	if err != nil {
		t.Fatal(err)
	}

	res, err := f.svc.Assign(ctx, "S1", "k1", packed.DeliveryDate, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Assigned != 1 {
		t.Fatalf("faqat qabul qilingan buyurtma biriktirilishi kerak: %+v", res)
	}
	got, _ := f.svc.Repo.Get(ctx, packed.ID)
	if got.CourierID != "k1" || got.CourierAssignedAt == nil {
		t.Fatalf("kuryer yozilmadi: %+v", got)
	}
	if n, _ := f.svc.Repo.Get(ctx, fresh[0].ID); n.CourierID != "" {
		t.Fatal("`new` buyurtma biriktirilmasligi kerak (do'kon hali ko'rmagan)")
	}

	// Boshqa kuryerga qayta biriktirilmaydi.
	res, _ = f.svc.Assign(ctx, "S1", "k2", packed.DeliveryDate, nil)
	if res.Assigned != 0 {
		t.Fatalf("biriktirilgan buyurtma qayta olinmasligi kerak: %+v", res)
	}
	res, _ = f.svc.Assign(ctx, "S1", "k2", "", []string{packed.ID})
	if res.Assigned != 0 || res.Skipped != 1 {
		t.Fatalf("aniq ro'yxatda ham o'tkazib yuboriladi: %+v", res)
	}
	if _, err := f.svc.Assign(ctx, "S1", "", packed.DeliveryDate, nil); err == nil {
		t.Fatal("kuryersiz biriktirish rad etilishi kerak")
	}
	if _, err := f.svc.Assign(ctx, "S1", "k1", "", nil); err == nil {
		t.Fatal("sana ham, ro'yxat ham yo'q — rad etilishi kerak")
	}
}

func TestDeliveryRecordsCashHeldThenSettleOnce(t *testing.T) {
	f := newFixture(t, 20, 20)
	ctx := context.Background()
	o := f.packedOrder(t) // 2 dona * 10 000 so'm = 20 000 so'm
	if _, err := f.svc.Assign(ctx, "S1", "k1", "", []string{o.ID}); err != nil {
		t.Fatal(err)
	}
	for _, to := range []qurtorders.Status{qurtorders.StatusCollected, qurtorders.StatusOutForDelivery, qurtorders.StatusDelivered} {
		if err := f.courierStep(t, o.ID, to, "k1", ""); err != nil {
			t.Fatalf("%s: %v", to, err)
		}
	}
	got, _ := f.svc.Repo.Get(ctx, o.ID)
	if got.CashStatus != qurtorders.CashHeld || got.CashTiyin != got.TotalTiyin || got.CashTiyin != 2_000_000 {
		t.Fatalf("pul kuryerda yozilishi kerak: %+v", got)
	}
	sum, _ := f.svc.CourierSummary(ctx, "k1")
	if sum.HeldTiyin != 2_000_000 || sum.HeldOrders != 1 || sum.ActiveCount != 0 {
		t.Fatalf("hisob: %+v", sum)
	}

	// Boshqa kuryer pulini qabul qilib bo'lmaydi.
	if r, _ := f.svc.Settle(ctx, "k2", "admin1", nil); r.Orders != 0 {
		t.Fatalf("begona kuryer puli: %+v", r)
	}
	r, err := f.svc.Settle(ctx, "k1", "admin1", nil)
	if err != nil || r.Orders != 1 || r.TotalTiyin != 2_000_000 {
		t.Fatalf("qabul: %+v %v", r, err)
	}
	got, _ = f.svc.Repo.Get(ctx, o.ID)
	if got.CashStatus != qurtorders.CashSettled || got.CashSettledBy != "admin1" || got.CashSettledAt == nil {
		t.Fatalf("settled yozilmadi: %+v", got)
	}
	// Ikkinchi marta qabul qilinsa pul ikki marta sanalmaydi.
	if r, _ := f.svc.Settle(ctx, "k1", "admin1", nil); r.Orders != 0 || r.TotalTiyin != 0 {
		t.Fatalf("takroriy qabul: %+v", r)
	}
	if sum, _ := f.svc.CourierSummary(ctx, "k1"); sum.HeldTiyin != 0 {
		t.Fatalf("pul qolmasligi kerak: %+v", sum)
	}
}

func TestSettleSelectedOrdersOnly(t *testing.T) {
	f := newFixture(t, 20, 20)
	ctx := context.Background()
	var ids []string
	for i := 0; i < 2; i++ {
		o := f.packedOrder(t)
		ids = append(ids, o.ID)
		if _, err := f.svc.Assign(ctx, "S1", "k1", "", []string{o.ID}); err != nil {
			t.Fatal(err)
		}
		for _, to := range []qurtorders.Status{qurtorders.StatusCollected, qurtorders.StatusOutForDelivery, qurtorders.StatusDelivered} {
			if err := f.courierStep(t, o.ID, to, "k1", ""); err != nil {
				t.Fatal(err)
			}
		}
	}
	r, _ := f.svc.Settle(ctx, "k1", "admin1", []string{ids[0]})
	if r.Orders != 1 || r.StillHeld != 1 {
		t.Fatalf("faqat tanlangani: %+v", r)
	}
	if o, _ := f.svc.Repo.Get(ctx, ids[1]); o.CashStatus != qurtorders.CashHeld {
		t.Fatal("tanlanmagan buyurtma pulida o'zgarish bo'lmasligi kerak")
	}
}

func TestDeliveredWithoutCourierRecordsNoCash(t *testing.T) {
	f := newFixture(t, 20, 20)
	o := f.packedOrder(t)
	for _, to := range []qurtorders.Status{qurtorders.StatusCollected, qurtorders.StatusOutForDelivery, qurtorders.StatusDelivered} {
		if err := f.courierStep(t, o.ID, to, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := f.svc.Repo.Get(context.Background(), o.ID)
	if got.CashStatus != "" || got.CashTiyin != 0 {
		t.Fatalf("kuryersiz yetkazishda pul 'kuryerda' deb yozilmasligi kerak: %+v", got)
	}
}

func TestFailedNeedsReasonAndRetryKeepsCourier(t *testing.T) {
	f := newFixture(t, 20, 20)
	ctx := context.Background()
	o := f.packedOrder(t)
	_, _ = f.svc.Assign(ctx, "S1", "k1", "", []string{o.ID})
	for _, to := range []qurtorders.Status{qurtorders.StatusCollected, qurtorders.StatusOutForDelivery} {
		if err := f.courierStep(t, o.ID, to, "k1", ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.courierStep(t, o.ID, qurtorders.StatusFailed, "k1", ""); err == nil {
		t.Fatal("sababsiz 'yetkazib bo'lmadi' rad etilishi kerak")
	}
	if err := f.courierStep(t, o.ID, qurtorders.StatusFailed, "k1", "Mijoz telefonni ko'tarmadi"); err != nil {
		t.Fatal(err)
	}
	if err := f.courierStep(t, o.ID, qurtorders.StatusOutForDelivery, "k1", ""); err != nil {
		t.Fatalf("qayta urinish: %v", err)
	}
	if got, _ := f.svc.Repo.Get(ctx, o.ID); got.CourierID != "k1" || got.CashStatus != "" {
		t.Fatalf("kuryer saqlanadi, pul hali yo'q: %+v", got)
	}
}

func TestUnassignOnlyBeforeDeparture(t *testing.T) {
	f := newFixture(t, 20, 20)
	ctx := context.Background()
	o := f.packedOrder(t)
	_, _ = f.svc.Assign(ctx, "S1", "k1", "", []string{o.ID})
	if err := f.svc.Unassign(ctx, o.ID, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.svc.Repo.Get(ctx, o.ID); got.CourierID != "" {
		t.Fatal("kuryer olinmadi")
	}
	_, _ = f.svc.Assign(ctx, "S1", "k1", "", []string{o.ID})
	_ = f.courierStep(t, o.ID, qurtorders.StatusCollected, "k1", "")
	_ = f.courierStep(t, o.ID, qurtorders.StatusOutForDelivery, "k1", "")
	if err := f.svc.Unassign(ctx, o.ID, nil); err == nil {
		t.Fatal("yo'ldagi buyurtmani kuryerdan olib bo'lmaydi")
	}
}

func TestAssignIgnoresOtherStoresOrders(t *testing.T) {
	f := newFixture(t, 20, 20)
	ctx := context.Background()
	o := f.packedOrder(t) // S1
	if r, _ := f.svc.Assign(ctx, "S2", "k1", o.DeliveryDate, nil); r.Assigned != 0 {
		t.Fatalf("begona do'kon buyurtmasi biriktirildi: %+v", r)
	}
	if r, _ := f.svc.Assign(ctx, "S2", "k1", "", []string{o.ID}); r.Assigned != 0 || r.Skipped != 1 {
		t.Fatalf("aniq ro'yxatda ham begona do'kon o'tkazib yuboriladi: %+v", r)
	}
	check := func(*qurtorders.Order) error { return qurtorders.ErrNotFound }
	_, _ = f.svc.Assign(ctx, "S1", "k1", "", []string{o.ID})
	if err := f.svc.Unassign(ctx, o.ID, check); err == nil {
		t.Fatal("egalik tekshiruvi o'tmasa kuryerdan olib bo'lmaydi")
	}
}
