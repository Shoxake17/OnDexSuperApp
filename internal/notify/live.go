package notify

import (
	"time"

	"chustapp/internal/orders"
	"chustapp/internal/ws"
)

// Live — eventlarni ham logga, ham WebSocket orqali aloqador tomonlarga yuboradi.
// Keyingi bosqichda shu yerga FCM push ham qo'shiladi (ilova yopiq bo'lganda).
type Live struct {
	hub *ws.Hub
}

func NewLive(hub *ws.Hub) *Live { return &Live{hub: hub} }

func (l *Live) OrderStatusChanged(o *orders.Order, from orders.Status) {
	LogNotifier{}.OrderStatusChanged(o, from)
	event := map[string]any{
		"type":       "order_status",
		"order_id":   o.ID,
		"from":       from,
		"status":     o.Status,
		"courier_id": o.CourierID,
	}
	l.hub.Send(o.CustomerID, event)   // mijoz (user ID)
	l.hub.Send(o.RestaurantID, event) // restoran paneli (entity ID)
	l.hub.Send(o.CourierID, event)    // kuryer ilovasi (entity ID)
}

func (l *Live) SendOffer(courierID, orderID string, expiresIn time.Duration) {
	LogNotifier{}.SendOffer(courierID, orderID, expiresIn)
	l.hub.Send(courierID, map[string]any{
		"type":           "offer",
		"order_id":       orderID,
		"expires_in_sec": int(expiresIn.Seconds()),
	})
}

func (l *Live) CancelOffer(courierID, orderID string) {
	LogNotifier{}.CancelOffer(courierID, orderID)
	l.hub.Send(courierID, map[string]any{
		"type":     "offer_cancelled",
		"order_id": orderID,
	})
}
