// Package alerts — restoran paneli "Bildirishnomalar" markazi.
//
// ┌─ NIMA BU VA NEGA ALOHIDA ─────────────────────────────────────────┐
// `notify` — FOYDALANUVCHIga shaxsiy xabar (mijoz, kuryer) va jonli
// ish signallari. Bu paket esa RESTORANning o'z bildirishnomalar
// tarixi: yangi buyurtma, to'lov, xodimlar o'zgarishi, eslatma, kunlik
// hisobot. Ular restoran egasiga tegishli (affitsiantga EMAS) va panel
// yopiq bo'lsa ham saqlanib qolishi kerak.
//
// Hammasi HAQIQIY hodisalardan yasaladi — soxta/namuna xabar yo'q.
// └───────────────────────────────────────────────────────────────────┘
package alerts

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type Category string

const (
	CategoryNew       Category = "new"
	CategorySuccess   Category = "success"
	CategoryInfo      Category = "info"
	CategoryImportant Category = "important"
	CategoryReport    Category = "report"
	CategoryUpdate    Category = "update"
	CategoryActivity  Category = "activity"
	CategoryReminder  Category = "reminder"
)

// CategoryMeta — tur kaliti va o'zbekcha nomi (panel filtri uchun).
type CategoryMeta struct {
	Key   Category `json:"key"`
	Title string   `json:"title"`
}

var categories = []CategoryMeta{
	{CategoryNew, "Yangi"},
	{CategorySuccess, "Muvaffaqiyatli"},
	{CategoryInfo, "Ma'lumot"},
	{CategoryImportant, "Muhim"},
	{CategoryReport, "Hisobot"},
	{CategoryUpdate, "Yangilanish"},
	{CategoryActivity, "Faoliyat"},
	{CategoryReminder, "Eslatma"},
}

// Categories — panel filtri va taqsimot kartasi tartibi.
func Categories() []CategoryMeta { return append([]CategoryMeta(nil), categories...) }

func (c Category) Title() string {
	for _, x := range categories {
		if x.Key == c {
			return x.Title
		}
	}
	return string(c)
}

// ParseCategory — bo'sh qiymat "barchasi".
func ParseCategory(s string) (Category, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	for _, x := range categories {
		if string(x.Key) == s {
			return x.Key, nil
		}
	}
	return "", invalid("bildirishnoma turi noto'g'ri")
}

// Hodisa turlari (panel ikonka va o'tish joyini shunga qarab tanlaydi).
const (
	KindNewOrder        = "new_order"
	KindOrderCancelled  = "order_cancelled"
	KindOrderWaiting    = "order_waiting"
	KindPaymentReceived = "payment_received"
	KindDispatchFailed  = "dispatch_failed"
	KindCourierNotFound = "courier_not_found"
	KindStaffAdded      = "staff_added"
	KindStaffStatus     = "staff_status"
	KindStaffPosition   = "staff_position"
	KindDailyReport     = "daily_report"
	KindPlatformUpdate  = "platform_update"
)

const (
	MaxTitleLen  = 120
	MaxBodyLen   = 600
	MaxSearchLen = 100
	MaxLimit     = 100
	// Retention — shundan eski bildirishnomalar avtomatik o'chiriladi.
	Retention = 90 * 24 * time.Hour
)

// dataKeys — `Data` da ruxsat etilgan kalitlar. Ular faqat panelda
// kerakli sahifaga o'tish uchun; boshqa hech narsa (telefon, summa
// tafsiloti) bu yerga yozilmaydi.
var dataKeys = map[string]bool{"order_id": true, "order_number": true, "staff_id": true, "date": true}

type Notification struct {
	ID           string
	RestaurantID string
	// Seq — o'suvchi tartib raqami. Jonli kanal uzilib qolsa panel
	// "shu raqamdan keyingilarini ber" deb so'raydi — hech narsa
	// tushib qolmaydi.
	Seq       int64
	Kind      string
	Category  Category
	Title     string
	Body      string
	Data      map[string]string
	DedupeKey string
	ReadAt    *time.Time
	CreatedAt time.Time
}

// View — HTTP va WebSocket javobi (ichki `DedupeKey` chiqmaydi).
type View struct {
	ID            string            `json:"id"`
	Seq           int64             `json:"seq"`
	Kind          string            `json:"kind"`
	Category      Category          `json:"category"`
	CategoryTitle string            `json:"category_title"`
	Title         string            `json:"title"`
	Body          string            `json:"body"`
	Data          map[string]string `json:"data"`
	Read          bool              `json:"read"`
	CreatedAt     time.Time         `json:"created_at"`
}

func ToView(n *Notification) View {
	data := n.Data
	if data == nil {
		data = map[string]string{}
	}
	return View{ID: n.ID, Seq: n.Seq, Kind: n.Kind, Category: n.Category, CategoryTitle: n.Category.Title(),
		Title: n.Title, Body: n.Body, Data: data, Read: n.ReadAt != nil, CreatedAt: n.CreatedAt}
}

type Query struct {
	Category Category
	Since    time.Time
	Search   string
	// BeforeSeq — sahifalash (eskiroqlari); AfterSeq — qayta ulanganda
	// o'tkazib yuborilganlar (o'sish tartibida qaytadi).
	BeforeSeq  int64
	AfterSeq   int64
	UnreadOnly bool
	Limit      int
}

type Counts struct {
	Total      int              `json:"total"`
	Unread     int              `json:"unread"`
	ByCategory map[Category]int `json:"by_category"`
}

type Store interface {
	// Insert — `Seq` ni beradi. Shu restoranda `DedupeKey` allaqachon
	// bo'lsa hech narsa yozmaydi va `false` qaytaradi (bir hodisa —
	// bitta bildirishnoma, qayta urinishlar va takroriy callback'lar
	// ikkinchisini yaratmaydi).
	Insert(ctx context.Context, n *Notification) (bool, error)
	List(ctx context.Context, restaurantID string, q Query) ([]*Notification, error)
	// Counts — `since` dan beri (nol — hammasi).
	Counts(ctx context.Context, restaurantID string, since time.Time) (Counts, error)
	UnreadCount(ctx context.Context, restaurantID string) (int, error)
	LatestSeq(ctx context.Context, restaurantID string) (int64, error)
	// MarkRead — faqat SHU restoranning yozuvi; o'zgargan bo'lsa true.
	MarkRead(ctx context.Context, restaurantID, id string, at time.Time) (bool, error)
	// MarkAllRead — `upToSeq` gacha (panel ko'rgan oxirgi raqam): o'qish
	// tugmasi bosilayotgan paytda kelgan yangi xabar o'qilmagan qoladi.
	MarkAllRead(ctx context.Context, restaurantID string, upToSeq int64, at time.Time) (int, error)
	DeleteOlderThan(ctx context.Context, before time.Time) (int, error)
}

// ValidationError — foydalanuvchi kiritmasidagi xato (400).
type ValidationError struct{ Msg string }

func (e ValidationError) Error() string { return e.Msg }

func invalid(msg string) error { return ValidationError{Msg: msg} }

func IsValidation(err error) bool {
	var ve ValidationError
	return errors.As(err, &ve)
}

// sanitize — tizim yasagan matn: boshqaruv belgilari olib tashlanadi,
// uzunlik chegaralanadi (xodim ismi kabi qismlar uzun bo'lishi mumkin).
func sanitize(s string, max int) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		if r == '\n' || !(unicode.IsControl(r) || unicode.Is(unicode.Cf, r)) {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if utf8.RuneCountInString(out) > max {
		out = string([]rune(out)[:max-1]) + "…"
	}
	return out
}

// ValidateText — ADMIN kiritgan matn: qat'iy tekshiruv (kesib tashlanmaydi).
func ValidateText(s string, max int, required bool, field string) (string, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
	if s == "" {
		if required {
			return "", invalid(field + " bo'sh bo'lishi mumkin emas")
		}
		return "", nil
	}
	for _, r := range s {
		if r != '\n' && (unicode.IsControl(r) || unicode.Is(unicode.Cf, r)) {
			return "", invalid(field + "da ko'rinmas boshqaruv belgilari bo'lishi mumkin emas")
		}
	}
	if utf8.RuneCountInString(s) > max {
		return "", invalid(fmt.Sprintf("%s %d belgidan oshmasligi kerak", field, max))
	}
	return s, nil
}

// Location — Toshkent (UTC+5, yozgi vaqt yo'q).
var Location = time.FixedZone("Asia/Tashkent", 5*60*60)

// PeriodSince — panel filtri: "", "all", "today", "week", "month".
func PeriodSince(period string, now time.Time) (time.Time, error) {
	local := now.In(Location)
	switch strings.TrimSpace(period) {
	case "", "all":
		return time.Time{}, nil
	case "today":
		return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, Location), nil
	case "week":
		return now.Add(-7 * 24 * time.Hour), nil
	case "month":
		return now.Add(-30 * 24 * time.Hour), nil
	}
	return time.Time{}, invalid("davr noto'g'ri (all, today, week, month)")
}

var uzMonths = [...]string{"yanvar", "fevral", "mart", "aprel", "may", "iyun",
	"iyul", "avgust", "sentabr", "oktabr", "noyabr", "dekabr"}

// formatSum — 18000000 tiyin → "180 000".
func formatSum(tiyin int64) string {
	neg := tiyin < 0
	if neg {
		tiyin = -tiyin
	}
	s := fmt.Sprintf("%d", tiyin/100)
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}
