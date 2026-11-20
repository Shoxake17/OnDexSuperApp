package payments

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeDeposits struct {
	amount int64
	paid   []string
	failed []string
}

func (f *fakeDeposits) DepositAmountTiyin(context.Context, string) (int64, error) {
	return f.amount, nil
}
func (f *fakeDeposits) OnDepositPaid(_ context.Context, id string) error {
	f.paid = append(f.paid, id)
	return nil
}
func (f *fakeDeposits) OnDepositFailed(_ context.Context, id string) error {
	f.failed = append(f.failed, id)
	return nil
}

// RS-3: zaklad — darhol yechiladigan to'lov (hold emas), callback bronni
// tasdiqlaydi, buyurtma tomoniga TEGILMAYDI; erta bekor qilish = refund.
func TestReservationDepositFlow(t *testing.T) {
	ctx := context.Background()
	prov := &fakeProvider{}
	ord := &fakeOrders{amount: 999}
	svc, repo := newTestService(t, prov, ord, Options{})

	if _, err := svc.StartForReservation(ctx, "rsv1", "u1", "r1", "zaklad", ""); !errors.Is(err, ErrDepositsDisabled) {
		t.Fatalf("sink'siz: %v", err)
	}
	dep := &fakeDeposits{amount: 20000000}
	svc.SetDepositSink(dep)

	p, err := svc.StartForReservation(ctx, "rsv1", "u1", "r1", "zaklad", "")
	if err != nil {
		t.Fatal(err)
	}
	if p.ReservationID != "rsv1" || p.OrderID != "" || p.AmountTiyin != 20000000 || p.PayURL == "" {
		t.Fatalf("to'lov: %+v", p)
	}
	// Takroriy chaqiriq — o'sha havola.
	p2, err := svc.StartForReservation(ctx, "rsv1", "u1", "r1", "zaklad", "")
	if err != nil || p2.ID != p.ID {
		t.Fatalf("takror: %v %v", p2, err)
	}

	// Imzoli callback: to'landi -> bron tasdiqlanadi, buyurtma tomoniga xabar YO'Q.
	if err := svc.HandleCallback(ctx, CallbackData{PaymentID: p.ID, ProviderPaymentID: "prov-" + p.ID,
		Status: StatusPaid, AmountTiyin: 20000000, SignatureChecked: true, SignatureValid: true}); err != nil {
		t.Fatal(err)
	}
	if len(dep.paid) != 1 || dep.paid[0] != "rsv1" || ord.held != 0 {
		t.Fatalf("paid=%v orders.held=%d", dep.paid, ord.held)
	}
	if _, err := svc.StartForReservation(ctx, "rsv1", "u1", "r1", "zaklad", ""); !errors.Is(err, ErrAlreadyPaid) {
		t.Fatalf("to'langandan keyin: %v", err)
	}

	// Erta bekor qilish: to'liq qaytariladi, ikkinchi chaqiriq hech narsa qilmaydi.
	if err := svc.RefundReservation(ctx, "rsv1"); err != nil {
		t.Fatal(err)
	}
	if err := svc.RefundReservation(ctx, "rsv1"); err != nil {
		t.Fatal(err)
	}
	if prov.refunded != 1 {
		t.Fatalf("refund soni: %d", prov.refunded)
	}
	got, _ := repo.GetByID(ctx, p.ID)
	if got.Status != StatusRefunded {
		t.Fatalf("holat: %s", got.Status)
	}
}

func TestReservationDepositExpiredNotifiesReservation(t *testing.T) {
	ctx := context.Background()
	prov := &fakeProvider{status: StatusPending}
	ord := &fakeOrders{}
	svc, _ := newTestService(t, prov, ord, Options{})
	dep := &fakeDeposits{amount: 5000000}
	svc.SetDepositSink(dep)
	if _, err := svc.StartForReservation(ctx, "rsv2", "u1", "r1", "zaklad", ""); err != nil {
		t.Fatal(err)
	}
	svc.now = func() time.Time { return time.Now().Add(16 * time.Minute) }
	n, err := svc.ExpireStale(ctx, 10)
	if err != nil || n != 1 {
		t.Fatalf("expire: %d %v", n, err)
	}
	if len(dep.failed) != 1 || dep.failed[0] != "rsv2" || ord.failed != 0 {
		t.Fatalf("failed=%v orders.failed=%d", dep.failed, ord.failed)
	}
}
