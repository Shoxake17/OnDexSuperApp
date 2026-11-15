package reservation

import (
	"context"
	"sort"
	"sync"
	"time"
)

// MemStore — xotiradagi ombor (testlar va DATABASE_URL'siz dev). Bazadagi
// EXCLUDE cheklovini aynan takrorlaydi: bir stolning faol bronlari
// kesishmaydi.
type MemStore struct {
	mu       sync.Mutex
	settings map[string]Settings
	items    map[string]Reservation
}

func NewMemStore() *MemStore {
	return &MemStore{settings: map[string]Settings{}, items: map[string]Reservation{}}
}

func cloneSettings(s Settings) Settings {
	z := make(map[string]int64, len(s.ZoneDeposits))
	for k, v := range s.ZoneDeposits {
		z[k] = v
	}
	s.ZoneDeposits = z
	return s
}

func (m *MemStore) GetSettings(_ context.Context, restaurantID string) (*Settings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.settings[restaurantID]
	if !ok {
		return nil, ErrNotFound
	}
	c := cloneSettings(s)
	return &c, nil
}

func (m *MemStore) PutSettings(_ context.Context, s *Settings) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.settings[s.RestaurantID] = cloneSettings(*s)
	return nil
}

// overlapLocked — r (yangi holatida) boshqa faol bron bilan kesishadimi.
func (m *MemStore) overlapLocked(r *Reservation) bool {
	if !r.Status.Active() {
		return false
	}
	for id, x := range m.items {
		if id == r.ID || x.TableID != r.TableID || !x.Status.Active() {
			continue
		}
		if x.StartsAt.Before(r.EndsAt) && r.StartsAt.Before(x.EndsAt) {
			return true
		}
	}
	return false
}

func (m *MemStore) Insert(_ context.Context, r *Reservation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.items[r.ID]; ok {
		return ErrConflict
	}
	if m.overlapLocked(r) {
		return ErrSlotTaken
	}
	m.items[r.ID] = *r
	return nil
}

func (m *MemStore) Get(_ context.Context, id string) (*Reservation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.items[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &r, nil
}

func (m *MemStore) Update(_ context.Context, r *Reservation, from Status) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.items[r.ID]
	if !ok {
		return ErrNotFound
	}
	if cur.Status != from {
		return ErrConflict
	}
	if m.overlapLocked(r) {
		return ErrSlotTaken
	}
	m.items[r.ID] = *r
	return nil
}

func (m *MemStore) ListByRestaurant(_ context.Context, restaurantID string, from, to time.Time) ([]*Reservation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*Reservation
	for _, r := range m.items {
		if r.RestaurantID == restaurantID && r.StartsAt.Before(to) && from.Before(r.EndsAt) {
			c := r
			out = append(out, &c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartsAt.Before(out[j].StartsAt) })
	return out, nil
}

func (m *MemStore) ListByCustomer(_ context.Context, customerID string, limit int) ([]*Reservation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*Reservation
	for _, r := range m.items {
		if r.CustomerID == customerID {
			c := r
			out = append(out, &c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartsAt.After(out[j].StartsAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *MemStore) ListDue(_ context.Context, now time.Time, limit int) ([]*Reservation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*Reservation
	for _, r := range m.items {
		if isDue(&r, now) {
			c := r
			out = append(out, &c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartsAt.Before(out[j].StartsAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// isDue — `ListDue` sharti (Postgres so'rovi bilan bir xil).
func isDue(r *Reservation, now time.Time) bool {
	switch r.Status {
	case StatusHeld:
		return r.HoldExpiresAt != nil && now.After(*r.HoldExpiresAt)
	case StatusConfirmed:
		return now.After(r.StartsAt.Add(NoShowAfter)) ||
			(r.RemindedAt == nil && !now.Before(r.StartsAt.Add(-ReminderBefore)))
	case StatusSeated:
		return now.After(r.EndsAt.Add(2 * time.Hour))
	}
	return false
}
