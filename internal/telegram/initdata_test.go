package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Telegram Mini App `initData` tekshiruvi.
//
// Bu — TMA'dagi ENG MUHIM xavfsizlik chegarasi: `initData` klient
// tomonida turadi va uni istalgan odam tahrirlashi mumkin. Imzo
// tekshirilmasa, `user.id` ni almashtirib BEGONA AKKAUNTGA kirish
// mumkin bo'lardi — parolsiz, kodsiz.

const testBotToken = "123456:AAH-test-token-xyz"

// signInitDataRaw — Telegram HAQIQATAN qanday quradi, xuddi shunday.
//
// ┌─ NEGA `url.Values.Encode()` EMAS ─────────────────────────────────┐
// `Encode()` probelni `+` bilan kodlaydi (form-urlencoded qoidasi).
// Telegram esa `encodeURIComponent` ishlatadi: probel `%20`, `+` esa
// O'ZGARISHSIZ qoladi.
//
// Bu farq JONLI QURILMADA nosozlik keltirib chiqardi: `query_id` —
// base64 satr va uning ichida `+` bo'lishi mumkin. Server `ParseQuery`
// bilan o'qiganda `+` probelga aylanardi, imzo mos kelmasdi va
// foydalanuvchi "Telegram ma'lumoti tasdiqlanmadi" xabarini olardi.
//
// Eski test yordamchisi buni USHLAY OLMASDI, chunki u ham `Encode()`
// ishlatardi — kodlash va dekodlash o'z-o'ziga mos edi.
// └───────────────────────────────────────────────────────────────────┘
func encodeURIComponent(s string) string {
	// `url.PathEscape` probelni `%20` qiladi va `+` ga tegmaydi —
	// aynan `encodeURIComponent` kabi.
	return strings.ReplaceAll(url.PathEscape(s), "&", "%26")
}

func signInitDataRaw(t *testing.T, botToken string, fields map[string]string) string {
	t.Helper()
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var check strings.Builder
	for i, k := range keys {
		if i > 0 {
			check.WriteByte('\n')
		}
		check.WriteString(k + "=" + fields[k])
	}
	mac := hmac.New(sha256.New, []byte("WebAppData"))
	mac.Write([]byte(botToken))
	secret := mac.Sum(nil)
	mac2 := hmac.New(sha256.New, secret)
	mac2.Write([]byte(check.String()))
	hash := hex.EncodeToString(mac2.Sum(nil))

	parts := make([]string, 0, len(fields)+1)
	for _, k := range keys {
		parts = append(parts, encodeURIComponent(k)+"="+encodeURIComponent(fields[k]))
	}
	parts = append(parts, "hash="+hash)
	return strings.Join(parts, "&")
}

// ★ JONLI NOSOZLIK: `query_id` ichida `+` bo'lsa.
//
// Haqiqiy qurilmada aynan shu holat "Telegram ma'lumoti tasdiqlanmadi"
// xatosini bergan edi.
func TestPlusSignInQueryIDAccepted(t *testing.T) {
	now := time.Now()
	f := map[string]string{
		"auth_date": strconv.FormatInt(now.Unix(), 10),
		// base64 da `+` va `/` uchraydi — Telegram ularni kodlamaydi.
		"query_id": "AAH+dF6IQ/AAAAAN0Xoh+Drc",
		"user":     `{"id":1109793017,"first_name":"Shoxrux"}`,
	}
	raw := signInitDataRaw(t, testBotToken, f)

	if _, err := ValidateInitData(raw, testBotToken, now); err != nil {
		t.Fatalf("`+` belgili query_id rad etildi: %v", err)
	}
}

// Probel `%20` bilan kelganda ham to'g'ri ochilishi kerak.
func TestSpaceEncodedAsPercent20(t *testing.T) {
	now := time.Now()
	f := map[string]string{
		"auth_date": strconv.FormatInt(now.Unix(), 10),
		"user":      `{"id":42,"first_name":"Ali Vali"}`,
	}
	raw := signInitDataRaw(t, testBotToken, f)

	u, err := ValidateInitData(raw, testBotToken, now)
	if err != nil {
		t.Fatalf("probelli ism rad etildi: %v", err)
	}
	if u.FirstName != "Ali Vali" {
		t.Fatalf("ism buzildi: %q", u.FirstName)
	}
}

// signInitData — Telegram serveri qanday imzolasa, xuddi shunday.
func signInitData(t *testing.T, botToken string, fields map[string]string) string {
	t.Helper()
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(k + "=" + fields[k])
	}

	mac := hmac.New(sha256.New, []byte("WebAppData"))
	mac.Write([]byte(botToken))
	secret := mac.Sum(nil)
	mac2 := hmac.New(sha256.New, secret)
	mac2.Write([]byte(sb.String()))
	hash := hex.EncodeToString(mac2.Sum(nil))

	q := url.Values{}
	for k, v := range fields {
		q.Set(k, v)
	}
	q.Set("hash", hash)
	return q.Encode()
}

func validFields(now time.Time) map[string]string {
	return map[string]string{
		"auth_date": strconv.FormatInt(now.Unix(), 10),
		"query_id":  "AAHdF6IQAAAAAN0XohDhrOrc",
		"user":      `{"id":123456789,"first_name":"Shoxrux","username":"shoxrux","language_code":"uz"}`,
	}
}

// ★ ASOSIY: to'g'ri imzolangan ma'lumot qabul qilinadi.
func TestValidInitDataAccepted(t *testing.T) {
	now := time.Now()
	raw := signInitData(t, testBotToken, validFields(now))

	u, err := ValidateInitData(raw, testBotToken, now)
	if err != nil {
		t.Fatalf("to'g'ri imzo rad etildi: %v", err)
	}
	if u.ID != 123456789 {
		t.Fatalf("noto'g'ri foydalanuvchi ID: %d", u.ID)
	}
	if u.FirstName != "Shoxrux" {
		t.Fatalf("ism o'qilmadi: %q", u.FirstName)
	}
}

// ★ XAVFSIZLIK: `user.id` o'zgartirilsa RAD ETILADI.
//
// Bu aynan hujum stsenariysi: hujumchi o'z `initData` sini olib,
// ichidagi ID'ni qurbonnikiga almashtiradi.
func TestTamperedUserIDRejected(t *testing.T) {
	now := time.Now()
	f := validFields(now)
	raw := signInitData(t, testBotToken, f)

	// Imzo o'zgarmaydi, faqat `user` almashtiriladi.
	vals, _ := url.ParseQuery(raw)
	vals.Set("user", `{"id":999999999,"first_name":"Hujumchi"}`)

	if _, err := ValidateInitData(vals.Encode(), testBotToken, now); !errors.Is(err, ErrInitDataInvalid) {
		t.Fatalf("O'ZGARTIRILGAN user.id QABUL QILINDI — akkaunt egallash mumkin: %v", err)
	}
}

// ★ XAVFSIZLIK: boshqa bot tokeni bilan imzolangan ma'lumot rad etiladi.
func TestWrongBotTokenRejected(t *testing.T) {
	now := time.Now()
	raw := signInitData(t, "999999:BOSHQA-bot-token", validFields(now))

	if _, err := ValidateInitData(raw, testBotToken, now); !errors.Is(err, ErrInitDataInvalid) {
		t.Fatalf("begona token bilan imzolangan ma'lumot qabul qilindi: %v", err)
	}
}

// ★ TAKRORIY HUJUM: eski `initData` rad etiladi.
//
// Imzo hech qachon eskirmaydi, shuning uchun muddat tekshiruvisiz
// bir marta o'g'irlangan satr abadiy kalit bo'lib qolardi.
func TestExpiredInitDataRejected(t *testing.T) {
	past := time.Now().Add(-25 * time.Hour)
	raw := signInitData(t, testBotToken, validFields(past))

	if _, err := ValidateInitData(raw, testBotToken, time.Now()); !errors.Is(err, ErrInitDataExpired) {
		t.Fatalf("25 soatlik initData qabul qilindi: %v", err)
	}
}

// Chegarada (23 soat) hali yaroqli — foydalanuvchi bezovta qilinmasin.
func TestNotYetExpiredAccepted(t *testing.T) {
	past := time.Now().Add(-23 * time.Hour)
	raw := signInitData(t, testBotToken, validFields(past))

	if _, err := ValidateInitData(raw, testBotToken, time.Now()); err != nil {
		t.Fatalf("23 soatlik initData rad etildi: %v", err)
	}
}

// Kelajakdagi sana — soat manipulyatsiyasi belgisi.
func TestFutureAuthDateRejected(t *testing.T) {
	future := time.Now().Add(2 * time.Hour)
	raw := signInitData(t, testBotToken, validFields(future))

	if _, err := ValidateInitData(raw, testBotToken, time.Now()); err == nil {
		t.Fatal("kelajakdagi auth_date qabul qilindi")
	}
}

// ★ Bot tokeni sozlanmagan bo'lsa — XATO, "o'tkazib yuborish" EMAS.
//
// Aks holda token yo'q muhitda tekshiruv jimgina o'chib qolardi va
// himoya borday tuyulardi.
func TestMissingBotTokenIsError(t *testing.T) {
	now := time.Now()
	raw := signInitData(t, testBotToken, validFields(now))

	if _, err := ValidateInitData(raw, "", now); err == nil {
		t.Fatal("bot tokensiz tekshiruv MUVAFFAQIYATLI bo'ldi")
	}
}

// `hash` umuman yo'q bo'lsa.
func TestMissingHashRejected(t *testing.T) {
	q := url.Values{}
	q.Set("auth_date", strconv.FormatInt(time.Now().Unix(), 10))
	q.Set("user", `{"id":1}`)

	if _, err := ValidateInitData(q.Encode(), testBotToken, time.Now()); !errors.Is(err, ErrInitDataInvalid) {
		t.Fatal("imzosiz initData qabul qilindi")
	}
}

// Foydalanuvchi maydoni yo'q — kimlik aniqlanmaydi.
func TestMissingUserRejected(t *testing.T) {
	now := time.Now()
	raw := signInitData(t, testBotToken, map[string]string{
		"auth_date": strconv.FormatInt(now.Unix(), 10),
	})
	if _, err := ValidateInitData(raw, testBotToken, now); !errors.Is(err, ErrInitDataNoUser) {
		t.Fatalf("foydalanuvchisiz initData qabul qilindi: %v", err)
	}
}

// Bot akkaunti bilan kirish taqiqlanadi.
func TestBotAccountRejected(t *testing.T) {
	now := time.Now()
	f := validFields(now)
	f["user"] = `{"id":777,"first_name":"SomeBot","is_bot":true}`
	raw := signInitData(t, testBotToken, f)

	if _, err := ValidateInitData(raw, testBotToken, now); err == nil {
		t.Fatal("bot akkaunti qabul qilindi")
	}
}

// `signature` maydoni imzo hisobiga KIRMASLIGI kerak (Telegram'ning
// yangi Ed25519 maydoni). Kirsa — haqiqiy klientlar rad etilardi.
func TestSignatureFieldIgnored(t *testing.T) {
	now := time.Now()
	f := validFields(now)
	raw := signInitData(t, testBotToken, f)

	vals, _ := url.ParseQuery(raw)
	vals.Set("signature", "ed25519-imzo-bu-yerda")

	if _, err := ValidateInitData(vals.Encode(), testBotToken, now); err != nil {
		t.Fatalf("`signature` maydoni tekshiruvni buzdi: %v", err)
	}
}
