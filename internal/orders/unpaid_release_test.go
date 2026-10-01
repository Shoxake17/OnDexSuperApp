package orders

import (
	"context"
	"testing"
)

// ★ To'lanmagan karta buyurtmasini mijoz bekor qilsa, to'lov provayderiga "yop"
// deb aytilishi shart: aks holda mijoz to'lov havolasini keyin to'lab qo'yadi,
// pul yechiladi, buyurtma esa allaqachon yo'q. (Multicard sandbox'dagi haqiqiy
// sinov va E2E ochgan bo'shliq.) Restoran to'lanmagan buyurtmani rad eta olmaydi
// (`TestUnpaidOrderCannotTransition`), shuning uchun faqat mijoz bekor qilishi.
func TestUnpaidCardOrderCancelClosesPayment(t *testing.T) {
	svc, _, g := newCardOrderService(t)
	o, err := svc.Create(context.Background(), cardOrder())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ChangeStatus(context.Background(), o.ID, StatusCancelled, ActorCustomer); err != nil {
		t.Fatal(err)
	}
	if g.released != 1 {
		t.Errorf("to'lanmagan to'lov yopilmadi: ReleaseForOrder %d marta chaqirildi, 1 kutilgan", g.released)
	}
}

// Naqd buyurtma bekor qilinsa to'lov tizimiga umuman murojaat qilinmaydi.
func TestCashOrderCancelDoesNotTouchPayments(t *testing.T) {
	svc, _, g := newCardOrderService(t)
	o := cardOrder()
	o.PaymentMethod = PaymentCash
	o.PaymentState = ""
	created, err := svc.Create(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ChangeStatus(context.Background(), created.ID, StatusCancelled, ActorCustomer); err != nil {
		t.Fatal(err)
	}
	if g.released != 0 {
		t.Errorf("naqd buyurtma uchun to'lov tizimiga murojaat qilindi: %d", g.released)
	}
}
