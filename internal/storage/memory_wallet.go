package storage

import (
	"context"
	"sort"
	"sync"

	"chustapp/internal/wallet"
)

// MemoryWalletRepo — testlar va dev rejimi uchun (Postgres'siz).
type MemoryWalletRepo struct {
	mu    sync.Mutex
	items []wallet.Transaction
}

func NewMemoryWalletRepo() *MemoryWalletRepo {
	return &MemoryWalletRepo{}
}

func (r *MemoryWalletRepo) balanceLocked(userID string) int64 {
	var balance int64
	for _, t := range r.items {
		if t.UserID == userID {
			balance += t.AmountTiyin // saqlanganda ISHORALI (credit=+, debit=-)
		}
	}
	return balance
}

func (r *MemoryWalletRepo) apply(ctx context.Context, t *wallet.Transaction, signedAmount int64,
	check func(before int64) error) (int64, bool, error) {

	r.mu.Lock()
	defer r.mu.Unlock()

	if t.OrderID != "" {
		for _, ex := range r.items {
			if ex.OrderID == t.OrderID && ex.Type == t.Type {
				return r.balanceLocked(t.UserID), false, nil
			}
		}
	}

	before := r.balanceLocked(t.UserID)
	if check != nil {
		if err := check(before); err != nil {
			return before, false, err
		}
	}

	cp := *t
	cp.AmountTiyin = signedAmount
	r.items = append(r.items, cp)
	return before + signedAmount, true, nil
}

func (r *MemoryWalletRepo) Credit(ctx context.Context, t *wallet.Transaction) (int64, bool, error) {
	return r.apply(ctx, t, t.AmountTiyin, nil)
}

func (r *MemoryWalletRepo) Debit(ctx context.Context, t *wallet.Transaction) (int64, bool, error) {
	return r.apply(ctx, t, -t.AmountTiyin, func(before int64) error {
		if before < t.AmountTiyin {
			return wallet.ErrInsufficientBalance
		}
		return nil
	})
}

func (r *MemoryWalletRepo) Balance(ctx context.Context, userID string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.balanceLocked(userID), nil
}

func (r *MemoryWalletRepo) ListTransactions(ctx context.Context, userID string, limit int) ([]*wallet.Transaction, error) {
	if limit <= 0 {
		limit = 50
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*wallet.Transaction
	for _, t := range r.items {
		if t.UserID != userID {
			continue
		}
		cp := t
		if cp.AmountTiyin < 0 {
			cp.AmountTiyin = -cp.AmountTiyin
		}
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
