package identity

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// GuardStore — taqsimlangan xato urinishlar hisobi (Redis:
// `storage.RedisGuardStore`). Bir nechta API nusxasi va restartlar bitta
// hisobni ko'radi.
type GuardStore interface {
	Begin(ctx context.Context, key string, max int, window time.Duration) (bool, error)
	Finish(ctx context.Context, key, result string, window time.Duration) error
	Fail(ctx context.Context, key string, window time.Duration) error
	Reset(ctx context.Context, key string) error
	Count(ctx context.Context, key string) (int, error)
}

// attemptGuard — `failGuard` (xotira) va `distGuard` (Redis) umumiy shakli.
type attemptGuard interface {
	Begin(key string) (end func(attemptResult), ok bool)
	Blocked(key string) bool
	Fail(key string)
	Reset(key string)
}

var _ attemptGuard = (*failGuard)(nil)

// guardTimeout — Redis javob bermasa kirish KUTIB QOLMASIN.
const guardTimeout = 300 * time.Millisecond

// distGuard — avval Redis; xato bo'lsa (Redis o'chgan) — xotiradagi
// `local` (fail-open EMAS: lokal hisob baribir cheklaydi).
type distGuard struct {
	store    GuardStore
	local    *failGuard
	max      int
	window   time.Duration
	lastWarn atomic.Int64
}

func newDistGuard(store GuardStore, local *failGuard) *distGuard {
	return &distGuard{store: store, local: local, max: local.max, window: local.window}
}

func (g *distGuard) warn(op string, err error) {
	now := time.Now().Unix()
	if prev := g.lastWarn.Load(); now-prev >= 30 && g.lastWarn.CompareAndSwap(prev, now) {
		slog.Warn("identity: urinishlar hisobi Redis'da ishlamadi — xotiradagi hisob ishlatilmoqda", "op", op, "err", err)
	}
}

func guardCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), guardTimeout)
}

func (g *distGuard) Begin(key string) (func(attemptResult), bool) {
	ctx, cancel := guardCtx()
	ok, err := g.store.Begin(ctx, key, g.max, g.window)
	cancel()
	if err != nil {
		g.warn("begin", err)
		return g.local.Begin(key)
	}
	if !ok {
		return nil, false
	}
	var once sync.Once
	return func(r attemptResult) {
		once.Do(func() {
			res := "abort"
			switch r {
			case attemptOK:
				res = "ok"
			case attemptFail:
				res = "fail"
			}
			ctx, cancel := guardCtx()
			defer cancel()
			if err := g.store.Finish(ctx, key, res, g.window); err != nil {
				g.warn("finish", err)
				if r == attemptFail {
					g.local.Fail(key)
				}
			}
		})
	}, true
}

func (g *distGuard) Blocked(key string) bool {
	ctx, cancel := guardCtx()
	n, err := g.store.Count(ctx, key)
	cancel()
	if err != nil {
		g.warn("count", err)
		return g.local.Blocked(key)
	}
	return n >= g.max || g.local.Blocked(key)
}

func (g *distGuard) Fail(key string) {
	ctx, cancel := guardCtx()
	defer cancel()
	if err := g.store.Fail(ctx, key, g.window); err != nil {
		g.warn("fail", err)
		g.local.Fail(key)
	}
}

func (g *distGuard) Reset(key string) {
	ctx, cancel := guardCtx()
	defer cancel()
	if err := g.store.Reset(ctx, key); err != nil {
		g.warn("reset", err)
	}
	g.local.Reset(key)
}
