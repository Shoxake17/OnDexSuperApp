package users

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"
)

const (
	codeTTL        = 5 * time.Minute
	resendCooldown = 60 * time.Second
	maxAttempts    = 5
)

var phoneRe = regexp.MustCompile(`^\+998\d{9}$`)

type Service struct {
	users  Repository
	codes  CodeStore
	sms    SmsSender
	tokens *TokenIssuer
	idgen  func() string
	now    func() time.Time
}

func NewService(users Repository, codes CodeStore, sms SmsSender, tokens *TokenIssuer, idgen func() string) *Service {
	return &Service{users: users, codes: codes, sms: sms, tokens: tokens, idgen: idgen, now: time.Now}
}

// NormalizePhone — "+998 90 123-45-67" yoki "998901234567" ni "+998901234567" ga keltiradi.
func NormalizePhone(raw string) (string, error) {
	p := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(strings.TrimSpace(raw))
	if strings.HasPrefix(p, "998") && len(p) == 12 {
		p = "+" + p
	}
	if !phoneRe.MatchString(p) {
		return "", ErrInvalidPhone
	}
	return p, nil
}

// RequestCode — telefonga 6 xonali kod yuboradi. Kodni qaytaradi, lekin handler
// uni faqat dev rejimda javobga qo'shadi (production'da faqat SMS orqali boradi).
func (s *Service) RequestCode(ctx context.Context, rawPhone string) (phone, code string, err error) {
	phone, err = NormalizePhone(rawPhone)
	if err != nil {
		return "", "", err
	}
	if existing, err := s.codes.Get(ctx, phone); err == nil {
		if s.now().Sub(existing.CreatedAt) < resendCooldown {
			return "", "", ErrTooSoon
		}
	}
	code, err = randomCode()
	if err != nil {
		return "", "", err
	}
	if err := s.codes.Save(ctx, &Code{
		Phone:     phone,
		CodeHash:  hashCode(code),
		ExpiresAt: s.now().Add(codeTTL),
		CreatedAt: s.now(),
	}); err != nil {
		return "", "", err
	}
	if err := s.sms.Send(phone, fmt.Sprintf("ChustApp tasdiqlash kodi: %s", code)); err != nil {
		return "", "", err
	}
	return phone, code, nil
}

// Verify — kod to'g'ri bo'lsa foydalanuvchini topadi (yo'q bo'lsa mijoz sifatida
// yaratadi) va JWT token qaytaradi.
func (s *Service) Verify(ctx context.Context, rawPhone, code string) (string, *User, error) {
	phone, err := NormalizePhone(rawPhone)
	if err != nil {
		return "", nil, err
	}
	c, err := s.codes.Get(ctx, phone)
	if err != nil {
		return "", nil, ErrInvalidCode
	}
	if s.now().After(c.ExpiresAt) {
		s.codes.Delete(ctx, phone)
		return "", nil, ErrInvalidCode
	}
	if c.Attempts >= maxAttempts {
		s.codes.Delete(ctx, phone)
		return "", nil, ErrTooManyAttempts
	}
	if hashCode(strings.TrimSpace(code)) != c.CodeHash {
		s.codes.IncrementAttempts(ctx, phone)
		return "", nil, ErrInvalidCode
	}
	s.codes.Delete(ctx, phone)

	u, err := s.users.GetByPhone(ctx, phone)
	if errors.Is(err, ErrUserNotFound) {
		u = &User{
			ID:        s.idgen(),
			Phone:     phone,
			Role:      RoleCustomer,
			CreatedAt: s.now(),
		}
		if err := s.users.Create(ctx, u); err != nil {
			return "", nil, err
		}
	} else if err != nil {
		return "", nil, err
	}

	token, err := s.tokens.Issue(u)
	if err != nil {
		return "", nil, err
	}
	return token, u, nil
}

func randomCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func hashCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}
