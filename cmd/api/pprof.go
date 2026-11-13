package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof" //nolint:gosec // G108: handlerlar FAQAT loopback'dagi alohida mux'ga ulanadi (pastda)
	"os"
	"time"

	"chustapp/internal/safego"

	"github.com/jackc/pgx/v5/pgxpool"
)

// pgPoolStat — pool holati; hisoblagichlar jarayon boshidan beri o'sib boradi,
// shuning uchun ikki o'lchov orasidagi farq olinadi.
type pgPoolStat struct {
	MaxConns          int32   `json:"max_conns"`
	TotalConns        int32   `json:"total_conns"`
	AcquiredConns     int32   `json:"acquired_conns"`
	IdleConns         int32   `json:"idle_conns"`
	AcquireCount      int64   `json:"acquire_count"`
	EmptyAcquireCount int64   `json:"empty_acquire_count"` // bo'sh ulanish yo'q edi -> kutdi
	AcquireMs         float64 `json:"acquire_ms_total"`
	CanceledAcquire   int64   `json:"canceled_acquire_count"`
	NewConnsCount     int64   `json:"new_conns_count"`
}

func pgPoolStats(s *pgxpool.Stat) pgPoolStat {
	return pgPoolStat{
		MaxConns: s.MaxConns(), TotalConns: s.TotalConns(), AcquiredConns: s.AcquiredConns(), IdleConns: s.IdleConns(),
		AcquireCount: s.AcquireCount(), EmptyAcquireCount: s.EmptyAcquireCount(),
		AcquireMs: float64(s.AcquireDuration().Microseconds()) / 1000, CanceledAcquire: s.CanceledAcquireCount(),
		NewConnsCount: s.NewConnsCount(),
	}
}

// startPprof — PPROF_ADDR berilsa profiler'ni ALOHIDA, faqat loopback
// manzilda ishga tushiradi (2026-11-12, INFRA-8 / yuklama testi).
//
// ┌─ XAVFSIZLIK ───────────────────────────────────────────────────────────┐
//   - Standart holatda O'CHIQ (PPROF_ADDR bo'sh).
//   - Faqat 127.0.0.1 / ::1 — boshqa manzil berilsa server TO'XTAYDI (sirli
//     ma'lumot: xotira, goroutine steklari tashqariga chiqmasin).
//   - Asosiy API mux'iga ulanmaydi; Caddy/Cloudflare orqali yetib bo'lmaydi.
//     Profil faqat konteyner tarmoq nomlar fazosidan olinadi
//     (`docker run --network container:ondex-api-1 ...`).
// └────────────────────────────────────────────────────────────────────────┘
//
// /debug/pgpool — pgx pool statistikasi (JSON): ulanish olishni kutish soni
// va jami vaqti. Yuklamada to'siq pool navbatimi yoki Postgres CPU'simi —
// shu farqlaydi (2026-11-14, INFRA-3 / PgBouncer). pool nil bo'lsa ulanmaydi.
func startPprof(pool *pgxpool.Pool) {
	addr := os.Getenv("PPROF_ADDR")
	if addr == "" {
		return
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil || !net.ParseIP(host).IsLoopback() {
		slog.Error("PPROF_ADDR faqat loopback bo'lishi mumkin (masalan 127.0.0.1:6060) — server to'xtatildi", "qiymat", addr)
		os.Exit(1)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	if pool != nil {
		mux.HandleFunc("/debug/pgpool", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(pgPoolStats(pool.Stat()))
		})
	}
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 2 * time.Minute}
	safego.Go("pprof", func() {
		slog.Info("profiler (faqat loopback)", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("profiler", "err", err)
		}
	})
}
