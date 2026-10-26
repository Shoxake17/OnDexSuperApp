// Package ratings — mijoz buyurtma tugaganidan keyin restoranni
// baholaydi (1-5 yulduz). Yetkazib berish tugmachalarini bosib
// chiqadigan marketplace ilovalaridagi kabi: "Qalay bo'ldi?" bloki.
//
// ┌─ NEGA BITTA BUYURTMAGA BITTA BAHO ──────────────────────────────────┐
// `order_id` UNIQUE (`internal/storage/migrations/0060_restaurant_
// ratings.sql`). Mijoz bitta buyurtmani necha marta ochmasin, faqat
// BITTA baho qoladi — aks holda bitta mijoz bitta tajribani ko'p marta
// hisoblab, o'rtacha reytingni sun'iy siljitib yuborishi mumkin edi.
// └───────────────────────────────────────────────────────────────────┘
//
// ┌─ O'RTACHA VA SONI — HAR DOIM HAQIQIY HISOB ─────────────────────────┐
// `catalog.Restaurant.Rating`/`RatingCount` ENDI qo'lda kiritilmaydi
// (ROADMAP'da uzoq vaqt rejalashtirilgan band — ko'ring `catalog.go`
// dagi tarixiy izoh). Ular `internal/httpapi/scene_access.go` da HAR
// SO'ROVDA shu paketning `Summary` metodidan qayta hisoblanadi.
// └───────────────────────────────────────────────────────────────────┘
package ratings

import (
	"context"
	"errors"
	"time"
)

// ErrAlreadyRated — shu buyurtma uchun baho allaqachon qo'yilgan.
var ErrAlreadyRated = errors.New("bu buyurtma uchun baho allaqachon qo'yilgan")

// ErrNotFound — baho topilmadi.
var ErrNotFound = errors.New("baho topilmadi")

type Rating struct {
	ID           string `json:"id"`
	OrderID      string `json:"order_id"`
	RestaurantID string `json:"restaurant_id"`
	CustomerID   string `json:"customer_id"`
	// Stars — 1 dan 5 gacha (tekshiruv `routes_ratings.go` va baza
	// CHECK cheklovida — ikkala qatlamda ham, bitta yetarli emas).
	Stars     int       `json:"stars"`
	CreatedAt time.Time `json:"created_at"`
}

type Repository interface {
	// Create — `order_id` UNIQUE buzilsa `ErrAlreadyRated` qaytaradi.
	Create(ctx context.Context, rt *Rating) error
	// GetByOrder — shu buyurtma uchun baho (mijoz ilovasi "baholandi"
	// holatini bilish uchun, qayta so'ralmasligi uchun). Topilmasa
	// `ErrNotFound`.
	GetByOrder(ctx context.Context, orderID string) (*Rating, error)
	// Summary — restoran uchun o'rtacha va soni. Baho yo'q bo'lsa
	// (0, 0, nil) — "0.0 ★" emas, chaqiruvchi buni "ko'rsatkich yo'q"
	// deb talqin qiladi (`catalog.go` dagi 0 = "ma'lumot yo'q" qoidasi).
	Summary(ctx context.Context, restaurantID string) (avg float64, count int, err error)
}
