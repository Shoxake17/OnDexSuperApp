package e2e

import (
	"fmt"
	"sort"

	"chustapp/internal/e2e/report"
	"chustapp/internal/httpapi"
)

// metricsCoverageProblems — ssenariyda URILGAN har bir endpoint uchun
// Prometheus'da `ondex_http_requests_total{route="<METHOD /pattern>"}` qatori
// bo'lishi SHART (Plane ONDEXECOSY-481: "hech biri qolib ketmasin").
//
// Bu yangi endpoint qo'shilganda ham avtomatik ishlaydi: E2E qamrov
// ratcheti uni ssenariyga majburlaydi, bu tekshiruv esa uning metrikada
// ko'rinishini kafolatlaydi (masalan kimdir handler'ni middleware
// zanjiridan tashqariga ulab qo'ysa — yiqiladi).
func metricsCoverageProblems(routes []Route, outcomes map[string]report.Outcome) []string {
	fams, err := httpapi.MetricsRegistry().Gather()
	if err != nil {
		return []string{"metrikalarni yig'ib bo'lmadi: " + err.Error()}
	}
	seen := map[string]bool{}
	for _, f := range fams {
		if f.GetName() != "ondex_http_requests_total" {
			continue
		}
		for _, m := range f.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "route" {
					seen[l.GetValue()] = true
				}
			}
		}
	}
	var problems []string
	for _, r := range routes {
		o := outcomes[r.Key()]
		if o.Success+o.Other == 0 {
			// Urilmagan — qamrov ratcheti talab qiladi. FAQAT "o'chiq
			// funksiya" kontrakti (Gated) bilan urilgan endpoint E2E'da
			// umuman ro'yxatga olinmagan bo'lishi mumkin (masalan
			// `GET /ai/live` — ovoz sozlanmagan, mux 404 -> "unmatched").
			// Ular uchun kafolat tuzilmaviy: `TestSingleMuxWrappedByMetrics`.
			continue
		}
		if !seen[r.Key()] {
			problems = append(problems, fmt.Sprintf("%s — chaqirilgan, lekin metrikada YO'Q (withMetrics/captureRoute zanjiridan tashqarida?)", r.Key()))
		}
	}
	sort.Strings(problems)
	return problems
}
