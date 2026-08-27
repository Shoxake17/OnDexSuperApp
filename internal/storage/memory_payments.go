package storage

import (
	"context"
	"sort"
	"sync"
	"time"

	"chustapp/internal/payments"
)

// MemoryPaymentRepo — testlar va dev rejimi uchun (Postgres'siz).
//
// Nusxa qaytaradi: chaqiruvchi olgan obyektni o'zgartirsa, ombordagi
// yozuv jimgina o'zgarib qolmasligi kerak — Postgres implementatsiyasi
// bilan bir xil xatti-harakat, aks holda testlar haqiqatni
// ko'rsatmasdi.
type MemoryPaymentRepo struct {
	mu    sync.RWMutex
	items map[string]payments.Payment
}

func NewMemoryPaymentRepo() *MemoryPaymentRepo {
	return &MemoryPaymentRepo{items: map[string]payments.Payment{}}
}

func (r *MemoryPaymentRepo) Create(ctx context.Context, p *payments.Payment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[p.ID] = *p
	return nil
}

func (r *MemoryPaymentRepo) Update(ctx context.Context, p *payments.Payment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[p.ID]; !ok {
		return payments.ErrNotFound
	}
	r.items[p.ID] = *p
	return nil
}

func (r *MemoryPaymentRepo) GetByID(ctx context.Context, id string) (*payments.Payment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.items[id]
	if !ok {
		return nil, payments.ErrNotFound
	}
	return &p, nil
}

func (r *MemoryPaymentRepo) GetByProviderID(ctx context.Context, provider, providerPaymentID string) (*payments.Payment, error) {
	if providerPaymentID == "" {
		return nil, payments.ErrNotFound
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.items {
		if p.Provider == provider && p.ProviderPaymentID == providerPaymentID {
			out := p
			return &out, nil
		}
	}
	return nil, payments.ErrNotFound
}

func (r *MemoryPaymentRepo) ListByOrder(ctx context.Context, orderID string) ([]*payments.Payment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*payments.Payment
	for _, p := range r.items {
		if p.OrderID == orderID {
			cp := p
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (r *MemoryPaymentRepo) ListExpired(ctx context.Context, now time.Time, limit int) ([]*payments.Payment, error) {
	if limit <= 0 {
		limit = 100
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*payments.Payment
	for _, p := range r.items {
		if p.Status == payments.StatusPending && p.ExpiresAt.Before(now) {
			cp := p
			out = append(out, &cp)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}
