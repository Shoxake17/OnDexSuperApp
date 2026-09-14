package stats

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"

	"chustapp/internal/orders"
)

// ─── Buyurtmalar tarixi ("Barcha buyurtmalar" sahifasi) ────────────────
//
// ┌─ NEGA OFFSET EMAS, KURSOR ─────────────────────────────────────────┐
// `OFFSET 5000` bazani 5000 qatorni o'qib tashlashga majbur qiladi.
// Bundan ham yomoni: sahifalar orasida yangi buyurtma kelsa ro'yxat
// suriladi va bitta buyurtma ikki marta ko'rinadi (yoki umuman tushib
// qoladi). Kursor — oxirgi ko'rsatilgan buyurtmaning (created_at, id)
// juftligi: keyingi sahifa aynan undan ESKILARIDAN boshlanadi.
// └────────────────────────────────────────────────────────────────────┘

const (
	// HistoryDefaultLimit — `limit` berilmaganda bir sahifadagi buyurtmalar.
	HistoryDefaultLimit = 30
	// HistoryMaxLimit — bir sahifada eng ko'p buyurtma. Har biriga mijoz
	// telefoni qo'shiladi, ya'ni katta sahifa — ko'p qo'shimcha so'rov.
	HistoryMaxLimit = 50

	maxCursorLen   = 256
	maxCursorIDLen = 128
)

// HistoryStatus — ro'yxat filtri. Guruhlar statistikadagi bilan AYNAN
// bir xil (`classify`): sahifadagi "Bajarilgan 12 ta" soni bilan
// statistika kartochkasidagi son bir qoidadan chiqadi.
type HistoryStatus string

const (
	HistoryAll        HistoryStatus = "all"
	HistoryInProgress HistoryStatus = "in_progress"
	HistoryCompleted  HistoryStatus = "completed"
	HistoryCancelled  HistoryStatus = "cancelled"
)

var (
	errBadHistoryStatus = errors.New("status noto'g'ri: all, in_progress, completed yoki cancelled bo'lishi kerak")
	errBadHistoryLimit  = errors.New("limit 1 dan " + strconv.Itoa(HistoryMaxLimit) + " gacha butun son bo'lishi kerak")
	errBadCursor        = errors.New("cursor noto'g'ri yoki buzilgan — ro'yxatni boshidan yuklang")
)

// ParseHistoryStatus — bo'sh qiymat "all" degani.
func ParseHistoryStatus(s string) (HistoryStatus, error) {
	switch v := HistoryStatus(strings.TrimSpace(s)); v {
	case "":
		return HistoryAll, nil
	case HistoryAll, HistoryInProgress, HistoryCompleted, HistoryCancelled:
		return v, nil
	}
	return "", errBadHistoryStatus
}

// ParseHistoryLimit — bo'sh qiymat [HistoryDefaultLimit] degani.
// Chegaradan tashqari son JIMGINA qisqartirilmaydi — aniq xato.
func ParseHistoryLimit(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return HistoryDefaultLimit, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > HistoryMaxLimit {
		return 0, errBadHistoryLimit
	}
	return n, nil
}

// Matches — buyurtma holati filtrga tushadimi (xotira ombori uchun;
// Postgres xuddi shu ro'yxatlarni SQL parametri sifatida oladi).
func (h HistoryStatus) Matches(s orders.Status) bool {
	switch h {
	case HistoryInProgress:
		return classify(s) == inProgress
	case HistoryCompleted:
		return classify(s) == completed
	case HistoryCancelled:
		return classify(s) == cancelled
	}
	return true
}

// TerminalStatusStrings — bajarilgan + bekor qilingan (SQL parametri:
// "jarayonda" = shulardan hech biri emas).
func TerminalStatusStrings() []string {
	return append(CompletedStatusStrings(), CancelledStatusStrings()...)
}

// Cursor — oxirgi ko'rsatilgan buyurtma. Tartib: created_at KAMAYISH,
// teng bo'lsa id KAMAYISH (bayt tartibida).
//
// Vaqt MIKROSEKUNDGACHA: Postgres `timestamptz` shu aniqlikda saqlaydi.
// Nanosekund bilan solishtirilsa, xotira omborida kursorning o'zi
// "o'zidan eskiroq" bo'lib chiqib, buyurtma ikki marta ko'rinardi.
type Cursor struct {
	CreatedAt time.Time
	ID        string
}

// CursorOf — buyurtmadan keyingi sahifa kursori.
func CursorOf(o *orders.Order) Cursor {
	return Cursor{CreatedAt: o.CreatedAt.Truncate(time.Microsecond).UTC(), ID: o.ID}
}

// Encode — klientga beriladigan shaffof bo'lmagan satr. Ichida sir yo'q
// (vaqt va buyurtma ID'si, ikkalasi ham shu javobda bor) — base64 faqat
// URL'da xavfsiz tashish uchun.
func (c Cursor) Encode() string {
	raw := strconv.FormatInt(c.CreatedAt.UnixMicro(), 10) + "|" + c.ID
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// ParseCursor — bo'sh satr "birinchi sahifa" degani (`nil, nil`).
func ParseCursor(s string) (*Cursor, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if len(s) > maxCursorLen {
		return nil, errBadCursor
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, errBadCursor
	}
	ts, id, ok := strings.Cut(string(raw), "|")
	if !ok || id == "" || len(id) > maxCursorIDLen {
		return nil, errBadCursor
	}
	micros, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || micros <= 0 {
		return nil, errBadCursor
	}
	return &Cursor{CreatedAt: time.UnixMicro(micros).UTC(), ID: id}, nil
}

// Precedes — (createdAt, id) buyurtmasi kursordan KEYIN keladimi, ya'ni
// keyingi sahifaga tegishlimi.
func (c Cursor) Precedes(createdAt time.Time, id string) bool {
	t := createdAt.Truncate(time.Microsecond)
	return t.Before(c.CreatedAt) || (t.Equal(c.CreatedAt) && id < c.ID)
}

// Lifetime — restoran ochilgandan beri bo'lgan barcha (ko'rinadigan)
// buyurtmalar xulosasi. Hisob qoidalari [Summary] bilan bir xil.
type Lifetime struct {
	Orders          int    `json:"orders"`
	Completed       int    `json:"completed"`
	InProgress      int    `json:"in_progress"`
	Cancelled       int    `json:"cancelled"`
	RevenueTiyin    int64  `json:"revenue_tiyin"`
	InProgressTiyin int64  `json:"in_progress_tiyin"`
	AvgOrderTiyin   *int64 `json:"avg_order_tiyin"`
	// FirstOrderAt / LastOrderAt — buyurtma bo'lmasa `null`.
	FirstOrderAt *time.Time `json:"first_order_at"`
	LastOrderAt  *time.Time `json:"last_order_at"`
}

// Add — bitta buyurtmani qo'shadi (xotira ombori).
func (l *Lifetime) Add(status orders.Status, totalTiyin int64, createdAt time.Time) {
	l.Orders++
	switch classify(status) {
	case completed:
		l.Completed++
		if totalTiyin > 0 {
			l.RevenueTiyin += totalTiyin
		}
	case cancelled:
		l.Cancelled++
	default:
		l.InProgress++
		if totalTiyin > 0 {
			l.InProgressTiyin += totalTiyin
		}
	}
	if l.FirstOrderAt == nil || createdAt.Before(*l.FirstOrderAt) {
		t := createdAt
		l.FirstOrderAt = &t
	}
	if l.LastOrderAt == nil || createdAt.After(*l.LastOrderAt) {
		t := createdAt
		l.LastOrderAt = &t
	}
}

// Finish — o'rtacha buyurtma (tushum ÷ bajarilganlar). Bajarilgan
// buyurtma bo'lmasa `nil`: nolga bo'lish yo'q.
func (l *Lifetime) Finish() {
	l.AvgOrderTiyin = nil
	if l.Completed > 0 {
		avg := l.RevenueTiyin / int64(l.Completed)
		l.AvgOrderTiyin = &avg
	}
}

// Period — [From, To) oralig'i. Nol qiymat — o'sha tomondan chegarasiz;
// ikkalasi ham nol — butun tarix.
type Period struct {
	From time.Time
	To   time.Time
}

// Contains — vaqt davrga tushadimi (xotira ombori uchun).
func (p Period) Contains(t time.Time) bool {
	return (p.From.IsZero() || !t.Before(p.From)) && (p.To.IsZero() || t.Before(p.To))
}

// ParseHistoryPeriod — `from`/`to` (YYYY-MM-DD, Toshkent kunlari, ikkala
// chet ham kiradi). Ikkalasi bo'sh — butun tarix.
//
// Statistikadan farqli, uzunlik chegarasi YO'Q: ro'yxat sahifalab
// o'qiladi, ya'ni uzun davr bitta so'rovda yuklanmaydi.
func ParseHistoryPeriod(from, to string, now time.Time) (Period, error) {
	from, to = strings.TrimSpace(from), strings.TrimSpace(to)
	if from == "" && to == "" {
		return Period{}, nil
	}
	if from == "" || to == "" {
		return Period{}, invalid("from va to sanalari birga berilishi kerak (YYYY-MM-DD)")
	}
	f, err := time.ParseInLocation(dateLayout, from, Location)
	if err != nil {
		return Period{}, invalid("from sanasi noto'g'ri — YYYY-MM-DD shaklida bo'lishi kerak")
	}
	t, err := time.ParseInLocation(dateLayout, to, Location)
	if err != nil {
		return Period{}, invalid("to sanasi noto'g'ri — YYYY-MM-DD shaklida bo'lishi kerak")
	}
	if t.Before(f) {
		return Period{}, invalid("to sanasi from sanasidan oldin bo'lishi mumkin emas")
	}
	if f.Before(minDate) {
		return Period{}, invalid("from sanasi 2020-01-01 dan oldin bo'lishi mumkin emas")
	}
	if t.After(dayOf(now).AddDate(0, 0, 1)) {
		return Period{}, invalid("kelajakdagi sanani tanlab bo'lmaydi")
	}
	return Period{From: f, To: t.AddDate(0, 0, 1)}, nil
}

// HistoryQuery — bitta sahifa so'rovi.
type HistoryQuery struct {
	Status HistoryStatus
	// After — oldingi sahifaning kursori (`nil` — birinchi sahifa).
	After  *Cursor
	Period Period
}

// HistorySource — buyurtmalar ombori ("Barcha buyurtmalar" sahifasi).
// Ko'rinish qoidasi `ListByRestaurant` bilan bir xil: to'lanmagan karta
// buyurtmalari hech qayerda ko'rinmaydi.
type HistorySource interface {
	// OrderHistory — eng yangisidan, kursordan keyingi ko'pi bilan
	// `limit` ta buyurtma.
	OrderHistory(ctx context.Context, restaurantID string, q HistoryQuery, limit int) ([]*orders.Order, error)
	// Lifetime — davr (bo'sh davr — butun tarix) xulosasi.
	Lifetime(ctx context.Context, restaurantID string, p Period) (Lifetime, error)
}
