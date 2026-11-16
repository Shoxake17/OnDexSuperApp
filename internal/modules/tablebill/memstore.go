package tablebill

import (
	"context"
	"sort"
	"sync"
	"time"
)

// MemStore — xotiradagi ombor (testlar, DATABASE_URL'siz dev). Bazadagi
// cheklovlarni takrorlaydi: stolda bitta ochiq hisob, buyurtma bitta
// hisobda, AddShare versiya qulfi.
type MemStore struct {
	mu     sync.Mutex
	bills  map[string]Bill
	orders map[string]string // order -> bill
	shares map[string]Share
}

func NewMemStore() *MemStore {
	return &MemStore{bills: map[string]Bill{}, orders: map[string]string{}, shares: map[string]Share{}}
}

func (m *MemStore) GetOpen(_ context.Context, tableID string) (*Bill, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, b := range m.bills {
		if b.TableID == tableID && b.Status == StatusOpen {
			c := b
			return &c, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MemStore) CreateOpen(_ context.Context, b *Bill) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.bills {
		if x.TableID == b.TableID && x.Status == StatusOpen {
			return ErrConflict
		}
	}
	m.bills[b.ID] = *b
	return nil
}

func (m *MemStore) Get(_ context.Context, id string) (*Bill, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.bills[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &b, nil
}

func (m *MemStore) ListOpen(_ context.Context, restaurantID string) ([]*Bill, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*Bill
	for _, b := range m.bills {
		if b.RestaurantID == restaurantID && b.Status == StatusOpen {
			c := b
			out = append(out, &c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (m *MemStore) Attach(_ context.Context, billID string, orderIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range orderIDs {
		if _, ok := m.orders[id]; !ok {
			m.orders[id] = billID
		}
	}
	return nil
}

func (m *MemStore) OrderIDs(_ context.Context, billID string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	for o, b := range m.orders {
		if b == billID {
			out = append(out, o)
		}
	}
	return out, nil
}

func (m *MemStore) Attached(_ context.Context, orderIDs []string) (map[string]bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]bool{}
	for _, id := range orderIDs {
		if _, ok := m.orders[id]; ok {
			out[id] = true
		}
	}
	return out, nil
}

func (m *MemStore) SetDeposit(_ context.Context, billID, reservationID string, amount int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.bills[billID]
	if !ok {
		return ErrNotFound
	}
	if b.DepositReservationID != "" {
		return ErrConflict
	}
	b.DepositReservationID, b.DepositTiyin = reservationID, amount
	m.bills[billID] = b
	return nil
}

func (m *MemStore) Shares(_ context.Context, billID string) ([]*Share, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*Share
	for _, s := range m.shares {
		if s.BillID == billID {
			c := s
			out = append(out, &c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (m *MemStore) GetShare(_ context.Context, id string) (*Share, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.shares[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &s, nil
}

func (m *MemStore) AddShare(_ context.Context, s *Share, version int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.bills[s.BillID]
	if !ok {
		return ErrNotFound
	}
	if b.Version != version {
		return ErrConflict
	}
	b.Version++
	m.bills[s.BillID] = b
	m.shares[s.ID] = *s
	return nil
}

func (m *MemStore) SetShareStatus(_ context.Context, id, from, to string, paidAt *time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.shares[id]
	if !ok {
		return ErrNotFound
	}
	if s.Status != from {
		return ErrConflict
	}
	s.Status, s.PaidAt = to, paidAt
	m.shares[id] = s
	if b, ok := m.bills[s.BillID]; ok {
		b.Version++
		m.bills[s.BillID] = b
	}
	return nil
}

func (m *MemStore) Close(_ context.Context, billID string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.bills[billID]
	if !ok {
		return ErrNotFound
	}
	if b.Status != StatusOpen {
		return nil
	}
	b.Status, b.ClosedAt, b.UpdatedAt = StatusPaid, &at, at
	b.Version++
	m.bills[billID] = b
	return nil
}

func (m *MemStore) Tips(_ context.Context, restaurantID string, from, to time.Time) ([]WaiterTips, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	agg := map[string]*WaiterTips{}
	for _, s := range m.shares {
		b := m.bills[s.BillID]
		if b.RestaurantID != restaurantID || s.Status != SharePaid || s.TipTiyin <= 0 || s.PaidAt == nil ||
			s.PaidAt.Before(from) || !s.PaidAt.Before(to) {
			continue
		}
		w := agg[s.WaiterName]
		if w == nil {
			w = &WaiterTips{WaiterName: s.WaiterName}
			agg[s.WaiterName] = w
		}
		w.TipsTiyin += s.TipTiyin
		w.Count++
	}
	out := make([]WaiterTips, 0, len(agg))
	for _, w := range agg {
		out = append(out, *w)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TipsTiyin > out[j].TipsTiyin })
	return out, nil
}