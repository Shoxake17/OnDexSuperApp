package orders

import "time"

// Status — buyurtma holati. Holatlar orasidagi o'tishlar statemachine.go da qat'iy belgilangan.
type Status string

const (
	StatusCreated   Status = "created"    // mijoz yaratdi, restoran hali ko'rmadi
	StatusAccepted  Status = "accepted"   // restoran qabul qildi
	StatusPreparing Status = "preparing"  // tayyorlanmoqda
	StatusReady     Status = "ready"      // tayyor, kuryer olib ketishi mumkin
	StatusPickedUp  Status = "picked_up"  // kuryer oldi, yo'lda
	StatusDelivered Status = "delivered"  // yetkazildi (terminal)
	StatusRejected  Status = "rejected"   // restoran rad etdi (terminal)
	StatusCancelled Status = "cancelled"  // bekor qilindi (terminal)
)

// Actor — holatni kim o'zgartirmoqchi. Har bir o'tish faqat ruxsat etilgan aktorlarga ochiq.
type Actor string

const (
	ActorCustomer   Actor = "customer"
	ActorRestaurant Actor = "restaurant"
	ActorCourier    Actor = "courier"
	ActorAdmin      Actor = "admin"
	ActorSystem     Actor = "system"
)

type Item struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
	Qty       int    `json:"qty"`
	PriceTiyin int64 `json:"price_tiyin"` // pul har doim tiyinda, float ishlatilmaydi
}

type StatusChange struct {
	From Status    `json:"from"`
	To   Status    `json:"to"`
	By   Actor     `json:"by"`
	At   time.Time `json:"at"`
}

type Order struct {
	ID           string         `json:"id"`
	CustomerID   string         `json:"customer_id"`
	RestaurantID string         `json:"restaurant_id"`
	CourierID    string         `json:"courier_id,omitempty"` // dispatch muvaffaqiyatli bo'lganda to'ladi
	Items        []Item         `json:"items"`
	TotalTiyin   int64          `json:"total_tiyin"`
	Status       Status         `json:"status"`
	History      []StatusChange `json:"history"`
	DeliveryLat  float64        `json:"delivery_lat"`
	DeliveryLng  float64        `json:"delivery_lng"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

func (o *Order) IsTerminal() bool {
	switch o.Status {
	case StatusDelivered, StatusRejected, StatusCancelled:
		return true
	}
	return false
}
