package identity

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"chustapp/internal/users"
)

// AccessTTL — access token umri. Qisqa: o'g'irlangan token 10 daqiqadan
// ortiq ishlamaydi, sessiya markazda bekor qilinsa yangisi chiqmaydi.
const AccessTTL = 10 * time.Minute

// clockLeeway — serverlar soati orasidagi kichik farq uchun.
const clockLeeway = 30 * time.Second

var (
	// ErrTokenInvalid — token yaroqsiz (imzo, format, muddat, iss/aud, realm).
	// Sababi ATAYLAB ajratilmaydi: hujumchiga "qaysi tekshiruvdan o'tdim"
	// degan ma'lumot berilmaydi (sabab faqat ichki logda).
	ErrTokenInvalid = errors.New("token yaroqsiz yoki muddati o'tgan")
)

// AccessClaims — access token ichidagi ma'lumot.
//
// ROL VA RUXSATLAR YO'Q — ataylab: ular har so'rovda markazdan (authz)
// tekshiriladi. Tokenga yozilsa, ishdan bo'shatilgan xodim token muddati
// tugaguncha eski huquqlari bilan ishlayverardi.
type AccessClaims struct {
	Realm     users.Realm `json:"realm"`
	SessionID string      `json:"sid"`
	OrgID     string      `json:"org,omitempty"`
	AMR       []string    `json:"amr,omitempty"`
	ClientID  string      `json:"azp"`
	jwt.RegisteredClaims
}

// Issuer — access tokenlarni chiqaradi va tekshiradi.
type Issuer struct {
	keys     *KeySet
	issuer   string
	audience string
	now      func() time.Time
	// platform — admin (platform realmi) tokenlari uchun ALOHIDA kalitlar
	// (A6). Merchant/mijoz kaliti sizib chiqsa ham admin tokeni yasab
	// bo'lmaydi va aksincha. Berilmasa platform tokeni CHIQARILMAYDI.
	platform *KeySet
}

// ErrPlatformKeyMissing — platform (admin) kaliti sozlanmagan.
var ErrPlatformKeyMissing = errors.New("identity: admin (platform) imzolash kaliti sozlanmagan")

// WithPlatformKeys — admin tokenlari uchun alohida kalitlar (A6).
func (i *Issuer) WithPlatformKeys(ks *KeySet) *Issuer {
	i.platform = ks
	return i
}

// keysFor — realm uchun kalitlar: platform -> faqat platform kalitlari,
// qolganlar -> faqat asosiy kalitlar. Ikki to'plam ARALASHMAYDI.
func (i *Issuer) keysFor(realm users.Realm) *KeySet {
	if realm == users.RealmPlatform {
		return i.platform
	}
	return i.keys
}

// NewIssuer — `issuer` (masalan "https://api.ondex.uz") va `audience`
// ("ondex-api") MAJBURIY: tekshiruv ularsiz o'tmaydi.
func NewIssuer(keys *KeySet, issuer, audience string) (*Issuer, error) {
	if keys == nil || issuer == "" || audience == "" {
		return nil, errors.New("identity: kalit, issuer va audience majburiy")
	}
	return &Issuer{keys: keys, issuer: issuer, audience: audience, now: time.Now}, nil
}

// Keys — JWKS uchun.
func (i *Issuer) Keys() *KeySet { return i.keys }

func newJTI() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// issue — sessiya uchun access token. `notAfter` — sessiyaning mutlaq
// tugash vaqti: token undan uzoq yashamaydi.
func (i *Issuer) issue(s *Session, notAfter time.Time) (string, time.Time, error) {
	now := i.now()
	exp := now.Add(AccessTTL)
	if !notAfter.IsZero() && notAfter.Before(exp) {
		exp = notAfter
	}
	claims := AccessClaims{
		Realm:     s.Realm,
		SessionID: s.ID,
		OrgID:     s.OrgID,
		AMR:       s.AMR,
		ClientID:  s.ClientID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    i.issuer,
			Subject:   s.UserID,
			Audience:  jwt.ClaimStrings{i.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        newJTI(),
		},
	}
	ks := i.keysFor(s.Realm)
	if ks == nil {
		return "", time.Time{}, ErrPlatformKeyMissing
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tok.Header["kid"] = ks.active.KID
	signed, err := tok.SignedString(ks.active.Private)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

// Verify — tokenni tekshiradi. `realms` bo'sh bo'lmasa, token shu
// realmlardan biriga tegishli bo'lishi SHART (mijoz tokeni merchant
// marshrutida o'tmaydi).
//
// Tekshiruvlar: algoritm FAQAT EdDSA (`none`, HS256 va boshqalar rad);
// `kid` ma'lum kalit; imzo; iss; aud; exp/nbf/iat; realm ma'lum;
// sid va sub bo'sh emas.
//
// SESSIYA FAOLLIGI BU YERDA TEKSHIRILMAYDI — bu `Service.Authenticate`
// ishi (bazaga murojaat talab qiladi).
func (i *Issuer) Verify(raw string, realms ...users.Realm) (*AccessClaims, error) {
	var claims AccessClaims
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithIssuer(i.issuer),
		jwt.WithAudience(i.audience),
		jwt.WithLeeway(clockLeeway),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(i.now),
	)
	_, err := parser.ParseWithClaims(raw, &claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("kid yo'q")
		}
		// Kalit to'plami TOKENDAGI realmga qarab: admin tokeni faqat platform
		// kaliti bilan, boshqasi faqat asosiy kalit bilan o'tadi.
		c, _ := t.Claims.(*AccessClaims)
		if c == nil {
			return nil, errors.New("claims yo'q")
		}
		ks := i.keysFor(c.Realm)
		if ks == nil {
			return nil, ErrPlatformKeyMissing
		}
		pub, ok := ks.PublicKey(kid)
		if !ok {
			return nil, fmt.Errorf("noma'lum kid %q", kid)
		}
		return pub, nil
	})
	if err != nil {
		return nil, ErrTokenInvalid
	}
	if !claims.Realm.Valid() || claims.SessionID == "" || claims.Subject == "" {
		return nil, ErrTokenInvalid
	}
	if len(realms) > 0 && !slices.Contains(realms, claims.Realm) {
		return nil, ErrTokenInvalid
	}
	return &claims, nil
}
