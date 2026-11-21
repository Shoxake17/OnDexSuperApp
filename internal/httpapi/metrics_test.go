package httpapi

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func countFor(route, code, surface string) float64 {
	return testutil.ToFloat64(httpRequests.WithLabelValues(route, code, surface))
}

func TestMetricsLabelsByMuxPattern(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /mtest/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	mux.HandleFunc("POST /mtest/{id}/x", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	h := withMetrics(captureRoute(mux))

	before := countFor("GET /mtest/{id}", "418", "api")
	for _, id := range []string{"a1", "b2", "c3"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/mtest/"+id, nil))
	}
	if got := countFor("GET /mtest/{id}", "418", "api") - before; got != 3 {
		t.Fatalf("pattern bo'yicha 3 ta kutilgan edi, %v", got)
	}

	before = countFor("POST /mtest/{id}/x", "200", "api")
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/mtest/z/x", nil))
	if got := countFor("POST /mtest/{id}/x", "200", "api") - before; got != 1 {
		t.Fatalf("Write -> 200 kutilgan edi, %v", got)
	}
}

func TestMetricsUnmatchedAndPreflight(t *testing.T) {
	h := withMetrics(captureRoute(http.NewServeMux()))
	b404 := countFor(routeUnmatched, "404", "api")
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/random/scan/path.php", nil))
	if countFor(routeUnmatched, "404", "api")-b404 != 1 {
		t.Fatal("mos kelmagan yo'l 'unmatched' bo'lishi kerak (haqiqiy yo'l yorliq bo'lmasin)")
	}

	// CORS qatlami preflight'ni mux'ga yetkazmay javob beradi.
	pre := withMetrics(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	bp := countFor(routePreflight, "204", "api")
	pre.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("OPTIONS", "/orders", nil))
	if countFor(routePreflight, "204", "api")-bp != 1 {
		t.Fatal("OPTIONS preflight alohida hisoblanishi kerak")
	}
}

func TestMetricsSeesPatternThroughClonedRequest(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /mclone/{id}", func(http.ResponseWriter, *http.Request) {})
	inner := captureRoute(mux)
	// BFF kabi: so'rov klonlanadi va yo'li qayta yoziladi.
	bff := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r2 := r.Clone(r.Context())
		r2.URL.Path = strings.TrimPrefix(r.URL.Path, "/r/api")
		inner.ServeHTTP(w, r2)
	})
	h := withMetrics(bff)
	b := countFor("GET /mclone/{id}", "200", "panel_web")
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/r/api/mclone/7", nil))
	if countFor("GET /mclone/{id}", "200", "panel_web")-b != 1 {
		t.Fatal("klonlangan (BFF) so'rovda ham pattern va panel_web yuzasi bo'lishi kerak")
	}
}

func TestMetricsWriterKeepsHijackAndFlush(t *testing.T) {
	var hijackOK, flushOK bool
	h := withMetrics(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, flushOK = w.(http.Flusher)
		hj, ok := w.(http.Hijacker)
		hijackOK = ok
		if !ok {
			return
		}
		c, rw, err := hj.Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\n\r\n")
		_ = rw.Flush()
		_ = c.Close()
	}))
	srv := httptest.NewServer(h)
	defer srv.Close()
	res, err := http.Get(srv.URL + "/ws")
	if err == nil {
		_ = res.Body.Close()
	}
	if !hijackOK || !flushOK {
		t.Fatalf("Hijacker=%v Flusher=%v — WebSocket/oqim buziladi", hijackOK, flushOK)
	}
	if countFor(routeUnmatched, "101", "api") < 1 {
		t.Fatal("hijack 101 sifatida hisoblanishi kerak")
	}
}

func TestMetricsHandlerExposesSeries(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /mexpose", func(http.ResponseWriter, *http.Request) {})
	withMetrics(captureRoute(mux)).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/mexpose", nil))

	rec := httptest.NewRecorder()
	MetricsHandler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body := rec.Body.String()
	for _, want := range []string{
		`ondex_http_requests_total{code="200",route="GET /mexpose",surface="api"}`,
		`ondex_http_request_duration_seconds_bucket{route="GET /mexpose",surface="api",le="0.005"}`,
		"ondex_http_requests_in_flight",
		"go_goroutines",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/metrics da yo'q: %s", want)
		}
	}
	sc := bufio.NewScanner(strings.NewReader(body))
	for sc.Scan() {
		if strings.Contains(sc.Text(), "/mtest/a1") {
			t.Fatal("haqiqiy yo'l yorliqqa tushdi — kardinallik chegarasi buzilgan")
		}
	}
}
