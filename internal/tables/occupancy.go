package tables

import (
	"context"
	"time"
)

// Status — joyning hozirgi holati (panelda "Band", "Bo'sh"...).
//
// ┌─ HOLAT SAQLANMAYDI, HISOBLANADI ──────────────────────────────────┐
// "Band" ustunini bazada saqlash — ikki haqiqat manbai: buyurtma
// yopilganda uni tushirishni bir joyda unutish yetarli va stol abadiy
// "band" bo'lib qolardi. Shuning uchun "band" — shu joyga bog'langan
// YAKUNLANMAGAN stol buyurtmasi borligi, har so'rovda buyurtmalardan.
//
// Faqat "tozalanmoqda" — xodimning qo'lda qo'ygan belgisi (buyurtmadan
// bilib bo'lmaydi: mehmon qachon turib ketgani tizimga kelmaydi).
// └───────────────────────────────────────────────────────────────────┘
type Status string

const (
	StatusAvailable Status = "available"
	StatusOccupied  Status = "occupied"
	StatusCleaning  Status = "cleaning"
	StatusInactive  Status = "inactive"
)

// OrderSnapshot — joyga bog'langan buyurtmaning yengil ko'rinishi.
// Mijoz haqida hech narsa yo'q: panelning joylar sahifasiga kerak emas.
type OrderSnapshot struct {
	ID          string    `json:"id"`
	OrderNumber string    `json:"order_number"`
	TableID     string    `json:"-"`
	Status      string    `json:"status"`
	Items       int       `json:"items"`
	TotalTiyin  int64     `json:"total_tiyin"`
	CreatedAt   time.Time `json:"created_at"`
}

// OrdersSource — buyurtmalar omboridan joylar uchun ma'lumot.
type OrdersSource interface {
	// ActiveDineInOrders — restoranning yakunlanmagan stol buyurtmalari
	// (to'lanmagan karta buyurtmalari kirmaydi), eng yangisi birinchi.
	ActiveDineInOrders(ctx context.Context, restaurantID string) ([]OrderSnapshot, error)
	// LatestDineInOrders — har bir joyning ENG OXIRGI stol buyurtmasi
	// (holatidan qat'i nazar). Buyurtmasi yo'q joy xaritada bo'lmaydi.
	LatestDineInOrders(ctx context.Context, restaurantID string, tableIDs []string) (map[string]OrderSnapshot, error)
}

// StatusOf — joy holati.
//
//   - yakunlanmagan buyurtma bor — band (joy yopilgan bo'lsa ham: ichida
//     mehmon o'tiribdi);
//   - yopilgan — faol emas;
//   - "tozalanmoqda" belgisidan KEYIN yangi buyurtma kelmagan — tozalanmoqda
//     (keyin kelgan bo'lsa belgi eskirgan: joyni kimdir egallagan);
//   - aks holda — bo'sh.
func StatusOf(t *Table, active []OrderSnapshot, last *OrderSnapshot) Status {
	if len(active) > 0 {
		return StatusOccupied
	}
	if !t.Active {
		return StatusInactive
	}
	if t.CleaningSince != nil && (last == nil || !last.CreatedAt.After(*t.CleaningSince)) {
		return StatusCleaning
	}
	return StatusAvailable
}
