package orders

import (
	"context"
	"strings"
	"sync"
	"testing"
)

// Karta orqali to'lov: buyurtma pul BLOKLANMAGUNCHA oshxonaga
// tushmasligi kerak. Bu — pul yo'lidagi eng muhim kafolat, shuning
// uchun u har tomondan tekshiriladi.

// countingNotifier — restoranga nechta xabar ketganini sanaydi.
type countingNotifier struct {
	mu      sync.Mutex
	created int
	changed int
}

func (n *countingNotifier) OrderCreated(*Order) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.created++
}
func (n *countingNotifier) OrderStatusChanged(*Order, Status) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.changed++
}

// recordingGateway — to'lov provayderining soxta nusxasi.
type recordingGateway struct {
	captured []int64
	released int
}

func (g *recordingGateway) CaptureForOrder(_ context.Context, _ string, amount int64) error {
	g.captured = append(g.captured, amount)
	return nil
}
func (g *recordingGateway) ReleaseForOrder(_ context.Context, _ string) error {
	g.released++
	return nil
}

func newCardOrderService(t *testing.T) (*Service, *countingNotifier, *recordingGateway) {
	t.Helper()
	n := &countingNotifier{}
	g := &recordingGateway{}
	svc := NewService(newFakeOCCRepo(), n, func() string { return "order-1" }, nil)
	svc.WithPayments(g)
	return svc, n, g
}

func cardOrder() *Order {
	return &Order{
		CustomerID: "cust-1", RestaurantID: "rest-1",
		Items:         []Item{{ProductID: "p1", Name: "Osh", Qty: 1, PriceTiyin: 1200000}},
		PaymentMethod: PaymentCard,
		PaymentState:  PaymentAwaiting,
	}
}

// ★ Buyurtma yaratilganda restoranga XABAR KETMAYDI.
func TestCardOrderNotSentToRestaurantUntilPaid(t *testing.T) {
	svc, notifier, _ := newCardOrderService(t)

	o, err := svc.Create(context.Background(), cardOrder())
	if err != nil {
		t.Fatal(err)
	}
	if notifier.created != 0 {
		t.Error("to'lanmagan buyurtma restoranga YUBORILDI")
	}
	if !o.AwaitingPayment() {
		t.Error("buyurtma to'lov kutayotgan holatda bo'lishi kerak")
	}
}

// ★ To'lanmagan buyurtma HARAKATLANA OLMAYDI — hech bir aktor uni
// oldinga sura olmaydi.
func TestUnpaidOrderCannotTransition(t *testing.T) {
	svc, _, _ := newCardOrderService(t)
	o, err := svc.Create(context.Background(), cardOrder())
	if err != nil {
		t.Fatal(err)
	}

	for _, to := range []Status{StatusAccepted, StatusPreparing, StatusReady} {
		if _, err := svc.ChangeStatus(context.Background(), o.ID, to, ActorRestaurant); err == nil {
			t.Errorf("to'lanmagan buyurtma %q holatiga o'tdi", to)
		}
	}

	// Bekor qilish esa MUMKIN: mijoz fikridan qaytsa yoki to'lov
	// muddati tugasa, buyurtma yopilishi kerak.
	if _, err := svc.ChangeStatus(context.Background(), o.ID, StatusCancelled, ActorCustomer); err != nil {
		t.Errorf("bekor qilish mumkin bo'lishi kerak edi: %v", err)
	}
}

// Naqd buyurtma avvalgidek ishlaydi — to'lov tekshiruvi unga
// UMUMAN tegmaydi.
func TestCashOrderUnaffected(t *testing.T) {
	svc, notifier, _ := newCardOrderService(t)
	o := cardOrder()
	o.PaymentMethod = PaymentCash
	o.PaymentState = ""

	created, err := svc.Create(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	if notifier.created != 1 {
		t.Errorf("naqd buyurtma darhol restoranga yuborilishi kerak: %d", notifier.created)
	}
	if _, err := svc.ChangeStatus(context.Background(), created.ID, StatusAccepted, ActorRestaurant); err != nil {
		t.Errorf("naqd buyurtma qabul qilinishi kerak edi: %v", err)
	}
}

// Pul bloklangach buyurtma restoranga BIR MARTA yuboriladi va
// harakatlana boshlaydi.
func TestOrderReleasedAfterPaymentHeld(t *testing.T) {
	svc, notifier, _ := newCardOrderService(t)
	o, _ := svc.Create(context.Background(), cardOrder())

	// To'lov tizimi xabarni takrorlashi mumkin — bir marta ishlashi kerak.
	for i := 0; i < 3; i++ {
		if err := svc.OnPaymentHeld(context.Background(), o.ID); err != nil {
			t.Fatal(err)
		}
	}
	if notifier.created != 1 {
		t.Errorf("restoranga %d marta yuborildi, kutilgan 1", notifier.created)
	}
	if _, err := svc.ChangeStatus(context.Background(), o.ID, StatusAccepted, ActorRestaurant); err != nil {
		t.Errorf("to'langan buyurtma qabul qilinishi kerak edi: %v", err)
	}
}

// Restoran QABUL QILGANDA bloklangan pul yechiladi.
func TestCaptureOnAccept(t *testing.T) {
	svc, _, gw := newCardOrderService(t)
	o, _ := svc.Create(context.Background(), cardOrder())
	if err := svc.OnPaymentHeld(context.Background(), o.ID); err != nil {
		t.Fatal(err)
	}

	got, err := svc.ChangeStatus(context.Background(), o.ID, StatusAccepted, ActorRestaurant)
	if err != nil {
		t.Fatal(err)
	}
	if len(gw.captured) != 1 || gw.captured[0] != got.TotalTiyin {
		t.Errorf("yechilgan summa: %v, kutilgan [%d]", gw.captured, got.TotalTiyin)
	}
	if got.PaymentState != PaymentPaid {
		t.Errorf("to'lov holati: %s, kutilgan paid", got.PaymentState)
	}
}

// Restoran RAD ETGANDA blok bo'shatiladi (mijozdan pul yechilmaydi).
func TestReleaseOnReject(t *testing.T) {
	svc, _, gw := newCardOrderService(t)
	o, _ := svc.Create(context.Background(), cardOrder())
	if err := svc.OnPaymentHeld(context.Background(), o.ID); err != nil {
		t.Fatal(err)
	}

	got, err := svc.ChangeStatus(context.Background(), o.ID, StatusRejected, ActorRestaurant)
	if err != nil {
		t.Fatal(err)
	}
	if gw.released != 1 {
		t.Errorf("blok bo'shatilmadi: %d", gw.released)
	}
	if got.PaymentState != PaymentFailed {
		t.Errorf("to'lov holati: %s, kutilgan failed", got.PaymentState)
	}
}

// To'lov amalga oshmasa buyurtma bekor qilinadi.
func TestOnPaymentFailedCancelsOrder(t *testing.T) {
	svc, _, _ := newCardOrderService(t)
	o, _ := svc.Create(context.Background(), cardOrder())

	if err := svc.OnPaymentFailed(context.Background(), o.ID); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Get(context.Background(), o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusCancelled {
		t.Errorf("holat: %s, kutilgan cancelled", got.Status)
	}
	if got.PaymentState != PaymentFailed {
		t.Errorf("to'lov holati: %s, kutilgan failed", got.PaymentState)
	}
}

// To'lov summasi HAR DOIM buyurtmadan olinadi.
func TestOrderAmountComesFromOrder(t *testing.T) {
	svc, _, _ := newCardOrderService(t)
	o, _ := svc.Create(context.Background(), cardOrder())

	amount, err := svc.OrderAmountTiyin(context.Background(), o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if amount != o.TotalTiyin {
		t.Errorf("summa: %d, kutilgan %d", amount, o.TotalTiyin)
	}
}

// Xato matni to'lov holatini aytadi — mijoz nima kutayotganini
// tushunishi kerak.
func TestUnpaidTransitionErrorMentionsPayment(t *testing.T) {
	svc, _, _ := newCardOrderService(t)
	o, _ := svc.Create(context.Background(), cardOrder())

	_, err := svc.ChangeStatus(context.Background(), o.ID, StatusAccepted, ActorRestaurant)
	if err == nil || !strings.Contains(err.Error(), "to'lov") {
		t.Errorf("xato to'lov haqida bo'lishi kerak: %v", err)
	}
}
