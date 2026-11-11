// Package passkey — fishingga chidamli ikkinchi bosqich (WebAuthn / FIDO2
// passkey: Windows Hello, Touch ID, Android/iPhone ekran qulfi, xavfsizlik
// kaliti). AUD-4.
//
// ┌─ NEGA ────────────────────────────────────────────────────────────────┐
// TOTP kodi real vaqt fishing proksisi (evilginx kabi) bilan o'g'irlanadi:
// foydalanuvchi soxta sahifaga kodni yozadi, proksi uni darhol haqiqiy
// saytga uzatadi. Passkey imzosi brauzer tomonidan SAYT MANZILIGA (origin,
// rpId) bog'lanadi — soxta domenda yaratilgan imzo serverda rad etiladi,
// foydalanuvchi aldangan bo'lsa ham.
// └───────────────────────────────────────────────────────────────────────┘
//
// Protokol tekshiruvi (imzo, rpIdHash, origin, challenge, flaglar) —
// github.com/go-webauthn/webauthn. Bu paket faqat saqlash: kalit (ochiq
// kalit — sir emas) va bir martalik challenge.
package passkey

import (
	"context"
	"errors"
	"sync"
	"time"
)

// MaxPerUser — bitta akkauntdagi passkey'lar chegarasi (bazani to'ldirish va
// allowCredentials ro'yxatini cheksiz o'stirishga qarshi).
const MaxPerUser = 10

var (
	ErrNotFound = errors.New("passkey topilmadi")
	ErrExists   = errors.New("bu passkey allaqachon ulangan")
	ErrTooMany  = errors.New("passkey'lar soni chegarasiga yetdi — avval keraksizini o'chiring")
)

// Credential — saqlangan passkey. Data — `webauthn.Credential` JSON'i
// (ochiq kalit, imzo hisoblagichi, flaglar); sir emas.
type Credential struct {
	ID         string // credential ID, base64url (global noyob)
	UserID     string
	Name       string
	Data       []byte
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

// Store — passkey ombori.
type Store interface {
	List(ctx context.Context, userID string) ([]Credential, error)
	// Add — ErrExists (ID band), ErrTooMany (MaxPerUser).
	Add(ctx context.Context, c *Credential) error
	// Touch — muvaffaqiyatli kirishdan keyin: yangilangan Data (hisoblagich) va vaqt.
	Touch(ctx context.Context, userID, id string, data []byte, at time.Time) error
	Delete(ctx context.Context, userID, id string) error
	// UseChallenge — challenge'ni ATOMIK "ishlatildi" deb belgilaydi; ikkinchi
	// marta — false (javobni qayta yuborish / replay).
	UseChallenge(ctx context.Context, hash []byte, expires time.Time) (bool, error)
}

// MemStore — test va DB'siz dev uchun.
type MemStore struct {
	mu    sync.Mutex
	creds map[string]Credential // ID ->
	used  map[string]time.Time
	// lastSweep — eskirganlar ko'pi bilan 30 soniyada bir tozalanadi (har
	// chaqiruvda butun xaritani aylanish yuklama ostida kvadratik edi).
	lastSweep time.Time
}

func NewMemStore() *MemStore {
	return &MemStore{creds: map[string]Credential{}, used: map[string]time.Time{}}
}

func (s *MemStore) List(_ context.Context, userID string) ([]Credential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Credential
	for _, c := range s.creds {
		if c.UserID == userID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (s *MemStore) Add(_ context.Context, c *Credential) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.creds[c.ID]; ok {
		return ErrExists
	}
	n := 0
	for _, x := range s.creds {
		if x.UserID == c.UserID {
			n++
		}
	}
	if n >= MaxPerUser {
		return ErrTooMany
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	s.creds[c.ID] = *c
	return nil
}

func (s *MemStore) Touch(_ context.Context, userID, id string, data []byte, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.creds[id]
	if !ok || c.UserID != userID {
		return ErrNotFound
	}
	c.Data, c.LastUsedAt = data, &at
	s.creds[id] = c
	return nil
}

func (s *MemStore) Delete(_ context.Context, userID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.creds[id]
	if !ok || c.UserID != userID {
		return ErrNotFound
	}
	delete(s.creds, id)
	return nil
}

func (s *MemStore) UseChallenge(_ context.Context, hash []byte, expires time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if now.Sub(s.lastSweep) >= 30*time.Second {
		s.lastSweep = now
		for k, exp := range s.used {
			if exp.Before(now) {
				delete(s.used, k)
			}
		}
	}
	k := string(hash)
	if _, ok := s.used[k]; ok {
		return false, nil
	}
	s.used[k] = expires
	return true, nil
}
