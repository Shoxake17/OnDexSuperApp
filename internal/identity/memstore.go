package identity

import (
	"bytes"
	"context"
	"sort"
	"sync"
	"time"
)

// MemoryStore — xotiradagi sessiya ombori (testlar, e2e, Postgres'siz dev).
// Postgres implementatsiyasi bilan BIR XIL qoidalar (`store_contract_test.go`
// ikkalasini bitta test to'plami bilan sinaydi).
type MemoryStore struct {
	mu   sync.Mutex
	rows map[string]*memRow
}

type memRow struct {
	s    Session
	hash []byte
	prev []byte
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{rows: map[string]*memRow{}} }

func cloneSession(s *Session) *Session {
	c := *s
	c.AMR = append([]string(nil), s.AMR...)
	c.IPHash = append([]byte(nil), s.IPHash...)
	return &c
}

func (m *MemoryStore) Create(_ context.Context, s *Session, refreshHash []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.rows {
		if bytes.Equal(r.hash, refreshHash) {
			return ErrRefreshInvalid
		}
	}
	m.rows[s.ID] = &memRow{s: *cloneSession(s), hash: append([]byte(nil), refreshHash...)}
	return nil
}

func (m *MemoryStore) Rotate(_ context.Context, oldHash, newHash []byte, now time.Time,
	idleFor func(string) time.Duration, grace time.Duration) (*Session, RotateResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.rows {
		if bytes.Equal(r.hash, oldHash) {
			idle := idleFor(r.s.ClientID)
			if !r.s.Active(now) || idle <= 0 {
				if r.s.RevokedAt == nil && idle <= 0 {
					t := now
					r.s.RevokedAt, r.s.RevokeReason = &t, "client_removed"
				}
				return cloneSession(&r.s), RotateExpired, nil
			}
			r.prev, r.hash = r.hash, append([]byte(nil), newHash...)
			r.s.LastUsedAt = now
			r.s.IdleExpiresAt = minTime(now.Add(idle), r.s.AbsExpiresAt)
			return cloneSession(&r.s), RotateOK, nil
		}
	}
	for _, r := range m.rows {
		if r.prev != nil && bytes.Equal(r.prev, oldHash) {
			if r.s.RevokedAt == nil && now.Sub(r.s.LastUsedAt) <= grace {
				return cloneSession(&r.s), RotateRaced, nil
			}
			if r.s.RevokedAt == nil {
				t := now
				r.s.RevokedAt, r.s.RevokeReason = &t, "refresh_reuse"
			}
			return cloneSession(&r.s), RotateReused, nil
		}
	}
	return nil, RotateNotFound, nil
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func (m *MemoryStore) Get(_ context.Context, id string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rows[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneSession(&r.s), nil
}

func (m *MemoryStore) ListActive(_ context.Context, userID string, now time.Time) ([]*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*Session
	for _, r := range m.rows {
		if r.s.UserID == userID && r.s.Active(now) {
			out = append(out, cloneSession(&r.s))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastUsedAt.After(out[j].LastUsedAt) })
	return out, nil
}

func (m *MemoryStore) Revoke(_ context.Context, id, reason string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rows[id]
	if !ok {
		return ErrNotFound
	}
	if r.s.RevokedAt == nil {
		t := now
		r.s.RevokedAt, r.s.RevokeReason = &t, reason
	}
	return nil
}

func (m *MemoryStore) RevokeChildren(_ context.Context, parentID, reason string, now time.Time) ([]*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*Session
	for _, r := range m.rows {
		if parentID != "" && r.s.ParentID == parentID && r.s.RevokedAt == nil {
			t := now
			r.s.RevokedAt, r.s.RevokeReason = &t, reason
			out = append(out, cloneSession(&r.s))
		}
	}
	return out, nil
}

func (m *MemoryStore) RevokeAllForUser(_ context.Context, userID, reason string, now time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, r := range m.rows {
		if r.s.UserID == userID && r.s.RevokedAt == nil {
			t := now
			r.s.RevokedAt, r.s.RevokeReason = &t, reason
			n++
		}
	}
	return n, nil
}
