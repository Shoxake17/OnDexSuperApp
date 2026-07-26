// Package notify — xabar yuborish qatlami. Hozircha log'ga yozadi;
// keyingi qadam: FCM push + WebSocket hub shu interface'lar orqasida.
package notify

import (
	"log/slog"
	"time"

	"chustapp/internal/orders"
)

type LogNotifier struct{}

func (LogNotifier) OrderStatusChanged(o *orders.Order, from orders.Status) {
	slog.Info("notify: buyurtma holati o'zgardi",
		"order", o.ID, "from", from, "to", o.Status, "courier", o.CourierID)
}

func (LogNotifier) SendOffer(courierID, orderID string, expiresIn time.Duration) {
	slog.Info("notify: kuryerga taklif yuborildi",
		"courier", courierID, "order", orderID, "expires_in", expiresIn)
}

func (LogNotifier) CancelOffer(courierID, orderID string) {
	slog.Info("notify: taklif bekor qilindi", "courier", courierID, "order", orderID)
}
