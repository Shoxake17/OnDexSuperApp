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
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"sync"
	"time"
)

// SigningKey — imzolash kaliti va uning identifikatori (`kid`).
//
// Ikki xil bo'ladi:
//   - FAYL kaliti (dev/test, `cmd/authkey`): `Private` to'ldirilgan;
//   - KMS kaliti (prod, OpenBao/Vault Transit): `Private` BO'SH, imzo
//     `signer` orqali KMS'da qo'yiladi — maxfiy kalit jarayon xotirasiga
//     umuman kirmaydi.
type SigningKey struct {
	KID     string
	Private ed25519.PrivateKey
	Public  ed25519.PublicKey
	signer  crypto.Signer
}

// Signer — JWT imzolash uchun (`golang-jwt` EdDSA `crypto.Signer` qabul qiladi).
func (k *SigningKey) Signer() crypto.Signer {
	if k.signer != nil {
		return k.signer
	}
	return k.Private
}

// RemoteKey — KMS'dagi kalitning bitta versiyasi (ochiq qismi + imzolovchi).
type RemoteKey struct {
	Version int
	Public  ed25519.PublicKey
	Created time.Time
	Signer  crypto.Signer
}

// KeySource — KMS'dagi kalitning barcha (o'chirilmagan) versiyalari.
type KeySource interface {
	Versions(ctx context.Context) ([]RemoteKey, error)
}

// KeySet — bitta FAOL imzolash kaliti va tekshirish uchun qabul qilinadigan
// barcha ochiq kalitlar (rotatsiya: eski kalit bilan imzolangan tokenlar
// muddati tugaguncha tekshirilishi kerak).
//
// KMS rejimida to'plam vaqti-vaqti bilan yangilanadi (`StartRefresh`).
type KeySet struct {
	mu     sync.RWMutex
	active *SigningKey
	verify map[string]ed25519.PublicKey
	// deriveRoot — yordamchi HMAC sirlari (SSO cookie, mfa_token, passkey
	// holati, IP xeshi) uchun ildiz. KMS rejimida imzolash kalitidan
	// MUSTAQIL (rotatsiyada sessiyalar uzilmaydi).
	deriveRoot []byte

	// extra — faqat TEKSHIRISH uchun qo'shimcha kalitlar (fayl rejimidan
	// KMS'ga o'tishda eski kalit bilan chiqarilgan tokenlar muddati tugaguncha).
	extra map[string]ed25519.PublicKey

	// KMS rejimi.
	src          KeySource
	publishDelay time.Duration
	now          func() time.Time
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
	if k.Private == nil {
		return nil, errors.New("identity: KMS kalitini eksport qilib bo'lmaydi")
	}
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

// NewKeySet — FAYL kalitlari: faol kalit va (rotatsiya davrida) qo'shimcha
// tekshirish kalitlari. Faol kalit har doim tekshirish ro'yxatida bo'ladi.
//
// Yordamchi sirlar ildizi faol kalitdan hosil qilinadi (orqaga moslik:
// prod'dagi SSO cookie va sessiya IP xeshlari shu bilan yaratilgan).
func NewKeySet(active *SigningKey, previous ...*SigningKey) (*KeySet, error) {
	if active == nil {
		return nil, errors.New("identity: faol imzolash kaliti yo'q")
	}
	if active.Private == nil {
		return nil, errors.New("identity: fayl kaliti emas — KMS uchun NewRemoteKeySet")
	}
	ks := &KeySet{active: active, verify: map[string]ed25519.PublicKey{active.KID: active.Public}, now: time.Now}
	for _, p := range previous {
		if p == nil {
			continue
		}
		if _, dup := ks.verify[p.KID]; dup {
			continue
		}
		ks.verify[p.KID] = p.Public
	}
	ks.deriveRoot = active.Private.Seed()
	return ks, nil
}

// MinDeriveRoot — KMS rejimida yordamchi sirlar ildizining eng kam uzunligi.
const MinDeriveRoot = 32

// DefaultPublishDelay — yangi kalit versiyasi JWKS'da e'lon qilingandan
// keyin imzolashda ishlatilguncha kutiladigan vaqt. Boshqa servislar
// (ShoxPos, OnDexMap) JWKS'ni keshlaydi: kalit ular keshida paydo
// bo'lmasdan imzolansa, ular yangi tokenni rad etardi.
const DefaultPublishDelay = 15 * time.Minute

// NewRemoteKeySet — KMS (OpenBao/Vault Transit) kalitlari. `root` —
// KMS'da hisoblangan yordamchi sirlar ildizi (masalan Transit HMAC).
func NewRemoteKeySet(ctx context.Context, src KeySource, root []byte, publishDelay time.Duration) (*KeySet, error) {
	if src == nil {
		return nil, errors.New("identity: KMS kalit manbai yo'q")
	}
	if len(root) < MinDeriveRoot {
		return nil, errors.New("identity: KMS yordamchi sir ildizi juda qisqa")
	}
	if publishDelay < 0 {
		publishDelay = 0
	}
	ks := &KeySet{src: src, deriveRoot: append([]byte(nil), root...), publishDelay: publishDelay, now: time.Now}
	if err := ks.Refresh(ctx); err != nil {
		return nil, err
	}
	return ks, nil
}

// Refresh — KMS'dan versiyalarni qayta o'qiydi. Faol kalit — e'lon
// qilinganiga `publishDelay` o'tgan ENG YANGI versiya (hali "yosh" versiya
// faqat tekshirish ro'yxatida turadi). Hamma versiya yosh bo'lsa (birinchi
// ishga tushish) — eng eskisi.
func (ks *KeySet) Refresh(ctx context.Context) error {
	if ks.src == nil {
		return nil
	}
	vs, err := ks.src.Versions(ctx)
	if err != nil {
		return fmt.Errorf("identity: KMS kalitlarini o'qib bo'lmadi: %w", err)
	}
	if len(vs) == 0 {
		return errors.New("identity: KMS'da kalit versiyasi yo'q")
	}
	sort.Slice(vs, func(a, b int) bool { return vs[a].Version < vs[b].Version })
	verify := make(map[string]ed25519.PublicKey, len(vs))
	var chosen *RemoteKey
	cutoff := ks.now().Add(-ks.publishDelay)
	for i := range vs {
		v := &vs[i]
		if len(v.Public) != ed25519.PublicKeySize || v.Signer == nil {
			return fmt.Errorf("identity: KMS kaliti v%d Ed25519 emas", v.Version)
		}
		verify[KIDFor(v.Public)] = v.Public
		if !v.Created.After(cutoff) {
			chosen = v
		}
	}
	if chosen == nil {
		chosen = &vs[0]
	}
	if pub, ok := chosen.Signer.Public().(ed25519.PublicKey); !ok || !pub.Equal(chosen.Public) {
		return errors.New("identity: KMS imzolovchisi ochiq kalitga mos emas")
	}
	active := &SigningKey{KID: KIDFor(chosen.Public), Public: chosen.Public, signer: chosen.Signer}
	ks.mu.Lock()
	for kid, pub := range ks.extra {
		if _, ok := verify[kid]; !ok {
			verify[kid] = pub
		}
	}
	prev := ""
	if ks.active != nil {
		prev = ks.active.KID
	}
	ks.active, ks.verify = active, verify
	ks.mu.Unlock()
	if prev != "" && prev != active.KID {
		slog.Info("identity: faol imzolash kaliti almashdi (KMS rotatsiyasi)", "kid", active.KID, "eski", prev)
	}
	return nil
}

// StartRefresh — KMS rejimida versiyalarni `every` da bir yangilaydi
// (rotatsiyani restartsiz qabul qilish). Fayl rejimida hech narsa qilmaydi.
func (ks *KeySet) StartRefresh(ctx context.Context, every time.Duration) {
	if ks.src == nil || every <= 0 {
		return
	}
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				rctx, cancel := context.WithTimeout(ctx, 15*time.Second)
				if err := ks.Refresh(rctx); err != nil {
					// Eski to'plam bilan ishlash davom etadi — KMS qisqa
					// uzilishi kirishni to'xtatmasin.
					slog.Warn("identity: KMS kalitlarini yangilab bo'lmadi", "err", err)
				}
				cancel()
			}
		}
	}()
}

// AddVerifyOnly — imzolashda ISHLATILMAYDIGAN, faqat tekshiriladigan kalitlar
// (KMS'ga o'tish: AUTH_PREVIOUS_KEY_FILES). Rotatsiya yangilanishida saqlanadi.
func (ks *KeySet) AddVerifyOnly(keys ...*SigningKey) {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	if ks.extra == nil {
		ks.extra = map[string]ed25519.PublicKey{}
	}
	for _, k := range keys {
		if k == nil {
			continue
		}
		ks.extra[k.KID] = k.Public
		if _, ok := ks.verify[k.KID]; !ok {
			ks.verify[k.KID] = k.Public
		}
	}
}

// Remote — kalitlar KMS'dami.
func (ks *KeySet) Remote() bool { return ks.src != nil }

// Active — imzolash uchun kalit.
func (ks *KeySet) Active() *SigningKey {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.active
}

// PublicKey — `kid` bo'yicha tekshirish kaliti.
func (ks *KeySet) PublicKey(kid string) (ed25519.PublicKey, bool) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	k, ok := ks.verify[kid]
	return k, ok
}

// SharesKeyWith — ikki to'plamda umumiy kalit bormi (admin va asosiy
// kalitlar ARALASHMASLIGI tekshiruvi uchun).
func (ks *KeySet) SharesKeyWith(other *KeySet) bool {
	if ks == nil || other == nil {
		return false
	}
	ks.mu.RLock()
	kids := make([]string, 0, len(ks.verify))
	for kid := range ks.verify {
		kids = append(kids, kid)
	}
	ks.mu.RUnlock()
	for _, kid := range kids {
		if _, ok := other.PublicKey(kid); ok {
			return true
		}
	}
	return false
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
	ks.mu.RLock()
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
	ks.mu.RUnlock()
	return json.Marshal(map[string]any{"keys": keys})
}

// derivedSecret — ildizdan domen bilan ajratilgan sir (SSO cookie,
// mfa_token, passkey holati, IP xeshi). Imzolash kalitining o'zi boshqa
// maqsadda ISHLATILMAYDI — bir sirning oqishi ikkinchisini ochmasin.
//
// Fayl rejimida ildiz = faol kalit urug'i (oldingi formula bilan BIR XIL —
// prod'dagi cookie'lar yaroqli qoladi).
func (ks *KeySet) derivedSecret(domain string) []byte {
	sum := sha256.Sum256(append([]byte("ondex-identity-"+domain+"-v1:"), ks.deriveRoot...))
	return sum[:]
}
