// Package tracking — yetkazish yo'li: buyurtmaga biriktiriladigan A→B
// marshruti (A — restoran, B — mijoz manzili).
//
// ┌─ NEGA BAZADA ──────────────────────────────────────────────────────┐
// Mijoz buyurtmani olgach ham kuzatuv sahifasida o'sha yo'l ko'rinib
// turishi kerak (foydalanuvchi talabi, 2026-09-15). Har ochilganda Google
// Directions'dan qayta so'rash PULLIK va natija vaqt o'tib o'zgarishi
// mumkin (yo'l yopilgan, restoran ko'chgan) — tarix esa o'zgarmasligi
// kerak. Shuning uchun yo'l BIR MARTA hisoblanib saqlanadi.
// └────────────────────────────────────────────────────────────────────┘
package tracking

import (
	"context"
	"errors"
	"time"
)

// Point — geografik nuqta.
type Point struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// MaxRoutePoints — bitta marshrutdagi eng ko'p nuqta. Google "overview"
// polilinasi odatda bir necha yuz nuqta; chegara buzilgan yoki g'ayritabiiy
// javob bazani va mijoz xotirasini to'ldirmasligi uchun.
const MaxRoutePoints = 2000

// Route — buyurtmaning yetkazish yo'li.
type Route struct {
	OrderID         string
	Origin          Point
	Destination     Point
	Points          []Point
	DistanceMeters  int
	DurationSeconds int
	CreatedAt       time.Time
}

// ErrNotFound — buyurtma uchun yo'l hali saqlanmagan.
var ErrNotFound = errors.New("yetkazish yo'li saqlanmagan")

// Validate — saqlashdan oldingi tekshiruv.
func (r *Route) Validate() error {
	switch {
	case r == nil || r.OrderID == "":
		return errors.New("yetkazish yo'li: order_id bo'sh")
	case len(r.Points) > MaxRoutePoints:
		return errors.New("yetkazish yo'li: nuqtalar juda ko'p")
	case r.DistanceMeters < 0 || r.DurationSeconds < 0:
		return errors.New("yetkazish yo'li: masofa yoki vaqt manfiy")
	}
	return nil
}

// Repository — yo'llar ombori (Postgres yoki xotira).
type Repository interface {
	// GetRoute — topilmasa `ErrNotFound`.
	GetRoute(ctx context.Context, orderID string) (*Route, error)
	// SaveRoute — BIRINCHI yozuv qoladi: yo'l buyurtma tarixining bir
	// qismi. Takroriy chaqiruv (masalan ikki so'rov bir vaqtda hisobladi)
	// xato qaytarmaydi va saqlanganini o'zgartirmaydi.
	SaveRoute(ctx context.Context, r *Route) error
}

// Downsample — nuqtalar `MaxRoutePoints` dan ko'p bo'lsa teng qadam bilan
// siyraklashtiradi; birinchi va oxirgi nuqta DOIM qoladi (A va B ga
// tegib turishi uchun).
func Downsample(points []Point) []Point {
	if len(points) <= MaxRoutePoints {
		return points
	}
	step := (len(points) + MaxRoutePoints - 2) / (MaxRoutePoints - 1)
	out := make([]Point, 0, MaxRoutePoints)
	for i := 0; i < len(points)-1; i += step {
		out = append(out, points[i])
	}
	return append(out, points[len(points)-1])
}
