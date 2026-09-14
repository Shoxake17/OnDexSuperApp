package staff

import (
	"context"
	"strconv"
	"strings"
	"time"
)

// ┌─ OYLIK HISOBOT: NIMAGA ASOSLANADI ─────────────────────────────────┐
// Davomat (keldi/ketdi) tizimda QAYD ETILMAYDI. Shuning uchun hisobot
// "jadval bo'yicha" hisoblanadi va panelda aynan shunday yoziladi:
//   - har bir kun uchun o'sha kungi HOLAT va JADVAL hodisalar
//     jurnalidan tiklanadi (ta'til, ishdan bo'shatish, jadval
//     o'zgarishi o'z sanasidan kuchga kiradi);
//   - ish kuni — jadvaldagi kun; jadvalda yo'q kun — dam olish;
//   - ish soati — smena uzunligi (tunda tugaydigan smena ham to'g'ri);
//   - hisoblangan maosh = oylik × to'lanadigan kunlar ÷ oyning ish
//     kunlari. Ta'til kunlari to'lanadigan kunlarga KIRMAYDI (alohida
//     ko'rsatiladi — ta'til puli buxgalteriya qarori).
// Jadvali yo'q xodim: kalendar kunlari bo'yicha, soat hisoblanmaydi.
// └────────────────────────────────────────────────────────────────────┘

// Kun belgilari (`ReportRow.Days`, oyning har bir kuni uchun bitta harf).
const (
	DayWorked  = 'W' // o'tgan ish kuni
	DayPlanned = 'P' // bugungi yoki kelgusi ish kuni
	DayOff     = 'O' // dam olish kuni (jadvalda yo'q)
	DayLeave   = 'L' // ta'til
	DayNone    = 'N' // ishga qabul qilinmagan yoki ishdan bo'shagan
)

var ErrBadMonth = invalid("oy noto'g'ri (YYYY-MM, 2000-yildan keyingi mavjud oy)")

type ReportRow struct {
	Member *Member
	// Days — oyning har bir kuni uchun belgi (W/P/O/L/N).
	Days string
	// NormDays — oyning ish kunlari (jadval bo'yicha, butun oy).
	NormDays int
	// PayableDays — ishda bo'lgan ish kunlari (o'tgan + kelgusi).
	PayableDays    int
	WorkedDays     int
	LeaveDays      int
	OffDays        int
	WorkedMinutes  int
	PlannedMinutes int
	// AccruedTiyin — hisoblangan maosh; maosh kiritilmagan bo'lsa nil.
	AccruedTiyin *int64
	// Scheduled — oy davomida ish jadvali bo'lganmi (soat shunga bog'liq).
	Scheduled bool
}

type ReportTotals struct {
	Members         int   `json:"members"`
	SalaryFundTiyin int64 `json:"salary_fund_tiyin"`
	AccruedTiyin    int64 `json:"accrued_tiyin"`
	WorkedDays      int   `json:"worked_days"`
	LeaveDays       int   `json:"leave_days"`
	WorkedMinutes   int   `json:"worked_minutes"`
	PlannedMinutes  int   `json:"planned_minutes"`
}

type Report struct {
	Year        int
	Month       time.Month
	DaysInMonth int
	Today       time.Time
	Rows        []ReportRow
	Totals      ReportTotals
}

// ParseMonth — "YYYY-MM"; bo'sh — joriy oy (Toshkent). Kelgusi oydan
// keyingisi va 2000-yildan oldingisi rad etiladi.
func ParseMonth(s string, now time.Time) (int, time.Month, error) {
	local := now.In(Location)
	s = strings.TrimSpace(s)
	if s == "" {
		return local.Year(), local.Month(), nil
	}
	t, err := time.ParseInLocation("2006-01", s, Location)
	if err != nil || t.Year() < 2000 {
		return 0, 0, ErrBadMonth
	}
	if t.After(time.Date(local.Year(), local.Month()+1, 1, 0, 0, 0, 0, Location)) {
		return 0, 0, ErrBadMonth
	}
	return t.Year(), t.Month(), nil
}

func (s *Service) MonthlyReport(ctx context.Context, restaurantID string, year int, month time.Month) (Report, error) {
	list, err := s.repo.List(ctx, restaurantID)
	if err != nil {
		return Report{}, err
	}
	start := time.Date(year, month, 1, 0, 0, 0, 0, Location)
	// Oy boshidan keyingi hodisalar yetarli: kun holati hozirgi holatdan
	// orqaga "o'ynab" tiklanadi.
	events, err := s.repo.Events(ctx, restaurantID, "", start, 200_000)
	if err != nil {
		return Report{}, err
	}
	return BuildReport(list, events, year, month, s.now()), nil
}

func clockMinutes(s string) int {
	h, _ := strconv.Atoi(s[:2])
	m, _ := strconv.Atoi(s[3:])
	return h*60 + m
}

// ShiftMinutes — smena uzunligi: 18:00–02:00 = 8 soat, 09:00–09:00 = 24 soat.
func (s *Schedule) ShiftMinutes() int {
	if s == nil || !validClock(s.Start) || !validClock(s.End) {
		return 0
	}
	a, b := clockMinutes(s.Start), clockMinutes(s.End)
	if b > a {
		return b - a
	}
	return 24*60 - a + b
}

// parseScheduleKey — `Schedule.key()` ning teskarisi ("1,2,3 08:00-22:00").
func parseScheduleKey(k string) *Schedule {
	daysPart, times, ok := strings.Cut(k, " ")
	if !ok {
		return nil
	}
	start, end, ok := strings.Cut(times, "-")
	if !ok {
		return nil
	}
	var days []int
	for _, p := range strings.Split(daysPart, ",") {
		if d, err := strconv.Atoi(p); err == nil {
			days = append(days, d)
		}
	}
	return &Schedule{Days: days, Start: start, End: end}
}

// stateAt — `t` paytidagi holat va jadval. `evs` — shu xodimning
// hodisalari, ENG YANGISI BIRINCHI.
func stateAt(m *Member, evs []Event, t time.Time) (Status, *Schedule) {
	status, sch := m.Status, m.Schedule
	for _, ev := range evs {
		if !ev.At.After(t) {
			break
		}
		switch ev.Kind {
		case EventStatusChanged:
			status = Status(ev.From)
		case EventScheduleChanged:
			sch = parseScheduleKey(ev.From)
		}
	}
	return status, sch
}

func localDate(t time.Time) time.Time {
	l := t.In(Location)
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, Location)
}

// BuildReport — sof funksiya (testlanadi). Kun holati kun BOSHIDAGI
// (00:00) holat: shu kuni ishdan bo'shagan xodimning oxirgi kuni ish kuni.
func BuildReport(list []*Member, events []Event, year int, month time.Month, now time.Time) Report {
	start := time.Date(year, month, 1, 0, 0, 0, 0, Location)
	daysIn := start.AddDate(0, 1, -1).Day()
	today := localDate(now)

	byMember := map[string][]Event{}
	for _, ev := range events {
		if ev.Kind == EventStatusChanged || ev.Kind == EventScheduleChanged {
			byMember[ev.MemberID] = append(byMember[ev.MemberID], ev)
		}
	}

	rep := Report{Year: year, Month: month, DaysInMonth: daysIn, Today: today}
	for _, m := range list {
		evs := byMember[m.ID]
		hired := localDate(m.CreatedAt)
		if m.HiredOn != nil {
			hired = time.Date(m.HiredOn.Year(), m.HiredOn.Month(), m.HiredOn.Day(), 0, 0, 0, 0, Location)
		}
		row := ReportRow{Member: m}
		marks := make([]byte, daysIn)
		employed := false
		for i := 0; i < daysIn; i++ {
			day := start.AddDate(0, 0, i)
			status, sch := stateAt(m, evs, day)
			works := sch == nil || sch.WorksOn(isoWeekday(day))
			if works {
				row.NormDays++
			}
			switch {
			case day.Before(hired) || status == StatusDismissed:
				marks[i] = DayNone
				continue
			case !works:
				marks[i] = DayOff
				row.OffDays++
			case status == StatusOnLeave:
				marks[i] = DayLeave
				row.LeaveDays++
			default:
				mins := sch.ShiftMinutes()
				row.PayableDays++
				row.PlannedMinutes += mins
				if day.Before(today) {
					marks[i] = DayWorked
					row.WorkedDays++
					row.WorkedMinutes += mins
				} else {
					marks[i] = DayPlanned
				}
			}
			employed = true
			if sch != nil {
				row.Scheduled = true
			}
		}
		if !employed {
			continue
		}
		row.Days = string(marks)
		if m.SalaryTiyin != nil && row.NormDays > 0 {
			// Yarim tiyindan yuqori — yuqoriga yaxlitlanadi.
			v := (*m.SalaryTiyin*int64(row.PayableDays)*2 + int64(row.NormDays)) / (2 * int64(row.NormDays))
			row.AccruedTiyin = &v
			rep.Totals.SalaryFundTiyin += *m.SalaryTiyin
			rep.Totals.AccruedTiyin += v
		}
		rep.Totals.Members++
		rep.Totals.WorkedDays += row.WorkedDays
		rep.Totals.LeaveDays += row.LeaveDays
		rep.Totals.WorkedMinutes += row.WorkedMinutes
		rep.Totals.PlannedMinutes += row.PlannedMinutes
		rep.Rows = append(rep.Rows, row)
	}
	return rep
}
