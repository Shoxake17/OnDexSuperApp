package main

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof" //nolint:gosec // G108: handlerlar FAQAT loopback'dagi alohida mux'ga ulanadi (pastda)
	"os"
	"time"

	"chustapp/internal/safego"
)

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
func startPprof() {
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
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 2 * time.Minute}
	safego.Go("pprof", func() {
		slog.Info("profiler (faqat loopback)", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("profiler", "err", err)
		}
	})
}
