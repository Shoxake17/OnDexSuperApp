package storage

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/wallet"
)

// PostgresWalletRepo — OnDex Wallet ledger (migratsiya 0057).
type PostgresWalletRepo struct{ pool *pgxpool.Pool }

func NewPostgresWalletRepo(pool *pgxpool.Pool) *PostgresWalletRepo {
	return &PostgresWalletRepo{pool: pool}
}

// applyLocked — Credit/Debit umumiy o'zagi: foydalanuvchi bo'yicha
// advisory lock ostida joriy balansni o'qiydi, `check` orqali
// tekshiradi (Debit uchun yetarlilik), yozuvni qo'shadi.
//
// ┌─ NEGA ALOHIDA "balans" USTUNI EMAS, ADVISORY LOCK ────────────────┐
// Balans doim `SUM(amount_tiyin)` dan hisoblanadi — qo'shimcha ustun
// bo'lmagani uchun u ledger'dan HECH QACHON og'ib ketolmaydi (bitta
// haqiqat manbai). Concurrency muammosi (ikkita parallel debit bir
// vaqtda "yetarli" deb topib, balansni manfiyga tushirib qo'yishi)
// esa foydalanuvchi bo'yicha `pg_advisory_xact_lock` bilan yopiladi —
// xuddi `internal/staff`da restoran bo'yicha tartib raqami berishda
// ishlatilgan naqsh (postgres_staff.go).
// └───────────────────────────────────────────────────────────────────┘
func (r *PostgresWalletRepo) applyLocked(ctx context.Context, t *wallet.Transaction, signedAmount int64,
	check func(balanceBefore int64) error) (balanceAfter int64, applied bool, err error) {

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Commit'dan keyin no-op

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('wallet:' || $1))`, t.UserID); err != nil {
		return 0, false, err
	}

	var before int64
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount_tiyin),0) FROM wallet_transactions WHERE user_id = $1`,
		t.UserID).Scan(&before); err != nil {
		return 0, false, err
	}
	if check != nil {
		if err := check(before); err != nil {
			return before, false, err
		}
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO wallet_transactions (id, user_id, order_id, amount_tiyin, type, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		t.ID, t.UserID, t.OrderID, signedAmount, t.Type, t.CreatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		// Bu (order_id, type) juftligi ALLAQACHON qayd etilgan —
		// idempotent: ikkinchi marta qo'shmaymiz, xato ham emas.
		return before, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, false, err
	}
	return before + signedAmount, true, nil
}

func (r *PostgresWalletRepo) Credit(ctx context.Context, t *wallet.Transaction) (int64, bool, error) {
	return r.applyLocked(ctx, t, t.AmountTiyin, nil)
}

func (r *PostgresWalletRepo) Debit(ctx context.Context, t *wallet.Transaction) (int64, bool, error) {
	return r.applyLocked(ctx, t, -t.AmountTiyin, func(before int64) error {
		if before < t.AmountTiyin {
			return wallet.ErrInsufficientBalance
		}
		return nil
	})
}

func (r *PostgresWalletRepo) Balance(ctx context.Context, userID string) (int64, error) {
	var balance int64
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount_tiyin),0) FROM wallet_transactions WHERE user_id = $1`,
		userID).Scan(&balance)
	return balance, err
}

func (r *PostgresWalletRepo) ListTransactions(ctx context.Context, userID string, limit int) ([]*wallet.Transaction, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, order_id, amount_tiyin, type, created_at
		FROM wallet_transactions WHERE user_id = $1
		ORDER BY created_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*wallet.Transaction
	for rows.Next() {
		var t wallet.Transaction
		var signed int64
		if err := rows.Scan(&t.ID, &t.UserID, &t.OrderID, &signed, &t.Type, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.AmountTiyin = signed
		if t.AmountTiyin < 0 {
			t.AmountTiyin = -t.AmountTiyin
		}
		out = append(out, &t)
	}
	return out, rows.Err()
}
