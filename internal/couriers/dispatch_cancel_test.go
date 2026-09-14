package couriers

import (
	"context"
	"errors"
	"testing"
	"time"
)

func waitRunning(t *testing.T, d *Dispatcher, orderID string, want bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if d.IsRunning(orderID) == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("IsRunning(%s) %v bo'lmadi", orderID, want)
}

// Cancel qidiruvni taklif muddatini KUTMASDAN to'xtatadi va kuryerdagi
// ochiq taklifni yopadi — "kuryer topilmadi" yoki bekor qilingan
// buyurtma kuryer ekranida osilib qolmasligi kerak.
func TestDispatchCancelStopsSearchAndClosesOffers(t *testing.T) {
	repo := newFakeRepo("c1")
	n := &fakeNotifier{}
	// Uzoq TTL: Cancel ishlamasa test 5 soniya kutib yiqiladi.
	d := NewDispatcher(repo, n, newFakeGeoClient(nil), 5*time.Second)

	done := make(chan error, 1)
	go func() {
		_, err := d.Dispatch(context.Background(), "o1", DispatchParams{})
		done <- err
	}()
	waitOffered(t, n, "c1")
	if !d.IsRunning("o1") {
		t.Fatal("qidiruv ishlayotgan ko'rinmadi")
	}
	if !d.Cancel("o1") {
		t.Fatal("Cancel ishlayotgan qidiruvni topmadi")
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("context.Canceled kutilgan edi, olindi: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Cancel qidiruvni to'xtatmadi")
	}
	if !n.wasCancelled("c1") {
		t.Error("kuryerdagi ochiq taklif yopilmadi")
	}
	if d.IsRunning("o1") {
		t.Error("to'xtatilgan qidiruv hamon ishlayotgan ko'rinadi")
	}
	if d.Cancel("o1") {
		t.Error("qidiruv yo'q bo'lganda Cancel false qaytarishi kerak")
	}
	if d.HandleResponse("o1", Response{CourierID: "c1", Accepted: true}) {
		t.Error("to'xtatilgan qidiruvga kechikkan javob qabul qilindi")
	}
}

// Restoran "Kuryer qidirish" bosganda yangi qidiruv DARHOL boshlanadi:
// ErrAlreadyRunning yo'q va eski goroutine chiqib ketayotib yangisining
// yozuvini o'chirib yubormaydi.
func TestDispatchRestartRightAfterCancel(t *testing.T) {
	d := NewDispatcher(newFakeRepo(), &fakeNotifier{}, newFakeGeoClient(nil), 20*time.Millisecond)

	first := make(chan error, 1)
	go func() {
		_, err := d.Dispatch(context.Background(), "o1", DispatchParams{})
		first <- err
	}()
	waitRunning(t, d, "o1", true)
	d.Cancel("o1")

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	second := make(chan error, 1)
	go func() {
		_, err := d.Dispatch(ctx2, "o1", DispatchParams{})
		second <- err
	}()

	select {
	case err := <-first:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("birinchi qidiruv: context.Canceled kutilgan edi, olindi: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("birinchi qidiruv to'xtamadi")
	}
	waitRunning(t, d, "o1", true)
	time.Sleep(60 * time.Millisecond)
	if !d.IsRunning("o1") {
		t.Fatal("eski goroutine yangi qidiruv yozuvini o'chirib yubordi")
	}

	cancel2()
	select {
	case err := <-second:
		if errors.Is(err, ErrAlreadyRunning) {
			t.Fatal("yangi qidiruv ErrAlreadyRunning oldi")
		}
	case <-time.After(time.Second):
		t.Fatal("ikkinchi qidiruv to'xtamadi")
	}
}
