package main

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"chustapp/internal/httpapi"
	"chustapp/internal/safego"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// startMetrics — METRICS_ADDR berilsa Prometheus `/metrics` ni ALOHIDA
// portda ochadi (Plane ONDEXECOSY-481).
//
// ┌─ XAVFSIZLIK ──────────────────────────────────────────────────────┐
//   - Standart holatda O'CHIQ (METRICS_ADDR bo'sh).
//   - Asosiy API mux'ida EMAS: Caddy/Cloudflare bu portga yo'naltirmaydi,
//     host'ga ham chiqarilmaydi — faqat docker monitoring tarmog'idan
//     Prometheus o'qiydi.
//   - Ichida SHAXSIY MA'LUMOT YO'Q: endpoint pattern'i, status kodi,
//     vaqt va jarayon/pool hisoblagichlari xolos.
//   - Faqat GET /metrics; boshqa hamma yo'l 404.
//
// └───────────────────────────────────────────────────────────────────┘
func startMetrics(pool *pgxpool.Pool) {
	addr := os.Getenv("METRICS_ADDR")
	if addr == "" {
		return
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		slog.Error("METRICS_ADDR noto'g'ri (masalan :9464) — metrika o'chiq", "qiymat", addr, "err", err)
		return
	}
	if pool != nil {
		registerPoolMetrics(pool)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", httpapi.MetricsHandler())
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	safego.Go("metrics", func() {
		slog.Info("rejim: Prometheus metrikasi yoqilgan (ichki port)", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("metrika serveri to'xtadi", "err", err)
		}
	})
}

// registerPoolMetrics — Postgres ulanish pool'i: navbat (bo'sh ulanish
// yo'qligidan kutish) — so'rovlar sekinlashishining birinchi sababi.
func registerPoolMetrics(pool *pgxpool.Pool) {
	g := func(name, help string, f func(*pgxpool.Stat) float64) prometheus.Collector {
		return prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: name, Help: help},
			func() float64 { return f(pool.Stat()) })
	}
	c := func(name, help string, f func(*pgxpool.Stat) float64) prometheus.Collector {
		return prometheus.NewCounterFunc(prometheus.CounterOpts{Name: name, Help: help},
			func() float64 { return f(pool.Stat()) })
	}
	httpapi.MetricsRegistry().MustRegister(
		g("ondex_pg_pool_max_conns", "Pool chegarasi.", func(s *pgxpool.Stat) float64 { return float64(s.MaxConns()) }),
		g("ondex_pg_pool_total_conns", "Ochiq ulanishlar.", func(s *pgxpool.Stat) float64 { return float64(s.TotalConns()) }),
		g("ondex_pg_pool_acquired_conns", "Band ulanishlar.", func(s *pgxpool.Stat) float64 { return float64(s.AcquiredConns()) }),
		g("ondex_pg_pool_idle_conns", "Bo'sh ulanishlar.", func(s *pgxpool.Stat) float64 { return float64(s.IdleConns()) }),
		c("ondex_pg_pool_acquire_total", "Ulanish olish soni.", func(s *pgxpool.Stat) float64 { return float64(s.AcquireCount()) }),
		c("ondex_pg_pool_empty_acquire_total", "Bo'sh ulanish yo'q edi — kutildi.", func(s *pgxpool.Stat) float64 { return float64(s.EmptyAcquireCount()) }),
		c("ondex_pg_pool_acquire_seconds_total", "Ulanish olishga ketgan jami vaqt.", func(s *pgxpool.Stat) float64 { return s.AcquireDuration().Seconds() }),
	)
}
