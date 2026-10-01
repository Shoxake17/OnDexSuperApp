package report

import (
	"fmt"
	"html"
	"sort"
	"strings"
)

// (hisobot HTML'i: qadamlar, endpoint tezligi, takroriy so'rovlar, invariantlar, qamrov)

// HTML — hisobotni bitta mustaqil HTML faylga aylantiradi (tashqi
// resurssiz: CI artefakti sifatida ham, ikki marta bosib ham ochiladi).
func HTML(rep Report) string {
	var b strings.Builder
	esc := html.EscapeString

	passed, failed := 0, 0
	for _, s := range rep.Steps {
		switch {
		case s.Skipped:
		case s.Passed:
			passed++
		default:
			failed++
		}
	}
	verdict, vclass := "O'TDI", "ok"
	if failed > 0 || len(rep.Violations) > 0 || len(rep.Coverage.Uncovered) > 0 {
		verdict, vclass = "YIQILDI", "bad"
	}

	b.WriteString(`<!doctype html><html lang="uz"><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>OnDex E2E hisobot</title>
<style>
:root{--bg:#fff;--fg:#1a1a1a;--dim:#666;--line:#e5e5e5;--ok:#15803d;--bad:#b91c1c;--warn:#b45309;--card:#fafafa}
@media(prefers-color-scheme:dark){:root{--bg:#141414;--fg:#eee;--dim:#999;--line:#2c2c2c;--card:#1c1c1c;--ok:#4ade80;--bad:#f87171;--warn:#fbbf24}}
body{font:14px/1.5 system-ui,Segoe UI,sans-serif;background:var(--bg);color:var(--fg);margin:0;padding:24px;max-width:1200px;margin-inline:auto}
h1{font-size:22px;margin:0 0 4px}h2{font-size:16px;margin:28px 0 8px}
.dim{color:var(--dim)}.ok{color:var(--ok)}.bad{color:var(--bad)}.warn{color:var(--warn)}
.cards{display:flex;flex-wrap:wrap;gap:10px;margin:14px 0}
.card{background:var(--card);border:1px solid var(--line);border-radius:10px;padding:10px 14px;min-width:130px}
.card b{display:block;font-size:20px}
table{border-collapse:collapse;width:100%;font-size:13px}
th,td{border-bottom:1px solid var(--line);padding:5px 8px;text-align:left;vertical-align:top}
th{color:var(--dim);font-weight:600}td.n,th.n{text-align:right;font-variant-numeric:tabular-nums}
code{font-family:ui-monospace,Consolas,monospace;font-size:12px}
details{margin:4px 0}summary{cursor:pointer}
</style><body>`)

	fmt.Fprintf(&b, `<h1>OnDex E2E hisobot <span class="%s">— %s</span></h1>`, vclass, verdict)
	fmt.Fprintf(&b, `<div class="dim">%s · %.1f s · commit %s</div>`,
		esc(rep.StartedAt.Format("2006-01-02 15:04:05")), rep.DurationMs/1000, esc(orDash(rep.GitCommit)))

	covPct := 0.0
	if rep.Coverage.Total > 0 {
		covPct = 100 * float64(rep.Coverage.Covered) / float64(rep.Coverage.Total)
	}
	totalChecks := 0
	for _, v := range rep.InvariantChecks {
		totalChecks += v
	}
	b.WriteString(`<div class="cards">`)
	card := func(label, val, cls string) {
		fmt.Fprintf(&b, `<div class="card"><span class="dim">%s</span><b class="%s">%s</b></div>`, esc(label), cls, esc(val))
	}
	card("Qadamlar o'tdi", fmt.Sprint(passed), "ok")
	card("Qadamlar yiqildi", fmt.Sprint(failed), clsIf(failed > 0, "bad", "ok"))
	card("Invariant buzilishi", fmt.Sprint(len(rep.Violations)), clsIf(len(rep.Violations) > 0, "bad", "ok"))
	card("Invariant tekshiruvlari", fmt.Sprint(totalChecks), "")
	card("HTTP so'rovlar", fmt.Sprint(len(rep.Calls)), "")
	card("Endpoint qamrovi", fmt.Sprintf("%d/%d (%.0f%%)", rep.Coverage.Covered, rep.Coverage.Total, covPct), clsIf(len(rep.Coverage.Uncovered) > 0, "bad", "ok"))
	card("Muvaffaqiyatli yo'li bilan", fmt.Sprintf("%d/%d", rep.Coverage.WithSuccess, rep.Coverage.Total), "ok")
	card("Faqat yopiq kontrakt", fmt.Sprint(len(rep.Coverage.GatedOnly)), clsIf(len(rep.Coverage.GatedOnly) > 0, "warn", "ok"))
	card("Qamralmagan (istisno)", fmt.Sprint(rep.Coverage.Exempt), clsIf(rep.Coverage.Exempt > 0, "warn", "ok"))
	b.WriteString(`</div>`)

	// Buzilishlar
	if len(rep.Violations) > 0 {
		b.WriteString(`<h2 class="bad">Buzilgan invariantlar</h2><table><tr><th>Ssenariy</th><th>Qadam</th><th>Invariant</th><th>Tafsilot</th></tr>`)
		for _, v := range rep.Violations {
			fmt.Fprintf(&b, `<tr><td>%s</td><td>%s</td><td><code>%s</code></td><td>%s</td></tr>`,
				esc(v.Scenario), esc(v.Step), esc(v.Invariant), esc(v.Detail))
		}
		b.WriteString(`</table>`)
	}

	// Qamralmagan yangi endpointlar
	if len(rep.Coverage.Uncovered) > 0 {
		b.WriteString(`<h2 class="bad">Ssenariysiz YANGI endpointlar</h2>
<p class="dim">Har endpoint ssenariyda urilishi yoki <code>routes_exempt.txt</code> da sababi bilan turishi shart.</p><ul>`)
		for _, r := range rep.Coverage.Uncovered {
			fmt.Fprintf(&b, `<li><code>%s</code></li>`, esc(r))
		}
		b.WriteString(`</ul>`)
	}

	// Qadamlar
	b.WriteString(`<h2>Ssenariylar</h2>`)
	byScenario := map[string][]Step{}
	var order []string
	for _, s := range rep.Steps {
		if _, ok := byScenario[s.Scenario]; !ok {
			order = append(order, s.Scenario)
		}
		byScenario[s.Scenario] = append(byScenario[s.Scenario], s)
	}
	for _, sc := range order {
		steps := byScenario[sc]
		bad := 0
		var ms float64
		for _, s := range steps {
			if !s.Passed && !s.Skipped {
				bad++
			}
			ms += s.Ms
		}
		open := ""
		if bad > 0 {
			open = " open"
		}
		fmt.Fprintf(&b, `<details%s><summary><b>%s</b> <span class="%s">%d/%d o'tdi</span> <span class="dim">· %.0f ms</span></summary><table><tr><th>Qadam</th><th class="n">ms</th><th class="n">so'rov</th><th>Natija</th></tr>`,
			open, esc(sc), clsIf(bad > 0, "bad", "ok"), len(steps)-bad, len(steps), ms)
		for _, s := range steps {
			res := `<span class="ok">o'tdi</span>`
			if s.Skipped {
				res = `<span class="dim">o'tkazildi</span>`
			} else if !s.Passed {
				res = `<span class="bad">YIQILDI</span> ` + esc(s.Error)
			}
			fmt.Fprintf(&b, `<tr><td>%s</td><td class="n">%.1f</td><td class="n">%d</td><td>%s</td></tr>`,
				esc(s.Name), s.Ms, s.Calls, res)
		}
		b.WriteString(`</table></details>`)
	}

	// Tezlik
	fmt.Fprintf(&b, `<h2>Endpoint tezligi (eng sekin birinchi, chegara %.0f ms)</h2><table><tr><th>Endpoint</th><th class="n">so'rov</th><th class="n">p50 ms</th><th class="n">p95 ms</th><th class="n">max ms</th><th class="n">5xx</th></tr>`, rep.SlowBudgetMs)
	for _, r := range rep.Routes {
		cls := ""
		if r.Max > rep.SlowBudgetMs {
			cls = "warn"
		}
		if r.Errors5 > 0 {
			cls = "bad"
		}
		closed := ""
		if r.Closed > 0 {
			closed = fmt.Sprintf(` <span class="dim" title="Tashqi xizmat sozlanmagan — kutilgan 503, xato emas">(yopiq 503 × %d)</span>`, r.Closed)
		}
		fmt.Fprintf(&b, `<tr class="%s"><td><code>%s</code>%s</td><td class="n">%d</td><td class="n">%.1f</td><td class="n">%.1f</td><td class="n">%.1f</td><td class="n">%d</td></tr>`,
			cls, esc(r.Route), closed, r.Calls, r.P50, r.P95, r.Max, r.Errors5)
	}
	b.WriteString(`</table>`)

	// Takroriy so'rovlar
	if len(rep.Duplicates) > 0 {
		fmt.Fprintf(&b, `<h2 class="warn">Takroriy so'rovlar (bir qadamda %d+ marta)</h2><p class="dim">Ortiqcha reload yoki N+1 belgisi bo'lishi mumkin.</p><table><tr><th>Ssenariy</th><th>Qadam</th><th>So'rov</th><th class="n">marta</th></tr>`, DuplicateThreshold)
		for _, d := range rep.Duplicates {
			fmt.Fprintf(&b, `<tr><td>%s</td><td>%s</td><td><code>%s %s</code></td><td class="n">%d</td></tr>`,
				esc(d.Scenario), esc(d.Step), esc(d.Method), esc(d.Path), d.Count)
		}
		b.WriteString(`</table>`)
	}

	// Invariantlar
	b.WriteString(`<h2>Invariantlar</h2><table><tr><th>Invariant</th><th class="n">tekshirildi</th></tr>`)
	names := make([]string, 0, len(rep.InvariantChecks))
	for k := range rep.InvariantChecks {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		fmt.Fprintf(&b, `<tr><td><code>%s</code></td><td class="n">%d</td></tr>`, esc(k), rep.InvariantChecks[k])
	}
	b.WriteString(`</table>`)

	// Istisnolar
	if len(rep.Coverage.Exempted) > 0 {
		fmt.Fprintf(&b, `<h2 class="warn">Hali qamralmagan endpointlar (%d)</h2><details><summary>Ro'yxat</summary><ul>`, len(rep.Coverage.Exempted))
		for _, r := range rep.Coverage.Exempted {
			fmt.Fprintf(&b, `<li><code>%s</code></li>`, esc(r))
		}
		b.WriteString(`</ul></details>`)
	}

	b.WriteString(`</body></html>`)
	return b.String()
}

func clsIf(cond bool, yes, no string) string {
	if cond {
		return yes
	}
	return no
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
