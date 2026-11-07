// Package identity — OnDex'ning MARKAZIY sessiya va token xizmati
// (ADR-000/002, docs/architecture/AUTH-PHASE-A.md, A2 bosqichi).
//
// ┌─ NIMA QILADI ─────────────────────────────────────────────────────────┐
//   - Ed25519 (EdDSA) imzoli qisqa access tokenlar (10 daqiqa) chiqaradi va
//     tekshiradi; ochiq kalitlar JWKS orqali e'lon qilinadi — boshqa
//     servislar tokenni O'ZI tekshiradi, maxfiy kalit faqat shu yerda.
//   - Sessiyalar BITTA markazda (`auth_sessions`): refresh token rotatsiyasi,
//     qayta ishlatishni aniqlash, bekor qilish. Markazda bekor qilingan
//     sessiya barcha klientlarda (web, desktop, mobil) tugaydi.
//
// └───────────────────────────────────────────────────────────────────────┘
//
// Eski HS256 tokenlar (`users.TokenIssuer`) o'tish davrida ishlashda davom
// etadi; bu paket ularga tegmaydi.
package identity

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"sort"
)

// SigningKey — imzolash kaliti va uning identifikatori (`kid`).
type SigningKey struct {
	KID     string
	Private ed25519.PrivateKey
	Public  ed25519.PublicKey
}

// KeySet — bitta FAOL imzolash kaliti va tekshirish uchun qabul qilinadigan
// barcha ochiq kalitlar (rotatsiya: eski kalit bilan imzolangan tokenlar
// muddati tugaguncha tekshirilishi kerak).
type KeySet struct {
	active *SigningKey
	verify map[string]ed25519.PublicKey
}

// KIDFor — ochiq kalitdan barqaror identifikator: sha256 ning birinchi 12
// bayti, base64url. Kalitning o'zidan hisoblanadi — konfiguratsiyada alohida
// `kid` yozish (va uni noto'g'ri yozib qo'yish) imkoni yo'q.
func KIDFor(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return base64.RawURLEncoding.EncodeToString(sum[:12])
}

// GenerateKey — yangi Ed25519 kaliti (test va `cmd/authkey` uchun).
func GenerateKey() (*SigningKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &SigningKey{KID: KIDFor(pub), Private: priv, Public: pub}, nil
}

// EncodePrivatePEM — PKCS#8 PEM ("PRIVATE KEY").
func EncodePrivatePEM(k *SigningKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(k.Private)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// ParsePrivatePEM — PKCS#8 PEM'dan Ed25519 kaliti. Boshqa turdagi kalit
// (RSA, EC) ATAYLAB rad etiladi — algoritm aralashuviga yo'l yo'q.
func ParsePrivatePEM(data []byte) (*SigningKey, error) {
	block, rest := pem.Decode(data)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, errors.New("identity: PEM'da 'PRIVATE KEY' bloki topilmadi")
	}
	if len(bytes.TrimSpace(rest)) != 0 {
		return nil, errors.New("identity: PEM faylida ortiqcha ma'lumot bor")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("identity: kalitni o'qib bo'lmadi: %w", err)
	}
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("identity: kalit Ed25519 emas")
	}
	pub := priv.Public().(ed25519.PublicKey)
	return &SigningKey{KID: KIDFor(pub), Private: priv, Public: pub}, nil
}

// LoadPrivateKeyFile — fayldan kalit. Fayl ruxsatlari Windows'da
// tekshirib bo'lmaydi, shuning uchun bu yerda emas — deploy qoidasi (600).
func LoadPrivateKeyFile(path string) (*SigningKey, error) {
	// Yo'l FAQAT server sozlamasidan (AUTH_SIGNING_KEY_FILE) keladi, hech
	// qachon so'rovdan emas — G304 bu yerda ataylab qabul qilingan.
	data, err := os.ReadFile(path) //nolint:gosec // yo'l faqat operator sozlamasidan
	if err != nil {
		return nil, fmt.Errorf("identity: kalit faylini o'qib bo'lmadi: %w", err)
	}
	return ParsePrivatePEM(data)
}

// NewKeySet — faol kalit va (rotatsiya davrida) qo'shimcha tekshirish
// kalitlari. Faol kalit har doim tekshirish ro'yxatida bo'ladi.
func NewKeySet(active *SigningKey, previous ...*SigningKey) (*KeySet, error) {
	if active == nil {
		return nil, errors.New("identity: faol imzolash kaliti yo'q")
	}
	ks := &KeySet{active: active, verify: map[string]ed25519.PublicKey{active.KID: active.Public}}
	for _, p := range previous {
		if p == nil {
			continue
		}
		if _, dup := ks.verify[p.KID]; dup {
			continue
		}
		ks.verify[p.KID] = p.Public
	}
	return ks, nil
}

// Active — imzolash uchun kalit.
func (ks *KeySet) Active() *SigningKey { return ks.active }

// PublicKey — `kid` bo'yicha tekshirish kaliti.
func (ks *KeySet) PublicKey(kid string) (ed25519.PublicKey, bool) {
	k, ok := ks.verify[kid]
	return k, ok
}

// jwk — RFC 8037 (OKP, Ed25519) ochiq kalit.
type jwk struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
}

// JWKS — `/.well-known/jwks.json` javobi. FAQAT ochiq kalitlar; tartib
// barqaror (kesh va solishtirish uchun).
func (ks *KeySet) JWKS() ([]byte, error) {
	kids := make([]string, 0, len(ks.verify))
	for kid := range ks.verify {
		kids = append(kids, kid)
	}
	sort.Strings(kids)
	keys := make([]jwk, 0, len(kids))
	for _, kid := range kids {
		keys = append(keys, jwk{
			Kty: "OKP", Crv: "Ed25519", Use: "sig", Alg: "EdDSA", Kid: kid,
			X: base64.RawURLEncoding.EncodeToString(ks.verify[kid]),
		})
	}
	return json.Marshal(map[string]any{"keys": keys})
}

// derivedSecret — kalitning maxfiy qismidan domen bilan ajratilgan sir
// (IP xeshi kabi yordamchi maqsadlar uchun). Imzolash kalitining o'zi
// boshqa maqsadda ISHLATILMAYDI — bir sirning oqishi ikkinchisini ochmasin.
func (ks *KeySet) derivedSecret(domain string) []byte {
	sum := sha256.Sum256(append([]byte("ondex-identity-"+domain+"-v1:"), ks.active.Private.Seed()...))
	return sum[:]
}
