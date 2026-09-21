package stats

import (
	"context"
	"time"

	"chustapp/internal/orders"
)

// ─── Platforma statistikasi (superadmin paneli) ─────────────────────────
//
// Restoran statistikasi (`stats.go`) BITTA restoranning qatorlarini xotirada
// hisoblaydi. Superadmin esa BARCHA restoranlarni ko'radi: har biri uchun
// qatorlarni o'qish bir necha yuz ming qatorni serverga ko'chirardi. Shuning
// uchun bu yerda hisob ombor ichida (SQL agregatsiya) bajariladi va faqat
// yig'indilar qaytadi.
//
// Tasniflash (`classify`) restoran statistikasi bilan AYNAN bir xil:
// bajarilgan = yetkazildi + stolga xizmat qilindi (pul tushgan), bekor =
// rad etildi + bekor qilindi, qolgani "jarayonda".

// PlatformQuery — bitta so'rov: asosiy davr va kunlik grafik oynasi.
type PlatformQuery struct {
	// From <= created_at < To — asosiy davr (jadval, kartalar, holatlar).
	From, To time.Time
	// DailyFrom dan boshlab DailyDays ta KUN — grafik. Davrdan mustaqil:
	// admin "Bugun"ni tanlasa ham grafikda so'nggi kunlar ko'rinadi.
	DailyFrom time.Time
	DailyDays int
}

// RestaurantTotals — bitta restoranning davrdagi buyurtma yig'indilari.
type RestaurantTotals struct {
	RestaurantID string `json:"restaurant_id"`
	Orders       int    `json:"orders"`
	// New — restoran hali ko'rmagan (`created`).
	New int `json:"new"`
	// InProgress — qabul qilingan, lekin tugamagan (qabul qilindi, tayyorlanmoqda,
	// tayyor, yo'lda). `New` bunga KIRMAYDI.
	InProgress int `json:"in_progress"`
	// Completed — yetkazildi + stolga xizmat qilindi.
	Completed int `json:"completed"`
	// Cancelled — rad etildi + bekor qilindi.
	Cancelled int `json:"cancelled"`
	// RevenueTiyin — FAQAT bajarilgan buyurtmalar summasi.
	RevenueTiyin int64 `json:"revenue_tiyin"`
}

// Accepted — restoran QABUL QILGAN buyurtmalar: qabul qilinib bekor/rad
// etilmagan (jarayonda + bajarilgan). Qabul qilingandan keyin bekor bo'lgan
// buyurtma bu yerda hisoblanmaydi: tarixdan ajratib bo'lmaydi va u
// "bekor" ustunida ko'rinadi.
func (t RestaurantTotals) Accepted() int { return t.InProgress + t.Completed }

// Add — yig'indiga qo'shadi (jami qator uchun).
func (t *RestaurantTotals) Add(o RestaurantTotals) {
	t.Orders += o.Orders
	t.New += o.New
	t.InProgress += o.InProgress
	t.Completed += o.Completed
	t.Cancelled += o.Cancelled
	t.RevenueTiyin += o.RevenueTiyin
}

// DayTotals — bitta kun.
type DayTotals struct {
	// Date — "2006-01-02" ([Location] dagi kun).
	Date         string `json:"date"`
	Orders       int    `json:"orders"`
	Completed    int    `json:"completed"`
	RevenueTiyin int64  `json:"revenue_tiyin"`
}

// PlatformData — ombor javobi.
type PlatformData struct {
	// Restaurants — faqat buyurtmasi bor restoranlar (yo'qlari nol hisoblanadi;
	// ularni chaqiruvchi katalogdan to'ldiradi).
	Restaurants []RestaurantTotals
	// Statuses — holat -> soni (asosiy davr).
	Statuses map[string]int
	// Daily — DailyDays ta element, kunlar to'liq (buyurtmasiz kun — nollar).
	Daily []DayTotals
}

// PlatformSource — platforma agregatsiyasi. `orders.Repository` ning bir
// qismi EMAS: bu ixtiyoriy imkoniyat, uni ombor bo'lmasa handler 501 emas,
// bo'sh natija bilan javob beradi.
type PlatformSource interface {
	PlatformStats(ctx context.Context, q PlatformQuery) (PlatformData, error)
}

// NewDaily — DailyDays ta bo'sh kun.
func NewDaily(from time.Time, days int) []DayTotals {
	out := make([]DayTotals, days)
	start := dayOf(from)
	for i := range out {
		out[i].Date = start.AddDate(0, 0, i).Format("2006-01-02")
	}
	return out
}

// DayIndex — vaqt qaysi kunga tushadi (-1 — oynadan tashqarida).
func DayIndex(from time.Time, days int, t time.Time) int {
	start := dayOf(from)
	if t.Before(start) {
		return -1
	}
	i := int(dayOf(t).Sub(start).Hours()/24 + 0.5)
	if i < 0 || i >= days {
		return -1
	}
	return i
}

// ClassifyStatus — holatni hisob guruhiga ajratadi (ombor implementatsiyalari
// uchun): "completed", "cancelled", "new" yoki "in_progress".
func ClassifyStatus(s orders.Status) string {
	if s == orders.StatusCreated {
		return "new"
	}
	switch classify(s) {
	case completed:
		return "completed"
	case cancelled:
		return "cancelled"
	default:
		return "in_progress"
	}
}
