package storage

import (
	"context"
	"sync"
	"time"

	"chustapp/internal/users"
)

// MemoryDeviceStore — `users.DeviceStore` ning bazasiz varianti
// (testlar va `DB_URL` siz dev rejimi uchun).
type MemoryDeviceStore struct {
	mu   sync.RWMutex
	data map[string]map[string]users.Device // userID -> platform -> device
}

func NewMemoryDeviceStore() *MemoryDeviceStore {
	return &MemoryDeviceStore{data: make(map[string]map[string]users.Device)}
}

func (s *MemoryDeviceStore) Touch(_ context.Context, userID, platform, appVersion string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	byPlatform := s.data[userID]
	if byPlatform == nil {
		byPlatform = make(map[string]users.Device)
		s.data[userID] = byPlatform
	}
	now := time.Now()
	d, ok := byPlatform[platform]
	if !ok {
		d = users.Device{Platform: platform, FirstSeen: now}
	}
	d.LastSeen = now
	// Bo'sh versiya mavjud qiymatni o'chirmaydi (Postgres varianti
	// bilan bir xil xatti-harakat).
	if appVersion != "" {
		d.AppVersion = appVersion
	}
	byPlatform[platform] = d
	return nil
}

func (s *MemoryDeviceStore) ListByUsers(_ context.Context, userIDs []string) (map[string][]users.Device, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string][]users.Device, len(userIDs))
	for _, id := range userIDs {
		for _, d := range s.data[id] {
			out[id] = append(out[id], d)
		}
	}
	return out, nil
}
