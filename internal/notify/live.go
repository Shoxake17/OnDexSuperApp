package notify

import (
	"context"
	"fmt"

	"chustapp/internal/couriers"
	"chustapp/internal/orders"
)

// Live — `orders.Notifier` va `couriers.OfferNotifier` interfeyslarini
// UMUMIY bildirishnoma xizmati ustida bajaradi.
//
// ┌─ BU QATLAM NEGA BOR ──────────────────────────────────────────────┐
// `Service` modulga bog'liq EMAS (`Event` bilan ishlaydi). `orders` va
// `couriers` esa o'z interfeyslarini talab qiladi. Shu ikkisi
// o'rtasidagi tarjimon — mana shu fayl.
//
// Yangi modul qo'shilganda `Service` ga TEGILMAYDI: o'sha modul
// shunchaki `Notify(ctx, userID, Event{Module: "shop", ...})` ni
// chaqiradi.
// └───────────────────────────────────────────────────────────────────┘
type Live struct {
	svc *Service
}

func NewLive(svc *Service) *Live { return &Live{svc: svc} }

// ---------- orders.Notifier ----------

func (l *Live) OrderCreated(o *orders.Order) {
	LogNotifier{}.OrderCreated(o)
	// Restoran xodimlariga — ENTITY kanali (kim ishlab tursa, o'sha
	// eshitadi), shaxsiy emas.
	l.svc.Broadcast(Entity(ModuleFood, o.RestaurantID), map[string]any{
		"type":        "new_order",
		"order_id":    o.ID,
		"total_tiyin": o.TotalTiyin,
	})
}

func (l *Live) OrderStatusChanged(o *orders.Order, from orders.Status) {
	LogNotifier{}.OrderStatusChanged(o, from)

	event := map[string]any{
		"type":       "order_status",
		"order_id":   o.ID,
		"from":       from,
		"status":     o.Status,
		"courier_id": o.CourierID,
	}
	// Restoran va kuryer — ish kanallari (jonli, tarixsiz).
	l.svc.Broadcast(Entity(ModuleFood, o.RestaurantID), event)
	l.svc.Broadcast(Entity(ModuleFood, o.CourierID), event)

	// MIJOZGA — shaxsiy bildirishnoma: saqlanadi va ilova yopiq
	// bo'lsa push ketadi. Buyurtma holati — foydalanuvchi
	// O'TKAZIB YUBORMASLIGI kerak bo'lgan yagona narsa.
	ctx := context.Background()
	l.svc.Notify(ctx, o.CustomerID, Event{
		Module: ModuleFood,
		Kind:   "order_status",
		Title:  "Buyurtma holati",
		Body:   orderStatusText(o),
		Data: map[string]string{
			"order_id": o.ID,
			"status":   string(o.Status),
		},
	})
}

// orderStatusText — foydalanuvchi ko'radigan matn (push'da ham shu).
func orderStatusText(o *orders.Order) string {
	switch o.Status {
	case orders.StatusAccepted:
		return "Restoran buyurtmangizni qabul qildi"
	case orders.StatusPreparing:
		return "Buyurtmangiz tayyorlanmoqda"
	case orders.StatusReady:
		return "Buyurtmangiz tayyor"
	case orders.StatusPickedUp:
		return "Kuryer buyurtmangizni oldi"
	case orders.StatusDelivered:
		return "Buyurtmangiz yetkazildi"
	case orders.StatusCancelled:
		return "Buyurtma bekor qilindi"
	case orders.StatusRejected:
		return "Restoran buyurtmani rad etdi"
	default:
		return fmt.Sprintf("Buyurtma holati: %s", o.Status)
	}
}

// ---------- couriers.OfferNotifier ----------

// SendOffer — kuryerga taklif.
//
// SHAXSIY bildirishnoma sifatida ketadi: taklif 20 soniya amal
// qiladi va uni O'TKAZIB YUBORISH kuryer uchun yo'qotilgan pul
// demak. Ilova yopiq bo'lsa push bilan uyg'otiladi.
func (l *Live) SendOffer(courierID string, info couriers.OfferInfo) {
	LogNotifier{}.SendOffer(courierID, info.OrderID, info.ExpiresIn)

	// Jonli kanal — taklif kartochkasi uchun to'liq kontekst.
	l.svc.Broadcast(Entity(ModuleFood, courierID), map[string]any{
		"type":                "offer",
		"order_id":            info.OrderID,
		"expires_in_sec":      int(info.ExpiresIn.Seconds()),
		"restaurant_id":       info.RestaurantID,
		"restaurant_name":     info.RestaurantName,
		"restaurant_address":  info.RestaurantAddress,
		"restaurant_lat":      info.RestaurantLat,
		"restaurant_lng":      info.RestaurantLng,
		"restaurant_logo_url": info.RestaurantLogoURL,
	})
}

func (l *Live) CancelOffer(courierID, orderID string) {
	LogNotifier{}.CancelOffer(courierID, orderID)
	l.svc.Broadcast(Entity(ModuleFood, courierID), map[string]any{
		"type":     "offer_cancelled",
		"order_id": orderID,
	})
}
