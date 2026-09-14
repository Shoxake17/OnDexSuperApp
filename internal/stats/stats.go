// Package stats — restoran paneli "Statistika" sahifasining hisob-kitobi.
//
// ┌─ NEGA HISOB-KITOB BITTA JOYDA (Go'da), SQL'DA EMAS ────────────────┐
// Buyurtmalar ikki xil omborda yashaydi: Postgres (production) va
// xotira (dev va testlar). Agregatsiya SQL'da yozilsa, xotira ombori
// uchun uni Go'da QAYTA yozish kerak bo'lardi — va ikki nusxa albatta
// ajralib ketadi. Aynan shu xato bug.md 45-bandda bo'lgan: terminal
// holatlar ro'yxati SQL literalida Go'dagidan farq qilgan va faqat
// production'da ko'ringan.
//
// Shuning uchun omborlar faqat YENGIL qatorlarni ([Row]) qaytaradi, "tushum
// nima", "qaytgan mijoz kim", "hafta qachon boshlanadi" kabi barcha
// qoidalar esa shu paketda BIR MARTA yozilgan va testlangan.
// └────────────────────────────────────────────────────────────────────┘
package stats

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"chustapp/internal/orders"
)

// Location — barcha kunlik/soatlik guruhlash shu mintaqada bajariladi.
//
// IANA bazasi (`time.LoadLocation`) ATAYLAB ishlatilmaydi: Windows'da va
// minimal konteynerlarda u bo'lmasligi mumkin, xato esa faqat ishga
// tushganda chiqardi. O'zbekiston 1992-yildan beri yozgi vaqtga
// o'tmaydi, ya'ni qat'iy +05:00 aynan bir xil natija beradi.
var Location = time.FixedZone("Asia/Tashkent", 5*60*60)

const (
	// MaxRangeDays — bitta so'rovdagi eng uzun davr.
	MaxRangeDays = 366
	// MaxRows — ikki davr (joriy + o'tgan) bo'yicha omborga yuklanadigan
	// eng ko'p qator. Oshsa ANIQ xato qaytariladi: kesilgan ro'yxatdan
	// hisoblangan (ya'ni yolg'on) statistika hech qachon ko'rsatilmaydi.
	MaxRows = 50_000
	// TopProductsLimit — javobdagi mahsulotlar ro'yxati chegarasi.
	TopProductsLimit = 50

	// maxPrepMinutes — bundan uzun "qabul qilindi → tayyor" oralig'i
	// buzuq ma'lumot deb tashlanadi (bosh sahifadagi hisob bilan bir xil).
	maxPrepMinutes = 180
	// uncategorized — turkumi yozilmagan taomlar guruhi.
	uncategorized = "Turkumsiz"

	dateLayout = "2006-01-02"
)

// minDate — bundan oldingi sanalar ma'nosiz (platforma bo'lmagan).
var minDate = time.Date(2020, 1, 1, 0, 0, 0, 0, Location)

// Granularity — grafik nuqtalarining o'lchami.
type Granularity string

const (
	Day   Granularity = "day"
	Week  Granularity = "week" // dushanbadan boshlanadi
	Month Granularity = "month"
)

// ErrTooManyRows — davrdagi buyurtmalar [MaxRows] dan ko'p.
var ErrTooManyRows = errors.New("tanlangan davrda buyurtmalar juda ko'p — qisqaroq davr tanlang")

// InvalidQueryError — foydalanuvchi yuborgan parametr noto'g'ri (HTTP 400).
// Matni foydalanuvchiga to'g'ridan-to'g'ri ko'rsatiladi.
type InvalidQueryError struct{ Msg string }

func (e *InvalidQueryError) Error() string { return e.Msg }

func invalid(msg string) error { return &InvalidQueryError{Msg: msg} }

// Query — tekshirilgan so'rov. `From` va `To` — [Location] dagi kun
// boshlari (00:00), `To` kuni davrga KIRADI.
type Query struct {
	From        time.Time
	To          time.Time
	Granularity Granularity
}

// Days — davrdagi kunlar soni (ikkala chekka ham kiradi).
func (q Query) Days() int { return int(q.To.Sub(q.From)/(24*time.Hour)) + 1 }

// End — davrning OCHIQ oxiri: `To` dan keyingi kunning boshi.
func (q Query) End() time.Time { return q.To.AddDate(0, 0, 1) }

// PrevFrom — solishtiriladigan o'tgan davrning boshi (xuddi shuncha kun).
func (q Query) PrevFrom() time.Time { return q.From.AddDate(0, 0, -q.Days()) }

// PrevTo — o'tgan davrning oxirgi kuni.
func (q Query) PrevTo() time.Time { return q.From.AddDate(0, 0, -1) }

// ParseQuery — URL parametrlarini tekshiradi.
//
// `to` uchun BIR KUNLIK tolerantlik bor: panel kompyuterining soati
// yarim tunga yaqin bir necha daqiqa oldinda bo'lsa, u "ertangi" sanani
// yuborishi mumkin. Bu holatda xato berish foydalanuvchini sababsiz
// to'xtatardi; kelajakdagi kun esa shunchaki bo'sh nuqta bo'lib chiqadi.
func ParseQuery(from, to, granularity string, now time.Time) (Query, error) {
	if from == "" || to == "" {
		return Query{}, invalid("from va to sanalari talab qilinadi (YYYY-MM-DD)")
	}
	f, err := time.ParseInLocation(dateLayout, from, Location)
	if err != nil {
		return Query{}, invalid("from sanasi noto'g'ri — YYYY-MM-DD shaklida bo'lishi kerak")
	}
	t, err := time.ParseInLocation(dateLayout, to, Location)
	if err != nil {
		return Query{}, invalid("to sanasi noto'g'ri — YYYY-MM-DD shaklida bo'lishi kerak")
	}
	if t.Before(f) {
		return Query{}, invalid("to sanasi from sanasidan oldin bo'lishi mumkin emas")
	}
	if f.Before(minDate) {
		return Query{}, invalid("from sanasi 2020-01-01 dan oldin bo'lishi mumkin emas")
	}
	if t.After(dayOf(now).AddDate(0, 0, 1)) {
		return Query{}, invalid("kelajakdagi sanani tanlab bo'lmaydi")
	}
	q := Query{From: f, To: t, Granularity: Granularity(granularity)}
	if q.Days() > MaxRangeDays {
		return Query{}, invalid(fmt.Sprintf("davr %d kundan oshmasligi kerak", MaxRangeDays))
	}
	switch q.Granularity {
	case "":
		q.Granularity = Day
	case Day, Week, Month:
	default:
		return Query{}, invalid("granularity faqat day, week yoki month bo'lishi mumkin")
	}
	return q, nil
}

// dayOf — vaqtning [Location] dagi kuni, 00:00.
func dayOf(t time.Time) time.Time {
	l := t.In(Location)
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, Location)
}

// ─── Holatlar tasnifi ───────────────────────────────────────────────────

var (
	// completedStatuses — pul HAQIQATAN tushgan buyurtmalar.
	completedStatuses = []orders.Status{orders.StatusDelivered, orders.StatusServed}
	// cancelledStatuses — bajarilmagan (restoran rad etdi yoki bekor bo'ldi).
	cancelledStatuses = []orders.Status{orders.StatusRejected, orders.StatusCancelled}
)

type class int

const (
	inProgress class = iota
	completed
	cancelled
)

// classify — qolgan hamma holat (yangi, qabul qilingan, tayyorlanmoqda,
// tayyor, yo'lda) "jarayonda" hisoblanadi. Terminal holatlarning har
// biri aniq ro'yxatda turishi test bilan qulflangan
// (`TestEveryTerminalStatusIsClassified`).
func classify(s orders.Status) class {
	for _, c := range completedStatuses {
		if s == c {
			return completed
		}
	}
	for _, c := range cancelledStatuses {
		if s == c {
			return cancelled
		}
	}
	return inProgress
}

// CompletedStatusStrings — SQL parametri uchun.
func CompletedStatusStrings() []string { return statusStrings(completedStatuses) }

// CancelledStatusStrings — SQL parametri uchun.
func CancelledStatusStrings() []string { return statusStrings(cancelledStatuses) }

func statusStrings(list []orders.Status) []string {
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = string(s)
	}
	return out
}

// ─── Ombor bilan kelishuv ───────────────────────────────────────────────

// Row — statistika uchun buyurtmaning yengil nusxasi.
type Row struct {
	CustomerID string
	Status     orders.Status
	TotalTiyin int64
	CreatedAt  time.Time
	// AcceptedAt / ReadyAt — `history` dagi HAQIQIY o'tish vaqtlari
	// (`Order.ReadyAt` EMAS — u taxminiy vaqt). Nol = noma'lum.
	AcceptedAt time.Time
	ReadyAt    time.Time
	// Items — faqat JORIY davrdagi BAJARILGAN buyurtmalar uchun kerak.
	// Ombor boshqalar uchun uni bo'sh qoldirishi mumkin (trafikni
	// kamaytirish uchun); [Compute] baribir o'zi ham tekshiradi.
	Items []orders.Item
}

// Source — statistika ma'lumot manbai.
//
// ┌─ IKKALA METOD HAM FAQAT RESTORANGA KO'RINADIGAN BUYURTMALARNI OLADI ┐
// Qoida `ListByRestaurant` bilan AYNAN bir xil: to'lanmagan karta
// buyurtmasi (pul bloklanmagan) restoranga hech qachon ko'rsatilmagan,
// ya'ni u tushumga ham, buyurtmalar soniga ham kirmasligi SHART.
// └────────────────────────────────────────────────────────────────────┘
type Source interface {
	// StatRows — `created_at` [from, to) oralig'idagi qatorlar. Ko'pi
	// bilan `limit+1` ta qaytaradi — chaqiruvchi shu orqali chegara
	// oshganini aniqlaydi. Items faqat `created_at >= itemsFrom` va
	// bajarilgan holatdagi buyurtmalar uchun to'ldiriladi.
	StatRows(ctx context.Context, restaurantID string, from, to, itemsFrom time.Time, limit int) ([]Row, error)
	// FirstOrderAt — har bir mijozning shu restorandagi BIRINCHI
	// (bekor qilinmagan) buyurtmasi vaqti, butun tarix bo'yicha.
	FirstOrderAt(ctx context.Context, restaurantID string, customerIDs []string) (map[string]time.Time, error)
}

// CustomerIDs — [Source.FirstOrderAt] ga beriladigan ro'yxat: bekor
// qilinmagan buyurtmasi bor mijozlar (takrorlanmasdan, tartiblangan).
func CustomerIDs(rows []Row) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range rows {
		if r.CustomerID == "" || classify(r.Status) == cancelled || seen[r.CustomerID] {
			continue
		}
		seen[r.CustomerID] = true
		out = append(out, r.CustomerID)
	}
	sort.Strings(out)
	return out
}

// ─── Natija ─────────────────────────────────────────────────────────────

// Summary — bitta davrning asosiy ko'rsatkichlari.
type Summary struct {
	// RevenueTiyin — FAQAT bajarilgan buyurtmalar summasi (chegirmadan
	// keyingi, mijoz haqiqatan to'lagan `total_tiyin`).
	RevenueTiyin int64 `json:"revenue_tiyin"`
	Orders       int   `json:"orders"`
	Completed    int   `json:"completed"`
	InProgress   int   `json:"in_progress"`
	// InProgressTiyin — hali yakunlanmagan buyurtmalar summasi. Tushumga
	// QO'SHILMAYDI (buyurtma hali bekor bo'lishi mumkin) — panel uni
	// alohida ko'rsatadi, shunda "0 so'm" nega 0 ekani darhol tushunarli.
	InProgressTiyin int64 `json:"in_progress_tiyin"`
	Cancelled       int   `json:"cancelled"`
	// AvgOrderTiyin — tushum / bajarilgan buyurtmalar. Bajarilgan yo'q
	// bo'lsa `null` ("0 so'm" emas — bu yolg'on bo'lardi).
	AvgOrderTiyin *int64 `json:"avg_order_tiyin"`
	// AvgPrepMinutes — "qabul qilindi → tayyor" o'rtachasi; ma'lumot
	// bo'lmasa `null`.
	AvgPrepMinutes *int `json:"avg_prep_minutes"`
	// NewCustomers — shu davrda restoranga BIRINCHI marta buyurtma bergan.
	NewCustomers int `json:"new_customers"`
	// ReturningCustomers — shu davrda buyurtma bergan va undan OLDIN ham
	// buyurtma bergan mijozlar.
	ReturningCustomers int `json:"returning_customers"`
}

// Point — grafikdagi bitta nuqta. `Start`/`End` — davr bilan kesilgan
// chegaralar (ikkalasi ham kiradi).
type Point struct {
	Start        string `json:"start"`
	End          string `json:"end"`
	RevenueTiyin int64  `json:"revenue_tiyin"`
	Orders       int    `json:"orders"`
}

// Category — turkum bo'yicha sotuv (bajarilgan buyurtmalar).
type Category struct {
	Name string `json:"name"`
	Qty  int64  `json:"qty"`
	// SalesTiyin — menyu narxi bo'yicha (taomning o'z chegirma narxi
	// hisobga olinadi, buyurtma darajasidagi aksiya chegirmasi EMAS —
	// u taomlarga bo'linmaydi).
	SalesTiyin int64 `json:"sales_tiyin"`
}

// Product — mahsulot bo'yicha sotuv (bajarilgan buyurtmalar).
type Product struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
	// Category — eng so'nggi buyurtmadagi turkum (nom va rasm kabi).
	Category   string `json:"category"`
	ImageURL   string `json:"image_url"`
	Qty        int64  `json:"qty"`
	SalesTiyin int64  `json:"sales_tiyin"`
}

// BusyHours — eng ko'p buyurtma kelgan ikki soatlik oraliq.
type BusyHours struct {
	StartHour int `json:"start_hour"`
	EndHour   int `json:"end_hour"`
	Orders    int `json:"orders"`
}

// BusyWeekday — kuniga o'rtacha eng ko'p buyurtma kelgan hafta kuni.
//
// O'RTACHA, jami emas: 10 kunlik davrda ba'zi kunlar ikki marta,
// boshqalari bir marta uchraydi — jami bo'yicha solishtirish ikki marta
// uchragan kunni sababsiz "eng faol" qilib ko'rsatardi.
type BusyWeekday struct {
	Weekday   int     `json:"weekday"` // 1 = dushanba ... 7 = yakshanba
	AvgOrders float64 `json:"avg_orders"`
	Orders    int     `json:"orders"`
}

// Result — `GET /restaurants/{id}/stats` javobi.
type Result struct {
	From             string       `json:"from"`
	To               string       `json:"to"`
	PreviousFrom     string       `json:"previous_from"`
	PreviousTo       string       `json:"previous_to"`
	Granularity      Granularity  `json:"granularity"`
	Timezone         string       `json:"timezone"`
	Current          Summary      `json:"current"`
	Previous         Summary      `json:"previous"`
	Series           []Point      `json:"series"`
	Categories       []Category   `json:"categories"`
	TopProducts      []Product    `json:"top_products"`
	TopProductsTotal int          `json:"top_products_total"`
	BusiestHours     *BusyHours   `json:"busiest_hours"`
	BusiestWeekday   *BusyWeekday `json:"busiest_weekday"`
}

// Compute — barcha ko'rsatkichlarni hisoblaydi. `rows` — ikkala davr
// ([Query.PrevFrom], [Query.End]) qatorlari.
func Compute(q Query, rows []Row, firstOrderAt map[string]time.Time) Result {
	from, end, prevFrom := q.From, q.End(), q.PrevFrom()

	var cur, prev []Row
	for _, r := range rows {
		switch {
		case !r.CreatedAt.Before(from) && r.CreatedAt.Before(end):
			cur = append(cur, r)
		case !r.CreatedAt.Before(prevFrom) && r.CreatedAt.Before(from):
			prev = append(prev, r)
		}
	}

	// Mijozning birinchi buyurtmasi — ombor bergan qiymat va qo'ldagi
	// qatorlarning eng ertasi, qaysi biri oldin bo'lsa. Ombor to'g'ri
	// ishlasa ikkinchisi hech narsani o'zgartirmaydi; ishlamasa ham
	// mijoz hech qachon haqiqiydan "yangiroq" ko'rinmaydi.
	first := make(map[string]time.Time, len(firstOrderAt))
	for id, t := range firstOrderAt {
		first[id] = t
	}
	for _, r := range rows {
		if r.CustomerID == "" || classify(r.Status) == cancelled {
			continue
		}
		if t, ok := first[r.CustomerID]; !ok || r.CreatedAt.Before(t) {
			first[r.CustomerID] = r.CreatedAt
		}
	}

	res := Result{
		From:         q.From.Format(dateLayout),
		To:           q.To.Format(dateLayout),
		PreviousFrom: prevFrom.Format(dateLayout),
		PreviousTo:   q.PrevTo().Format(dateLayout),
		Granularity:  q.Granularity,
		Timezone:     Location.String(),
		Current:      summarize(cur, from, first),
		Previous:     summarize(prev, prevFrom, first),
		Series:       series(q, cur),
	}
	res.Categories, res.TopProducts, res.TopProductsTotal = sales(cur)
	res.BusiestHours = busiestHours(cur)
	res.BusiestWeekday = busiestWeekday(q, cur)
	return res
}

func summarize(rows []Row, periodStart time.Time, first map[string]time.Time) Summary {
	var s Summary
	var prepSum float64
	var prepN int
	seen := map[string]bool{}
	for _, r := range rows {
		s.Orders++
		c := classify(r.Status)
		switch c {
		case completed:
			s.Completed++
			if r.TotalTiyin > 0 {
				s.RevenueTiyin += r.TotalTiyin
			}
		case cancelled:
			s.Cancelled++
		default:
			s.InProgress++
			if r.TotalTiyin > 0 {
				s.InProgressTiyin += r.TotalTiyin
			}
		}
		if !r.AcceptedAt.IsZero() && !r.ReadyAt.IsZero() {
			m := r.ReadyAt.Sub(r.AcceptedAt).Minutes()
			if m >= 0 && m < maxPrepMinutes {
				prepSum += m
				prepN++
			}
		}
		if r.CustomerID != "" && c != cancelled && !seen[r.CustomerID] {
			seen[r.CustomerID] = true
			if first[r.CustomerID].Before(periodStart) {
				s.ReturningCustomers++
			} else {
				s.NewCustomers++
			}
		}
	}
	if s.Completed > 0 {
		avg := s.RevenueTiyin / int64(s.Completed)
		s.AvgOrderTiyin = &avg
	}
	if prepN > 0 {
		p := int(math.Round(prepSum / float64(prepN)))
		s.AvgPrepMinutes = &p
	}
	return s
}

// bucketStart — kun tegishli bo'lgan guruhning boshi (davr bilan
// kesilmagan).
func bucketStart(g Granularity, day time.Time) time.Time {
	switch g {
	case Week:
		return day.AddDate(0, 0, -isoIndex(day.Weekday()))
	case Month:
		return time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, Location)
	default:
		return day
	}
}

func nextBucket(g Granularity, start time.Time) time.Time {
	switch g {
	case Week:
		return start.AddDate(0, 0, 7)
	case Month:
		return start.AddDate(0, 1, 0)
	default:
		return start.AddDate(0, 0, 1)
	}
}

// series — davrning HAR bir guruhi uchun nuqta (buyurtmasiz kunlar
// ham nol bilan chiqadi, aks holda grafik chizig'i yolg'on "sakrardi").
func series(q Query, rows []Row) []Point {
	points := make([]Point, 0, q.Days())
	index := map[int64]int{}
	for s := bucketStart(q.Granularity, q.From); !s.After(q.To); s = nextBucket(q.Granularity, s) {
		start := s
		if start.Before(q.From) {
			start = q.From
		}
		last := nextBucket(q.Granularity, s).AddDate(0, 0, -1)
		if last.After(q.To) {
			last = q.To
		}
		index[s.Unix()] = len(points)
		points = append(points, Point{Start: start.Format(dateLayout), End: last.Format(dateLayout)})
	}
	for _, r := range rows {
		i, ok := index[bucketStart(q.Granularity, dayOf(r.CreatedAt)).Unix()]
		if !ok {
			continue
		}
		points[i].Orders++
		if classify(r.Status) == completed && r.TotalTiyin > 0 {
			points[i].RevenueTiyin += r.TotalTiyin
		}
	}
	return points
}

// sales — turkum va mahsulot bo'yicha sotuv (faqat bajarilgan
// buyurtmalar). Uchinchi qiymat — kesilishidan oldingi mahsulotlar soni.
func sales(rows []Row) ([]Category, []Product, int) {
	type productAgg struct {
		Product
		lastAt time.Time
	}
	cats := map[string]*Category{}
	prods := map[string]*productAgg{}

	for _, r := range rows {
		if classify(r.Status) != completed {
			continue
		}
		for _, it := range r.Items {
			if it.Qty <= 0 {
				continue
			}
			unit := it.PriceTiyin
			if it.DiscountPriceTiyin > 0 && it.DiscountPriceTiyin < unit {
				unit = it.DiscountPriceTiyin
			}
			if unit < 0 {
				unit = 0
			}
			qty := int64(it.Qty)
			amount := unit * qty

			catName := strings.TrimSpace(it.Category)
			if catName == "" {
				catName = uncategorized
			}
			c := cats[catName]
			if c == nil {
				c = &Category{Name: catName}
				cats[catName] = c
			}
			c.Qty += qty
			c.SalesTiyin += amount

			// Mahsulot ID bo'yicha: taom keyin qayta nomlansa ham bitta
			// qator bo'lib qoladi va ENG SO'NGGI nomi/rasmi ko'rsatiladi.
			key := it.ProductID
			if key == "" {
				key = "name:" + strings.TrimSpace(it.Name)
			}
			p := prods[key]
			if p == nil {
				p = &productAgg{Product: Product{ProductID: it.ProductID}}
				prods[key] = p
			}
			p.Qty += qty
			p.SalesTiyin += amount
			if p.lastAt.IsZero() || !r.CreatedAt.Before(p.lastAt) {
				p.lastAt = r.CreatedAt
				p.Name = it.Name
				p.Category = strings.TrimSpace(it.Category)
				p.ImageURL = it.ImageURL
			}
		}
	}

	categories := make([]Category, 0, len(cats))
	for _, c := range cats {
		categories = append(categories, *c)
	}
	sort.Slice(categories, func(i, j int) bool {
		a, b := categories[i], categories[j]
		if a.Qty != b.Qty {
			return a.Qty > b.Qty
		}
		if a.SalesTiyin != b.SalesTiyin {
			return a.SalesTiyin > b.SalesTiyin
		}
		return a.Name < b.Name
	})

	products := make([]Product, 0, len(prods))
	for _, p := range prods {
		products = append(products, p.Product)
	}
	sort.Slice(products, func(i, j int) bool {
		a, b := products[i], products[j]
		if a.Qty != b.Qty {
			return a.Qty > b.Qty
		}
		if a.SalesTiyin != b.SalesTiyin {
			return a.SalesTiyin > b.SalesTiyin
		}
		return a.Name < b.Name
	})
	total := len(products)
	if len(products) > TopProductsLimit {
		products = products[:TopProductsLimit]
	}
	return categories, products, total
}

func busiestHours(rows []Row) *BusyHours {
	var hist [24]int
	for _, r := range rows {
		hist[r.CreatedAt.In(Location).Hour()]++
	}
	best, bestSum := -1, 0
	for h := 0; h < 24; h++ {
		// Oraliq yarim tundan o'tishi mumkin (23:00 - 01:00).
		if sum := hist[h] + hist[(h+1)%24]; sum > bestSum {
			best, bestSum = h, sum
		}
	}
	if best < 0 {
		return nil
	}
	return &BusyHours{StartHour: best, EndHour: (best + 2) % 24, Orders: bestSum}
}

func busiestWeekday(q Query, rows []Row) *BusyWeekday {
	if len(rows) == 0 {
		return nil
	}
	var counts, occurrences [7]int
	for d := q.From; !d.After(q.To); d = d.AddDate(0, 0, 1) {
		occurrences[isoIndex(d.Weekday())]++
	}
	for _, r := range rows {
		counts[isoIndex(r.CreatedAt.In(Location).Weekday())]++
	}
	best := -1
	var bestAvg float64
	for i := 0; i < 7; i++ {
		if occurrences[i] == 0 {
			continue
		}
		if avg := float64(counts[i]) / float64(occurrences[i]); best < 0 || avg > bestAvg {
			best, bestAvg = i, avg
		}
	}
	if best < 0 || counts[best] == 0 {
		return nil
	}
	return &BusyWeekday{
		Weekday:   best + 1,
		AvgOrders: math.Round(bestAvg*10) / 10,
		Orders:    counts[best],
	}
}

// isoIndex — dushanba = 0 ... yakshanba = 6.
func isoIndex(w time.Weekday) int { return (int(w) + 6) % 7 }
