package e2e

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"chustapp/internal/modules/reservation"
	"chustapp/internal/payments/multicard"
)

// RS-3/RS-4: stol broni + zaklad — sozlamalar, bo'sh vaqtlar, karta va
// hamyon zakladi, 2 soat qoidasi, keldi/yakun, kelmadi, restoran bekor
// qilishi, to'lanmagan zaklad muddati, begona tenant/mijoz izolyatsiyasi.
func TestReservationFlow(t *testing.T) {
	w := NewWorld(t, "Stol broni va zaklad")
	ctx := context.Background()

	// Ertaga 19:00 Toshkent (min 60 daq oldin, 30 daq qadam).
	now := time.Now().In(reservation.Tashkent)
	tomorrow := time.Date(now.Year(), now.Month(), now.Day(), 19, 0, 0, 0, reservation.Tashkent).Add(24 * time.Hour)
	day := tomorrow.Format("2006-01-02")
	at := func(d time.Duration) string { return tomorrow.Add(d).UTC().Format(time.RFC3339) }
	settingsURL := "/restaurants/" + RestA + "/reservation-settings"
	availURL := func(party int) string {
		return fmt.Sprintf("/restaurants/%s/reservations/availability?date=%s&party=%d", RestA, day, party)
	}
	book := func(t *testing.T, who *Actor, starts string, party int, method string) map[string]any {
		t.Helper()
		return w.Do(t, who, "POST", "/restaurants/"+RestA+"/reservations", map[string]any{
			"starts_at": starts, "party_size": party, "method": method, "comment": "deraza yonida",
		}).Expect(t, 201).Map(t)
	}
	rsvID := func(m map[string]any) string { return m["reservation"].(map[string]any)["id"].(string) }
	payDeposit := func(t *testing.T, id string) {
		t.Helper()
		list, err := w.Payments.ListByReservation(ctx, id)
		if err != nil || len(list) == 0 {
			t.Fatalf("zaklad to'lovi yo'q: %v", err)
		}
		p := list[0]
		w.Gateway.mu.Lock()
		w.Gateway.StatusFails = true
		w.Gateway.mu.Unlock()
		w.Do(t, nil, "POST", "/payments/multicard/callback", multicard.Callback{
			UUID: p.ProviderPaymentID, InvoiceID: p.ID, Amount: multicard.Tiyin(p.AmountTiyin), Status: "success",
			Sign: Sign(p.ProviderPaymentID, p.ID, p.AmountTiyin, GatewaySecret),
		}).Expect(t, 200)
	}
	myStatus := func(t *testing.T, who *Actor, id string) (string, string) {
		t.Helper()
		var list []map[string]any
		w.Do(t, who, "GET", "/me/reservations", nil).Expect(t, 200).JSON(t, &list)
		for _, x := range list {
			if x["id"] == id {
				return x["status"].(string), x["deposit_state"].(string)
			}
		}
		t.Fatalf("bron ro'yxatda yo'q: %s", id)
		return "", ""
	}

	w.Step(t, "bron o'chiq: sozlama standart, mijozga 422", func(t *testing.T) {
		st := w.Do(t, w.Owner, "GET", settingsURL, nil).Expect(t, 200).Map(t)
		if st["enabled"] != false {
			t.Fatalf("standart yoqiq: %v", st)
		}
		w.Do(t, w.Customer, "GET", availURL(2), nil).Expect(t, 422)
	})

	w.Step(t, "restoran stollar va zaklad sozlaydi", func(t *testing.T) {
		w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/tables", map[string]any{"label": "1", "capacity": 4}).Expect(t, 201)
		w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/tables", map[string]any{"label": "1", "zone": "VIP", "capacity": 6}).Expect(t, 201)
		body := map[string]any{"enabled": true, "deposit_per_person_tiyin": 5_000_000,
			"zone_deposits": map[string]any{"VIP": 10_000_000}, "slot_minutes": 120,
			"min_advance_minutes": 60, "max_days_ahead": 30}
		w.Do(t, w.Owner, "PUT", settingsURL, body).Expect(t, 200)
		w.ExpectDenied(t, w.OwnerB, "PUT", settingsURL, body)
		w.ExpectDenied(t, w.Customer, "PUT", settingsURL, body)
		bad := map[string]any{"enabled": true, "slot_minutes": 10, "min_advance_minutes": 60, "max_days_ahead": 30}
		w.Do(t, w.Owner, "PUT", settingsURL, bad).Expect(t, 400)
	})

	var cardID string
	w.Step(t, "mijoz bo'sh vaqt ko'radi va karta zakladi bilan bron qiladi", func(t *testing.T) {
		av := w.Do(t, w.Customer, "GET", availURL(2), nil).Expect(t, 200).Map(t)
		slots, _ := av["slots"].([]any)
		found := false
		for _, s := range slots {
			found = found || s.(map[string]any)["starts_at"] == at(0)
		}
		if !found {
			t.Fatalf("19:00 bo'sh vaqtlarda yo'q: %v", av)
		}
		res := book(t, w.Customer, at(0), 2, "card")
		pay, _ := res["pay"].(map[string]any)
		if pay == nil || pay["pay_url"] == "" {
			t.Fatalf("to'lov havolasi yo'q: %v", res)
		}
		r := res["reservation"].(map[string]any)
		if r["status"] != "held" || r["deposit_tiyin"].(float64) != 10_000_000 || !strings.HasSuffix(r["table_label"].(string), "1") {
			t.Fatalf("bron: %v", r)
		}
		cardID = rsvID(res)
		// To'lov havolasini qayta olish (sahifa yopilib qolgan bo'lsa).
		w.Do(t, w.Customer, "POST", "/me/reservations/"+cardID+"/pay", nil).Expect(t, 200)
		w.ExpectDenied(t, w.Other, "POST", "/me/reservations/"+cardID+"/pay", nil)
		payDeposit(t, cardID)
		if st, dep := myStatus(t, w.Customer, cardID); st != "confirmed" || dep != "paid" {
			t.Fatalf("tasdiq: %s %s", st, dep)
		}
		w.Do(t, w.Customer, "POST", "/me/reservations/"+cardID+"/pay", nil).Expect(t, 409)
	})

	w.Step(t, "noto'g'ri so'rovlar va band stol", func(t *testing.T) {
		w.Do(t, w.Customer, "POST", "/restaurants/"+RestA+"/reservations", map[string]any{
			"starts_at": at(10 * time.Minute), "party_size": 2, "method": "card"}).Expect(t, 400)
		w.Do(t, w.Customer, "POST", "/restaurants/"+RestA+"/reservations", map[string]any{
			"starts_at": at(0), "party_size": 2}).Expect(t, 400) // zaklad usulisiz
		w.ExpectDenied(t, w.Owner, "POST", "/restaurants/"+RestA+"/reservations", map[string]any{
			"starts_at": at(0), "party_size": 2, "method": "card"})
		// 4 kishilik oddiy stol band -> VIP (zona zakladi 100 000 x 5).
		vip := book(t, w.Other, at(0), 5, "card")["reservation"].(map[string]any)
		if vip["deposit_tiyin"].(float64) != 50_000_000 {
			t.Fatalf("VIP zaklad: %v", vip)
		}
		// Endi 19:00 da joy yo'q.
		w.Do(t, w.Customer, "POST", "/restaurants/"+RestA+"/reservations", map[string]any{
			"starts_at": at(0), "party_size": 2, "method": "card"}).Expect(t, 400) // o'zining kesishuvchi broni
	})

	w.Step(t, "to'lanmagan zaklad 15 daqiqada bekor bo'ladi", func(t *testing.T) {
		var list []map[string]any
		w.Do(t, w.Other, "GET", "/me/reservations", nil).Expect(t, 200).JSON(t, &list)
		if len(list) != 1 || list[0]["status"] != "held" {
			t.Fatalf("held: %v", list)
		}
		w.Advance(16 * time.Minute)
		if n := w.SweepReservations(); n < 1 {
			t.Fatalf("nazoratchi hech narsa qilmadi")
		}
		w.Do(t, w.Other, "GET", "/me/reservations", nil).Expect(t, 200).JSON(t, &list)
		if list[0]["status"] != "expired" {
			t.Fatalf("muddati o'tmadi: %v", list[0])
		}
	})

	w.Step(t, "hamyon zakladi: 2+ soat oldin bekor = to'liq qaytadi", func(t *testing.T) {
		w.Do(t, w.Other, "POST", "/restaurants/"+RestA+"/reservations", map[string]any{
			"starts_at": at(2 * time.Hour), "party_size": 2, "method": "wallet"}).Expect(t, 402)
		if err := w.Wallet.RefundForOrder(ctx, "e2e-seed-rsv", w.Other.UserID, 30_000_000); err != nil {
			t.Fatal(err)
		}
		res := book(t, w.Other, at(2*time.Hour), 2, "wallet")
		id := rsvID(res)
		if st, dep := myStatus(t, w.Other, id); st != "confirmed" || dep != "paid" {
			t.Fatalf("hamyon tasdiq: %s %s", st, dep)
		}
		bal, _ := w.Wallet.Balance(ctx, w.Other.UserID)
		if bal != 20_000_000 {
			t.Fatalf("yechilmadi: %d", bal)
		}
		w.ExpectDenied(t, w.Customer, "POST", "/me/reservations/"+id+"/cancel", nil)
		c := w.Do(t, w.Other, "POST", "/me/reservations/"+id+"/cancel", nil).Expect(t, 200).Map(t)
		if c["deposit_state"] != "refunded" {
			t.Fatalf("qaytmadi: %v", c)
		}
		bal, _ = w.Wallet.Balance(ctx, w.Other.UserID)
		if bal != 30_000_000 {
			t.Fatalf("balans: %d", bal)
		}
	})

	w.Step(t, "restoran ro'yxati: mijoz telefoni bilan, begona restoran ko'rmaydi", func(t *testing.T) {
		q := "?from=" + url.QueryEscape(tomorrow.Add(-24*time.Hour).UTC().Format(time.RFC3339)) +
			"&to=" + url.QueryEscape(tomorrow.Add(24*time.Hour).UTC().Format(time.RFC3339))
		var list []map[string]any
		w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/reservations"+q, nil).Expect(t, 200).JSON(t, &list)
		ok := false
		for _, x := range list {
			ok = ok || (x["id"] == cardID && x["customer_phone"] == w.Customer.Phone)
		}
		if !ok {
			t.Fatalf("ro'yxatda yo'q: %v", list)
		}
		w.ExpectDenied(t, w.OwnerB, "GET", "/restaurants/"+RestA+"/reservations"+q, nil)
		w.ExpectDenied(t, w.Customer, "GET", "/restaurants/"+RestA+"/reservations"+q, nil)
		w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/reservations?from=x&to=y", nil).Expect(t, 400)
	})

	var other2 string
	w.Step(t, "restoran bekor qiladi: sabab majburiy, zaklad qaytadi", func(t *testing.T) {
		other2 = rsvID(book(t, w.Other, at(4*time.Hour), 2, "wallet"))
		w.ExpectDenied(t, w.OwnerB, "POST", "/reservations/"+other2+"/cancel", map[string]any{"reason": "x"})
		w.Do(t, w.Owner, "POST", "/reservations/"+other2+"/cancel", map[string]any{"reason": ""}).Expect(t, 400)
		c := w.Do(t, w.Owner, "POST", "/reservations/"+other2+"/cancel", map[string]any{"reason": "zal band"}).Expect(t, 200).Map(t)
		if c["status"] != "cancelled" || c["deposit_state"] != "refunded" || c["cancelled_by"] != "restaurant" {
			t.Fatalf("restoran bekor: %v", c)
		}
	})

	w.Step(t, "mijoz keldi -> zaklad hisobdan ayriladi; yakun; kelmagan", func(t *testing.T) {
		w.Do(t, w.Owner, "POST", "/reservations/"+cardID+"/seat", nil).Expect(t, 409) // hali erta
		noShow := rsvID(book(t, w.Other, at(6*time.Hour), 2, "wallet"))

		// Ertaga 18:55 ga suramiz (bron 19:00).
		w.Advance(time.Until(tomorrow.Add(-5 * time.Minute)))
		s := w.Do(t, w.Owner, "POST", "/reservations/"+cardID+"/seat", nil).Expect(t, 200).Map(t)
		if s["status"] != "seated" || s["deposit_state"] != "applied" {
			t.Fatalf("keldi: %v", s)
		}
		w.ExpectDenied(t, w.OwnerB, "POST", "/reservations/"+cardID+"/complete", nil)
		w.Do(t, w.Owner, "POST", "/reservations/"+cardID+"/complete", nil).Expect(t, 200)

		w.Do(t, w.Owner, "POST", "/reservations/"+noShow+"/no-show", nil).Expect(t, 409) // hali vaqti emas
		w.Advance(6*time.Hour + 20*time.Minute)
		ns := w.Do(t, w.Owner, "POST", "/reservations/"+noShow+"/no-show", nil).Expect(t, 200).Map(t)
		if ns["status"] != "no_show" || ns["deposit_state"] != "kept" {
			t.Fatalf("kelmadi: %v", ns)
		}
	})

	// RS-6: admin jonli holati — restoranlar, faol buyurtmalar, bronlar.
	w.Step(t, "admin restoranlarning jonli holatini ko'radi", func(t *testing.T) {
		live := w.Do(t, w.Admin, "GET", "/admin/restaurants/live", nil).Expect(t, 200).Map(t)
		found := false
		for _, x := range live["restaurants"].([]any) {
			r := x.(map[string]any)
			if r["id"] == RestA {
				found = true
				if _, ok := r["reservations"].(map[string]any); !ok {
					t.Fatalf("bronlar bo'limi yo'q: %v", r)
				}
			}
		}
		if !found {
			t.Fatalf("RestA yo'q: %v", live)
		}
		if contains(mustJSON(live), w.Customer.Phone) {
			t.Fatal("jonli holatda mijoz telefoni")
		}
		w.ExpectDenied(t, w.Owner, "GET", "/admin/restaurants/live", nil)
		w.ExpectDenied(t, w.Customer, "GET", "/admin/restaurants/live", nil)
	})
}
