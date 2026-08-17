package users

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeUserRepo struct {
	mu   sync.Mutex
	data map[string]*User // phone -> user
}

func (r *fakeUserRepo) GetByPhone(_ context.Context, phone string) (*User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if u, ok := r.data[phone]; ok {
		return u, nil
	}
	return nil, ErrUserNotFound
}
// Telegram Mini App bog'lanishi (migration 0031). Xotira va Postgres
// implementatsiyalari bilan BIR XIL semantika: eski bog'lanish avval
// uziladi, aks holda odam Mini App'da begona hisobga tushardi.
func (r *fakeUserRepo) GetByTelegramID(_ context.Context, tgID int64) (*User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if tgID == 0 {
		return nil, ErrUserNotFound
	}
	for _, u := range r.data {
		if u.TelegramID == tgID {
			return u, nil
		}
	}
	return nil, ErrUserNotFound
}

func (r *fakeUserRepo) LinkTelegram(_ context.Context, userID string, tgID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var target *User
	for _, u := range r.data {
		if u.ID == userID {
			target = u
		}
	}
	if target == nil {
		return ErrUserNotFound
	}
	for _, u := range r.data {
		if u.TelegramID == tgID && u.ID != userID {
			u.TelegramID = 0
		}
	}
	target.TelegramID = tgID
	return nil
}

func (r *fakeUserRepo) GetByID(_ context.Context, id string) (*User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.data {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, ErrUserNotFound
}
func (r *fakeUserRepo) Create(_ context.Context, u *User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[u.Phone] = u
	return nil
}
func (r *fakeUserRepo) GetByEmail(_ context.Context, email string) (*User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.data {
		if u.Email != "" && strings.EqualFold(u.Email, email) {
			return u, nil
		}
	}
	return nil, ErrUserNotFound
}
func (r *fakeUserRepo) UpdateProfile(_ context.Context, id string, p ProfileUpdate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.data {
		if u.ID != id {
			continue
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
		// Haqiqiy repolar bilan bir xil: `Name` faqat ism/familiya
		// berilganda qayta hisoblanadi.
		if p.FirstName != nil || p.LastName != nil {
			u.Name = strings.TrimSpace(u.FirstName + " " + u.LastName)
		}
		return nil
	}
	return ErrUserNotFound
}
func (r *fakeUserRepo) SetPasswordHash(_ context.Context, id, hash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.data {
		if u.ID == id {
			u.PasswordHash = hash
			return nil
		}
	}
	return ErrUserNotFound
}
func (r *fakeUserRepo) MarkEmailVerified(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.data {
		if u.ID == id {
			u.EmailVerified = true
			return nil
		}
	}
	return ErrUserNotFound
}
func (r *fakeUserRepo) MarkPhoneVerified(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.data {
		if u.ID == id {
			u.PhoneVerified = true
			return nil
		}
	}
	return ErrUserNotFound
}
func (r *fakeUserRepo) UpdateRole(_ context.Context, id string, role Role, entityID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.data {
		if u.ID == id {
			u.Role = role
			u.EntityID = entityID
			return nil
		}
	}
	return ErrUserNotFound
}
func (r *fakeUserRepo) UpdateAddress(_ context.Context, id string, addr AddressDetails) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.data {
		if u.ID == id {
			u.Address = addr
			return nil
		}
	}
	return ErrUserNotFound
}
func (r *fakeUserRepo) DeleteByRoleEntity(_ context.Context, role Role, entityID string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var ids []string
	for phone, u := range r.data {
		if u.Role == role && u.EntityID == entityID {
			ids = append(ids, u.ID)
			delete(r.data, phone)
		}
	}
	return ids, nil
}
func (r *fakeUserRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for phone, u := range r.data {
		if u.ID == id {
			delete(r.data, phone)
			return nil
		}
	}
	return ErrUserNotFound
}
func (r *fakeUserRepo) ListByRole(_ context.Context, role Role) ([]*User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var list []*User
	for _, u := range r.data {
		if u.Role == role {
			list = append(list, u)
		}
	}
	return list, nil
}

type fakeCodeStore struct {
	mu   sync.Mutex
	data map[string]*Code
}

func (s *fakeCodeStore) Save(_ context.Context, c *Code) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *c
	s.data[c.Target] = &cp
	return nil
}
func (s *fakeCodeStore) Get(_ context.Context, phone string) (*Code, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.data[phone]; ok {
		cp := *c
		return &cp, nil
	}
	return nil, ErrInvalidCode
}
func (s *fakeCodeStore) IncrementAttempts(_ context.Context, phone string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.data[phone]; ok {
		c.Attempts++
		return c.Attempts, nil
	}
	return 0, nil
}
func (s *fakeCodeStore) Delete(_ context.Context, phone string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, phone)
	return nil
}

type noopSms struct{}

func (noopSms) Send(_, _ string) error { return nil }

// noopEmail — testlarda "SMTP ulangan" holatni ifodalaydi. Yuborilgan
// xatlarni saqlab qo'yadi, shunda testlar kod haqiqatan jo'natilganini
// tekshira oladi.
type noopEmail struct {
	mu   sync.Mutex
	sent []string // "to|subject"
}

func (e *noopEmail) Send(to, subject, _, _ string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sent = append(e.sent, to+"|"+subject)
	return nil
}

func (e *noopEmail) count() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.sent)
}

func newTestService() *Service {
	s, _ := newTestServiceWithEmail()
	return s
}

// newTestServiceWithEmail — email yuboruvchiga ham murojaat kerak
// bo'lgan testlar uchun.
func newTestServiceWithEmail() (*Service, *noopEmail) {
	// atomic — sabab `internal/orders/service_test.go` dagi izohda.
	var n atomic.Int64
	mail := &noopEmail{}
	s := NewService(
		&fakeUserRepo{data: make(map[string]*User)},
		&fakeCodeStore{data: make(map[string]*Code)},
		noopSms{},
		NewTokenIssuer("test-secret", time.Hour),
		func() string { return "id" + string(rune('0'+n.Add(1))) },
	).WithEmail(mail, true)
	return s, mail
}

func TestNormalizePhone(t *testing.T) {
	valid := map[string]string{
		"+998901234567":     "+998901234567",
		"998901234567":      "+998901234567",
		"+998 90 123-45-67": "+998901234567",
	}
	for in, want := range valid {
		got, err := NormalizePhone(in)
		if err != nil || got != want {
			t.Errorf("NormalizePhone(%q) = %q, %v; kutilgan %q", in, got, err, want)
		}
	}
	invalid := []string{"", "901234567", "+7999123456", "+99890123456", "+9989012345678", "salom"}
	for _, in := range invalid {
		if _, err := NormalizePhone(in); err == nil {
			t.Errorf("NormalizePhone(%q): xato kutilgan edi", in)
		}
	}
}

func TestRequestAndVerifyFlow(t *testing.T) {
	s := newTestService()
	ctx := context.Background()

	phone, code, err := s.RequestCode(ctx, "+998901234567")
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 6 {
		t.Fatalf("6 xonali kod kutilgan, olindi %q", code)
	}

	token, u, err := s.Verify(ctx, phone, code)
	if err != nil {
		t.Fatal(err)
	}
	if u.Role != RoleCustomer {
		t.Errorf("yangi foydalanuvchi mijoz bo'lishi kerak, olindi %s", u.Role)
	}
	if token == "" {
		t.Error("token bo'sh")
	}

	// Kod bir marta ishlaydi
	if _, _, err := s.Verify(ctx, phone, code); err == nil {
		t.Error("ishlatilgan kod qayta qabul qilindi")
	}
}

func TestVerifyWrongCode(t *testing.T) {
	s := newTestService()
	ctx := context.Background()
	phone, code, _ := s.RequestCode(ctx, "+998901234567")

	wrong := "000000"
	if wrong == code {
		wrong = "000001"
	}
	if _, _, err := s.Verify(ctx, phone, wrong); !errors.Is(err, ErrInvalidCode) {
		t.Errorf("noto'g'ri kod ErrInvalidCode berishi kerak, olindi %v", err)
	}
	// To'g'ri kod hali ham ishlaydi (urinishlar chegarada)
	if _, _, err := s.Verify(ctx, phone, code); err != nil {
		t.Errorf("to'g'ri kod ishlashi kerak edi: %v", err)
	}
}

func TestResendCooldown(t *testing.T) {
	s := newTestService()
	ctx := context.Background()
	if _, _, err := s.RequestCode(ctx, "+998901234567"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.RequestCode(ctx, "+998901234567"); !errors.Is(err, ErrTooSoon) {
		t.Errorf("qayta so'rash ErrTooSoon berishi kerak, olindi %v", err)
	}
}

func TestTokenIssueAndParse(t *testing.T) {
	issuer := NewTokenIssuer("secret1", time.Hour)
	u := &User{ID: "u1", Role: RoleCourier, EntityID: "c1"}
	tok, err := issuer.Issue(u)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := issuer.Parse(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "u1" || claims.Role != RoleCourier || claims.EntityID != "c1" {
		t.Errorf("claims noto'g'ri: %+v", claims)
	}

	// Boshqa secret bilan imzolangan token rad etiladi
	other := NewTokenIssuer("secret2", time.Hour)
	if _, err := other.Parse(tok); err == nil {
		t.Error("begona secret bilan token qabul qilindi")
	}
	// Buzilgan token rad etiladi
	if _, err := issuer.Parse(strings.TrimSuffix(tok, "=") + "x"); err == nil {
		t.Error("buzilgan token qabul qilindi")
	}
}
