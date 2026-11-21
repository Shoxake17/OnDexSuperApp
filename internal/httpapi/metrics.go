package httpapi

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ┌─ HAR ENDPOINT METRIKASI (Plane ONDEXECOSY-481, 2026-11-21) ────────┐
// Ilgari OnDex API so'rov vaqtini umuman o'lchamasdi: "qaysi endpoint
// sekin?" savoliga faqat taxmin bilan javob berilardi.
//
// QANDAY ISHLAYDI:
//   - `withMetrics` — ENG TASHQI qatlam: CORS preflight, body limiti, BFF
//     rad etgan so'rovlar ham hisobga tushadi (hech biri chetda qolmaydi).
//   - `captureRoute` — mux'ni bevosita o'raydi va mux tanlagan PATTERN'ni
//     (`GET /restaurants/{id}`) tashqi qatlamga yetkazadi. ServeMux
//     `r.Pattern` ni AYNAN o'zi olgan so'rovga yozadi; BFF so'rovni
//     klonlagani uchun qiymat kontekstdagi umumiy "quti" orqali uzatiladi.
//
// KARDINALLIK CHEGARALANGAN: yorliq — haqiqiy yo'l EMAS, pattern
// (`/restaurants/abc123` emas, `/restaurants/{id}`). Hech qaysi pattern'ga
// mos kelmagan so'rov — "unmatched" (tasodifiy skaner yo'llari yangi
// qator yaratmaydi).
//
// SHAXSIY MA'LUMOT YO'Q: faqat pattern, status kodi va yuza. Metrikalar
// asosiy portda EMAS — alohida ichki portda (`cmd/api`, METRICS_ADDR).
// └───────────────────────────────────────────────────────────────────┘

const (
	routeUnmatched = "unmatched"
	routePreflight = "preflight"
)

var (
	metricsRegistry = prometheus.NewRegistry()

	httpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ondex_http_requests_total",
		Help: "API so'rovlari: route (ServeMux pattern), status kodi, yuza (api / panel_web).",
	}, []string{"route", "code", "surface"})

	httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "ondex_http_request_duration_seconds",
		Help: "API so'rovi davomiyligi (javob to'liq yozilguncha). WebSocket (101) kiritilmaydi.",
		// 5 ms dan 30 s gacha: oddiy so'rovlar 5-250 ms, AI/xarita 1-10 s.
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30},
	}, []string{"route", "surface"})

	httpInFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ondex_http_requests_in_flight",
		Help: "Ayni paytda bajarilayotgan API so'rovlari (WebSocket ulanishlari tugagach chiqariladi).",
	})
)

func init() {
	metricsRegistry.MustRegister(
		httpRequests, httpDuration, httpInFlight,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
}

// MetricsRegistry — qo'shimcha kollektorlar (masalan DB pool) shu yerga.
func MetricsRegistry() *prometheus.Registry { return metricsRegistry }

// MetricsHandler — `/metrics` (Prometheus matn formati). FAQAT ichki portga
// ulanadi (`cmd/api`), asosiy API mux'iga EMAS.
func MetricsHandler() http.Handler {
	return promhttp.HandlerFor(metricsRegistry, promhttp.HandlerOpts{})
}

// routeBox — mux tanlagan pattern'ni tashqi qatlamga yetkazadigan quti.
type routeBox struct{ pattern string }

type routeBoxKey struct{}

// captureRoute — mux'ni o'raydi: u tanlagan pattern qutiga yoziladi.
//
// Pattern handler chaqirilishidan OLDIN aniqlanadi (`mux.Handler`):
// WebSocket handler'i ulanish yopilguncha qaytmaydi va u upgrade
// paytidayoq hisoblanishi kerak (`metricsWriter.Hijack`). E2E metrika
// qamrovi aynan shuni topgan: `GET /ai/live` hech qachon ko'rinmasdi.
func captureRoute(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, ok := r.Context().Value(routeBoxKey{}).(*routeBox)
		if ok {
			_, b.pattern = mux.Handler(r)
		}
		mux.ServeHTTP(w, r)
		if ok && r.Pattern != "" {
			b.pattern = r.Pattern
		}
	})
}

// withMetrics — har so'rovni hisoblaydi va vaqtini o'lchaydi.
func withMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		box := &routeBox{}
		mw := &metricsWriter{ResponseWriter: w, box: box, method: r.Method, surface: metricSurface(r.URL.Path)}
		httpInFlight.Inc()
		defer func() {
			httpInFlight.Dec()
			if !mw.counted {
				code := mw.status
				if code == 0 {
					// Handler hech narsa yozmagan — net/http 200 yuboradi.
					code = http.StatusOK
				}
				mw.count(code)
			}
			if !mw.hijacked {
				httpDuration.WithLabelValues(mw.route(), mw.surface).Observe(time.Since(start).Seconds())
			}
		}()
		next.ServeHTTP(mw, r.WithContext(context.WithValue(r.Context(), routeBoxKey{}, box)))
	})
}

// metricSurface — so'rov qaysi yuzadan kelgan (BFF yoki to'g'ridan-to'g'ri).
func metricSurface(path string) string {
	if path == "/r/api" || strings.HasPrefix(path, "/r/api/") {
		return "panel_web"
	}
	return "api"
}

// metricsWriter — status kodini ushlaydi. Hijacker/Flusher/ReaderFrom
// SAQLANADI: WebSocket (gorilla `w.(http.Hijacker)`), oqimli javoblar va
// fayl uzatish (sendfile) o'zgarishsiz ishlashi shart.
type metricsWriter struct {
	http.ResponseWriter
	box      *routeBox
	method   string
	surface  string
	status   int
	hijacked bool
	counted  bool
}

// route — mux tanlagan pattern yoki chegaralangan zaxira yorliq.
func (m *metricsWriter) route() string {
	if m.box != nil && m.box.pattern != "" {
		return m.box.pattern
	}
	if m.method == http.MethodOptions {
		return routePreflight
	}
	return routeUnmatched
}

// count — so'rovni BIR MARTA hisoblaydi.
func (m *metricsWriter) count(code int) {
	if m.counted {
		return
	}
	m.counted = true
	httpRequests.WithLabelValues(m.route(), strconv.Itoa(code), m.surface).Inc()
}

func (m *metricsWriter) WriteHeader(code int) {
	if m.status == 0 {
		m.status = code
	}
	m.ResponseWriter.WriteHeader(code)
}

func (m *metricsWriter) Write(b []byte) (int, error) {
	if m.status == 0 {
		m.status = http.StatusOK
	}
	return m.ResponseWriter.Write(b)
}

func (m *metricsWriter) Flush() {
	if m.status == 0 {
		m.status = http.StatusOK
	}
	if f, ok := m.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (m *metricsWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := m.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("http.Hijacker qo'llab-quvvatlanmaydi")
	}
	c, rw, err := h.Hijack()
	if err == nil {
		m.hijacked = true
		m.status = http.StatusSwitchingProtocols
		// Uzoq yashaydigan ulanish — upgrade paytidayoq hisoblanadi.
		m.count(http.StatusSwitchingProtocols)
	}
	return c, rw, err
}

func (m *metricsWriter) ReadFrom(src io.Reader) (int64, error) {
	if m.status == 0 {
		m.status = http.StatusOK
	}
	if rf, ok := m.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(src)
	}
	return io.Copy(m.ResponseWriter, src)
}

// Unwrap — `http.ResponseController` ichki writer'ga yetsin.
func (m *metricsWriter) Unwrap() http.ResponseWriter { return m.ResponseWriter }
