package storage

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgconn/ctxwatch"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DefaultMaxConns — DSN'da `pool_max_conns` berilmagan bo'lsa ishlatiladigan
// ulanishlar chegarasi.
//
// Nega: pgx standarti max(4, CPU soni) — 6 yadroli VPS'da atigi 6 ulanish.
// Postgres so'rovlari ms'larda tugaydi, lekin buyurtma yaratish tranzaksiya
// ichida bir nechta so'rov qiladi: 6 ulanishda yuk oshganda so'rovlar
// pool navbatida kutadi (kechikish ortadi), bazaning o'zi esa bo'sh turadi.
// 20 — Postgres'ning standart `max_connections=100` chegarasidan ancha past,
// ya'ni boshqa mijozlar (zaxira, admin) uchun joy qoladi.
const DefaultMaxConns = 20

// NewPool — pgx pool; DSN'da `pool_max_conns` bo'lsa o'sha qiymat, aks holda
// DefaultMaxConns.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	if !strings.Contains(dsn, "pool_max_conns") {
		cfg.MaxConns = DefaultMaxConns
	}
	if !strings.Contains(dsn, "pool_min_conns") {
		cfg.MinConns = 2 // birinchi so'rovlar ulanish ochishni kutmasin
	}
	cfg.ConnConfig.BuildContextWatcherHandler = cancelRequestHandler
	return pgxpool.NewWithConfig(ctx, cfg)
}

// cancelDeadlineDelay — Postgres bekor qilish so'roviga javob bermasa,
// shundan keyin ulanish baribir uziladi (zaxira yo'l).
const cancelDeadlineDelay = 2 * time.Second

// cancelRequestHandler — so'rov konteksti bekor bo'lganda (mijoz uzildi,
// timeout) ulanishni YOPMAYDI: Postgres'ga cancel yuboradi, ulanish pool'ga
// qaytadi.
//
// ┌─ NEGA ─────────────────────────────────────────────────────────────┐
// pgx standarti ulanishga darhol deadline qo'yadi -> ulanish buziladi va
// yopiladi. Yuklama testida (2026-11-14) to'yinish paytida 1511 marta
// qayta ulanildi: har biri SCRAM autentifikatsiyasi (API va Postgres
// CPU'si) -> sekinlik -> yana timeout -> yana qayta ulanish (spiral).
// └────────────────────────────────────────────────────────────────────┘
func cancelRequestHandler(pgConn *pgconn.PgConn) ctxwatch.Handler {
	return &pgconn.CancelRequestContextWatcherHandler{Conn: pgConn, DeadlineDelay: cancelDeadlineDelay}
}
