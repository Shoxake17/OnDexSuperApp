package e2e

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Route — manba koddan o'qilgan bitta endpoint e'loni.
type Route struct {
	Method  string
	Pattern string // /restaurants/{id}/staff
	re      *regexp.Regexp
	wild    int // `{...}` soni: kamroq = aniqroq (ServeMux qoidasi)
}

func (r Route) Key() string { return r.Method + " " + r.Pattern }

var routeDecl = regexp.MustCompile(`HandleFunc\("([A-Z]+) ([^"]+)"`)

// DiscoverRoutes — `internal/httpapi` dagi BARCHA endpointlarni manba
// koddan o'qiydi (himoyalangan ham, ochiq ham).
//
// ┌─ NEGA MANBA KODDAN ───────────────────────────────────────────────┐
// Qo'lda yuritiladigan ro'yxat darhol eskiradi (avtorizatsiya
// matritsasi shu sabab manba koddan o'qiydi, `authz_matrix_test.go`).
// Yangi endpoint qo'shilishi bilan u shu ro'yxatga O'ZI tushadi va
// ssenariysiz qolsa test yiqiladi — kelajakdagi funksiyalar ham doim
// avtomatik testga ega bo'ladi.
// └───────────────────────────────────────────────────────────────────┘
func DiscoverRoutes(dir string) ([]Route, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []Route
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		for _, m := range routeDecl.FindAllStringSubmatch(string(src), -1) {
			r := newRoute(m[1], m[2])
			if seen[r.Key()] {
				continue
			}
			seen[r.Key()] = true
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("birorta endpoint topilmadi (%s) — regex buzilganmi?", dir)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out, nil
}

var wildcard = regexp.MustCompile(`\{[^}]+\}`)

func newRoute(method, pattern string) Route {
	wild := len(wildcard.FindAllString(pattern, -1))
	expr := regexp.QuoteMeta(pattern)
	expr = regexp.MustCompile(`\\\{[^}]*\.\.\.\\\}`).ReplaceAllString(expr, `.+`)
	expr = regexp.MustCompile(`\\\{[^}]*\\\}`).ReplaceAllString(expr, `[^/]+`)
	return Route{Method: method, Pattern: pattern, re: regexp.MustCompile("^" + expr + "$"), wild: wild}
}

// MatchRoute — so'rovga mos ENG ANIQ e'lonni topadi ("" — topilmadi).
func MatchRoute(routes []Route, method, path string) string {
	best := -1
	for i, r := range routes {
		if r.Method != method || !r.re.MatchString(path) {
			continue
		}
		if best < 0 || r.wild < routes[best].wild {
			best = i
		}
	}
	if best < 0 {
		return ""
	}
	return routes[best].Pattern
}

// LoadExempt — `routes_exempt.txt`: "METHOD /yo'l  # sabab". Sabab MAJBURIY.
func LoadExempt(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\xef\xbb\xbf"))
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		spec, reason, ok := strings.Cut(text, "#")
		reason = strings.TrimSpace(reason)
		if !ok || reason == "" {
			return nil, fmt.Errorf("%s:%d: istisno sababsiz — `# sabab` yozing", path, line)
		}
		key := strings.Join(strings.Fields(spec), " ")
		if _, dup := out[key]; dup {
			return nil, fmt.Errorf("%s:%d: takroriy istisno %q", path, line, key)
		}
		out[key] = reason
	}
	return out, sc.Err()
}
