package identity

import (
	"context"
	"sync"
	"time"
)

// MemoryCodeStore — xotiradagi `CodeStore` (testlar, Postgres'siz dev).
type MemoryCodeStore struct {
	mu   sync.Mutex
	rows map[string]*memCode
}

type memCode struct {
	c    AuthCode
	used bool
}

func NewMemoryCodeStore() *MemoryCodeStore { return &MemoryCodeStore{rows: map[string]*memCode{}} }

func (m *MemoryCodeStore) Put(_ context.Context, c *AuthCode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *c
	cp.AMR = append([]string(nil), c.AMR...)
	m.rows[string(c.Hash)] = &memCode{c: cp}
	return nil
}

func (m *MemoryCodeStore) Consume(_ context.Context, hash []byte, now time.Time) (*AuthCode, CodeResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rows[string(hash)]
	if !ok {
		return nil, CodeInvalid, nil
	}
	cp := r.c
	if r.used {
		return &cp, CodeReused, nil
	}
	if !now.Before(r.c.ExpiresAt) {
		return nil, CodeInvalid, nil
	}
	r.used = true
	return &cp, CodeOK, nil
}

func (m *MemoryCodeStore) SetIssued(_ context.Context, hash []byte, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.rows[string(hash)]; ok {
		r.c.IssuedSessionID = sessionID
	}
	return nil
}
