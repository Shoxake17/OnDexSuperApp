// Package secretstore — bazada saqlanadigan BEGONA sirlarni shifrlash.
//
// ┌─ NIMA UCHUN KERAK ────────────────────────────────────────────────┐
// Restoran o'zining to'lov shlyuzi kalitini (`secret`) panel orqali
// kiritadi. Bu kalit bilan uning hisobidagi pul harakati o'qiladi va
// (keyinchalik) to'lov yaratiladi — ya'ni u PAROLGA teng.
//
// Parollardan farqi: bizga uning ASL matni kerak, chunki uni shlyuzga
// uzatamiz. Shuning uchun `argon2` (bir tomonlama) yaramaydi —
// qaytariladigan shifrlash kerak.
//
// Baza nusxasi (zaxira, dump, o'g'irlangan disk) o'z-o'zidan kalitlarni
// bermasligi kerak: kalit `SECRETS_ENC_KEY` da, bazadan TASHQARIDA.
// └───────────────────────────────────────────────────────────────────┘
package secretstore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// KeyLen — AES-256 uchun kalit uzunligi (bayt).
const KeyLen = 32

var (
	// ErrNoKey — `SECRETS_ENC_KEY` sozlanmagan.
	ErrNoKey = errors.New("secretstore: shifrlash kaliti sozlanmagan")
	// ErrBadKey — kalit formati yoki uzunligi noto'g'ri.
	ErrBadKey = fmt.Errorf("secretstore: kalit %d bayt bo'lishi kerak (hex yoki base64)", KeyLen)
	// ErrBadData — shifrlangan matn buzilgan yoki boshqa kalitniki.
	ErrBadData = errors.New("secretstore: shifrlangan qiymatni ochib bo'lmadi")
)

// Box — AES-256-GCM shifrlagich.
//
// GCM tanlandi, CBC emas: u shifrlash BILAN BIRGA butunlikni ham
// tekshiradi. Aks holda bazaga kirgan odam shifrlangan baytlarni
// o'zgartirib, kalitni boshqa qiymatga "buray" olardi va buni hech kim
// sezmasdi.
type Box struct {
	aead cipher.AEAD
}

// New — kalitdan shifrlagich yasaydi.
//
// Kalit hex (64 belgi) yoki base64 bo'lishi mumkin: ikkalasi ham
// qo'llaniladi, chunki `openssl rand -hex 32` va `-base64 32` bir xil
// darajada keng ishlatiladi va qaysi biri yozilganini talab qilish
// deploy paytida bekorga xato beradi.
//
// Bo'sh matn — `ErrNoKey`: chaqiruvchi buni "imkoniyat o'chiq" deb
// qarashi kerak, xato deb emas.
func New(key string) (*Box, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, ErrNoKey
	}
	raw, err := decodeKey(key)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(raw)
	if err != nil {
		return nil, ErrBadKey
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

func decodeKey(key string) ([]byte, error) {
	if raw, err := hex.DecodeString(key); err == nil && len(raw) == KeyLen {
		return raw, nil
	}
	if raw, err := base64.StdEncoding.DecodeString(key); err == nil && len(raw) == KeyLen {
		return raw, nil
	}
	if raw, err := base64.RawStdEncoding.DecodeString(key); err == nil && len(raw) == KeyLen {
		return raw, nil
	}
	return nil, ErrBadKey
}

// Seal — matnni shifrlaydi. Natija: base64(nonce || ciphertext).
//
// Har chaqiruvda nonce TASODIFIY, ya'ni bir xil kalit ikki marta
// shifrlansa natija har xil bo'ladi. Bu shart: aks holda bazaga qarab
// "bu ikki restoranning kaliti bir xil" degan xulosa chiqarish mumkin
// bo'lardi.
func (b *Box) Seal(plain string) (string, error) {
	if b == nil {
		return "", ErrNoKey
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := b.aead.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

// Open — `Seal` natijasini qaytaradi.
func (b *Box) Open(enc string) (string, error) {
	if b == nil {
		return "", ErrNoKey
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(enc))
	if err != nil {
		return "", ErrBadData
	}
	n := b.aead.NonceSize()
	if len(raw) < n {
		return "", ErrBadData
	}
	plain, err := b.aead.Open(nil, raw[:n], raw[n:], nil)
	if err != nil {
		return "", ErrBadData
	}
	return string(plain), nil
}

// Mask — sirning oxirgi to'rt belgisini qoldirib, qolganini yashiradi.
//
// Logda va interfeysda "qaysi kalit saqlangan" degan savolga javob
// berish uchun: to'liq qiymat hech qayerda ko'rsatilmaydi, lekin
// foydalanuvchi o'zi kiritgan kalitni tanib oladi.
func Mask(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 4 {
		return strings.Repeat("*", len(s))
	}
	return strings.Repeat("*", len(s)-4) + s[len(s)-4:]
}
