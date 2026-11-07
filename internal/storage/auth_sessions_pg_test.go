package storage

import (
	"context"
	"testing"

	"chustapp/internal/identity"
	"chustapp/internal/identity/identitytest"
)

// PgAuthSessionStore xotira ombori bilan BIR XIL shartnomaga bo'ysunadi.
func TestPgAuthSessionStoreContract(t *testing.T) {
	pool := realmTestPool(t)
	ctx := context.Background()
	const uid = "ctr_user_pg"
	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM auth_sessions WHERE user_id = $1`, uid)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, uid)
	}
	cleanup()
	t.Cleanup(cleanup)
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, phone, role) VALUES ($1, '+998987650099', 'restaurant')`, uid); err != nil {
		t.Fatal(err)
	}
	identitytest.RunStoreContract(t, func(t *testing.T) identity.Store {
		_, _ = pool.Exec(ctx, `DELETE FROM auth_sessions WHERE user_id = $1`, uid)
		return NewPgAuthSessionStore(pool)
	}, uid)
}

// Audit yozuvchisi haqiqiy jadvalga yozadi (va UTF-8 kesish buzmaydi).
func TestPgAuditLogRecord(t *testing.T) {
	pool := realmTestPool(t)
	ctx := context.Background()
	long := ""
	for i := 0; i < 400; i++ {
		long += "ў" // 2 baytli harf: baytda kesilsa yaroqsiz UTF-8 qolardi
	}
	NewPgAuditLog(pool).Record(ctx, identity.Event{ActorUserID: "audit_pg_test", Action: "test.audit_record",
		Result: identity.ResultOK, UserAgent: long, Details: map[string]any{"k": "v"}})
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE actor_user_id = 'audit_pg_test'
		AND action = 'test.audit_record' AND length(user_agent) = 300 AND details->>'k' = 'v'`).Scan(&n); err != nil || n < 1 {
		t.Fatalf("audit yozilmadi: %d %v", n, err)
	}
}
