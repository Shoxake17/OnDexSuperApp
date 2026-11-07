package mfa

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("mfa: topilmadi")
	// ErrAlreadyEnrolled — akkauntda tasdiqlangan TOTP allaqachon bor.
	ErrAlreadyEnrolled = errors.New("ikki bosqichli tasdiq allaqachon yoqilgan")
)

// Factor — `mfa_factors` qatori (faqat TOTP).
type Factor struct {
	ID           string
	UserID       string
	SecretEnc    string // secretstore.Seal natijasi — ochiq kalit EMAS
	ConfirmedAt  *time.Time
	LastUsedStep int64
	CreatedAt    time.Time
}

// Store — 0067 jadvallari. Postgres va xotira ombori bir xil shartnomada
// (`mfatest`).
type Store interface {
	// Confirmed — tasdiqlangan TOTP; yo'q bo'lsa ErrNotFound.
	Confirmed(ctx context.Context, userID string) (*Factor, error)
	// SavePending — foydalanuvchining eski TASDIQLANMAGAN faktorlarini
	// o'chirib, yangisini yozadi. Tasdiqlangan bor bo'lsa ErrAlreadyEnrolled.
	SavePending(ctx context.Context, f *Factor) error
	// Pending — eng so'nggi tasdiqlanmagan faktor.
	Pending(ctx context.Context, userID string) (*Factor, error)
	// Confirm — ATOMIK: faktor tasdiqlanadi (`last_used_step = step`),
	// tiklash kodlari almashtiriladi. Faktor yo'q/allaqachon tasdiqlangan —
	// ErrNotFound; boshqa tasdiqlangan bor — ErrAlreadyEnrolled.
	Confirm(ctx context.Context, factorID string, step int64, now time.Time, recovery [][]byte) error
	// UseStep — `step > last_used_step` bo'lsa yangilaydi va true; aks holda
	// false (kod allaqachon ishlatilgan yoki eskiroq).
	UseStep(ctx context.Context, factorID string, step int64) (bool, error)
	// UseRecovery — ishlatilmagan kod bo'lsa belgilaydi va true.
	UseRecovery(ctx context.Context, userID string, hash []byte, now time.Time) (bool, error)
	// RecoveryLeft — ishlatilmagan tiklash kodlari soni.
	RecoveryLeft(ctx context.Context, userID string) (int, error)
}

// MemStore — xotira ombori (dev, testlar).
type MemStore struct {
	mu       sync.Mutex
	factors  map[string]*Factor
	recovery map[string][]*memCode
}

type memCode struct {
	hash []byte
	used bool
}

func NewMemStore() *MemStore {
	return &MemStore{factors: map[string]*Factor{}, recovery: map[string][]*memCode{}}
}

func (s *MemStore) Confirmed(_ context.Context, userID string) (*Factor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.factors {
		if f.UserID == userID && f.ConfirmedAt != nil {
			c := *f
			return &c, nil
		}
	}
	return nil, ErrNotFound
}

func (s *MemStore) SavePending(_ context.Context, f *Factor) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f.ID == "" || f.UserID == "" || f.SecretEnc == "" {
		return errors.New("mfa: faktor maydonlari bo'sh")
	}
	for id, x := range s.factors {
		if x.UserID != f.UserID {
			continue
		}
		if x.ConfirmedAt != nil {
			return ErrAlreadyEnrolled
		}
		delete(s.factors, id)
	}
	c := *f
	c.ConfirmedAt, c.LastUsedStep = nil, 0
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	s.factors[f.ID] = &c
	return nil
}

func (s *MemStore) Pending(_ context.Context, userID string) (*Factor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var best *Factor
	for _, f := range s.factors {
		if f.UserID == userID && f.ConfirmedAt == nil && (best == nil || f.CreatedAt.After(best.CreatedAt)) {
			best = f
		}
	}
	if best == nil {
		return nil, ErrNotFound
	}
	c := *best
	return &c, nil
}

func (s *MemStore) Confirm(_ context.Context, factorID string, step int64, now time.Time, recovery [][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.factors[factorID]
	if !ok || f.ConfirmedAt != nil {
		return ErrNotFound
	}
	for _, x := range s.factors {
		if x.UserID == f.UserID && x.ConfirmedAt != nil {
			return ErrAlreadyEnrolled
		}
	}
	t := now
	f.ConfirmedAt, f.LastUsedStep = &t, step
	codes := make([]*memCode, 0, len(recovery))
	for _, h := range recovery {
		codes = append(codes, &memCode{hash: bytes.Clone(h)})
	}
	s.recovery[f.UserID] = codes
	return nil
}

func (s *MemStore) UseStep(_ context.Context, factorID string, step int64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.factors[factorID]
	if !ok || f.ConfirmedAt == nil {
		return false, ErrNotFound
	}
	if step <= f.LastUsedStep {
		return false, nil
	}
	f.LastUsedStep = step
	return true, nil
}

func (s *MemStore) UseRecovery(_ context.Context, userID string, hash []byte, _ time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.recovery[userID] {
		if !c.used && bytes.Equal(c.hash, hash) {
			c.used = true
			return true, nil
		}
	}
	return false, nil
}

func (s *MemStore) RecoveryLeft(_ context.Context, userID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, c := range s.recovery[userID] {
		if !c.used {
			n++
		}
	}
	return n, nil
}
