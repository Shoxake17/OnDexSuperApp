package staff

import (
	"errors"
	"testing"
	"time"
)

func at(y int, mo time.Month, d, h, mi int) time.Time {
	return time.Date(y, mo, d, h, mi, 0, 0, Location)
}

func money(v int64) *int64 { return &v }

func TestShiftMinutes(t *testing.T) {
	for _, c := range []struct {
		start, end string
		want       int
	}{
		{"09:00", "18:00", 540},
		{"18:00", "02:00", 480},
		{"08:00", "08:00", 1440},
		{"00:00", "23:59", 1439},
	} {
		if got := (&Schedule{Days: []int{1}, Start: c.start, End: c.end}).ShiftMinutes(); got != c.want {
			t.Errorf("%s–%s: %d", c.start, c.end, got)
		}
	}
	if (*Schedule)(nil).ShiftMinutes() != 0 {
		t.Fatal("jadvalsiz — 0")
	}
}

func TestParseMonth(t *testing.T) {
	now := at(2026, 9, 14, 12, 0)
	if y, m, err := ParseMonth("", now); err != nil || y != 2026 || m != time.September {
		t.Fatal(y, m, err)
	}
	if _, _, err := ParseMonth("2026-10", now); err != nil {
		t.Fatal("kelgusi oy mumkin:", err)
	}
	for _, bad := range []string{"2026-13", "2026-11", "1999-12", "09.2026", "2026-9"} {
		if _, _, err := ParseMonth(bad, now); !errors.Is(err, ErrBadMonth) {
			t.Errorf("%q qabul qilindi", bad)
		}
	}
}

func TestBuildReport(t *testing.T) {
	now := at(2026, 9, 14, 12, 0) // dushanba; 1-sentabr — seshanba
	weekdays := &Schedule{Days: []int{1, 2, 3, 4, 5}, Start: "09:00", End: "18:00"}
	list := []*Member{
		// A: Du–Ju, 9–10-sentabr ta'tilda.
		{ID: "a", Status: StatusActive, Schedule: weekdays, SalaryTiyin: money(300_000_000), CreatedAt: at(2026, 8, 1, 10, 0)},
		// B: 15-sentabrdan ishga qabul qilingan, jadvali yo'q.
		{ID: "b", Status: StatusActive, SalaryTiyin: money(100_000_000), CreatedAt: at(2026, 9, 14, 10, 0),
			HiredOn: func() *time.Time { d := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC); return &d }()},
		// C: avgustda bo'shagan — hisobotda yo'q.
		{ID: "c", Status: StatusDismissed, Schedule: weekdays, CreatedAt: at(2026, 1, 1, 10, 0)},
		// D: har kuni 08–20, 3-sentabr soat 15:00 da bo'shatilgan.
		{ID: "d", Status: StatusDismissed, Schedule: &Schedule{Days: []int{1, 2, 3, 4, 5, 6, 7}, Start: "08:00", End: "20:00"},
			CreatedAt: at(2026, 1, 1, 10, 0)},
		// E: 7-sentabr 09:00 gacha har kuni, keyin Du–Ju.
		{ID: "e", Status: StatusActive, Schedule: &Schedule{Days: []int{1, 2, 3, 4, 5}, Start: "10:00", End: "19:00"},
			CreatedAt: at(2026, 1, 1, 10, 0)},
	}
	events := []Event{ // eng yangisi birinchi
		{MemberID: "a", Kind: EventStatusChanged, From: "on_leave", To: "active", At: at(2026, 9, 10, 10, 0)},
		{MemberID: "a", Kind: EventStatusChanged, From: "active", To: "on_leave", At: at(2026, 9, 8, 10, 0)},
		{MemberID: "e", Kind: EventScheduleChanged, From: "1,2,3,4,5,6,7 08:00-20:00", To: "1,2,3,4,5 10:00-19:00", At: at(2026, 9, 7, 9, 0)},
		{MemberID: "d", Kind: EventStatusChanged, From: "active", To: "dismissed", At: at(2026, 9, 3, 15, 0)},
	}
	rep := BuildReport(list, events, 2026, time.September, now)
	if rep.DaysInMonth != 30 || len(rep.Rows) != 4 {
		t.Fatalf("qatorlar: %d, kunlar: %d", len(rep.Rows), rep.DaysInMonth)
	}
	rows := map[string]ReportRow{}
	for _, r := range rep.Rows {
		rows[r.Member.ID] = r
	}
	if _, ok := rows["c"]; ok {
		t.Fatal("oydan oldin bo'shagan xodim hisobotga tushdi")
	}

	a := rows["a"]
	if a.NormDays != 22 || a.LeaveDays != 2 || a.PayableDays != 20 || a.WorkedDays != 7 ||
		a.WorkedMinutes != 7*540 || a.PlannedMinutes != 20*540 || a.OffDays != 8 || !a.Scheduled {
		t.Fatalf("A: %+v", a)
	}
	if a.Days[4] != DayOff || a.Days[7] != DayWorked || a.Days[8] != DayLeave || a.Days[9] != DayLeave ||
		a.Days[10] != DayWorked || a.Days[13] != DayPlanned {
		t.Fatalf("A kunlari: %s", a.Days)
	}
	if a.AccruedTiyin == nil || *a.AccruedTiyin != 272_727_273 {
		t.Fatalf("A maoshi: %v", *a.AccruedTiyin)
	}

	b := rows["b"]
	if b.NormDays != 30 || b.PayableDays != 16 || b.WorkedDays != 0 || b.Scheduled ||
		b.Days[13] != DayNone || b.Days[14] != DayPlanned || *b.AccruedTiyin != 53_333_333 {
		t.Fatalf("B: %+v", b)
	}

	d := rows["d"]
	if d.WorkedDays != 3 || d.PayableDays != 3 || d.Days[2] != DayWorked || d.Days[3] != DayNone || d.AccruedTiyin != nil {
		t.Fatalf("D: %+v", d)
	}

	e := rows["e"]
	if e.Days[4] != DayWorked || e.Days[5] != DayWorked || e.Days[12] != DayOff || e.Days[7] != DayWorked {
		t.Fatalf("E (jadval o'zgarishi): %s", e.Days)
	}
	// 1–7: eski jadval (12 soat; o'zgarish 7-sentabr 09:00 da, kun boshida
	// hali eski), 8–11: yangi (9 soat), 12–13: dam olish.
	if e.WorkedMinutes != 7*720+4*540 {
		t.Fatalf("E soatlari: %d", e.WorkedMinutes)
	}

	if rep.Totals.Members != 4 || rep.Totals.SalaryFundTiyin != 400_000_000 ||
		rep.Totals.AccruedTiyin != 272_727_273+53_333_333 {
		t.Fatalf("jami: %+v", rep.Totals)
	}
}
