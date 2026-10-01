package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// Senariy: Multicard'ning HAQIQIY callback shakli.
//
// Haqiqiy sandbox sinovi (2026-10-01) ochdi: Multicard muvaffaqiyatli yechilgandan keyin
// `store_id` BOR, `status` YO'Q, imzosi MD5 bo'lgan xabar yuboradi. Kod faqat `status` bor
// SHA1 xabarni bilardi: har bir haqiqiy to'lov "imzo noto'g'ri" deb rad etilardi, mijozdan
// pul yechilib, buyurtma tasdiqlanmasdi.
func TestMulticardSuccessCallbackShape(t *testing.T) {
	w := NewWorld(t, "Multicard SUCCESS callback shakli (store_id, MD5)")

	// Boshqa dalil bo'lmasin: shlyuz holat API'si ishlamaydi, yagona dalil - imzo.
	w.Gateway.mu.Lock()
	w.Gateway.StatusFails = true
	w.Gateway.mu.Unlock()

	body := func(storeID any, invoiceID, uuid string, amount int64, sign string) string {
		return fmt.Sprintf(`{"store_id":%v,"amount":%d,"invoice_id":%q,"billing_id":"b-1",
		  "payment_time":"2026-10-01 12:00:00","phone":"998901234567","card_pan":"860053******8829",
		  "ps":"uzcard","card_token":"tok","uuid":%q,"receipt_url":"https://dev-checkout.example/c","sign":%q}`,
			storeID, amount, invoiceID, uuid, sign)
	}
	start := func(key string) (id string, p *struct {
		ID, UUID string
		Amount   int64
	}) {
		o := w.PlaceOrder(t, w.Customer, Cart(ProdOsh, 1), "card", key)
		id, _ = o["id"].(string)
		w.Do(t, w.Customer, "POST", "/orders/"+id+"/pay", nil).Expect(t, 200)
		list, _ := w.Payments.ListByOrder(context.Background(), id)
		if len(list) == 0 {
			t.Fatal("to'lov yozuvi yo'q")
		}
		last := list[len(list)-1]
		return id, &struct {
			ID, UUID string
			Amount   int64
		}{last.ID, last.ProviderPaymentID, last.AmountTiyin}
	}

	w.Step(t, "SUCCESS callback (store_id raqam, status yo'q, MD5) to'lovni tasdiqlaydi", func(t *testing.T) {
		id, p := start("e2e-cbshape-1")
		good := body(6, p.ID, p.UUID, p.Amount, SignSuccess(GatewayStore, p.ID, p.Amount, GatewaySecret))
		r := w.Do(t, nil, "POST", "/payments/multicard/callback", good).Expect(t, 200)
		if !strings.Contains(string(r.Body), `"success":true`) {
			t.Errorf("javobda success:true yo'q (Multicard 200 + success:true kutadi): %s", r.Body)
		}
		if st := w.PaymentStatus(t, p.ID); st != "paid" {
			t.Errorf("to'lov holati %q, kutilgan paid", st)
		}
		o := w.Do(t, w.Customer, "GET", "/orders/"+id, nil).Expect(t, 200).Map(t)
		if st, _ := o["payment_state"].(string); st != "paid" && st != "held" {
			t.Errorf("buyurtma to'lov holati %q", st)
		}
		// Takroriy yuborish (Multicard qayta urinishi) xavfsiz: yana 200, holat o'zgarmaydi.
		w.Do(t, nil, "POST", "/payments/multicard/callback", good).Expect(t, 200)
		if st := w.PaymentStatus(t, p.ID); st != "paid" {
			t.Errorf("takroriy callbackdan keyin holat %q", st)
		}
	})

	w.Step(t, "noto'g'ri imzo: 200 EMAS (Multicard puli qaytaradi), to'lov tasdiqlanmaydi", func(t *testing.T) {
		_, p := start("e2e-cbshape-2")
		bad := body(6, p.ID, p.UUID, p.Amount, strings.Repeat("0", 32))
		r := w.Do(t, nil, "POST", "/payments/multicard/callback", bad)
		if r.Status == 200 {
			t.Errorf("noto'g'ri imzo 200 oldi: Multicard to'lovni qaytarmaydi, pul yechilib qoladi: %s", r.Body)
		}
		if !strings.Contains(string(r.Body), `"success":false`) {
			t.Errorf("javobda success:false yo'q: %s", r.Body)
		}
		if st := w.PaymentStatus(t, p.ID); st == "paid" {
			t.Error("noto'g'ri imzo bilan to'lov tasdiqlandi")
		}
	})

	w.Step(t, "boshqa kassa (store_id=7) imzo to'g'ri bo'lsa ham rad etiladi", func(t *testing.T) {
		_, p := start("e2e-cbshape-3")
		forged := body(7, p.ID, p.UUID, p.Amount, SignSuccess("7", p.ID, p.Amount, GatewaySecret))
		r := w.Do(t, nil, "POST", "/payments/multicard/callback", forged)
		if r.Status == 200 {
			t.Errorf("boshqa kassa xabari qabul qilindi: %s", r.Body)
		}
		if st := w.PaymentStatus(t, p.ID); st == "paid" {
			t.Error("boshqa kassa xabari bilan to'lov tasdiqlandi")
		}
	})
}
