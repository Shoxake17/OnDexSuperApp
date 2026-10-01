package e2e

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"chustapp/internal/e2e/report"
)

// TestMain — barcha ssenariylar tugagach QAMROV RATCHETI va hisobot.
//
// ┌─ QAMROV RATCHETI ─────────────────────────────────────────────────┐
// Har endpoint uch holatdan BIRIDA bo'lishi shart:
//   - ssenariyda urilgan (qamralgan);
//   - `routes_exempt.txt` da sababi bilan turadi (hali qamralmagan);
//
// Va ikki teskari qoida:
//   - istisnodagi endpoint qamralgan bo'lsa — ro'yxatdan OLIB TASHLANSIN
//     (ro'yxat faqat qisqarishi mumkin, "ratchet");
//   - istisnodagi endpoint endi mavjud bo'lmasa — ro'yxatdan olib tashlansin.
//
// Shu bilan kelajakdagi har bir funksiya avtomatik testga ega bo'ladi:
// yangi endpoint qo'shgan dasturchi ssenariy yozishga (yoki sababini
// aytib istisno qilishga) MAJBUR.
// └───────────────────────────────────────────────────────────────────┘
func TestMain(m *testing.M) {
	rs, err := DiscoverRoutes(filepath.Join("..", "httpapi"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "E2E:", err)
		os.Exit(2)
	}
	Routes = rs

	code := m.Run()

	exempt, err := LoadExempt("routes_exempt.txt")
	if err != nil {
		fmt.Fprintln(os.Stderr, "E2E:", err)
		os.Exit(2)
	}
	cov, problems := computeCoverage(Routes, Rec.Outcomes(), exempt)

	rep := Rec.Build(cov, slowBudget(), gitCommit())
	out := os.Getenv("E2E_REPORT")
	if out == "" {
		out = filepath.Join("..", "..", "logs", "e2e", "report.json")
	}
	if err := report.Save(out, rep); err != nil {
		fmt.Fprintln(os.Stderr, "E2E: hisobotni yozib bo'lmadi:", err)
	} else {
		htmlPath := strings.TrimSuffix(out, filepath.Ext(out)) + ".html"
		if err := os.WriteFile(htmlPath, []byte(report.HTML(rep)), 0o644); err == nil {
			fmt.Println("E2E hisobot:", htmlPath)
		}
	}
	fmt.Printf("E2E qamrov: %d/%d endpoint, shundan muvaffaqiyatli yo'li bilan %d, faqat yopiq kontrakt %d (istisno: %d)\n",
		cov.Covered, cov.Total, cov.WithSuccess, len(cov.GatedOnly), cov.Exempt)

	// Ratchet FAQAT to'liq yugurishda: `-run` bilan bitta ssenariy
	// ishlatilganda qamrov tabiiy ravishda to'liq emas.
	if code == 0 && fullRun() && len(problems) > 0 {
		fmt.Fprintln(os.Stderr, "\nE2E QAMROV RATCHETI buzildi:")
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, "  -", p)
		}
		fmt.Fprintln(os.Stderr, "\nYechim: ssenariy yozing (internal/e2e/README.md) yoki routes_exempt.txt ga sabab bilan qo'shing.")
		code = 1
	}
	os.Exit(code)
}

func fullRun() bool {
	f := flag.Lookup("test.run")
	return f == nil || f.Value.String() == ""
}

// computeCoverage — har endpoint holati:
//
//	muvaffaqiyatli chaqiruv bor   -> qamralgan (istisnoda turmasligi kerak);
//	faqat "yopiq" kontrakti       -> qamralgan, alohida belgilanadi (GatedOnly);
//	faqat rad etilgan (4xx) yo'l  -> ISTISNO ro'yxatida SABAB bilan bo'lishi shart;
//	umuman urilmagan              -> ISTISNO ro'yxatida SABAB bilan bo'lishi shart.
//
// Ya'ni "urildi" yetarli emas: endpointning ISHLASH yo'li kamida bir marta
// sinalgan bo'lishi kerak (yoki nega yo'qligi yozilgan bo'lishi kerak).
func computeCoverage(routes []Route, outcomes map[string]report.Outcome, exempt map[string]string) (report.Coverage, []string) {
	var cov report.Coverage
	var problems []string
	known := map[string]bool{}
	cov.Total = len(routes)
	for _, r := range routes {
		k := r.Key()
		known[k] = true
		o := outcomes[k]
		_, isExempt := exempt[k]
		switch {
		case o.Success > 0:
			cov.Covered++
			cov.WithSuccess++
			if isExempt {
				problems = append(problems, fmt.Sprintf("%s endi muvaffaqiyatli sinalgan — routes_exempt.txt dan olib tashlang", k))
			}
		case o.Gated > 0:
			cov.Covered++
			cov.GatedOnly = append(cov.GatedOnly, k)
			if isExempt {
				problems = append(problems, fmt.Sprintf("%s yopiq kontrakt bilan sinalgan — routes_exempt.txt dan olib tashlang", k))
			}
		case isExempt:
			cov.Exempt++
			cov.Exempted = append(cov.Exempted, k)
			if o.Success+o.Gated+o.Other > 0 {
				cov.Covered++
			}
		case o.Other > 0:
			cov.Covered++
			cov.Uncovered = append(cov.Uncovered, k)
			problems = append(problems, fmt.Sprintf("%s — faqat rad etilgan (4xx) yo'l bilan urilgan: muvaffaqiyatli yo'lini sinang yoki routes_exempt.txt ga sabab bilan qo'shing", k))
		default:
			cov.Uncovered = append(cov.Uncovered, k)
			problems = append(problems, fmt.Sprintf("%s — ssenariysiz va istisnosiz", k))
		}
	}
	for k := range exempt {
		if !known[k] {
			problems = append(problems, fmt.Sprintf("routes_exempt.txt: %q endi mavjud emas — olib tashlang", k))
		}
	}
	sort.Strings(problems)
	sort.Strings(cov.Exempted)
	return cov, problems
}

func gitCommit() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// slowBudget — sekin deb belgilash chegarasi (ms), muhitdan o'zgartiriladi.
func slowBudget() float64 {
	if v := os.Getenv("E2E_SLOW_MS"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
			return n
		}
	}
	return 250
}
