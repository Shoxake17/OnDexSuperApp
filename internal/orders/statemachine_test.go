package orders

import "testing"

func TestValidTransitions(t *testing.T) {
	cases := []struct {
		typ      Type
		from, to Status
		by       Actor
	}{
		{TypeDelivery, StatusCreated, StatusAccepted, ActorRestaurant},
		{TypeDelivery, StatusCreated, StatusRejected, ActorRestaurant},
		{TypeDelivery, StatusCreated, StatusCancelled, ActorCustomer},
		{TypeDelivery, StatusCreated, StatusCancelled, ActorAdmin},
		{TypeDelivery, StatusAccepted, StatusPreparing, ActorRestaurant},
		{TypeDelivery, StatusAccepted, StatusCancelled, ActorCustomer},
		{TypeDelivery, StatusPreparing, StatusReady, ActorRestaurant},
		// Ready->PickedUp endi kuryer TO'G'RIDAN-TO'G'RI o'zi qiladi —
		// alohida tasdiqlash kodi yo'q, restoran ham tugma bosmaydi
		// (Yandex Eats uslubi, foydalanuvchi so'rovi bo'yicha).
		{TypeDelivery, StatusReady, StatusPickedUp, ActorCourier},
		{TypeDelivery, StatusPickedUp, StatusDelivered, ActorCourier},

		// ── Stolda ovqatlanish ──
		{TypeDineIn, StatusCreated, StatusAccepted, ActorRestaurant},
		{TypeDineIn, StatusAccepted, StatusPreparing, ActorRestaurant},
		{TypeDineIn, StatusPreparing, StatusReady, ActorRestaurant},
		{TypeDineIn, StatusReady, StatusServed, ActorWaiter},
		// Kichik oshxonada affitsiant va oshpaz bir odam bo'lishi mumkin.
		{TypeDineIn, StatusReady, StatusServed, ActorRestaurant},

		// Bo'sh tur = delivery (eski buyurtmalar buzilmasin).
		{"", StatusReady, StatusPickedUp, ActorCourier},
	}
	for _, c := range cases {
		if err := ValidateTransition(c.typ, c.from, c.to, c.by); err != nil {
			t.Errorf("[%s] %s -> %s (%s): ruxsat bo'lishi kerak edi, xato: %v",
				c.typ, c.from, c.to, c.by, err)
		}
	}
}

func TestForbiddenTransitions(t *testing.T) {
	cases := []struct {
		typ      Type
		from, to Status
		by       Actor
		reason   string
	}{
		{TypeDelivery, StatusCreated, StatusDelivered, ActorCourier, "bosqich sakrash"},
		{TypeDelivery, StatusCreated, StatusAccepted, ActorCustomer, "mijoz restoran nomidan"},
		{TypeDelivery, StatusCreated, StatusAccepted, ActorCourier, "kuryer restoran nomidan"},
		{TypeDelivery, StatusPreparing, StatusCancelled, ActorCustomer, "tayyorlanayotganda mijoz bekor qila olmaydi"},
		{TypeDelivery, StatusReady, StatusPickedUp, ActorRestaurant, "faqat kuryer oladi"},
		{TypeDelivery, StatusReady, StatusPickedUp, ActorSystem, "endi ActorSystem emas, ActorCourier oladi"},
		{TypeDelivery, StatusDelivered, StatusCreated, ActorAdmin, "terminal holatdan chiqish"},
		{TypeDelivery, StatusCancelled, StatusAccepted, ActorRestaurant, "terminal holatdan chiqish"},
		{TypeDelivery, StatusRejected, StatusAccepted, ActorAdmin, "terminal holatdan chiqish"},
		{TypeDelivery, StatusPickedUp, StatusReady, ActorCourier, "orqaga qaytish"},

		// ★ XAVFSIZLIK: turlar bir-biriga o'tib ketmasligi kerak.
		//
		// Bu aynan tur bo'yicha ALOHIDA jadval qilinishining sababi.
		// Bitta umumiy jadvalda bu to'rt holat ham RUXSAT ETILGAN
		// bo'lardi va buyurtma noto'g'ri terminal holatga tushib,
		// qaytarib bo'lmasdi.
		{TypeDelivery, StatusReady, StatusServed, ActorWaiter,
			"affitsiant YETKAZISH buyurtmasini yopa olmasligi kerak"},
		{TypeDelivery, StatusReady, StatusServed, ActorRestaurant,
			"yetkazish buyurtmasida `served` umuman bo'lmaydi"},
		{TypeDineIn, StatusReady, StatusPickedUp, ActorCourier,
			"kuryer STOL buyurtmasini olib keta olmasligi kerak"},
		{TypeDineIn, StatusServed, StatusPickedUp, ActorCourier,
			"served — terminal"},

		// Affitsiant faqat OXIRGI qadamni qiladi, boshqa hech narsani.
		{TypeDineIn, StatusCreated, StatusAccepted, ActorWaiter, "affitsiant qabul qilmaydi"},
		{TypeDineIn, StatusPreparing, StatusReady, ActorWaiter, "affitsiant tayyor deb belgilamaydi"},
		{TypeDineIn, StatusCreated, StatusServed, ActorWaiter, "bosqich sakrash"},
	}
	for _, c := range cases {
		if err := ValidateTransition(c.typ, c.from, c.to, c.by); err == nil {
			t.Errorf("[%s] %s -> %s (%s): rad etilishi kerak edi (%s)",
				c.typ, c.from, c.to, c.by, c.reason)
		}
	}
}

func TestIsTerminal(t *testing.T) {
	terminal := []Status{StatusDelivered, StatusServed, StatusRejected, StatusCancelled}
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

// Bo'sh tur har doim `delivery` — bazadagi eski buyurtmalar va
// nol qiymatli struct'lar holat mashinasidan o'ta olishi shart.
func TestEmptyTypeIsDelivery(t *testing.T) {
	if Type("").Normalized() != TypeDelivery {
		t.Fatal("bo'sh tur `delivery` bo'lishi kerak")
	}
	if (&Order{}).IsDineIn() {
		t.Fatal("nol qiymatli buyurtma dine_in bo'lib qoldi")
	}
}
