package catalog

import (
	"testing"

	"chustapp/internal/orders"
)

func connected(pm *PaymentMethods) Restaurant {
	return Restaurant{ID: "r", MulticardStoreID: "6", PaymentMethods: pm}
}

// Mijozga faqat restoran YOQQAN usullar ko'rinadi.
func TestPaymentOptionsFollowRestaurantSwitches(t *testing.T) {
	cases := []struct {
		name string
		pm   PaymentMethods
		want PaymentOptions
	}{
		{"hammasi o'chiq (naqd bor)", PaymentMethods{Cash: true}, PaymentOptions{Cash: true}},
		{"faqat Uzum", PaymentMethods{Uzum: true}, PaymentOptions{Uzum: true}},
		{"faqat Payme", PaymentMethods{Payme: true}, PaymentOptions{Payme: true}},
		{"faqat Click", PaymentMethods{Click: true}, PaymentOptions{Click: true}},
		{"terminal ham naqd hisobida", PaymentMethods{CardTerminal: true}, PaymentOptions{Cash: true}},
		{"Bank kartasi", PaymentMethods{CardOnline: true}, PaymentOptions{Card: true}},
		{"hammasi", PaymentMethods{Cash: true, CardOnline: true, Payme: true, Click: true, Uzum: true},
			PaymentOptions{Cash: true, Card: true, Payme: true, Click: true, Uzum: true}},
	}
	for _, c := range cases {
		pm := c.pm
		got := connected(&pm).WithPaymentOptions().PaymentOptions
		if got == nil || *got != c.want {
			t.Errorf("%s: %+v, %+v kutilgan", c.name, got, c.want)
		}
	}
}

// Shlyuzga ulanmagan restoranda onlayn usullar ko'rinmaydi (yoqilgan bo'lsa ham).
func TestPaymentOptionsHideOnlineWhenNotConnected(t *testing.T) {
	pm := PaymentMethods{Cash: true, CardOnline: true, Payme: true, Click: true, Uzum: true}
	r := Restaurant{ID: "r", PaymentMethods: &pm}
	got := r.WithPaymentOptions().PaymentOptions
	if got.Card || got.Payme || got.Click || got.Uzum {
		t.Errorf("ulanmagan restoranda onlayn usul ko'rinadi: %+v", got)
	}
	if !got.Cash {
		t.Error("naqd yo'qolgan")
	}
}

func TestAcceptsChannel(t *testing.T) {
	pm := PaymentMethods{Cash: true, CardOnline: true, Uzum: true}
	r := connected(&pm)
	for ch, want := range map[string]bool{
		"": true, "uzum": true, "payme": false, "click": false, "bitcoin": false,
	} {
		if got := r.AcceptsChannel(ch); got != want {
			t.Errorf("%q: %v, %v kutilgan", ch, got, want)
		}
	}
}

func TestAnyOnlineAndValidate(t *testing.T) {
	for _, pm := range []PaymentMethods{{Payme: true}, {Click: true}, {Uzum: true}, {CardOnline: true}} {
		if !pm.AnyOnline() {
			t.Errorf("%+v: AnyOnline false", pm)
		}
		if err := pm.Validate(); err != nil {
			t.Errorf("%+v: %v", pm, err)
		}
	}
	if (PaymentMethods{Cash: true}).AnyOnline() {
		t.Error("faqat naqd — onlayn emas")
	}
	if err := (PaymentMethods{}).Validate(); err != ErrNoPaymentMethod {
		t.Errorf("bo'sh: %v", err)
	}
}

// Karta buyurtmasi: kamida bitta onlayn usul yoqiq bo'lsa qabul qilinadi.
func TestAcceptsPaymentCardNeedsAnyOnline(t *testing.T) {
	off := PaymentMethods{Cash: true}
	if r := connected(&off); r.AcceptsPayment(orders.PaymentCard) {
		t.Error("onlayn usulsiz karta buyurtmasi qabul qilindi")
	}
	on := PaymentMethods{Cash: true, Uzum: true}
	if r := connected(&on); !r.AcceptsPayment(orders.PaymentCard) {
		t.Error("Uzum yoqiq, lekin karta buyurtmasi rad etildi")
	}
}
