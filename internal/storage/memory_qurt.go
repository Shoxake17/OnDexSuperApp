package storage

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"chustapp/internal/qurt"
)

// MemoryQurtRepo — xotiradagi qurt mahsulotlari (Mongo yo'q rejim va testlar).
type MemoryQurtRepo struct {
	mu    sync.Mutex
	items map[string]*qurt.Product // id -> mahsulot
	newID func() string
	seq   int
}

func NewMemoryQurtRepo() *MemoryQurtRepo {
	r := &MemoryQurtRepo{items: map[string]*qurt.Product{}}
	r.newID = func() string {
		r.seq++
		return "qp-" + strings.Repeat("0", 4) + itoa(r.seq)
	}
	return r
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func (r *MemoryQurtRepo) Upsert(_ context.Context, p *qurt.Product) (*qurt.Product, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *p
	cp.UpdatedAt = time.Now()
	for id, old := range r.items {
		if old.StoreID == p.StoreID && old.ExternalID == p.ExternalID {
			cp.ID = id
			cp.Reserved = old.Reserved // band OnDex'niki — POS yangilashi uni o'chirmasin
			if !p.StockSet {
				cp.Stock = old.Stock
			}
			r.items[id] = &cp
			out := cp
			return &out, nil
		}
	}
	cp.ID = r.newID()
	r.items[cp.ID] = &cp
	out := cp
	return &out, nil
}

func (r *MemoryQurtRepo) Delete(_ context.Context, storeID, externalID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, old := range r.items {
		if old.StoreID == storeID && old.ExternalID == externalID {
			delete(r.items, id)
		}
	}
	return nil
}

func (r *MemoryQurtRepo) DeleteByStore(_ context.Context, storeID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, old := range r.items {
		if old.StoreID == storeID {
			delete(r.items, id)
		}
	}
	return nil
}

func (r *MemoryQurtRepo) List(_ context.Context, f qurt.Filter) ([]*qurt.Product, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	allowed := map[string]bool{}
	for _, id := range f.StoreIDs {
		allowed[id] = true
	}
	q := strings.ToLower(strings.TrimSpace(f.Query))
	var out []*qurt.Product
	for _, p := range r.items {
		if !allowed[p.StoreID] || !p.Available {
			continue
		}
		if f.Category != "" && p.Category != f.Category {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(p.Name+" "+p.Composition), q) {
			continue
		}
		cp := *p
		cp.StockLeft = cp.Left()
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		return out[i].ID < out[j].ID
	})
	if f.Offset > 0 {
		if f.Offset >= len(out) {
			return nil, nil
		}
		out = out[f.Offset:]
	}
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func (r *MemoryQurtRepo) Categories(_ context.Context, storeIDs []string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	allowed := map[string]bool{}
	for _, id := range storeIDs {
		allowed[id] = true
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range r.items {
		if allowed[p.StoreID] && p.Available && !seen[p.Category] {
			seen[p.Category] = true
			out = append(out, p.Category)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (r *MemoryQurtRepo) GetMany(_ context.Context, ids []string) ([]*qurt.Product, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*qurt.Product
	for _, id := range ids {
		if p, ok := r.items[id]; ok {
			cp := *p
			cp.StockLeft = cp.Left()
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *MemoryQurtRepo) Reserve(_ context.Context, items []qurt.Reservation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	need := map[string]int{}
	for _, it := range items {
		need[it.ProductID] += it.Qty
	}
	for id, qty := range need { // avval hammasini tekshiramiz — yarim band qolmasin
		p, ok := r.items[id]
		if !ok || qty <= 0 || p.Left() < qty {
			return qurt.ErrInsufficientStock
		}
	}
	for id, qty := range need {
		r.items[id].Reserved += qty
	}
	return nil
}

func (r *MemoryQurtRepo) Release(_ context.Context, items []qurt.Reservation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, it := range items {
		if p, ok := r.items[it.ProductID]; ok {
			p.Reserved -= it.Qty
			if p.Reserved < 0 {
				p.Reserved = 0
			}
		}
	}
	return nil
}