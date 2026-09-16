package catalog

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Location — ish vaqti Toshkent vaqtida hisoblanadi.
//
// IANA bazasi (`time.LoadLocation`) ATAYLAB ishlatilmaydi: Windows'da va
// minimal konteynerlarda u bo'lmasligi mumkin. O'zbekiston 1992-yildan
// beri yozgi vaqtga o'tmaydi, ya'ni qat'iy +05:00 aynan bir xil natija
// beradi (`internal/stats.Location` bilan bir xil qoida).
var Location = time.FixedZone("Asia/Tashkent", 5*60*60)

var (
	ErrBadHours = errors.New("ish vaqti noto'g'ri")
	// ErrOutsideHours — restoran "ochiq", lekin hozir ish vaqti emas.
	// `ErrRestaurantClosed` ni o'rab oladi: yopiq restoranni tekshiradigan
	// mavjud kod (`errors.Is`) bu holatni ham taniydi.
	ErrOutsideHours = fmt.Errorf("%w: hozir ish vaqti emas", ErrRestaurantClosed)
)

var dayNames = [8]string{"", "Dushanba", "Seshanba", "Chorshanba", "Payshanba", "Juma", "Shanba", "Yakshanba"}

// DayHours — bitta kunning ish vaqti.
//
// `Close` `Open` dan kichik bo'lsa — tunda yopiladi (masalan 18:00–02:00,
// choyxona va kechki kafelar uchun). Teng bo'lsa — kun bo'yi (24 soat).
type DayHours struct {
	// Day — 1 = dushanba ... 7 = yakshanba (ISO 8601).
	Day     int    `json:"day" bson:"day"`
	Enabled bool   `json:"enabled" bson:"enabled"`
	Open    string `json:"open" bson:"open"`
	Close   string `json:"close" bson:"close"`
}

// WorkingHours — haftalik jadval. Restoranda `nil` — cheklanmagan: ish
// vaqti belgilanmagan restoran avvalgidek faqat "ochiq/yopiq" tugmasiga
// bo'ysunadi (mavjud restoranlarning xatti-harakati o'zgarmaydi).
type WorkingHours struct {
	Days []DayHours `json:"days" bson:"days"`
}

// parseClock — "HH:MM" → daqiqa. Faqat aynan ikki raqam, ikki nuqta, ikki
// raqam: "8:00", "+1:00", "24:00" rad etiladi.
func parseClock(s string) (int, bool) {
	if len(s) != 5 || s[2] != ':' {
		return 0, false
	}
	for _, i := range []int{0, 1, 3, 4} {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	h := int(s[0]-'0')*10 + int(s[1]-'0')
	m := int(s[3]-'0')*10 + int(s[4]-'0')
	if h > 23 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

// Validate — tekshirilgan va kun tartibida saralangan nusxa.
// Yetti kunning HAR BIRI aynan bir marta bo'lishi shart: yarim jadval
// "qolgan kunlar ochiqmi yoki yopiq?" degan noaniqlik tug'dirardi.
func (w WorkingHours) Validate() (WorkingHours, error) {
	if len(w.Days) != 7 {
		return WorkingHours{}, fmt.Errorf("%w: haftaning 7 kuni ham ko'rsatilishi kerak", ErrBadHours)
	}
	var seen [8]bool
	out := make([]DayHours, 0, 7)
	for _, d := range w.Days {
		if d.Day < 1 || d.Day > 7 || seen[d.Day] {
			return WorkingHours{}, fmt.Errorf("%w: kunlar 1 dan 7 gacha, har biri bir marta", ErrBadHours)
		}
		seen[d.Day] = true
		open := strings.TrimSpace(d.Open)
		closeAt := strings.TrimSpace(d.Close)
		if _, ok := parseClock(open); !ok {
			return WorkingHours{}, fmt.Errorf("%w: %s — ochilish vaqti SS:DD shaklida bo'lsin", ErrBadHours, dayNames[d.Day])
		}
		if _, ok := parseClock(closeAt); !ok {
			return WorkingHours{}, fmt.Errorf("%w: %s — yopilish vaqti SS:DD shaklida bo'lsin", ErrBadHours, dayNames[d.Day])
		}
		out = append(out, DayHours{Day: d.Day, Enabled: d.Enabled, Open: open, Close: closeAt})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Day < out[j].Day })
	return WorkingHours{Days: out}, nil
}

func isoDay(wd time.Weekday) int {
	if wd == time.Sunday {
		return 7
	}
	return int(wd)
}

func (w *WorkingHours) day(n int) (DayHours, bool) {
	for _, d := range w.Days {
		if d.Day == n {
			return d, true
		}
	}
	return DayHours{}, false
}

// IsOpenAt — shu paytda ish vaqtimi. `nil` jadval — har doim.
//
// Kechagi kunning tungi davomi ham hisobga olinadi: dushanba 18:00–02:00
// bo'lsa, seshanba 01:30 da restoran hali ochiq.
func (w *WorkingHours) IsOpenAt(t time.Time) bool {
	if w == nil || len(w.Days) == 0 {
		return true
	}
	lt := t.In(Location)
	m := lt.Hour()*60 + lt.Minute()
	today := isoDay(lt.Weekday())

	if d, ok := w.day(today); ok && d.Enabled {
		o, okO := parseClock(d.Open)
		c, okC := parseClock(d.Close)
		if okO && okC {
			switch {
			case o == c:
				return true
			case o < c:
				if m >= o && m < c {
					return true
				}
			default:
				if m >= o {
					return true
				}
			}
		}
	}

	yesterday := today - 1
	if yesterday == 0 {
		yesterday = 7
	}
	if d, ok := w.day(yesterday); ok && d.Enabled {
		o, okO := parseClock(d.Open)
		c, okC := parseClock(d.Close)
		if okO && okC && o > c && m < c {
			return true
		}
	}
	return false
}

// NextChangeAfter — `t` dan keyin ish vaqti holati (ochiq↔yopiq) birinchi
// o'zgaradigan payt, bir hafta ichida. Jadval yo'q, kun bo'yi ochiq yoki
// butunlay yopiq bo'lsa — `false`.
//
// Nomzodlar — har kunning ochilish/yopilish daqiqalari va yarim tun (kun
// bo'yi ochiq kunning chegarasi). `IsOpenAt` daqiqa aniqligida ishlagani
// uchun chegara daqiqasining o'zida tekshirish aniq natija beradi.
func (w *WorkingHours) NextChangeAfter(t time.Time) (time.Time, bool) {
	if w == nil || len(w.Days) == 0 {
		return time.Time{}, false
	}
	lt := t.In(Location)
	midnight := time.Date(lt.Year(), lt.Month(), lt.Day(), 0, 0, 0, 0, Location)
	candidates := make([]time.Time, 0, 30)
	for i := -1; i <= 8; i++ {
		day := midnight.AddDate(0, 0, i)
		candidates = append(candidates, day)
		d, ok := w.day(isoDay(day.Weekday()))
		if !ok || !d.Enabled {
			continue
		}
		o, okO := parseClock(d.Open)
		c, okC := parseClock(d.Close)
		if !okO || !okC {
			continue
		}
		openAt := day.Add(time.Duration(o) * time.Minute)
		closeAt := day.Add(time.Duration(c) * time.Minute)
		if c < o {
			closeAt = closeAt.AddDate(0, 0, 1)
		}
		candidates = append(candidates, openAt, closeAt)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Before(candidates[j]) })
	openNow := w.IsOpenAt(t)
	for _, at := range candidates {
		if at.After(t) && w.IsOpenAt(at) != openNow {
			return at, true
		}
	}
	return time.Time{}, false
}

// Yopiqlik sabablari (`OpenState.Reason`, JSON `closed_reason`).
const (
	// ClosedManual — restoran o'zi "Yopiq" qilgan (panel tugmasi).
	ClosedManual = "manual"
	// ClosedHours — "Ochiq", lekin hozir ish vaqti emas.
	ClosedHours = "hours"
)

// OrderableAt — restoran shu paytda buyurtma qabul qiladimi; qabul
// qilmasa — sababi bilan xato (`ErrRestaurantClosed` / `ErrOutsideHours`).
//
// ┌─ YAGONA QOIDA (2026-09-15) ────────────────────────────────────────┐
// "Hozir ochiqmi" avval ikki xil hisoblanardi: narxlash (savat) ish
// vaqtini tekshirardi, qolgan hamma joy — restoran kartalari, menyu,
// qidiruv va sevimlilardagi `restaurant_open`, panel belgisi — faqat
// qo'lda bosiladigan `Open` tugmasiga qarardi. Ish vaqti tugagan restoran
// hamma joyda "Ochiq" ko'rinib, mijoz savatni to'ldirgach faqat oxirida
// "yopiq" javobini olardi. Endi har bir yo'l shu metoddan o'tadi.
// └────────────────────────────────────────────────────────────────────┘
func (r *Restaurant) OrderableAt(t time.Time) error {
	if !r.Open {
		return ErrRestaurantClosed
	}
	if !r.WorkingHours.IsOpenAt(t) {
		return ErrOutsideHours
	}
	return nil
}

// AcceptingOrdersAt — `OrderableAt` ning ha/yo'q ko'rinishi.
func (r *Restaurant) AcceptingOrdersAt(t time.Time) bool {
	return r.OrderableAt(t) == nil
}

// AcceptingOrdersNow — joriy payt uchun (javob tayyorlaydigan qatlamlar).
func (r *Restaurant) AcceptingOrdersNow() bool {
	return r.AcceptingOrdersAt(time.Now())
}

// OpenState — restoranning mijozga ko'rsatiladigan holati.
type OpenState struct {
	// Open — hozir buyurtma qabul qiladi.
	Open bool
	// Reason — yopiq bo'lsa sababi (`ClosedManual` / `ClosedHours`).
	Reason string
	// ChangesAt — holat O'Z-O'ZIDAN o'zgaradigan payt: ochiq bo'lsa yopilish,
	// ish vaqti sabab yopiq bo'lsa ochilish vaqti. `nil` — o'zgarmaydi
	// (qo'lda yopilgan, jadvalsiz, kun bo'yi ochiq yoki hafta davomida
	// ochilmaydi).
	ChangesAt *time.Time
}

// OpenStateAt — `t` paytidagi holat.
func (r *Restaurant) OpenStateAt(t time.Time) OpenState {
	if !r.Open {
		return OpenState{Reason: ClosedManual}
	}
	st := OpenState{Open: r.WorkingHours.IsOpenAt(t)}
	if !st.Open {
		st.Reason = ClosedHours
	}
	if at, ok := r.WorkingHours.NextChangeAfter(t); ok {
		st.ChangesAt = &at
	}
	return st
}

// WithOpenState — javob uchun nusxa: hisoblanadigan maydonlar (`open_now`,
// `closed_reason`, `open_changes_at`) to'ldiriladi. Keshdan KEYIN
// chaqiriladi — vaqtga bog'liq qiymat keshda eskirmasin.
func (r Restaurant) WithOpenState(t time.Time) Restaurant {
	st := r.OpenStateAt(t)
	open := st.Open
	r.OpenNow = &open
	r.ClosedReason = st.Reason
	r.OpenChangesAt = nil
	if st.ChangesAt != nil {
		at := st.ChangesAt.UTC()
		r.OpenChangesAt = &at
	}
	return r
}
