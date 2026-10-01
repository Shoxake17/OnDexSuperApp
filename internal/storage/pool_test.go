package storage

import (
	"context"
	"testing"
)

func TestNewPoolDefaultsAndOverride(t *testing.T) {
	ctx := context.Background()
	// pgxpool.NewWithConfig sekin ulanmaydi (lazy) — bazasiz tekshirsa bo'ladi.
	p, err := NewPool(ctx, "postgres://u:p@127.0.0.1:1/db?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if got := p.Config().MaxConns; got != DefaultMaxConns {
		t.Fatalf("MaxConns = %d, kutilgan %d", got, DefaultMaxConns)
	}
	q, err := NewPool(ctx, "postgres://u:p@127.0.0.1:1/db?sslmode=disable&pool_max_conns=7")
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	if got := q.Config().MaxConns; got != 7 {
		t.Fatalf("DSN qiymati e'tiborsiz qoldi: MaxConns = %d", got)
	}
}
