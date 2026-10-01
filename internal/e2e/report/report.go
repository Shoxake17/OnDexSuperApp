// Paket report — E2E oqim testlarining HISOBOTI: ma'lumot turlari,
// JSON saqlash va HTML ko'rinish.
//
// Test harness (`internal/e2e`) har chaqiruvni, har qadamni va har
// invariant tekshiruvini shu yerga yozadi; `cmd/e2ereport` esa JSON dan
// bitta HTML fayl yasaydi.
package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// DuplicateThreshold — bir qadamda bir xil GET shuncha marta bo'lsa belgilanadi.
const DuplicateThreshold = 4

// Call — bitta HTTP so'rov.
type Call struct {
	Scenario string  `json:"scenario"`
	Step     string  `json:"step"`
	Method   string  `json:"method"`
	Path     string  `json:"path"`
	Route    string  `json:"route"` // e'londagi shakl: /restaurants/{id}/staff
	Status   int     `json:"status"`
	Ms       float64 `json:"ms"`
	Actor    string  `json:"actor"`
	// Poll — fon jarayonini kutish uchun ataylab takrorlangan so'rov
	// (`Eventually`); takroriy so'rovlar hisobotiga kirmaydi.
	Poll bool `json:"poll,omitempty"`
	// Gated — sozlanmagan tashqi xizmat (`World.Gated`): 503 kutilgan kontrakt.
	Gated bool `json:"gated,omitempty"`
}

// Duplicate — BIR qadam ichida bir xil o'qish so'rovi ko'p marta
// yuborilgan (ortiqcha reload / N+1 belgisi).
type Duplicate struct {
	Scenario string `json:"scenario"`
	Step     string `json:"step"`
	Method   string `json:"method"`
	Path     string `json:"path"`
	Count    int    `json:"count"`
}

// Step — bitta qadam (subtest).
type Step struct {
	Scenario string  `json:"scenario"`
	Name     string  `json:"name"`
	Passed   bool    `json:"passed"`
	Skipped  bool    `json:"skipped,omitempty"`
	Ms       float64 `json:"ms"`
	Error    string  `json:"error,omitempty"`
	Calls    int     `json:"calls"`
}

// Violation — buzilgan invariant (test yiqiladi, lekin hisobotda ham ko'rinadi).
type Violation struct {
	Scenario  string `json:"scenario"`
	Step      string `json:"step"`
	Invariant string `json:"invariant"`
	Detail    string `json:"detail"`
}

// RouteStat — endpoint bo'yicha jamlama.
type RouteStat struct {
	Route   string  `json:"route"`
	Calls   int     `json:"calls"`
	P50     float64 `json:"p50_ms"`
	P95     float64 `json:"p95_ms"`
	Max     float64 `json:"max_ms"`
	Errors5 int     `json:"errors_5xx"`
	// Closed — kutilgan 503 ("xizmat sozlanmagan, xavfsiz yopiq"): xato emas.
	Closed int `json:"closed_503"`
}

// Coverage — endpointlar qamrovi.
type Coverage struct {
	Total int `json:"total"`
	// Covered — hamma urilgan endpointlar (muvaffaqiyat + yopiq + rad yo'li).
	Covered int `json:"covered"`
	// WithSuccess — kamida bitta muvaffaqiyatli (2xx/3xx) chaqiruvi bor.
	WithSuccess int `json:"with_success"`
	// GatedOnly — faqat "xavfsiz yopiq" kontrakti sinalgan (tashqi xizmat yo'q).
	GatedOnly []string `json:"gated_only"`
	Exempt    int      `json:"exempt"`
	Uncovered []string `json:"uncovered"`
	// Exempted — hali qamrab olinmagan, lekin asosli (yoki rejadagi) istisno.
	Exempted []string `json:"exempted"`
}

// Report — butun yugurish natijasi.
type Report struct {
	StartedAt  time.Time   `json:"started_at"`
	DurationMs float64     `json:"duration_ms"`
	GitCommit  string      `json:"git_commit,omitempty"`
	Steps      []Step      `json:"steps"`
	Calls      []Call      `json:"calls"`
	Violations []Violation `json:"violations"`
	// InvariantChecks — qaysi invariant necha marta tekshirilgani.
	InvariantChecks map[string]int `json:"invariant_checks"`
	Routes          []RouteStat    `json:"routes"`
	Duplicates      []Duplicate    `json:"duplicates"`
	Coverage        Coverage       `json:"coverage"`
	// SlowBudgetMs — sekin deb belgilash chegarasi.
	SlowBudgetMs float64 `json:"slow_budget_ms"`
}

// Recorder — parallel xavfsiz yig'uvchi.
type Recorder struct {
	mu         sync.Mutex
	started    time.Time
	steps      []Step
	calls      []Call
	violations []Violation
	checks     map[string]int
}

func NewRecorder() *Recorder {
	return &Recorder{started: time.Now(), checks: map[string]int{}}
}

func (r *Recorder) AddCall(c Call) {
	r.mu.Lock()
	r.calls = append(r.calls, c)
	r.mu.Unlock()
}

func (r *Recorder) AddStep(s Step) {
	r.mu.Lock()
	r.steps = append(r.steps, s)
	r.mu.Unlock()
}

func (r *Recorder) AddViolation(v Violation) {
	r.mu.Lock()
	r.violations = append(r.violations, v)
	r.mu.Unlock()
}

// Checked — invariant bir marta tekshirildi (o'tdi yoki yiqildi — baribir).
func (r *Recorder) Checked(name string) {
	r.mu.Lock()
	r.checks[name]++
	r.mu.Unlock()
}

// CallsSince — oxirgi n-chaqiruvdan beri nechta so'rov bo'lgani (qadam uchun).
func (r *Recorder) CallCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

// Outcome — bitta endpoint qanday natijalar bilan urilgani.
type Outcome struct {
	Success int // 2xx/3xx: haqiqiy ishlash yo'li
	Gated   int // sozlanmagan xizmat (kutilgan 503/404 kontrakti)
	Other   int // qolgan (4xx rad etish yo'llari)
}

// Outcomes — endpoint bo'yicha natijalar. Qamrov ratcheti shunga tayanadi:
// faqat xato javob bilan urilgan endpoint "ishlaydi" deb sanalmaydi.
func (r *Recorder) Outcomes() map[string]Outcome {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[string]Outcome{}
	for _, c := range r.calls {
		if c.Route == "" {
			continue
		}
		k := c.Method + " " + c.Route
		o := out[k]
		switch {
		case c.Status < 400:
			o.Success++
		case c.Gated:
			o.Gated++
		default:
			o.Other++
		}
		out[k] = o
	}
	return out
}

// CoveredRoutes — kamida bir marta urilgan endpointlar.
func (r *Recorder) CoveredRoutes() map[string]bool {
	out := map[string]bool{}
	for k := range r.Outcomes() {
		out[k] = true
	}
	return out
}

// Build — yig'ilganlardan Report yasaydi.
func (r *Recorder) Build(cov Coverage, slowBudgetMs float64, commit string) Report {
	r.mu.Lock()
	defer r.mu.Unlock()

	byRoute := map[string][]float64{}
	err5 := map[string]int{}
	closed := map[string]int{}
	for _, c := range r.calls {
		key := c.Method + " " + c.Route
		if c.Route == "" {
			key = c.Method + " (noma'lum) " + c.Path
		}
		byRoute[key] = append(byRoute[key], c.Ms)
		switch {
		case c.Gated && c.Status == 503:
			closed[key]++
		case c.Status >= 500:
			err5[key]++
		}
	}
	stats := make([]RouteStat, 0, len(byRoute))
	for k, ms := range byRoute {
		sort.Float64s(ms)
		stats = append(stats, RouteStat{
			Route: k, Calls: len(ms),
			P50: percentile(ms, 0.50), P95: percentile(ms, 0.95), Max: ms[len(ms)-1],
			Errors5: err5[k], Closed: closed[k],
		})
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].P95 > stats[j].P95 })

	// Takroriy o'qish so'rovlari: bir qadamda bir xil GET `DuplicateThreshold`
	// martadan ko'p bo'lsa — ortiqcha reload/N+1 gumoni.
	// Kalit ishtirokchini ham o'z ichiga oladi: bir nechta foydalanuvchining
	// bir xil so'rovi takror emas.
	type dupKey struct{ scenario, step, method, path, actor string }
	dups := map[dupKey]int{}
	for _, c := range r.calls {
		if c.Method == "GET" && !c.Poll && c.Step != "" {
			dups[dupKey{c.Scenario, c.Step, c.Method, c.Path, c.Actor}]++
		}
	}
	duplicates := []Duplicate{}
	for k, n := range dups {
		if n >= DuplicateThreshold {
			duplicates = append(duplicates, Duplicate{k.scenario, k.step, k.method, k.path, n})
		}
	}
	sort.Slice(duplicates, func(i, j int) bool { return duplicates[i].Count > duplicates[j].Count })

	checks := make(map[string]int, len(r.checks))
	for k, v := range r.checks {
		checks[k] = v
	}
	return Report{
		StartedAt:  r.started,
		DurationMs: float64(time.Since(r.started).Microseconds()) / 1000,
		GitCommit:  commit,
		// Bo'sh ro'yxat JSON'da "null" EMAS, "[]" bo'lishi shart: hisobotni
		// o'qiydigan skriptlar (CI xulosasi) len(...) chaqiradi.
		Steps:           append([]Step{}, r.steps...),
		Calls:           append([]Call{}, r.calls...),
		Violations:      append([]Violation{}, r.violations...),
		InvariantChecks: checks,
		Routes:          append([]RouteStat{}, stats...),
		Duplicates:      duplicates,
		Coverage:        nonNilCoverage(cov),
		SlowBudgetMs:    slowBudgetMs,
	}
}

// nonNilCoverage - qamrovdagi bo'sh ro'yxatlarni "[]" ga aylantiradi ("null" emas).
func nonNilCoverage(c Coverage) Coverage {
	if c.GatedOnly == nil {
		c.GatedOnly = []string{}
	}
	if c.Uncovered == nil {
		c.Uncovered = []string{}
	}
	if c.Exempted == nil {
		c.Exempted = []string{}
	}
	return c
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}

// Save — JSON faylga yozadi (papka bo'lmasa yaratadi).
func Save(path string, rep Report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Load — JSON dan o'qiydi.
func Load(path string) (Report, error) {
	var rep Report
	data, err := os.ReadFile(path)
	if err != nil {
		return rep, err
	}
	return rep, json.Unmarshal(data, &rep)
}
