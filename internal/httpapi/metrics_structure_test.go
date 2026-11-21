package httpapi

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestSingleMuxWrappedByMetrics — "hech bir endpoint metrikadan chetda
// qolmasin" (ONDEXECOSY-481) kafolatining TUZILMAVIY qismi:
//
//   - paketda yagona API ServeMux bor (`Routes` ichida) — boshqa mux'ga
//     ulangan endpoint metrika zanjiridan tashqarida qolardi;
//   - `Routes` aynan `withMetrics(... captureRoute(mux) ...)` qaytaradi;
//
// E2E metrika qamrovi (internal/e2e) esa ishlaydigan endpointlarni amalda
// tekshiradi; bu test sozlanmagan (E2E'da ro'yxatga olinmagan) endpointlarni
// ham qoplaydi.
func TestSingleMuxWrappedByMetrics(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	newMux := regexp.MustCompile(`http\.NewServeMux\(\)`)
	// Haqiqiy HTTP server ochish (Hub.Serve kabi handler emas).
	listen := regexp.MustCompile(`http\.ListenAndServe|&http\.Server\{|http\.Server\{`)
	var muxes []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		s := string(src)
		for range newMux.FindAllStringIndex(s, -1) {
			muxes = append(muxes, f)
		}
		if listen.MatchString(s) {
			t.Errorf("%s: httpapi o'z serverini ochmasligi kerak (metrikasiz port)", f)
		}
	}
	if len(muxes) != 1 || muxes[0] != "server.go" {
		t.Fatalf("yagona ServeMux server.go/Routes da bo'lishi kerak, topildi: %v", muxes)
	}
	srv, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(srv), "return withMetrics(") || !strings.Contains(string(srv), "captureRoute(mux)") {
		t.Fatal("Routes withMetrics(... captureRoute(mux) ...) qaytarishi shart")
	}
}
