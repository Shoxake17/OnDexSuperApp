package stats

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"chustapp/internal/orders"
)

func TestCursorRoundTrip(t *testing.T) {
	at := time.Date(2026, 9, 13, 14, 5, 6, 123456789, time.FixedZone("x", 3*3600))
	c := CursorOf(&orders.Order{ID: "ord-42", CreatedAt: at})
	got, err := ParseCursor(c.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "ord-42" || !got.CreatedAt.Equal(at.Truncate(time.Microsecond)) {
		t.Fatalf("kursor o'zgarib ketdi: %+v", got)
	}
	// Kursorning o'zi keyingi sahifaga tushmasligi SHART (aks holda
	// sahifa chegarasidagi buyurtma ikki marta ko'rinardi).
	if got.Precedes(at, "ord-42") {
		t.Fatal("kursordagi buyurtma keyingi sahifaga ham tushdi")
	}
	if !got.Precedes(at, "ord-41") || got.Precedes(at, "ord-43") {
		t.Fatal("bir xil vaqtda id bo'yicha tartib noto'g'ri")
	}
	if !got.Precedes(at.Add(-time.Microsecond), "zzz") || got.Precedes(at.Add(time.Microsecond), "a") {
		t.Fatal("vaqt bo'yicha tartib noto'g'ri")
	}
}

func TestParseCursorRejectsGarbage(t *testing.T) {
	if c, err := ParseCursor(""); c != nil || err != nil {
		t.Fatalf("bo'sh kursor birinchi sahifa bo'lishi kerak: %v %v", c, err)
	}
	enc := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	for _, bad := range []string{
		"!!!",
		enc("abc"),
		enc("123|"),
		enc("|id"),
		enc("0|id"),
		enc("-5|id"),
		enc("12x|id"),
		enc("123|" + strings.Repeat("a", maxCursorIDLen+1)),
		strings.Repeat("A", maxCursorLen+1),
	} {
		if _, err := ParseCursor(bad); err == nil {
			t.Errorf("%q qabul qilindi", bad)
		}
	}
}

func TestParseHistoryParams(t *testing.T) {
	for in, want := range map[string]HistoryStatus{
		"": HistoryAll, "all": HistoryAll, "in_progress": HistoryInProgress,
		"completed": HistoryCompleted, "cancelled": HistoryCancelled,
	} {
		if got, err := ParseHistoryStatus(in); err != nil || got != want {
			t.Errorf("%q: %q %v", in, got, err)
		}
	}
	if _, err := ParseHistoryStatus("delivered"); err == nil {
		t.Error("noma'lum filtr qabul qilindi")
	}

	if n, err := ParseHistoryLimit(""); err != nil || n != HistoryDefaultLimit {
		t.Errorf("standart limit: %d %v", n, err)
	}
	if n, err := ParseHistoryLimit("50"); err != nil || n != 50 {
		t.Errorf("50: %d %v", n, err)
	}
	for _, bad := range []string{"0", "-1", "51", "1e3", "abc"} {
		if _, err := ParseHistoryLimit(bad); err == nil {
			t.Errorf("limit %q qabul qilindi", bad)
		}
	}
}

func TestHistoryStatusMatchesClassification(t *testing.T) {
	all := []orders.Status{
		orders.StatusCreated, orders.StatusAccepted, orders.StatusPreparing, orders.StatusReady,
		orders.StatusPickedUp, orders.StatusDelivered, orders.StatusServed,
		orders.StatusRejected, orders.StatusCancelled,
	}
	for _, s := range all {
		hits := 0
		for _, f := range []HistoryStatus{HistoryInProgress, HistoryCompleted, HistoryCancelled} {
			if f.Matches(s) {
				hits++
			}
		}
		// Har bir holat AYNAN bitta guruhda: aks holda chiplardagi sonlar
		// yig'indisi "Barchasi" ga teng bo'lmasdi.
		if hits != 1 || !HistoryAll.Matches(s) {
			t.Errorf("%s holati %d ta guruhga tushdi", s, hits)
		}
	}
	if len(TerminalStatusStrings()) != 4 {
		t.Fatalf("terminal holatlar: %v", TerminalStatusStrings())
	}
}

func TestParseHistoryPeriod(t *testing.T) {
	now := time.Date(2026, 9, 14, 1, 30, 0, 0, Location)

	if p, err := ParseHistoryPeriod("", "", now); err != nil || !p.From.IsZero() || !p.To.IsZero() {
		t.Fatalf("bo'sh davr butun tarix bo'lishi kerak: %+v %v", p, err)
	}

	p, err := ParseHistoryPeriod("2026-09-01", "2026-09-13", now)
	if err != nil {
		t.Fatal(err)
	}
	if !p.From.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, Location)) ||
		!p.To.Equal(time.Date(2026, 9, 14, 0, 0, 0, 0, Location)) {
		t.Fatalf("chegaralar: %v — %v", p.From, p.To)
	}
	// Toshkent kuni: 13-sentabr 23:59 (+05) kiradi, 14-sentabr 00:00 kirmaydi.
	if !p.Contains(time.Date(2026, 9, 13, 18, 59, 0, 0, time.UTC)) ||
		p.Contains(time.Date(2026, 9, 13, 19, 0, 0, 0, time.UTC)) ||
		p.Contains(time.Date(2026, 8, 31, 18, 59, 0, 0, time.UTC)) {
		t.Fatal("kun chegarasi Toshkent vaqtida emas")
	}
	if _, err := ParseHistoryPeriod("2026-09-14", "2026-09-14", now); err != nil {
		t.Fatalf("bugungi kun: %v", err)
	}

	for _, bad := range [][2]string{
		{"2026-09-01", ""},
		{"", "2026-09-01"},
		{"bugun", "2026-09-01"},
		{"2026-09-01", "01.09.2026"},
		{"2026-09-10", "2026-09-01"},
		{"2019-12-31", "2026-09-01"},
		{"2026-09-01", "2026-09-20"},
	} {
		if _, err := ParseHistoryPeriod(bad[0], bad[1], now); err == nil {
			t.Errorf("%v qabul qilindi", bad)
		}
	}
}

func TestLifetimeAccumulates(t *testing.T) {
	t0 := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	var l Lifetime
	l.Finish()
	if l.AvgOrderTiyin != nil || l.FirstOrderAt != nil {
		t.Fatalf("bo'sh xulosa: %+v", l)
	}
	l.Add(orders.StatusDelivered, 50_000, t0.Add(48*time.Hour))
	l.Add(orders.StatusServed, 30_001, t0)
	l.Add(orders.StatusCancelled, 99_000, t0.Add(time.Hour))
	l.Add(orders.StatusReady, 14_000, t0.Add(72*time.Hour))
	l.Finish()
	if l.Orders != 4 || l.Completed != 2 || l.Cancelled != 1 || l.InProgress != 1 {
		t.Fatalf("sonlar: %+v", l)
	}
	if l.RevenueTiyin != 80_001 || l.InProgressTiyin != 14_000 {
		t.Fatalf("summalar: %+v", l)
	}
	if l.AvgOrderTiyin == nil || *l.AvgOrderTiyin != 40_000 {
		t.Fatalf("o'rtacha: %v", l.AvgOrderTiyin)
	}
	if !l.FirstOrderAt.Equal(t0) || !l.LastOrderAt.Equal(t0.Add(72*time.Hour)) {
		t.Fatalf("vaqtlar: %v %v", l.FirstOrderAt, l.LastOrderAt)
	}
}
