package users

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// ┌─ ARGON2 — QUROLGA AYLANISHI MUMKIN BO'LGAN HIMOYA ────────────────┐
// Argon2id ATAYLAB qimmat: har chaqiruv 64 MB xotira oladi. Bu
// brute-force'ga qarshi ajoyib, LEKIN cheklovsiz qoldirilsa hujumchi
// uni SERVERGA qarshi ishlatadi:
//
//	/auth/login ga bir vaqtda 50 ta so'rov -> 50 × 64 MB = 3.2 GB
//	-> server xotiradan chiqadi (OOM). Parol to'g'ri bo'lishi SHART
//	EMAS: `VerifyAgainstDummy` ham (akkaunt topilmaganda, vaqtni
//	tenglashtirish uchun) xuddi shu 64 MB ni oladi.
//
// IP bo'yicha cheklov buni to'xtatmaydi: portlash 30 ta va hujum
// taqsimlangan bo'lishi mumkin.
//
// Shu sabab BIR VAQTDA bajariladigan Argon2 amallari soni qat'iy
// chegaralangan. Chegaradan oshgan so'rovlar navbatda qisqa vaqt
// kutadi va keyin 503 oladi — server tirik qoladi, halol
// foydalanuvchi esa navbatdan o'tadi.
// └───────────────────────────────────────────────────────────────────┘

// argonConcurrency — bir vaqtda ruxsat etilgan Argon2 amallari.
//
// Har biri 64 MB oladi, ya'ni eng ko'p xotira = shu son × 64 MB.
// CPU yadrolarining yarmi (kamida 2, ko'pi bilan 8): hisoblash CPU'ga
// ham bog'liq, hammasini band qilish boshqa so'rovlarni o'ldirardi.
var argonGate = make(chan struct{}, argonConcurrency())

// argonConcurrency — `ARGON_MAX_CONCURRENCY` bo'lsa o'sha, aks holda
// yadrolar soniga qarab.
//
// XOTIRA HISOBI (sozlashda shu formuladan foydalaning):
//
//	eng ko'p xotira ≈ chegara × 64 MB
//
// Standart qiymat yadrolarga bog'liq (2..8), ya'ni 2 yadroli VPS'da
// 128 MB, 12 yadroli serverda 384 MB. Kichik xotirali mashinada uni
// QO'LDA pasaytirish kerak — CPU soni RAM hajmini bildirmaydi.
func argonConcurrency() int {
	if v := strings.TrimSpace(os.Getenv("ARGON_MAX_CONCURRENCY")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 64 {
			return n
		}
		slog.Warn("ARGON_MAX_CONCURRENCY qiymati yaroqsiz — standart ishlatiladi",
			"qiymat", v)
	}
	n := runtime.NumCPU() / 2
	if n < 2 {
		n = 2
	}
	if n > 8 {
		n = 8
	}
	return n
}

// argonQueueWait — navbatda kutishning eng uzun muddati.
//
// Undan uzoq kutish ma'nosiz: mijoz allaqachon taslim bo'ladi, biz esa
// goroutine'larni to'plab, holatni yomonlashtiramiz.
const argonQueueWait = 3 * time.Second

// withArgonSlot — Argon2 amalini navbat ostida bajaradi.
func withArgonSlot(ctx context.Context, fn func()) error {
	timer := time.NewTimer(argonQueueWait)
	defer timer.Stop()
	select {
	case argonGate <- struct{}{}:
		defer func() { <-argonGate }()
		fn()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ErrServerBusy
	}
}

// Parol saqlash — Argon2id.
//
// NEGA Argon2id: OWASP "Password Storage" tavsiyasining BIRINCHI tanlovi.
// U xotira-og'ir (memory-hard), ya'ni GPU/ASIC bilan parallel brute-force
// qilish bcrypt'ga qaraganda ancha qimmatga tushadi.
//
// MUHIM KONTEKST: bu loyihada parol ATAYLAB tanlangan yechim EMAS —
// asosiy kirish yo'li telefon + SMS kod (parolsiz), ya'ni o'g'irlanadigan
// parol bazasi umuman yo'q edi. Parol dizayn talabiga ko'ra qo'shildi
// (ROADMAP 57-band). Shu sabab bu yerdagi har bir chora — parol
// mavjudligining xavfini imkon qadar kamaytirish uchun.

// Argon2id parametrlari — OWASP minimal tavsiyasi (2024) va undan
// yuqoriroq xotira. 64 MB × 3 o'tish bitta serverda ham sezilarli
// yuk bermaydi (login kamdan-kam amal), lekin ommaviy brute-force'ni
// juda qimmat qiladi.
const (
	argonTime    uint32 = 3
	argonMemory  uint32 = 64 * 1024 // 64 MB
	argonThreads uint8  = 2
	argonKeyLen  uint32 = 32
	argonSaltLen        = 16
)

// Parol uzunligi chegaralari.
//
// MinPasswordLength — OWASP minimal 8. Tarkib qoidalari (katta harf,
// raqam, maxsus belgi MAJBURIY) ATAYLAB YO'Q: OWASP ularni tavsiya
// QILMAYDI — ular parolni kuchaytirmaydi, faqat foydalanuvchini
// `Parol123!` kabi bashorat qilinadigan naqshga majburlaydi.
//
// MaxPasswordLength — 128. Chegara KERAK: chegarasiz parol Argon2'ga
// megabaytlab kirish berish orqali DoS vositasiga aylanadi.
const (
	MinPasswordLength = 8
	MaxPasswordLength = 128
)

var (
	ErrPasswordTooShort  = fmt.Errorf("parol kamida %d belgidan iborat bo'lishi kerak", MinPasswordLength)
	ErrPasswordTooLong   = fmt.Errorf("parol %d belgidan uzun bo'lmasligi kerak", MaxPasswordLength)
	ErrPasswordTooCommon = errors.New("bu parol juda oson topiladi — boshqasini tanlang")
	ErrPasswordMismatch  = errors.New("parollar mos kelmadi")
	// ErrInvalidHash — bazadagi hash buzilgan/eski formatda.
	ErrInvalidHash = errors.New("parol hash formati noto'g'ri")
)

// commonPasswords — eng ko'p ishlatiladigan parollarning qisqa ro'yxati.
//
// To'liq ro'yxat (masalan HaveIBeenPwned k-anonymity API) kelajakda
// qo'shilishi mumkin; hozircha bu ro'yxat eng ommaviy variantlarni
// to'sadi. O'zbekcha/ruscha keng tarqalganlari ham kiritilgan.
var commonPasswords = map[string]bool{
	"12345678": true, "123456789": true, "1234567890": true,
	"password": true, "password1": true, "password123": true,
	"qwerty123": true, "qwertyui": true, "11111111": true,
	"00000000": true, "iloveyou": true, "admin123": true,
	"parol123": true, "toshkent": true, "uzbekistan": true,
	"12341234": true, "abc12345": true, "1q2w3e4r": true,
}

// ValidatePassword — parolni SAQLASHDAN OLDIN tekshiradi.
//
// Uzunlik BELGILARDA (rune) sanaladi, baytlarda emas: kirill yoki
// o'zbek lotin harflari ko'p baytli, bayt bo'yicha sanash
// foydalanuvchini adashtiradi.
func ValidatePassword(p string) error {
	n := utf8.RuneCountInString(p)
	if n < MinPasswordLength {
		return ErrPasswordTooShort
	}
	if n > MaxPasswordLength {
		return ErrPasswordTooLong
	}
	if commonPasswords[strings.ToLower(p)] {
		return ErrPasswordTooCommon
	}
	return nil
}

// HashPassword — Argon2id hash, PHC string formatida qaytaradi:
//
//	$argon2id$v=19$m=65536,t=3,p=2$<base64 salt>$<base64 hash>
//
// Parametrlar hash ICHIDA saqlanadi, shuning uchun kelajakda ularni
// kuchaytirsak, eski parollar ham tekshirilishda davom etadi (va
// keyingi muvaffaqiyatli kirishda qayta hash qilinadi — `NeedsRehash`).
func HashPassword(ctx context.Context, password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("tasodifiy tuz (salt) yaratib bo'lmadi: %w", err)
	}
	var key []byte
	if err := withArgonSlot(ctx, func() {
		key = argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	}); err != nil {
		return "", err
	}
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword — parol hash'ga mos kelishini tekshiradi.
//
// Taqqoslash `subtle.ConstantTimeCompare` bilan: oddiy `==` baytma-bayt
// erta chiqadi va tekshirish DAVOMIYLIGI orqali hash'ning nechta
// birinchi bayti to'g'ri ekanini oshkor qiladi (timing attack).
func VerifyPassword(ctx context.Context, password, encoded string) (bool, error) {
	mem, iters, threads, salt, want, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}
	var got []byte
	if err := withArgonSlot(ctx, func() {
		got = argon2.IDKey([]byte(password), salt, iters, mem, threads, uint32(len(want))) //nolint:gosec // decodeHash uzunlikni maxHashPartLen bilan cheklaydi
	}); err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// NeedsRehash — hash ESKI (hozirgidan zaifroq) parametrlar bilan
// yaratilganmi.
//
// Parametrlar hash ICHIDA saqlangani uchun eski parollar tekshirilishda
// davom etadi; bu funksiya esa ularni jimgina KUCHAYTIRISH imkonini
// beradi: muvaffaqiyatli kirishda (ochiq parol faqat o'sha lahzada
// mavjud) hash yangi parametrlar bilan qayta yoziladi.
//
// Buzilgan/tanib bo'lmaydigan hash uchun `true` qaytaradi — bunday
// yozuv baribir almashtirilishi kerak.
func NeedsRehash(encoded string) bool {
	mem, t, threads, _, key, err := decodeHash(encoded)
	if err != nil {
		return true
	}
	return mem < argonMemory || t < argonTime ||
		threads < argonThreads || uint32(len(key)) < argonKeyLen //nolint:gosec // decodeHash uzunlikni maxHashPartLen bilan cheklaydi
}

// maxHashPartLen — hash ichidagi tuz va kalitning eng katta uzunligi.
// Haqiqiy qiymatlar 16/32 bayt; chegara bazadagi buzilgan yozuv ulkan kalit
// bilan Argon2'ni ortiqcha ishlatishiga yo'l qo'ymaydi va uzunlikni
// uint32 ga o'girishni xavfsiz qiladi.
const maxHashPartLen = 1024

func decodeHash(encoded string) (mem, time uint32, threads uint8, salt, key []byte, err error) {
	parts := strings.Split(encoded, "$")
	// "", "argon2id", "v=19", "m=..,t=..,p=..", salt, key
	if len(parts) != 6 || parts[1] != "argon2id" {
		return 0, 0, 0, nil, nil, ErrInvalidHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return 0, 0, 0, nil, nil, ErrInvalidHash
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &time, &threads); err != nil {
		return 0, 0, 0, nil, nil, ErrInvalidHash
	}
	if salt, err = base64.RawStdEncoding.Strict().DecodeString(parts[4]); err != nil {
		return 0, 0, 0, nil, nil, ErrInvalidHash
	}
	if key, err = base64.RawStdEncoding.Strict().DecodeString(parts[5]); err != nil {
		return 0, 0, 0, nil, nil, ErrInvalidHash
	}
	if len(salt) == 0 || len(key) == 0 || len(salt) > maxHashPartLen || len(key) > maxHashPartLen {
		return 0, 0, 0, nil, nil, ErrInvalidHash
	}
	return mem, time, threads, salt, key, nil
}

// dummyHash — MAVJUD BO'LMAGAN foydalanuvchi uchun ham parol
// tekshirish ishini bajarish maqsadida ishlatiladigan haqiqiy hash.
//
// NEGA KERAK (foydalanuvchi sanab olish / user enumeration):
// akkaunt topilmasa darhol xato qaytarsak, javob ~1 ms da keladi;
// akkaunt bor bo'lsa Argon2 ~50 ms ishlaydi. Shu farq bilan hujumchi
// QAYSI telefon raqamlari ro'yxatda borligini aniqlab oladi. Endi
// ikkala holatda ham bir xil ish bajariladi.
var dummyHash string

func init() {
	// Ishga tushishda — navbat bo'sh, `context.Background()` yetarli.
	h, err := HashPassword(context.Background(),
		"dummy-password-for-timing-equalisation")
	if err != nil {
		panic("parol modulini ishga tushirib bo'lmadi: " + err.Error())
	}
	dummyHash = h
}

// VerifyAgainstDummy — akkaunt topilmaganda chaqiriladi. Natijasi
// e'tiborga olinmaydi, maqsad — javob vaqtini tenglashtirish.
func VerifyAgainstDummy(ctx context.Context, password string) {
	_, _ = VerifyPassword(ctx, password, dummyHash)
}
