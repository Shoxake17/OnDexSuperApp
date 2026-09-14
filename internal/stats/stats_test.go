package stats

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"chustapp/internal/orders"
)

func at(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, Location)
	if err != nil {
		panic(err)
	}
	return t
}

func mustQuery(t *testing.T, from, to string, g Granularity) Query {
	t.Helper()
	q, err := ParseQuery(from, to, string(g), at("2026-12-31 12:00"))
	if err != nil {
		t.Fatalf("ParseQuery(%s, %s): %v", from, to, err)
	}
	return q
}

func TestParseQuery(t *testing.T) {
	now := at("2026-09-13 10:00")
	cases := []struct {
		name, from, to, gran string
		ok                   bool
	}{
		{"oddiy", "2026-09-01", "2026-09-13", "", true},
		{"hafta", "2026-09-01", "2026-09-13", "week", true},
		{"ertangi kun (soat farqi)", "2026-09-01", "2026-09-14", "day", true},
		{"kelajak", "2026-09-01", "2026-09-15", "day", false},
		{"teskari", "2026-09-13", "2026-09-01", "day", false},
		{"noto'g'ri shakl", "2026-9-1", "2026-09-13", "day", false},
		{"mavjud bo'lmagan kun", "2026-02-30", "2026-03-01", "day", false},
		{"bo'sh", "", "2026-09-13", "day", false},
		{"366 kun — chegara", "2025-09-13", "2026-09-13", "day", true},
		{"367 kun", "2025-09-12", "2026-09-13", "day", false},
		{"noma'lum granularity", "2026-09-01", "2026-09-13", "year", false},
		{"2020 dan oldin", "2019-12-31", "2020-01-02", "day", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q, err := ParseQuery(c.from, c.to, c.gran, now)
			if c.ok {
				if err != nil {
					t.Fatalf("kutilmagan xato: %v", err)
				}
				if q.Granularity == "" {
					t.Fatal("granularity bo'sh qoldi")
				}
				return
			}
			var inv *InvalidQueryError
			if !errors.As(err, &inv) {
				t.Fatalf("InvalidQueryError kutilgan edi, keldi: %v", err)
			}
		})
	}

	q, _ := ParseQuery("2026-09-01", "2026-09-13", "", now)
	if q.Days() != 13 || q.Granularity != Day {
		t.Fatalf("days=%d gran=%s", q.Days(), q.Granularity)
	}
	if got := q.PrevFrom().Format(dateLayout); got != "2026-08-19" {
		t.Fatalf("o'tgan davr boshi: %s", got)
	}
	if got := q.PrevTo().Format(dateLayout); got != "2026-08-31" {
		t.Fatalf("o'tgan davr oxiri: %s", got)
	}
}

// Yangi terminal holat qo'shilsa-yu bu yerga yozilmasa, u jimgina
// "jarayonda" bo'lib qolib, tushumdan ham, bekor qilinganlardan ham
// tushib qolardi.
func TestEveryTerminalStatusIsClassified(t *testing.T) {
	terminal := map[string]bool{}
	for _, s := range orders.TerminalStatusStrings() {
		terminal[s] = true
		if classify(orders.Status(s)) == inProgress {
			t.Errorf("terminal holat %q na bajarilgan, na bekor qilingan", s)
		}
	}
	for _, s := range append(CompletedStatusStrings(), CancelledStatusStrings()...) {
		if !terminal[s] {
			t.Errorf("%q terminal emas, lekin yakunlangan deb tasniflangan", s)
		}
	}
}

func TestComputeSummary(t *testing.T) {
	q := mustQuery(t, "2026-09-07", "2026-09-13", Day) // dushanba..yakshanba
	rows := []Row{
		{CustomerID: "c1", Status: orders.StatusDelivered, TotalTiyin: 100_000, CreatedAt: at("2026-09-08 12:10"),
			AcceptedAt: at("2026-09-08 12:12"), ReadyAt: at("2026-09-08 12:30")},
		{CustomerID: "c2", Status: orders.StatusServed, TotalTiyin: 50_000, CreatedAt: at("2026-09-09 13:00")},
		{CustomerID: "c3", Status: orders.StatusCancelled, TotalTiyin: 70_000, CreatedAt: at("2026-09-10 19:00")},
		{CustomerID: "c1", Status: orders.StatusPreparing, TotalTiyin: 30_000, CreatedAt: at("2026-09-13 20:00")},
		// O'tgan davr.
		{CustomerID: "c1", Status: orders.StatusDelivered, TotalTiyin: 40_000, CreatedAt: at("2026-09-01 12:00")},
		{CustomerID: "c4", Status: orders.StatusRejected, TotalTiyin: 10_000, CreatedAt: at("2026-09-02 12:00")},
		// Ikkala davrdan ham tashqarida — e'tiborga olinmasligi kerak.
		{CustomerID: "c5", Status: orders.StatusDelivered, TotalTiyin: 999_999, CreatedAt: at("2026-08-01 12:00")},
	}
	first := map[string]time.Time{"c1": at("2026-05-01 10:00")}

	res := Compute(q, rows, first)
	cur := res.Current
	if cur.Orders != 4 || cur.Completed != 2 || cur.Cancelled != 1 || cur.InProgress != 1 {
		t.Fatalf("sonlar noto'g'ri: %+v", cur)
	}
	if cur.RevenueTiyin != 150_000 {
		t.Fatalf("tushum faqat bajarilganlardan bo'lishi kerak: %d", cur.RevenueTiyin)
	}
	// Jarayondagi (preparing) buyurtma alohida summada, bekor qilingan
	// hech qayerda.
	if cur.InProgressTiyin != 30_000 {
		t.Fatalf("jarayondagi summa: %d", cur.InProgressTiyin)
	}
	if cur.AvgOrderTiyin == nil || *cur.AvgOrderTiyin != 75_000 {
		t.Fatalf("o'rtacha buyurtma: %v", cur.AvgOrderTiyin)
	}
	if cur.AvgPrepMinutes == nil || *cur.AvgPrepMinutes != 18 {
		t.Fatalf("tayyorlash vaqti: %v", cur.AvgPrepMinutes)
	}
	// c1 — avval ham buyurtma bergan; c2 — birinchi marta; c3 — faqat
	// bekor qilingan, mijoz sifatida sanalmaydi.
	if cur.NewCustomers != 1 || cur.ReturningCustomers != 1 {
		t.Fatalf("mijozlar: yangi=%d qaytgan=%d", cur.NewCustomers, cur.ReturningCustomers)
	}

	prev := res.Previous
	if prev.Orders != 2 || prev.Completed != 1 || prev.Cancelled != 1 || prev.RevenueTiyin != 40_000 {
		t.Fatalf("o'tgan davr: %+v", prev)
	}
	if prev.AvgPrepMinutes != nil {
		t.Fatalf("o'tgan davrda tayyorlash ma'lumoti yo'q edi: %v", *prev.AvgPrepMinutes)
	}
	if prev.ReturningCustomers != 1 || prev.NewCustomers != 0 {
		t.Fatalf("o'tgan davr mijozlari: %+v", prev)
	}

	if len(res.Series) != 7 {
		t.Fatalf("kunlik nuqtalar: %d", len(res.Series))
	}
	if p := res.Series[1]; p.Start != "2026-09-08" || p.RevenueTiyin != 100_000 || p.Orders != 1 {
		t.Fatalf("08-sentabr: %+v", p)
	}
	if p := res.Series[6]; p.Orders != 1 || p.RevenueTiyin != 0 {
		t.Fatalf("jarayondagi buyurtma tushumga kirmasligi kerak: %+v", p)
	}

	if h := res.BusiestHours; h == nil || h.StartHour != 12 || h.EndHour != 14 || h.Orders != 2 {
		t.Fatalf("gavjum vaqt: %+v", res.BusiestHours)
	}
	if d := res.BusiestWeekday; d == nil || d.Weekday != 2 || d.Orders != 1 {
		t.Fatalf("faol kun: %+v", res.BusiestWeekday)
	}
}

// Ombor mijozni bermay qo'ysa ham u qo'ldagi qatorlar bo'yicha to'g'ri
// tasniflanadi: o'tgan davrdagi buyurtmasi bor mijoz "yangi" emas.
func TestComputeCustomerMissingFromFirstOrderMap(t *testing.T) {
	q := mustQuery(t, "2026-09-07", "2026-09-13", Day)
	rows := []Row{
		{CustomerID: "c9", Status: orders.StatusDelivered, CreatedAt: at("2026-09-02 10:00")},
		{CustomerID: "c9", Status: orders.StatusDelivered, CreatedAt: at("2026-09-08 10:00")},
	}
	res := Compute(q, rows, nil)
	if res.Current.ReturningCustomers != 1 || res.Current.NewCustomers != 0 {
		t.Fatalf("mijoz noto'g'ri tasniflandi: %+v", res.Current)
	}
}

func TestComputeWeekSeriesIsClampedToRange(t *testing.T) {
	q := mustQuery(t, "2026-09-09", "2026-09-15", Week) // chorshanba..seshanba
	rows := []Row{
		{Status: orders.StatusDelivered, TotalTiyin: 20_000, CreatedAt: at("2026-09-14 10:00")},
		{Status: orders.StatusDelivered, TotalTiyin: 5_000, CreatedAt: at("2026-09-09 10:00")},
	}
	res := Compute(q, rows, nil)
	if len(res.Series) != 2 {
		t.Fatalf("haftalik nuqtalar: %+v", res.Series)
	}
	if p := res.Series[0]; p.Start != "2026-09-09" || p.End != "2026-09-13" || p.RevenueTiyin != 5_000 {
		t.Fatalf("birinchi hafta: %+v", p)
	}
	if p := res.Series[1]; p.Start != "2026-09-14" || p.End != "2026-09-15" || p.RevenueTiyin != 20_000 {
		t.Fatalf("ikkinchi hafta: %+v", p)
	}
}

func TestComputeMonthSeries(t *testing.T) {
	q := mustQuery(t, "2026-08-20", "2026-10-05", Month)
	res := Compute(q, nil, nil)
	want := [][2]string{{"2026-08-20", "2026-08-31"}, {"2026-09-01", "2026-09-30"}, {"2026-10-01", "2026-10-05"}}
	if len(res.Series) != len(want) {
		t.Fatalf("oylik nuqtalar: %+v", res.Series)
	}
	for i, w := range want {
		if res.Series[i].Start != w[0] || res.Series[i].End != w[1] {
			t.Fatalf("%d-nuqta: %+v, kutilgan %v", i, res.Series[i], w)
		}
	}
}

// Server UTC'da ishlaydi, restoran esa Toshkentda: UTC bo'yicha 12-sentabr
// kechqurun berilgan buyurtma Toshkentda 13-sentabr tunda bo'lgan.
func TestComputeUsesTashkentDayAndHour(t *testing.T) {
	q := mustQuery(t, "2026-09-12", "2026-09-13", Day)
	rows := []Row{{Status: orders.StatusDelivered, TotalTiyin: 1, CreatedAt: time.Date(2026, 9, 12, 20, 30, 0, 0, time.UTC)}}
	res := Compute(q, rows, nil)
	if res.Series[0].Orders != 0 || res.Series[1].Orders != 1 {
		t.Fatalf("kun noto'g'ri aniqlandi: %+v", res.Series)
	}
	if res.BusiestHours == nil || res.BusiestHours.StartHour != 0 {
		t.Fatalf("soat noto'g'ri: %+v", res.BusiestHours)
	}
}

func TestComputeSales(t *testing.T) {
	q := mustQuery(t, "2026-09-07", "2026-09-13", Day)
	rows := []Row{
		{Status: orders.StatusDelivered, CreatedAt: at("2026-09-08 10:00"), Items: []orders.Item{
			{ProductID: "p1", Name: "Lavash", Category: "Fast Food", Qty: 2, PriceTiyin: 30_000, DiscountPriceTiyin: 25_000, ImageURL: "old.png"},
			{ProductID: "p2", Name: "Cola", Category: " ", Qty: 1, PriceTiyin: 10_000},
		}},
		{Status: orders.StatusServed, CreatedAt: at("2026-09-10 10:00"), Items: []orders.Item{
			{ProductID: "p1", Name: "Lavash katta", Category: "Fast Food", Qty: 1, PriceTiyin: 30_000, ImageURL: "new.png"},
			{ProductID: "p3", Name: "Nol", Qty: 0, PriceTiyin: 5_000},
		}},
		// Bekor qilingan buyurtma taomlari sotuv emas.
		{Status: orders.StatusCancelled, CreatedAt: at("2026-09-11 10:00"), Items: []orders.Item{
			{ProductID: "p9", Name: "Bekor", Category: "Fast Food", Qty: 50, PriceTiyin: 1_000},
		}},
	}
	res := Compute(q, rows, nil)

	if len(res.Categories) != 2 {
		t.Fatalf("turkumlar: %+v", res.Categories)
	}
	if c := res.Categories[0]; c.Name != "Fast Food" || c.Qty != 3 || c.SalesTiyin != 80_000 {
		t.Fatalf("Fast Food: %+v", c)
	}
	if c := res.Categories[1]; c.Name != uncategorized || c.Qty != 1 || c.SalesTiyin != 10_000 {
		t.Fatalf("turkumsiz: %+v", c)
	}

	if res.TopProductsTotal != 2 || len(res.TopProducts) != 2 {
		t.Fatalf("mahsulotlar: %+v", res.TopProducts)
	}
	if p := res.TopProducts[0]; p.ProductID != "p1" || p.Qty != 3 || p.SalesTiyin != 80_000 ||
		p.Name != "Lavash katta" || p.ImageURL != "new.png" || p.Category != "Fast Food" {
		t.Fatalf("eng ko'p sotilgan: %+v", p)
	}
	if p := res.TopProducts[1]; p.Category != "" {
		t.Fatalf("turkumi yozilmagan mahsulot kategoriyasi bo'sh qolishi kerak: %+v", p)
	}
}

func TestComputeTopProductsLimit(t *testing.T) {
	q := mustQuery(t, "2026-09-07", "2026-09-13", Day)
	var items []orders.Item
	for i := 0; i < TopProductsLimit+5; i++ {
		items = append(items, orders.Item{ProductID: strings.Repeat("x", i+1), Name: "T", Qty: i + 1, PriceTiyin: 100})
	}
	res := Compute(q, []Row{{Status: orders.StatusDelivered, CreatedAt: at("2026-09-08 10:00"), Items: items}}, nil)
	if len(res.TopProducts) != TopProductsLimit || res.TopProductsTotal != TopProductsLimit+5 {
		t.Fatalf("len=%d total=%d", len(res.TopProducts), res.TopProductsTotal)
	}
	if res.TopProducts[0].Qty != int64(TopProductsLimit+5) {
		t.Fatalf("tartib noto'g'ri: %+v", res.TopProducts[0])
	}
}

func TestBusiestHoursCrossesMidnight(t *testing.T) {
	q := mustQuery(t, "2026-09-07", "2026-09-13", Day)
	rows := []Row{
		{Status: orders.StatusDelivered, CreatedAt: at("2026-09-08 23:10")},
		{Status: orders.StatusDelivered, CreatedAt: at("2026-09-09 00:20")},
		{Status: orders.StatusDelivered, CreatedAt: at("2026-09-09 15:00")},
	}
	h := Compute(q, rows, nil).BusiestHours
	if h == nil || h.StartHour != 23 || h.EndHour != 1 || h.Orders != 2 {
		t.Fatalf("yarim tundan o'tuvchi oraliq: %+v", h)
	}
}

// 8 kunlik davrda dushanba ikki marta uchraydi: jami 4 ta (o'rtacha 2),
// seshanba bir marta — 3 ta. Eng faol kun seshanba bo'lishi kerak.
func TestBusiestWeekdayUsesAverage(t *testing.T) {
	q := mustQuery(t, "2026-09-07", "2026-09-14", Day)
	var rows []Row
	for _, s := range []string{"2026-09-07 10:00", "2026-09-07 11:00", "2026-09-14 10:00", "2026-09-14 11:00",
		"2026-09-08 10:00", "2026-09-08 11:00", "2026-09-08 12:00"} {
		rows = append(rows, Row{Status: orders.StatusDelivered, CreatedAt: at(s)})
	}
	d := Compute(q, rows, nil).BusiestWeekday
	if d == nil || d.Weekday != 2 || d.AvgOrders != 3 || d.Orders != 3 {
		t.Fatalf("faol kun: %+v", d)
	}
}

func TestPrepMinutesIgnoresBrokenIntervals(t *testing.T) {
	q := mustQuery(t, "2026-09-07", "2026-09-13", Day)
	base := at("2026-09-08 10:00")
	rows := []Row{
		{Status: orders.StatusDelivered, CreatedAt: base, AcceptedAt: base, ReadyAt: base.Add(200 * time.Minute)},
		{Status: orders.StatusDelivered, CreatedAt: base, AcceptedAt: base, ReadyAt: base.Add(-5 * time.Minute)},
		{Status: orders.StatusDelivered, CreatedAt: base, AcceptedAt: base, ReadyAt: base.Add(10 * time.Minute)},
		{Status: orders.StatusDelivered, CreatedAt: base, ReadyAt: base.Add(10 * time.Minute)},
	}
	p := Compute(q, rows, nil).Current.AvgPrepMinutes
	if p == nil || *p != 10 {
		t.Fatalf("tayyorlash vaqti: %v", p)
	}
}

// Bo'sh davr: ro'yxatlar `null` emas `[]`, o'rtachalar esa `null`
// (0 so'm ko'rsatish yolg'on bo'lardi).
func TestComputeEmptyJSONShape(t *testing.T) {
	q := mustQuery(t, "2026-09-07", "2026-09-13", Day)
	b, err := json.Marshal(Compute(q, nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"categories":[]`, `"top_products":[]`, `"avg_order_tiyin":null`,
		`"busiest_hours":null`, `"busiest_weekday":null`, `"timezone":"Asia/Tashkent"`} {
		if !strings.Contains(s, want) {
			t.Errorf("javobda %s yo'q: %s", want, s)
		}
	}
}

func TestCustomerIDs(t *testing.T) {
	rows := []Row{
		{CustomerID: "b", Status: orders.StatusDelivered},
		{CustomerID: "a", Status: orders.StatusPreparing},
		{CustomerID: "b", Status: orders.StatusDelivered},
		{CustomerID: "c", Status: orders.StatusCancelled},
		{CustomerID: "", Status: orders.StatusDelivered},
	}
	got := CustomerIDs(rows)
	if strings.Join(got, ",") != "a,b" {
		t.Fatalf("mijozlar: %v", got)
	}
}
