package storage

import (
	"context"
	"sort"
	"sync"
	"time"

	"chustapp/internal/staff"
)

// MemoryStaffRepo — DATABASE_URL berilmaganda (dev va testlar). Postgres
// bilan BIR XIL qoidalar: tartib raqami, ishlayotganlar orasida unikal
// telefon, bitta akkaunt — bitta xodim; qiymatlar chuqur nusxa.
type MemoryStaffRepo struct {
	mu      sync.RWMutex
	members map[string]staff.Member
	events  []staff.Event
}

func NewMemoryStaffRepo() *MemoryStaffRepo {
	return &MemoryStaffRepo{members: map[string]staff.Member{}}
}

func (r *MemoryStaffRepo) conflictLocked(m *staff.Member) error {
	for id, x := range r.members {
		if id == m.ID {
			continue
		}
		if m.Status != staff.StatusDismissed && x.Status != staff.StatusDismissed &&
			x.RestaurantID == m.RestaurantID && x.Phone == m.Phone {
			return staff.ErrPhoneTaken
		}
		if m.UserID != "" && x.UserID == m.UserID {
			return staff.ErrAccountConflict
		}
	}
	return nil
}

func (r *MemoryStaffRepo) Create(_ context.Context, m *staff.Member, events []staff.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.conflictLocked(m); err != nil {
		return err
	}
	number := 0
	for _, x := range r.members {
		if x.RestaurantID == m.RestaurantID && x.Number > number {
			number = x.Number
		}
	}
	m.Number = number + 1
	r.members[m.ID] = staff.Clone(*m)
	r.events = append(r.events, events...)
	return nil
}

func (r *MemoryStaffRepo) Update(_ context.Context, m *staff.Member, events []staff.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	old, ok := r.members[m.ID]
	if !ok || old.RestaurantID != m.RestaurantID {
		return staff.ErrNotFound
	}
	if err := r.conflictLocked(m); err != nil {
		return err
	}
	next := staff.Clone(*m)
	next.Number = old.Number
	next.CreatedAt = old.CreatedAt
	r.members[m.ID] = next
	r.events = append(r.events, events...)
	return nil
}

func (r *MemoryStaffRepo) Get(_ context.Context, restaurantID, id string) (*staff.Member, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	x, ok := r.members[id]
	if !ok || x.RestaurantID != restaurantID {
		return nil, staff.ErrNotFound
	}
	cp := staff.Clone(x)
	return &cp, nil
}

func (r *MemoryStaffRepo) List(_ context.Context, restaurantID string) ([]*staff.Member, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []*staff.Member
	for _, x := range r.members {
		if x.RestaurantID == restaurantID {
			cp := staff.Clone(x)
			list = append(list, &cp)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Number < list[j].Number })
	return list, nil
}

func (r *MemoryStaffRepo) Events(_ context.Context, restaurantID, memberID string, since time.Time, limit int) ([]staff.Event, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 {
		limit = 50
	}
	var list []staff.Event
	for i := len(r.events) - 1; i >= 0; i-- {
		ev := r.events[i]
		if ev.RestaurantID != restaurantID || (memberID != "" && ev.MemberID != memberID) || ev.At.Before(since) {
			continue
		}
		list = append(list, ev)
	}
	// Postgres bilan bir xil: vaqt bo'yicha kamayish, bir xil vaqtda —
	// keyin yozilgani birinchi (`seq DESC`). Ro'yxat allaqachon teskari
	// yozilish tartibida, barqaror saralash uni saqlaydi.
	sort.SliceStable(list, func(i, j int) bool { return list[i].At.After(list[j].At) })
	if len(list) > limit {
		list = list[:limit]
	}
	return list, nil
}
