package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"chustapp/internal/catalog"
)

// MIJOZ FAQAT RESTORAN YOQQAN USULLARNI KO'RADI.
//
// Mijoz ilovasi `payment_options` ga qarab usullarni YASHIRADI (kulrang
// emas, umuman chiqarmaydi). Bu maydon ochiq `GET /restaurants` va
// `GET /restaurants/{id}` javobida bo'lishi shart.

type publicRest struct {
	PaymentOptions *catalog.PaymentOptions `json:"payment_options"`
	// Ichki sozlama va kalitlar ochiq javobga CHIQMASLIGI kerak.
}

func getPublicRestaurant(t *testing.T, f *mcFixture) publicRest {
	t.Helper()
	w := do(t, f.h, "GET", "/restaurants/"+mcRestID, "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET: %d — %s", w.Code, w.Body.String())
	}
	var r publicRest
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestPublicRestaurantShowsOnlyEnabledPaymentOptions(t *testing.T) {
	f := mcPaymentsServer(t, true)
	f.setMethods(t, catalog.PaymentMethods{Cash: true, Uzum: true, Click: true})

	got := getPublicRestaurant(t, f).PaymentOptions
	if got == nil {
		t.Fatal("payment_options ochiq javobda yo'q")
	}
	want := catalog.PaymentOptions{Cash: true, Uzum: true, Click: true}
	if *got != want {
		t.Errorf("%+v, %+v kutilgan", *got, want)
	}

	// Restoran Uzum'ni o'chirdi — mijoz ko'rmaydi.
	f.setMethods(t, catalog.PaymentMethods{Cash: true, Click: true, Payme: true})
	got = getPublicRestaurant(t, f).PaymentOptions
	if got.Uzum || !got.Payme || !got.Click {
		t.Errorf("Uzum o'chirilgach: %+v", *got)
	}
}

// Ro'yxat Redis'ga YOZILISHI mumkin va u yerdan o'qilganda
// `MulticardStoreID` (json:"-") yo'qoladi. Shunda ham onlayn usullar
// to'g'ri chiqishi kerak: aks holda hamma restoran "ulanmagan" bo'lib
// qoladi.
func TestPaymentOptionsSurviveCacheRoundTrip(t *testing.T) {
	pm := catalog.PaymentMethods{Cash: true, CardOnline: true, Uzum: true}
	connected := &catalog.Restaurant{ID: "a", MulticardStoreID: "6", PaymentMethods: &pm}
	notConnected := &catalog.Restaurant{ID: "b", PaymentMethods: &pm}
	list := []*catalog.Restaurant{connected, notConnected}

	annotatePaymentOptions(list)
	raw, err := json.Marshal(list) // keshga yozilgandek
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "MulticardStoreID") {
		t.Fatal("do'kon identifikatori keshga/javobga tushyapti")
	}
	var fromCache []*catalog.Restaurant
	if err := json.Unmarshal(raw, &fromCache); err != nil {
		t.Fatal(err)
	}
	if fromCache[0].MulticardStoreID != "" {
		t.Fatal("test farazi buzildi: kesh do'kon ID sini saqlamasligi kerak")
	}

	a := withOpenState(*fromCache[0]).PaymentOptions
	if a == nil || !a.Card || !a.Uzum || !a.Cash {
		t.Errorf("ulangan restoran keshdan keyin: %+v", a)
	}
	b := withOpenState(*fromCache[1]).PaymentOptions
	if b == nil || b.Card || b.Uzum || !b.Cash {
		t.Errorf("ulanmagan restoran keshdan keyin: %+v", b)
	}
}
