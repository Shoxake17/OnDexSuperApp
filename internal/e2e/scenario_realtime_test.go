package e2e

import (
	"testing"
	"time"
)

// ssenariy: JONLI XABARLAR (WebSocket) — kim, qachon, nimani oladi.
//
// Qoidalar: har ishtirokchi FAQAT o'ziga tegishli buyurtma hodisalarini
// oladi; begona restoran/mijoz hech narsa olmaydi.

func orderIs(id string) func(map[string]any) bool {
	return func(m map[string]any) bool {
		if m["order_id"] == id {
			return true
		}
		if d, ok := m["data"].(map[string]any); ok && d["order_id"] == id {
			return true
		}
		return false
	}
}

func statusIs(id, status string) func(map[string]any) bool {
	return func(m map[string]any) bool {
		if m["order_id"] == id && m["status"] == status {
			return true
		}
		if d, ok := m["data"].(map[string]any); ok && d["order_id"] == id && d["status"] == status {
			return true
		}
		return false
	}
}

func TestRealtimeFeeds(t *testing.T) {
	w := NewWorld(t, "Jonli xabarlar (WebSocket)")
	var courier *Actor
	var owner, cust, courierFeed, otherCust, otherOwner *Feed
	var orderID string

	w.Step(t, "hamma ulanadi; begona ishtirokchilar ham tinglaydi", func(t *testing.T) {
		courier, _ = addCourier(t, w, "+998901110031", `,"delivery_pay_tiyin":800000`)
		owner = w.Listen(t, w.Owner, "")
		cust = w.Listen(t, w.Customer, "")
		courierFeed = w.Listen(t, courier, "")
		otherCust = w.Listen(t, w.Other, "")
		otherOwner = w.Listen(t, w.OwnerB, "")
		// Bilet BIR MARTALIK: qayta ishlatib bo'lmaydi.
		tk := w.Do(t, w.Customer, "POST", "/ws/ticket", nil).Expect(t, 201).Map(t)
		ticket, _ := tk["ticket"].(string)
		w.Do(t, nil, "GET", "/ws?ticket="+ticket, nil) // oddiy HTTP — upgrade yo'q, 400
		w.Do(t, nil, "GET", "/ws?ticket=soxta", nil)
		w.Do(t, nil, "GET", "/ws", nil)
	})

	w.Step(t, "kuryer smenaga chiqadi", func(t *testing.T) {
		w.Do(t, courier, "POST", "/couriers/"+courier.Entity+"/location",
			map[string]any{"lat": ChustLat, "lng": ChustLng}).Expect(t, 200)
		w.Do(t, courier, "POST", "/couriers/"+courier.Entity+"/available", map[string]any{"available": true}).Expect(t, 200)
	})

	w.Step(t, "yangi buyurtma: restoran DARHOL biladi", func(t *testing.T) {
		o := w.Do(t, w.Customer, "POST", "/orders", map[string]any{
			"items": []map[string]any{{"product_id": ProdOsh, "qty": 1}}, "payment_method": "cash",
			"idempotency_key": "e2e-rt-1",
		}).Expect(t, 201).Map(t)
		orderID, _ = o["id"].(string)
		m := owner.Wait(t, "restoranga yangi buyurtma", 3*time.Second, func(m map[string]any) bool {
			return m["type"] == "new_order" && m["order_id"] == orderID
		})
		if num(m, "total_tiyin") != num(o, "total_tiyin") {
			t.Fatalf("xabardagi jami %d, buyurtma jami %d", num(m, "total_tiyin"), num(o, "total_tiyin"))
		}
	})

	w.Step(t, "qabul qilindi: mijoz, restoran xabar oladi; kuryerga taklif keladi", func(t *testing.T) {
		w.Do(t, w.Owner, "POST", "/orders/"+orderID+"/transition",
			map[string]any{"to": "accepted", "preparation_minutes": 1}).Expect(t, 200)
		cust.Wait(t, "mijozga 'qabul qilindi'", 3*time.Second, statusIs(orderID, "accepted"))
		owner.Wait(t, "restoranga holat o'zgarishi", 3*time.Second, statusIs(orderID, "accepted"))
		courierFeed.Wait(t, "kuryerga taklif", 5*time.Second, func(m map[string]any) bool {
			return m["order_id"] == orderID && m["expires_in_sec"] != nil
		})
		w.Do(t, courier, "POST", "/couriers/"+courier.Entity+"/respond",
			map[string]any{"order_id": orderID, "accepted": true}).Expect(t, 200)
		cust.Wait(t, "mijozga 'kuryer topildi'", 5*time.Second, func(m map[string]any) bool {
			return m["type"] == "courier_assigned" && m["order_id"] == orderID
		})
		owner.Wait(t, "restoranga 'kuryer topildi'", 5*time.Second, func(m map[string]any) bool {
			return m["type"] == "courier_assigned" && m["order_id"] == orderID
		})
	})

	w.Step(t, "yo'lda: kuryer joylashuvi mijozga jonli yetib boradi", func(t *testing.T) {
		for _, to := range []string{"preparing", "ready"} {
			w.Do(t, w.Owner, "POST", "/orders/"+orderID+"/transition", map[string]any{"to": to}).Expect(t, 200)
		}
		w.Do(t, courier, "POST", "/orders/"+orderID+"/transition", map[string]any{"to": "picked_up"}).Expect(t, 200)
		cust.Wait(t, "mijozga 'olindi'", 3*time.Second, statusIs(orderID, "picked_up"))
		w.Do(t, courier, "POST", "/couriers/"+courier.Entity+"/location",
			map[string]any{"lat": ChustLat + 0.0005, "lng": ChustLng + 0.0005}).Expect(t, 200)
		cust.Wait(t, "mijozga kuryer joylashuvi", 3*time.Second, func(m map[string]any) bool {
			return m["type"] == "courier_location" && m["order_id"] == orderID
		})
		w.Do(t, courier, "POST", "/orders/"+orderID+"/transition", map[string]any{"to": "delivered"}).Expect(t, 200)
		cust.Wait(t, "mijozga 'yetkazildi'", 3*time.Second, statusIs(orderID, "delivered"))
	})

	w.Step(t, "begona restoran va begona mijoz bu buyurtma haqida HECH NARSA olmaydi", func(t *testing.T) {
		Rec.Checked(InvTenantIsolation)
		otherCust.Never(t, "begona mijoz", 300*time.Millisecond, orderIs(orderID))
		otherOwner.Never(t, "begona restoran", 0, orderIs(orderID))
		// Kuryerga boshqa mijozning ma'lumoti sizmaydi: mijoz telefoni yo'q.
		for _, m := range courierFeed.Dump() {
			if contains(mustJSON(m), w.Customer.Phone) {
				w.Violate(t, InvTenantIsolation, "kuryer oqimida mijoz telefoni: %v", m)
			}
		}
	})
}
