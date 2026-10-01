package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"chustapp/internal/e2e/report"
)

// META-TESTLAR: E2E suite'ning O'ZI to'g'ri ishlashini tekshiradi.
// Ratchet yoki invariant jimgina buzilsa, butun suite yolg'on "yashil" bo'lardi.

func TestMatchRoutePicksMostSpecific(t *testing.T) {
	rs := []Route{
		newRoute("GET", "/tables/kinds"),
		newRoute("GET", "/tables/{id}"),
		newRoute("GET", "/restaurants/{id}/menu"),
		newRoute("GET", "/restaurants/{id}"),
		newRoute("POST", "/restaurants/{id}/staff/{staffID}/status"),
	}
	cases := []struct{ method, path, want string }{
		{"GET", "/tables/kinds", "/tables/kinds"}, // literal wildcard'dan ustun
		{"GET", "/tables/abc", "/tables/{id}"},
		{"GET", "/restaurants/r1", "/restaurants/{id}"},
		{"GET", "/restaurants/r1/menu", "/restaurants/{id}/menu"},
		{"POST", "/restaurants/r1/staff/s1/status", "/restaurants/{id}/staff/{staffID}/status"},
		{"POST", "/tables/kinds", ""}, // usul mos emas
		{"GET", "/yoq/yol", ""},
		{"GET", "/restaurants/r1/menu/extra", ""}, // oxiri qat'iy
	}
	for _, c := range cases {
		if got := MatchRoute(rs, c.method, c.path); got != c.want {
			t.Errorf("%s %s -> %q, %q kutilgan", c.method, c.path, got, c.want)
		}
	}
}

func TestDiscoverRoutesFindsRealEndpoints(t *testing.T) {
	if len(Routes) < 100 {
		t.Fatalf("manba koddan faqat %d endpoint topildi — regex buzilgan bo'lishi mumkin", len(Routes))
	}
	must := []string{"POST /orders", "POST /orders/{id}/pay", "GET /restaurants/{id}/staff", "GET /ws"}
	have := map[string]bool{}
	for _, r := range Routes {
		have[r.Key()] = true
	}
	for _, k := range must {
		if !have[k] {
			t.Errorf("%q topilmadi", k)
		}
	}
}

func TestLoadExemptRequiresReason(t *testing.T) {
	dir := t.TempDir()
	write := func(content string) string {
		p := filepath.Join(dir, "ex.txt")
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if _, err := LoadExempt(write("GET /x # sabab bor\n")); err != nil {
		t.Fatalf("to'g'ri fayl rad etildi: %v", err)
	}
	for name, body := range map[string]string{
		"sababsiz":    "GET /x\n",
		"bo'sh sabab": "GET /x #   \n",
		"takroriy":    "GET /x # a\nGET /x # b\n",
	} {
		if _, err := LoadExempt(write(body)); err == nil {
			t.Errorf("%s: xato kutilgan edi", name)
		}
	}
	// BOM va izoh qatorlari.
	m, err := LoadExempt(write("\xef\xbb\xbf# sarlavha\nGET /y   # sabab\n"))
	if err != nil || len(m) != 1 || m["GET /y"] == "" {
		t.Fatalf("BOM/izoh: %v %v", m, err)
	}
}

// Ratchet mantig'i: yangi endpoint, faqat rad yo'li, eskirgan va keraksiz istisno — hammasi xato.
func TestCoverageRatchetLogic(t *testing.T) {
	routes := []Route{newRoute("GET", "/ok"), newRoute("GET", "/neg"), newRoute("GET", "/gate"), newRoute("GET", "/none")}
	ok := report.Outcome{Success: 2}
	neg := report.Outcome{Other: 3}
	gate := report.Outcome{Gated: 1}

	// Toza holat: muvaffaqiyat, yopiq kontrakt va sababli istisnolar.
	cov, problems := computeCoverage(routes,
		map[string]report.Outcome{"GET /ok": ok, "GET /neg": neg, "GET /gate": gate},
		map[string]string{"GET /neg": "faqat rad yo'li", "GET /none": "reja"})
	if len(problems) != 0 || cov.WithSuccess != 1 || len(cov.GatedOnly) != 1 || cov.Exempt != 2 {
		t.Fatalf("toza holat: %+v %v", cov, problems)
	}

	has := func(ps []string, sub string) bool {
		for _, p := range ps {
			if strings.Contains(p, sub) {
				return true
			}
		}
		return false
	}

	// Faqat rad etilgan yo'l bilan urilgan endpoint "qamralgan" EMAS.
	_, problems = computeCoverage(routes,
		map[string]report.Outcome{"GET /ok": ok, "GET /neg": neg, "GET /gate": gate},
		map[string]string{"GET /none": "reja"})
	if !has(problems, "GET /neg") || !has(problems, "faqat rad etilgan") {
		t.Fatalf("faqat-rad-yo'li ushlanmadi: %v", problems)
	}

	// Umuman urilmagan va istisnosiz endpoint.
	_, problems = computeCoverage(routes,
		map[string]report.Outcome{"GET /ok": ok, "GET /neg": neg, "GET /gate": gate},
		map[string]string{"GET /neg": "s"})
	if !has(problems, "GET /none") || !has(problems, "ssenariysiz") {
		t.Fatalf("yangi endpoint ushlanmadi: %v", problems)
	}

	// Muvaffaqiyatli sinalgan endpoint istisnoda turmasin.
	_, problems = computeCoverage(routes,
		map[string]report.Outcome{"GET /ok": ok, "GET /neg": neg, "GET /gate": gate},
		map[string]string{"GET /neg": "s", "GET /none": "s", "GET /ok": "s"})
	if !has(problems, "GET /ok") || !has(problems, "olib tashlang") {
		t.Fatalf("keraksiz istisno ushlanmadi: %v", problems)
	}

	// Mavjud bo'lmagan endpoint istisnoda turib qolmasin.
	_, problems = computeCoverage(routes,
		map[string]report.Outcome{"GET /ok": ok, "GET /neg": neg, "GET /gate": gate},
		map[string]string{"GET /neg": "s", "GET /none": "s", "GET /yoq": "s"})
	if !has(problems, "GET /yoq") {
		t.Fatalf("eskirgan istisno ushlanmadi: %v", problems)
	}
}

// Invariant tekshiruvlari jimgina "doim o'tadigan" bo'lib qolmasin: ataylab
// buzuq ma'lumotni ular rad ETISHI kerak.
func TestInvariantsRejectBrokenData(t *testing.T) {
	// Alohida yig'uvchi: meta-tekshiruvlar haqiqiy hisobotni ifloslantirmasin.
	rec := report.NewRecorder()
	w := &World{scenario: "meta", rec: rec}
	probe := &testing.T{} // buzilishni yozadi, lekin haqiqiy testni yiqitmaydi
	broken := func(f func()) (violated bool) {
		defer func() { recover() }() //nolint:errcheck // t.Errorf yiqitmaydi, faqat yozadi
		n := len(rec.Build(report.Coverage{}, 0, "").Violations)
		f()
		return len(rec.Build(report.Coverage{}, 0, "").Violations) > n
	}

	if !broken(func() {
		w.CheckQuote(probe, map[string]any{"subtotal_tiyin": 100.0, "total_tiyin": 100.0, "delivery_fee_tiyin": 50.0}, false)
	}) {
		t.Error("jami != qismlar ushlanmadi")
	}
	if !broken(func() {
		w.CheckQuote(probe, map[string]any{"subtotal_tiyin": 100.0, "total_tiyin": 150.0, "delivery_fee_tiyin": 50.0}, true)
	}) {
		t.Error("stolda yetkazish haqi ushlanmadi")
	}
	if !broken(func() {
		w.CheckOrder(probe, map[string]any{"subtotal_tiyin": 100.0, "total_tiyin": 100.0, "status": "delivered", "type": "dine_in",
			"items": []any{map[string]any{"price_tiyin": 100.0, "qty": 1.0}}})
	}) {
		t.Error("stol buyurtmasi `delivered` ushlanmadi")
	}
	if !broken(func() {
		w.CheckOrder(probe, map[string]any{"subtotal_tiyin": 100.0, "total_tiyin": 100.0, "status": "delivered",
			"items":   []any{map[string]any{"price_tiyin": 100.0, "qty": 1.0}},
			"history": []any{map[string]any{"from": "created", "to": "delivered"}}})
	}) {
		t.Error("mumkin bo'lmagan holat o'tishi ushlanmadi")
	}
	if !broken(func() { w.CheckGatewayPayment(probe, map[string]any{"amount": 100.0}, 200) }) {
		t.Error("shlyuz summasi != buyurtma jami ushlanmadi")
	}
}
