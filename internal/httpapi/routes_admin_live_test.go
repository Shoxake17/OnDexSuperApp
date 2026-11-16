package httpapi

import (
	"testing"
	"time"

	"chustapp/internal/orders"
)

// RS-6: kechikish qoidalari.
func TestOrderDelayRules(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	at := func(m int) time.Time { return now.Add(-time.Duration(m) * time.Minute) }
	mk := func(st orders.Status, typ orders.Type, prep int, hist ...orders.StatusChange) *orders.Order {
		return &orders.Order{ID: "o", Status: st, Type: typ, PreparationMinutes: prep, CreatedAt: at(120), History: hist}
	}
	cases := []struct {
		name string
		o    *orders.Order
		want bool
	}{
		{"yangi 3 daq", mk(orders.StatusCreated, "", 0, orders.StatusChange{To: orders.StatusCreated, At: at(3)}), false},
		{"yangi 6 daq", mk(orders.StatusCreated, "", 0, orders.StatusChange{To: orders.StatusCreated, At: at(6)}), true},
		{"tayyorlash 20+10 ichida", mk(orders.StatusPreparing, "", 20,
			orders.StatusChange{To: orders.StatusAccepted, At: at(25)}, orders.StatusChange{To: orders.StatusPreparing, At: at(20)}), false},
		{"tayyorlash 20+10 o'tdi", mk(orders.StatusPreparing, "", 20,
			orders.StatusChange{To: orders.StatusAccepted, At: at(35)}, orders.StatusChange{To: orders.StatusPreparing, At: at(5)}), true},
		{"tayyor yetkazish 11 daq", mk(orders.StatusReady, orders.TypeDelivery, 10, orders.StatusChange{To: orders.StatusReady, At: at(11)}), true},
		{"tayyor stol 11 daq", mk(orders.StatusReady, orders.TypeDineIn, 10, orders.StatusChange{To: orders.StatusReady, At: at(11)}), false},
		{"tayyor stol 16 daq", mk(orders.StatusReady, orders.TypeDineIn, 10, orders.StatusChange{To: orders.StatusReady, At: at(16)}), true},
		{"yo'lda 61 daq", mk(orders.StatusPickedUp, "", 10, orders.StatusChange{To: orders.StatusPickedUp, At: at(61)}), true},
	}
	for _, c := range cases {
		d, got := orderDelay(c.o, now)
		if got != c.want {
			t.Errorf("%s: %v (%+v)", c.name, got, d)
		}
		if got && d.Reason == "" {
			t.Errorf("%s: sabab yo'q", c.name)
		}
	}
}
