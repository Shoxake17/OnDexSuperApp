package storage

import (
	"context"
	"sort"
	"sync"
	"time"

	"chustapp/internal/notify"
)

// Bildirishnomalarning xotiradagi varianti — Postgres bilan BIR XIL
// xatti-harakat (testlar production'ni ifodalashi uchun).

type MemoryNotificationStore struct {
	mu   sync.RWMutex
	data map[string]*notify.Notification // id -> yozuv
}

func NewMemoryNotificationStore() *MemoryNotificationStore {
	return &MemoryNotificationStore{data: make(map[string]*notify.Notification)}
}

func (r *MemoryNotificationStore) Save(_ context.Context, n *notify.Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *n
	r.data[n.ID] = &cp
	return nil
}

func (r *MemoryNotificationStore) List(_ context.Context, userID string, limit int) ([]*notify.Notification, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	r.mu.RLock()
	var list []*notify.Notification
	for _, n := range r.data {
		if n.UserID == userID {
			cp := *n
			list = append(list, &cp)
		}
	}
	r.mu.RUnlock()

	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	if len(list) > limit {
		list = list[:limit]
	}
	return list, nil
}

func (r *MemoryNotificationStore) UnreadCount(_ context.Context, userID string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	k := 0
	for _, n := range r.data {
		if n.UserID == userID && n.ReadAt == nil {
			k++
		}
	}
	return k, nil
}

// MarkRead — `userID` tekshiruvi Postgres'dagi kabi MAJBURIY: begona
// yozuvni o'qilgan qilib bo'lmaydi.
func (r *MemoryNotificationStore) MarkRead(_ context.Context, userID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	n, ok := r.data[id]
	if !ok || n.UserID != userID || n.ReadAt != nil {
		return nil
	}
	now := time.Now()
	n.ReadAt = &now
	return nil
}

func (r *MemoryNotificationStore) MarkAllRead(_ context.Context, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for _, n := range r.data {
		if n.UserID == userID && n.ReadAt == nil {
			t := now
			n.ReadAt = &t
		}
	}
	return nil
}

// ---------- Push tokenlari ----------

type MemoryTokenStore struct {
	mu   sync.RWMutex
	data map[string]string // token -> userID
}

func NewMemoryTokenStore() *MemoryTokenStore {
	return &MemoryTokenStore{data: make(map[string]string)}
}

func (r *MemoryTokenStore) SaveToken(_ context.Context, userID, token, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Token YANGI egasiga o'tadi (Postgres'dagi ON CONFLICT bilan bir xil).
	r.data[token] = userID
	return nil
}

func (r *MemoryTokenStore) DeleteToken(_ context.Context, token string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.data, token)
	return nil
}

func (r *MemoryTokenStore) TokensFor(_ context.Context, userID string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []string
	for t, u := range r.data {
		if u == userID {
			out = append(out, t)
		}
	}
	sort.Strings(out) // barqaror tartib — testlar uchun
	return out, nil
}
