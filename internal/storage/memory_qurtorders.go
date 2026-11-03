package storage

import (
	"context"
	"sort"
	"sync"

	"chustapp/internal/qurtorders"
)

// MemoryQurtOrderRepo — xotiradagi qurt buyurtmalari (Mongo yo'q rejim va testlar).
type MemoryQurtOrderRepo struct {
	mu     sync.Mutex
	orders map[string]*qurtorders.Order
	seq    int64 // yaratilish tartibi (vaqt bir xil bo'lsa ham barqaror saralash)
	order  map[string]int64
}

func NewMemoryQurtOrderRepo() *MemoryQurtOrderRepo {
	return &MemoryQurtOrderRepo{orders: map[string]*qurtorders.Order{}, order: map[string]int64{}}
}

func cloneQurtOrder(o *qurtorders.Order) *qurtorders.Order {
	cp := *o
	cp.Items = append([]qurtorders.Item(nil), o.Items...)
	cp.History = append([]qurtorders.StatusChange(nil), o.History...)
	return &cp
}

func (r *MemoryQurtOrderRepo) CreateMany(_ context.Context, list []*qurtorders.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, o := range list {
		r.seq++
		r.orders[o.ID] = cloneQurtOrder(o)
		r.order[o.ID] = r.seq
	}
	return nil
}

func (r *MemoryQurtOrderRepo) Get(_ context.Context, id string) (*qurtorders.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	o, ok := r.orders[id]
	if !ok {
		return nil, qurtorders.ErrNotFound
	}
	return cloneQurtOrder(o), nil
}

func (r *MemoryQurtOrderRepo) List(_ context.Context, f qurtorders.Filter) ([]*qurtorders.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	want := map[qurtorders.Status]bool{}
	for _, s := range f.Statuses {
		want[s] = true
	}
	var out []*qurtorders.Order
	for _, o := range r.orders {
		if f.StoreID != "" && o.StoreID != f.StoreID {
			continue
		}
		if f.CustomerID != "" && o.CustomerID != f.CustomerID {
			continue
		}
		if f.DeliveryDate != "" && o.DeliveryDate != f.DeliveryDate {
			continue
		}
		if f.CourierID != "" && o.CourierID != f.CourierID {
			continue
		}
		if f.CashStatus != "" && o.CashStatus != f.CashStatus {
			continue
		}
		if len(want) > 0 && !want[o.Status] {
			continue
		}
		out = append(out, cloneQurtOrder(o))
	}
	sort.Slice(out, func(i, j int) bool { return r.order[out[i].ID] > r.order[out[j].ID] })
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

func (r *MemoryQurtOrderRepo) ByIdempotency(_ context.Context, customerID, key string) ([]*qurtorders.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*qurtorders.Order
	for _, o := range r.orders {
		if o.CustomerID == customerID && o.IdempotencyKey == key {
			out = append(out, cloneQurtOrder(o))
		}
	}
	sort.Slice(out, func(i, j int) bool { return r.order[out[i].ID] < r.order[out[j].ID] })
	return out, nil
}

func (r *MemoryQurtOrderRepo) Update(_ context.Context, o *qurtorders.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.orders[o.ID]
	if !ok {
		return qurtorders.ErrNotFound
	}
	if cur.Version != o.Version-1 {
		return qurtorders.ErrConflict
	}
	r.orders[o.ID] = cloneQurtOrder(o)
	return nil
}
