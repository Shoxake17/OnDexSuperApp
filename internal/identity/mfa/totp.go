// Package mfa — ikki bosqichli tasdiq: TOTP (RFC 6238) va bir martalik
// tiklash kodlari (AUTH-PHASE-A.md 9-bo'lim).
//
// ┌─ QOIDALAR ────────────────────────────────────────────────────────┐
//   - TOTP: SHA-1, 30 s, 6 raqam (Google Authenticator, Microsoft
//     Authenticator, 1Password bilan mos); soat farqi uchun ±1 qadam.
//   - Bir kod IKKI MARTA ishlatilmaydi: `last_used_step` dan katta qadam
//     talab qilinadi (o'g'irlangan kodni darhol qayta yuborish rad etiladi).
//   - Maxfiy kalit bazada FAQAT shifrlangan (`secretstore`, AES-256-GCM).
//   - Tiklash kodlari — 10 ta, bir martalik, bazada faqat SHA-256 xeshi.
//
// └───────────────────────────────────────────────────────────────────┘
package mfa

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // RFC 6238 TOTP standarti HMAC-SHA1; autentifikator ilovalari shuni kutadi
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	Period = 30 * time.Second
	Digits = 6
	// Skew — qo'shni qadamlar (soat farqi). Ko'proq qadam = taxmin oynasi kengroq.
	Skew = 1
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateSecret — 160-bit (RFC 4226 tavsiyasi) tasodifiy kalit, base32.
func GenerateSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return b32.EncodeToString(b), nil
}

// URI — autentifikator ilovasiga QR orqali beriladigan `otpauth://` havola.
func URI(issuer, account, secret string) string {
	label := url.PathEscape(issuer + ":" + account)
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprint(Digits))
	q.Set("period", fmt.Sprint(int(Period.Seconds())))
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// Step — vaqt qadami.
func Step(t time.Time) int64 { return t.Unix() / int64(Period.Seconds()) }

// codeAt — RFC 4226 HOTP (dinamik kesish).
func codeAt(key []byte, step int64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step)) //nolint:gosec // qadam manfiy emas
	m := hmac.New(sha1.New, key)
	m.Write(msg[:])
	sum := m.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	v := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", v%1_000_000)
}

func decodeSecret(secret string) ([]byte, error) {
	key, err := b32.DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil || len(key) < 10 {
		return nil, errors.New("mfa: TOTP kaliti yaroqsiz")
	}
	return key, nil
}

// Code — berilgan vaqtdagi kod (testlar va dev).
func Code(secret string, t time.Time) (string, error) {
	key, err := decodeSecret(secret)
	if err != nil {
		return "", err
	}
	return codeAt(key, Step(t)), nil
}

// Validate — kod `now` atrofidagi oynada to'g'rimi. To'g'ri bo'lsa MOS
// KELGAN qadamni qaytaradi: chaqiruvchi uni `last_used_step` dan katta
// ekanini atomik tekshirishi SHART (takroriy ishlatishga qarshi).
func Validate(secret, code string, now time.Time) (int64, bool) {
	code = strings.TrimSpace(code)
	if len(code) != Digits {
		return 0, false
	}
	key, err := decodeSecret(secret)
	if err != nil {
		return 0, false
	}
	cur := Step(now)
	var hit int64
	ok := 0
	// Barcha qadamlar tekshiriladi (vaqt bo'yicha farq oshkor bo'lmasin).
	for d := int64(-Skew); d <= Skew; d++ {
		if subtle.ConstantTimeCompare([]byte(codeAt(key, cur+d)), []byte(code)) == 1 {
			hit, ok = cur+d, 1
		}
	}
	return hit, ok == 1
}

// ── Tiklash kodlari ──────────────────────────────────────────────────

// RecoveryCount — bir to'plamdagi kodlar soni.
const RecoveryCount = 10

// recoveryAlphabet — o'qishda adashtiradigan belgilarsiz (0/O, 1/I/L).
const recoveryAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// NewRecoveryCodes — ochiq kodlar (foydalanuvchiga BIR MARTA ko'rsatiladi)
// va ularning xeshlari (bazaga). Har kod ~50 bit: `XXXXX-XXXXX`.
func NewRecoveryCodes() (plain []string, hashes [][]byte, err error) {
	for range RecoveryCount {
		b := make([]byte, 10)
		if _, err := rand.Read(b); err != nil {
			return nil, nil, err
		}
		var sb strings.Builder
		for i, x := range b {
			if i == 5 {
				sb.WriteByte('-')
			}
			sb.WriteByte(recoveryAlphabet[int(x)%len(recoveryAlphabet)])
		}
		c := sb.String()
		plain = append(plain, c)
		hashes = append(hashes, HashRecovery(c))
	}
	return plain, hashes, nil
}

// HashRecovery — kiritilgan kodni normallashtirib xeshlaydi (katta-kichik
// harf, tire va bo'shliq farq qilmaydi).
func HashRecovery(code string) []byte {
	c := strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(code)))
	sum := sha256.Sum256([]byte("ondex-mfa-recovery-v1:" + c))
	return sum[:]
}

// LooksLikeRecovery — kiritilgan qiymat TOTP emas, tiklash kodimi.
func LooksLikeRecovery(code string) bool {
	c := strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(code))
	return len(c) == 10
}
