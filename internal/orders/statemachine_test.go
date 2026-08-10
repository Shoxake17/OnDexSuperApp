package orders

import "testing"

func TestValidTransitions(t *testing.T) {
	cases := []struct {
		from, to Status
		by       Actor
	}{
		{StatusCreated, StatusAccepted, ActorRestaurant},
		{StatusCreated, StatusRejected, ActorRestaurant},
		{StatusCreated, StatusCancelled, ActorCustomer},
		{StatusCreated, StatusCancelled, ActorAdmin},
		{StatusAccepted, StatusPreparing, ActorRestaurant},
		{StatusAccepted, StatusCancelled, ActorCustomer},
		{StatusPreparing, StatusReady, ActorRestaurant},
		// Ready->PickedUp endi kuryer TO'G'RIDAN-TO'G'RI o'zi qiladi —
		// alohida tasdiqlash kodi yo'q, restoran ham tugma bosmaydi
		// (Yandex Eats uslubi, foydalanuvchi so'rovi bo'yicha).
		{StatusReady, StatusPickedUp, ActorCourier},
		{StatusPickedUp, StatusDelivered, ActorCourier},
	}
	for _, c := range cases {
		if err := ValidateTransition(c.from, c.to, c.by); err != nil {
			t.Errorf("%s -> %s (%s): ruxsat bo'lishi kerak edi, xato: %v", c.from, c.to, c.by, err)
		}
	}
}

func TestForbiddenTransitions(t *testing.T) {
	cases := []struct {
		from, to Status
		by       Actor
		reason   string
	}{
		{StatusCreated, StatusDelivered, ActorCourier, "bosqich sakrash"},
		{StatusCreated, StatusAccepted, ActorCustomer, "mijoz restoran nomidan"},
		{StatusCreated, StatusAccepted, ActorCourier, "kuryer restoran nomidan"},
		{StatusPreparing, StatusCancelled, ActorCustomer, "tayyorlanayotganda mijoz bekor qila olmaydi"},
		{StatusReady, StatusPickedUp, ActorRestaurant, "faqat kuryer oladi"},
		{StatusReady, StatusPickedUp, ActorSystem, "endi ActorSystem emas, ActorCourier oladi"},
		{StatusDelivered, StatusCreated, ActorAdmin, "terminal holatdan chiqish"},
		{StatusCancelled, StatusAccepted, ActorRestaurant, "terminal holatdan chiqish"},
		{StatusRejected, StatusAccepted, ActorAdmin, "terminal holatdan chiqish"},
		{StatusPickedUp, StatusReady, ActorCourier, "orqaga qaytish"},
	}
	for _, c := range cases {
		if err := ValidateTransition(c.from, c.to, c.by); err == nil {
			t.Errorf("%s -> %s (%s): rad etilishi kerak edi (%s)", c.from, c.to, c.by, c.reason)
		}
	}
}

func TestIsTerminal(t *testing.T) {
	terminal := []Status{StatusDelivered, StatusRejected, StatusCancelled}
	for _, s := range terminal {
		if !(&Order{Status: s}).IsTerminal() {
			t.Errorf("%s terminal bo'lishi kerak", s)
		}
	}
	active := []Status{StatusCreated, StatusAccepted, StatusPreparing, StatusReady, StatusPickedUp}
	for _, s := range active {
		if (&Order{Status: s}).IsTerminal() {
			t.Errorf("%s terminal bo'lmasligi kerak", s)
		}
	}
}
