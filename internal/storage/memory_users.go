package storage

import (
	"context"
	"strings"
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

// GetByTelegramID — Telegram Mini App kirishi (migration 0031).
func (r *MemoryUserRepo) GetByTelegramID(_ context.Context, telegramID int64) (*users.User, error) {
	if telegramID == 0 {
		return nil, users.ErrUserNotFound
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, u := range r.data {
		if u.TelegramID == telegramID {
			cp := u
			return &cp, nil
		}
	}
	return nil, users.ErrUserNotFound
}

// LinkTelegram — Postgres versiyasi bilan BIR XIL semantika: eski
// bog'lanish avval uziladi (izoh `postgres_users.go` da).
func (r *MemoryUserRepo) LinkTelegram(_ context.Context, userID string, telegramID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.data[userID]; !ok {
		return users.ErrUserNotFound
	}
	for id, u := range r.data {
		if u.TelegramID == telegramID && id != userID {
			u.TelegramID = 0
			r.data[id] = u
		}
	}
	u := r.data[userID]
	u.TelegramID = telegramID
	r.data[userID] = u
	return nil
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

// Create — telefon va email UNIKALLIGI shu yerda ham tekshiriladi.
//
// Avval faqat ID bo'yicha yozilardi, ya'ni memory rejimida bitta
// telefon bilan bir necha akkaunt paydo bo'lardi va `GetByPhone`
// map'ni aylanib BEQAROR natija qaytarardi (har login boshqa
// identifikatorga tushishi mumkin edi). Postgres'da esa unikal
// indeks bor — ikki backend bir xil ishlashi SHART.
func (r *MemoryUserRepo) Create(_ context.Context, u *users.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.data {
		if x.ID == u.ID {
			continue
		}
		if x.Phone == u.Phone {
			return users.ErrPhoneTaken
		}
		if u.Email != "" && strings.EqualFold(x.Email, u.Email) {
			return users.ErrEmailTaken
		}
	}
	r.data[u.ID] = *u
	return nil
}

func (r *MemoryUserRepo) GetByEmail(_ context.Context, email string) (*users.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, u := range r.data {
		if u.Email != "" && strings.EqualFold(u.Email, email) {
			cp := u
			return &cp, nil
		}
	}
	return nil, users.ErrUserNotFound
}

func (r *MemoryUserRepo) UpdateProfile(_ context.Context, id string, p users.ProfileUpdate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.data[id]
	if !ok {
		return users.ErrUserNotFound
	}
	if p.Email != nil && *p.Email != "" {
		for _, x := range r.data {
			if x.ID != id && strings.EqualFold(x.Email, *p.Email) {
				return users.ErrEmailTaken
			}
		}
	}
	if p.FirstName != nil {
		u.FirstName = *p.FirstName
	}
	if p.LastName != nil {
		u.LastName = *p.LastName
	}
	if p.Email != nil {
		u.Email = *p.Email
	}
	if p.PasswordHash != nil {
		u.PasswordHash = *p.PasswordHash
	}
	// `Name` FAQAT ism/familiya berilganda qayta hisoblanadi — Postgres
	// implementatsiyasi bilan bir xil (u yerdagi izohga qarang).
	if p.FirstName != nil || p.LastName != nil {
		u.Name = strings.TrimSpace(u.FirstName + " " + u.LastName)
	}
	r.data[id] = u
	return nil
}

// SetPasswordHash — FAQAT parol hash'i (`UpdateProfile` dan farqli
// o'laroq `Name` ni qayta hisoblamaydi).
func (r *MemoryUserRepo) SetPasswordHash(_ context.Context, id, hash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.data[id]
	if !ok {
		return users.ErrUserNotFound
	}
	u.PasswordHash = hash
	r.data[id] = u
	return nil
}

func (r *MemoryUserRepo) MarkEmailVerified(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.data[id]
	if !ok {
		return users.ErrUserNotFound
	}
	u.EmailVerified = true
	r.data[id] = u
	return nil
}

func (r *MemoryUserRepo) MarkPhoneVerified(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.data[id]
	if !ok {
		return users.ErrUserNotFound
	}
	u.PhoneVerified = true
	r.data[id] = u
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

func (r *MemoryUserRepo) UpdateAddress(_ context.Context, id string, addr users.AddressDetails) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.data[id]
	if !ok {
		return users.ErrUserNotFound
	}
	u.Address = addr
	r.data[id] = u
	return nil
}

func (r *MemoryUserRepo) DeleteByRoleEntity(_ context.Context, role users.Role, entityID string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var ids []string
	for id, u := range r.data {
		if u.Role == role && u.EntityID == entityID {
			ids = append(ids, u.ID)
			delete(r.data, id)
		}
	}
	return ids, nil
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
	s.data[c.Target] = *c
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

// IncrementAttempts — mutex ostida oshiradi va yangi qiymatni qaytaradi
// (oshirish va o'qish bo'linmasligi shart — service.go izohiga qarang).
func (s *MemoryCodeStore) IncrementAttempts(_ context.Context, phone string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.data[phone]
	if !ok {
		return 0, nil
	}
	c.Attempts++
	s.data[phone] = c
	return c.Attempts, nil
}

func (s *MemoryCodeStore) Delete(_ context.Context, phone string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, phone)
	return nil
}
