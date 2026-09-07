package users

import (
	"crypto/sha256"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	Role     Role   `json:"role"`
	EntityID string `json:"entity_id,omitempty"`
	// PhoneProven — token SMS kod tasdig'i orqali berilgan (`Verify`),
	// ya'ni egasi shu telefon raqamiga HOZIRGINA ega ekanini isbotladi.
	//
	// Bu FAQAT parol o'rnatishda ishlatiladi (`POST /me/password`):
	// parolini unutgan odam joriy parolni ayta olmaydi, lekin SMS kod
	// egalikni kamida parol darajasida isbotlaydi. Boshqa hech qanday
	// imtiyoz bermaydi.
	PhoneProven bool `json:"pv,omitempty"`
	jwt.RegisteredClaims
}

// PhoneProofWindow — SMS tasdig'i "yangi" hisoblanadigan muddat.
//
// NEGA CHEKLANGAN: `PhoneProven` tokenga 30 kunga yozilib qolsa,
// o'g'irlangan token butun shu muddat davomida parolni almashtirish
// (va haqiqiy egani `Revoke` bilan tizimdan chiqarib yuborish)
// huquqini berardi. 15 daqiqa — kirgandan keyin parol qo'yish uchun
// yetarli, hujum oynasi uchun esa juda tor.
const PhoneProofWindow = 15 * time.Minute

// HasFreshPhoneProof — token SMS tasdig'i orqali va YAQINDA berilganmi.
func (c *Claims) HasFreshPhoneProof(now time.Time) bool {
	if c == nil || !c.PhoneProven || c.IssuedAt == nil {
		return false
	}
	d := now.Sub(c.IssuedAt.Time)
	return d >= 0 && d <= PhoneProofWindow
}

type TokenIssuer struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenIssuer(secret string, ttl time.Duration) *TokenIssuer {
	return &TokenIssuer{secret: []byte(secret), ttl: ttl}
}

// codePepper — OTP kodlarini hashlash uchun SERVER TOMONIDAGI kalit
// (bug.md 48-band).
//
// JWT sirining O'ZI emas, undan HKDF-ga o'xshash yo'l bilan olingan
// ALOHIDA kalit: bitta sir ikki maqsadda ishlatilsa, birining oqishi
// ikkinchisini ham ochadi.
//
// Alohida `.env` o'zgaruvchisi qo'shilmadi ATAYLAB: u yana bir
// "unutilsa jimgina zaiflashadi" nuqtasi bo'lardi (42/99-bandlar
// aynan shu sinf). JWT siri esa production'da allaqachon MAJBURIY.
func (t *TokenIssuer) codePepper() []byte {
	sum := sha256.Sum256(append([]byte("ondex-otp-pepper-v1:"), t.secret...))
	return sum[:]
}

// Issue — oddiy kirish tokeni (parol bilan kirish, admin amallari).
func (t *TokenIssuer) Issue(u *User) (string, error) {
	return t.issue(u, false)
}

// IssuePhoneProven — SMS kod tasdiqlangandan keyingi token. `Verify`
// dan boshqa joyda ISHLATILMASLIGI kerak (`Claims.PhoneProven` izohiga
// qarang).
func (t *TokenIssuer) IssuePhoneProven(u *User) (string, error) {
	return t.issue(u, true)
}

func (t *TokenIssuer) issue(u *User, phoneProven bool) (string, error) {
	now := time.Now()
	claims := Claims{
		Role:        u.Role,
		EntityID:    u.EntityID,
		PhoneProven: phoneProven,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID,
			ExpiresAt: jwt.NewNumericDate(now.Add(t.ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
}

func (t *TokenIssuer) Parse(tokenStr string) (*Claims, error) {
	var claims Claims
	token, err := jwt.ParseWithClaims(tokenStr, &claims, func(tok *jwt.Token) (any, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("kutilmagan imzo algoritmi")
		}
		return t.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("token yaroqsiz")
	}
	return &claims, nil
}
