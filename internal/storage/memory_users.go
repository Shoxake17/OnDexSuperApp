package storage

import (
	"context"
	"sync"

	"chustapp/internal/users"
)

type MemoryUserRepo struct {
	mu   sync.RWMutex
	data map[string]users.User // id -> user
}

func NewMemoryUserRepo(seed ...users.User) *MemoryUserRepo {
	r := &MemoryUserRepo{data: make(map[string]users.User)}
	for _, u := range seed {
		r.data[u.ID] = u
	}
	return r
}

func (r *MemoryUserRepo) GetByPhone(_ context.Context, phone string) (*users.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, u := range r.data {
		if u.Phone == phone {
			cp := u
			return &cp, nil
		}
	}
	return nil, users.ErrUserNotFound
}

func (r *MemoryUserRepo) GetByID(_ context.Context, id string) (*users.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.data[id]
	if !ok {
		return nil, users.ErrUserNotFound
	}
	cp := u
	return &cp, nil
}

func (r *MemoryUserRepo) Create(_ context.Context, u *users.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[u.ID] = *u
	return nil
}

func (r *MemoryUserRepo) UpdateRole(_ context.Context, id string, role users.Role, entityID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.data[id]
	if !ok {
		return users.ErrUserNotFound
	}
	u.Role = role
	u.EntityID = entityID
	r.data[id] = u
	return nil
}

func (r *MemoryUserRepo) DeleteByRoleEntity(_ context.Context, role users.Role, entityID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, u := range r.data {
		if u.Role == role && u.EntityID == entityID {
			delete(r.data, id)
		}
	}
	return nil
}

func (r *MemoryUserRepo) ListByRole(_ context.Context, role users.Role) ([]*users.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []*users.User
	for _, u := range r.data {
		if u.Role == role {
			cp := u
			list = append(list, &cp)
		}
	}
	return list, nil
}

type MemoryCodeStore struct {
	mu   sync.Mutex
	data map[string]users.Code // phone -> code
}

func NewMemoryCodeStore() *MemoryCodeStore {
	return &MemoryCodeStore{data: make(map[string]users.Code)}
}

func (s *MemoryCodeStore) Save(_ context.Context, c *users.Code) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[c.Phone] = *c
	return nil
}

func (s *MemoryCodeStore) Get(_ context.Context, phone string) (*users.Code, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.data[phone]
	if !ok {
		return nil, users.ErrInvalidCode
	}
	cp := c
	return &cp, nil
}

func (s *MemoryCodeStore) IncrementAttempts(_ context.Context, phone string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.data[phone]; ok {
		c.Attempts++
		s.data[phone] = c
	}
	return nil
}

func (s *MemoryCodeStore) Delete(_ context.Context, phone string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, phone)
	return nil
}
