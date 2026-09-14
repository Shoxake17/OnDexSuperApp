package orders

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// Kuryer qidiruvi holati testlari. Har biri invariantni BUZISHGA
// urinadi: muddatdan oldin "topilmadi", qidiruv davomida bekor qilish,
// qayta qidirishdan keyin avtomatik bekor qilish va h.k.

type dispatchClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *dispatchClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *dispatchClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func newDispatchFixture(t *testing.T, typ Type) (*Service, *fakeOCCRepo, *dispatchClock) {
	t.Helper()
	repo := newFakeOCCRepo()
	clock := &dispatchClock{t: time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)}
	svc := NewService(repo, nil, func() string { return "o1" }, nil).WithClock(clock.now)
	repo.data["o1"] = Order{ID: "o1", Type: typ, Status: StatusCreated,
		RestaurantID: "r1", CustomerID: "u1", Version: 1}
	return svc, repo, clock
}

func mustChange(t *testing.T, svc *Service, to Status, by Actor) *Order {
	t.Helper()
	o, err := svc.ChangeStatus(context.Background(), "o1", to, by)
	if err != nil {
		t.Fatalf("-> %s (%s): %v", to, by, err)
	}
	return o
}

func toReady(t *testing.T, svc *Service) *Order {
	t.Helper()
	mustChange(t, svc, StatusAccepted, ActorRestaurant)
	mustChange(t, svc, StatusPreparing, ActorRestaurant)
	return mustChange(t, svc, StatusReady, ActorRestaurant)
}

func isTransitionErr(err error) bool {
	var te *TransitionError
	return errors.As(err, &te)
}

func markNotFound(t *testing.T, svc *Service, clock *dispatchClock) *Order {
	t.Helper()
	clock.advance(CourierSearchWindow)
	o, changed, err := svc.MarkCourierNotFound(context.Background(), "o1")
	if err != nil || !changed {
		t.Fatalf("kuryer topilmadi holatiga o'tmadi: changed=%v err=%v", changed, err)
	}
	return o
}

// Qidiruv qabul qilinganda boshlanadi, muddat esa TAYYOR paytdan
// hisoblanadi — uzoq tayyorlash muddatni yeb qo'ymaydi.
func TestDispatchPolicy_DeadlineStartsWhenReady(t *testing.T) {
	svc, _, clock := newDispatchFixture(t, TypeDelivery)
	o := mustChange(t, svc, StatusAccepted, ActorRestaurant)
	if o.DispatchState != DispatchSearching || o.DispatchDeadline != nil {
		t.Fatalf("qabul qilingach: holat=%q muddat=%v", o.DispatchState, o.DispatchDeadline)
	}
	clock.advance(25 * time.Minute)
	mustChange(t, svc, StatusPreparing, ActorRestaurant)
	o = mustChange(t, svc, StatusReady, ActorRestaurant)
	want := clock.now().Add(CourierSearchWindow)
	if o.DispatchState != DispatchSearching || o.DispatchDeadline == nil || !o.DispatchDeadline.Equal(want) {
		t.Fatalf("tayyor bo'lgach: holat=%q muddat=%v, kutilgan %v", o.DispatchState, o.DispatchDeadline, want)
	}
}

func TestMarkCourierNotFound_OnlyAfterWindow(t *testing.T) {
	svc, _, clock := newDispatchFixture(t, TypeDelivery)
	toReady(t, svc)
	ctx := context.Background()

	clock.advance(CourierSearchWindow - time.Second)
	if _, changed, err := svc.MarkCourierNotFound(ctx, "o1"); err != nil || changed {
		t.Fatalf("muddatdan OLDIN topilmadi deb belgilandi: changed=%v err=%v", changed, err)
	}
	clock.advance(time.Second)
	o, changed, err := svc.MarkCourierNotFound(ctx, "o1")
	if err != nil || !changed {
		t.Fatalf("muddat tugaganda belgilanmadi: changed=%v err=%v", changed, err)
	}
	want := clock.now().Add(CourierNotFoundCancelAfter)
	if o.DispatchState != DispatchNotFound || o.DispatchDeadline == nil || !o.DispatchDeadline.Equal(want) {
		t.Fatalf("holat=%q muddat=%v, kutilgan %v", o.DispatchState, o.DispatchDeadline, want)
	}

	// Takroriy chaqiruv avtomatik bekor qilish muddatini CHO'ZMAYDI.
	clock.advance(time.Minute)
	if _, changed, _ := svc.MarkCourierNotFound(ctx, "o1"); changed {
		t.Fatal("takroriy chaqiruv holatni qayta yozdi")
	}
}

// Tayyor yetkazish buyurtmasini restoran/tizim FAQAT kuryer topilmaganda
// bekor qila oladi. Mijoz va kuryer umuman bekor qila olmaydi.
func TestCancelReadyDelivery_OnlyWhenCourierNotFound(t *testing.T) {
	svc, _, clock := newDispatchFixture(t, TypeDelivery)
	toReady(t, svc)
	ctx := context.Background()

	for _, by := range []Actor{ActorRestaurant, ActorSystem, ActorCustomer, ActorCourier, ActorWaiter} {
		if _, err := svc.ChangeStatus(ctx, "o1", StatusCancelled, by); !isTransitionErr(err) {
			t.Fatalf("qidiruv davomida %s bekor qila oldi (err=%v)", by, err)
		}
	}

	markNotFound(t, svc, clock)
	for _, by := range []Actor{ActorCustomer, ActorCourier, ActorWaiter} {
		if _, err := svc.ChangeStatus(ctx, "o1", StatusCancelled, by); !isTransitionErr(err) {
			t.Fatalf("kuryer topilmaganda %s bekor qila oldi (err=%v)", by, err)
		}
	}
	o := mustChange(t, svc, StatusCancelled, ActorRestaurant)
	if o.DispatchDeadline != nil {
		t.Fatal("bekor qilingan buyurtmada muddat qolib ketdi")
	}
}

// Admin cheklanmaydi — nizolarni u hal qiladi.
func TestAdminCanCancelReadyDeliveryWhileSearching(t *testing.T) {
	svc, _, _ := newDispatchFixture(t, TypeDelivery)
	toReady(t, svc)
	mustChange(t, svc, StatusCancelled, ActorAdmin)
}

func TestRestartCourierSearch(t *testing.T) {
	svc, _, clock := newDispatchFixture(t, TypeDelivery)
	toReady(t, svc)
	ctx := context.Background()

	if _, err := svc.RestartCourierSearch(ctx, "o1"); !errors.Is(err, ErrDispatchNotRestartable) {
		t.Fatalf("qidiruv davomida qayta boshlash rad etilishi kerak edi: %v", err)
	}
	markNotFound(t, svc, clock)
	clock.advance(7 * time.Minute)

	o, err := svc.RestartCourierSearch(ctx, "o1")
	if err != nil {
		t.Fatal(err)
	}
	want := clock.now().Add(CourierSearchWindow)
	if o.DispatchState != DispatchSearching || o.DispatchDeadline == nil || !o.DispatchDeadline.Equal(want) {
		t.Fatalf("holat=%q muddat=%v, kutilgan %v", o.DispatchState, o.DispatchDeadline, want)
	}
	// Qayta qidiruv boshlangach restoran yana kutishi kerak.
	if _, err := svc.ChangeStatus(ctx, "o1", StatusCancelled, ActorRestaurant); !isTransitionErr(err) {
		t.Fatalf("qayta qidiruv davomida bekor qilish rad etilishi kerak edi: %v", err)
	}
	if _, err := svc.RestartCourierSearch(ctx, "o1"); !errors.Is(err, ErrDispatchNotRestartable) {
		t.Fatalf("ikkinchi bosish rad etilishi kerak edi: %v", err)
	}
}

func TestAutoCancelCourierNotFound(t *testing.T) {
	svc, _, clock := newDispatchFixture(t, TypeDelivery)
	toReady(t, svc)
	ctx := context.Background()
	markNotFound(t, svc, clock)

	clock.advance(CourierNotFoundCancelAfter - time.Second)
	if _, changed, err := svc.AutoCancelCourierNotFound(ctx, "o1"); err != nil || changed {
		t.Fatalf("muddatdan OLDIN bekor qilindi: changed=%v err=%v", changed, err)
	}
	clock.advance(time.Second)
	o, changed, err := svc.AutoCancelCourierNotFound(ctx, "o1")
	if err != nil || !changed {
		t.Fatalf("muddat tugaganda bekor qilinmadi: changed=%v err=%v", changed, err)
	}
	last := o.History[len(o.History)-1]
	if o.Status != StatusCancelled || last.By != ActorSystem || last.From != StatusReady {
		t.Fatalf("holat=%s, oxirgi o'tish=%+v", o.Status, last)
	}
}

// Restoran muddat tugashidan bir lahza oldin "Kuryer qidirish" bossa,
// avtomatik bekor qilish QO'LLANMAYDI.
func TestAutoCancelSkippedAfterRestart(t *testing.T) {
	svc, _, clock := newDispatchFixture(t, TypeDelivery)
	toReady(t, svc)
	ctx := context.Background()
	markNotFound(t, svc, clock)
	clock.advance(CourierNotFoundCancelAfter)
	if _, err := svc.RestartCourierSearch(ctx, "o1"); err != nil {
		t.Fatal(err)
	}
	o, changed, err := svc.AutoCancelCourierNotFound(ctx, "o1")
	if err != nil || changed || o.Status != StatusReady {
		t.Fatalf("qayta qidirilgan buyurtma bekor qilindi: holat=%s changed=%v err=%v", o.Status, changed, err)
	}
}

// Kuryer topilsa (hatto "topilmadi" holatidan keyin ham) qidiruv tugaydi
// va buyurtmani endi hech qanday taymer bekor qilmaydi.
func TestAssignCourierEndsSearch(t *testing.T) {
	svc, _, clock := newDispatchFixture(t, TypeDelivery)
	toReady(t, svc)
	ctx := context.Background()
	markNotFound(t, svc, clock)

	o, err := svc.AssignCourier(ctx, "o1", "c1")
	if err != nil {
		t.Fatal(err)
	}
	if o.DispatchState != DispatchNone || o.DispatchDeadline != nil {
		t.Fatalf("kuryer biriktirilgach: holat=%q muddat=%v", o.DispatchState, o.DispatchDeadline)
	}
	clock.advance(CourierNotFoundCancelAfter)
	if _, changed, _ := svc.AutoCancelCourierNotFound(ctx, "o1"); changed {
		t.Fatal("kuryeri bor buyurtma avtomatik bekor qilindi")
	}
	if _, err := svc.ChangeStatus(ctx, "o1", StatusCancelled, ActorRestaurant); !isTransitionErr(err) {
		t.Fatalf("kuryeri bor buyurtmani restoran bekor qila oldi: %v", err)
	}
}

// Stol buyurtmasi kuryer qidiruviga HECH QACHON tushmaydi.
func TestDineInNeverEntersCourierSearch(t *testing.T) {
	svc, _, clock := newDispatchFixture(t, TypeDineIn)
	o := toReady(t, svc)
	if o.DispatchState != DispatchNone || o.DispatchDeadline != nil {
		t.Fatalf("stol buyurtmasi: holat=%q muddat=%v", o.DispatchState, o.DispatchDeadline)
	}
	clock.advance(time.Hour)
	if _, changed, err := svc.EnsureCourierSearchDeadline(context.Background(), "o1"); err != nil || changed {
		t.Fatalf("stol buyurtmasiga muddat qo'yildi: changed=%v err=%v", changed, err)
	}
	if o.CourierSearchExpired(clock.now()) || o.AwaitsCourier() {
		t.Fatal("stol buyurtmasi kuryer kutayotgan deb hisoblandi")
	}
}

// Migratsiyadan oldin tayyor bo'lgan (muddatsiz) buyurtma: muddat
// haqiqiy "tayyor" paytidan hisoblanadi, ya'ni uzoq osilib qolgan
// buyurtma DARHOL "kuryer topilmadi" bo'ladi.
func TestEnsureCourierSearchDeadline_LegacyReadyOrder(t *testing.T) {
	repo := newFakeOCCRepo()
	clock := &dispatchClock{t: time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)}
	svc := NewService(repo, nil, func() string { return "o1" }, nil).WithClock(clock.now)
	readyAt := clock.now().Add(-time.Hour)
	repo.data["o1"] = Order{ID: "o1", Type: TypeDelivery, Status: StatusReady, Version: 1,
		History: []StatusChange{{From: StatusPreparing, To: StatusReady, By: ActorRestaurant, At: readyAt}}}
	ctx := context.Background()

	o, changed, err := svc.EnsureCourierSearchDeadline(ctx, "o1")
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	if o.DispatchState != DispatchSearching || !o.DispatchDeadline.Equal(readyAt.Add(CourierSearchWindow)) {
		t.Fatalf("holat=%q muddat=%v", o.DispatchState, o.DispatchDeadline)
	}
	if !o.CourierSearchExpired(clock.now()) {
		t.Fatal("bir soat oldin tayyor bo'lgan buyurtma darhol muddati o'tgan bo'lishi kerak")
	}
	if _, changed, _ := svc.EnsureCourierSearchDeadline(ctx, "o1"); changed {
		t.Fatal("takroriy chaqiruv muddatni qayta yozdi")
	}
}
