package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"chustapp/internal/catalog"
	"chustapp/internal/orders"
)

// RESTORAN O'CHIRGAN TO'LOV USULI SERVERDA RAD ETILADI.
//
// Mijoz ilovasi o'chirilgan usulni yashiradi, lekin bu bezak: eski ilova
// yoki to'g'ridan-to'g'ri so'rov baribir `?ps=uzum` yuborishi mumkin.
// To'siq `/orders/{id}/pay` ning o'zida bo'lishi shart.

const mcRestID = mcTestRestID

func (f *mcFixture) setMethods(t *testing.T, pm catalog.PaymentMethods) {
	t.Helper()
	ctx := context.Background()
	rest, err := f.catalog.GetRestaurant(ctx, mcRestID)
	if err != nil {
		t.Fatal(err)
	}
	rest.PaymentMethods = &pm
	if err := f.catalog.SaveRestaurant(ctx, rest); err != nil {
		t.Fatal(err)
	}
}

func (f *mcFixture) newCardOrder(t *testing.T, key string) string {
	t.Helper()
	body := `{"items":[{"product_id":"` + mcTestProductID + `","qty":1}],` +
		`"payment_method":"card","idempotency_key":"` + key + `"}`
	w := do(t, f.h, "POST", "/orders", f.jwt, body)
	if w.Code != http.StatusCreated {
		t.Fatalf("buyurtma yaratilmadi: %d — %s", w.Code, w.Body.String())
	}
	var o orders.Order
	if err := json.Unmarshal(w.Body.Bytes(), &o); err != nil {
		t.Fatal(err)
	}
	return o.ID
}

func TestPayRejectsChannelRestaurantDisabled(t *testing.T) {
	f := mcPaymentsServer(t, true)
	// Payme yoqiq, Click va Uzum — o'chiq.
	f.setMethods(t, catalog.PaymentMethods{Cash: true, CardOnline: true, Payme: true})

	for _, ch := range []string{"click", "uzum"} {
		id := f.newCardOrder(t, "dis-"+ch)
		w := do(t, f.h, "POST", "/orders/"+id+"/pay?ps="+ch, f.jwt, `{}`)
		if w.Code != http.StatusConflict {
			t.Errorf("%s o'chiq: %d (409 kutilgan) — %s", ch, w.Code, w.Body.String())
			continue
		}
		if !strings.Contains(w.Body.String(), "qabul qilmaydi") {
			t.Errorf("%s: xabar tushunarsiz — %s", ch, w.Body.String())
		}
		list, _ := f.repo.ListByOrder(context.Background(), id)
		if len(list) != 0 {
			t.Errorf("%s: rad etilgan, lekin to'lov yozuvi ochilgan", ch)
		}
	}
}

func TestPayAcceptsChannelRestaurantEnabled(t *testing.T) {
	f := mcPaymentsServer(t, true)
	f.setMethods(t, catalog.PaymentMethods{Cash: true, CardOnline: true, Payme: true, Click: true, Uzum: true})

	for _, ch := range []string{"payme", "click", "uzum", ""} {
		id := f.newCardOrder(t, "en-"+ch)
		path := "/orders/" + id + "/pay"
		if ch != "" {
			path += "?ps=" + ch
		}
		if w := do(t, f.h, "POST", path, f.jwt, `{}`); w.Code != http.StatusOK {
			t.Errorf("%q yoqiq: %d — %s", ch, w.Code, w.Body.String())
		}
	}
}

// Umumiy sahifa ("Bank kartasi") ham o'z bayrog'iga bog'liq.
func TestPayRejectsGenericCardWhenCardOnlineOff(t *testing.T) {
	f := mcPaymentsServer(t, true)
	// Buyurtma yaratish uchun biror onlayn usul kerak (Payme), lekin
	// "Bank kartasi" o'chiq.
	f.setMethods(t, catalog.PaymentMethods{Cash: true, Payme: true})

	id := f.newCardOrder(t, "gen-off")
	if w := do(t, f.h, "POST", "/orders/"+id+"/pay", f.jwt, `{}`); w.Code != http.StatusConflict {
		t.Errorf("umumiy sahifa o'chiq: %d (409 kutilgan) — %s", w.Code, w.Body.String())
	}
	if w := do(t, f.h, "POST", "/orders/"+id+"/pay?ps=payme", f.jwt, `{}`); w.Code != http.StatusOK {
		t.Errorf("payme yoqiq: %d — %s", w.Code, w.Body.String())
	}
}

// Noma'lum kanal — 400, jimgina umumiy sahifa emas (avvalgi xulq saqlanadi).
func TestPayUnknownChannelStill400(t *testing.T) {
	f := mcPaymentsServer(t, true)
	id := f.newCardOrder(t, "unk")
	if w := do(t, f.h, "POST", "/orders/"+id+"/pay?ps=bitcoin", f.jwt, `{}`); w.Code != http.StatusBadRequest {
		t.Errorf("noma'lum kanal: %d (400 kutilgan)", w.Code)
	}
}
