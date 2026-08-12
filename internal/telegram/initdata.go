package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Telegram Mini App (TMA) — `initData` tekshiruvi.
//
// ┌─ BU FAYL NIMA UCHUN ──────────────────────────────────────────────┐
// Mini App ochilganda Telegram sahifaga `initData` degan satr beradi:
// foydalanuvchi ID'si, ismi, `auth_date` va `hash`. Bu satr KLIENT
// tomonida turadi, ya'ni uni istalgan odam o'zgartira oladi.
//
// Agar server unga shunchaki ishonsa, hujumchi `user.id` ni boshqa
// raqamga almashtirib, BEGONA AKKAUNTGA kirib olardi — bir ham parol,
// bir ham kod talab qilinmasdan. Bu Telegram Mini App'lardagi eng
// ko'p uchraydigan va eng og'ir zaiflik.
//
// Himoya: `hash` bot tokeni yordamida HMAC-SHA256 bilan qayta
// hisoblanadi. Bot tokenini faqat server biladi, shuning uchun
// to'g'ri imzoni klient YASAY OLMAYDI.
// └───────────────────────────────────────────────────────────────────┘

// WebAppUser — `initData` ichidagi `user` obyektining KERAKLI qismi.
//
// DIQQAT: bu yerda TELEFON RAQAMI YO'Q va hech qachon bo'lmaydi —
// Telegram uni Mini App'ga umuman bermaydi. Raqamni olishning yagona
// yo'li — botda kontakt ulashish (`AskContact`). Shu sabab TMA oqimi
// bot bilan bog'liq bo'lishi SHART.
type WebAppUser struct {
	ID           int64  `json:"id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Username     string `json:"username"`
	LanguageCode string `json:"language_code"`
	IsBot        bool   `json:"is_bot"`
	PhotoURL     string `json:"photo_url"`
}

var (
	ErrInitDataInvalid = errors.New("initData imzosi noto'g'ri")
	ErrInitDataExpired = errors.New("initData muddati o'tgan")
	ErrInitDataNoUser  = errors.New("initData ichida foydalanuvchi yo'q")
)

// initDataMaxAge — `auth_date` dan qancha vaqt o'tishi mumkin.
//
// ┌─ NEGA CHEKLOV KERAK (takroriy hujum) ─────────────────────────────┐
// Imzo hech qachon eskirmaydi: bir marta olingan `initData` abadiy
// yaroqli bo'lardi. Kimdir uni qo'lga kiritsa (masalan qurilma
// jurnalidan, proksi logidan yoki almashilgan telefondan), oylar
// o'tib ham o'sha akkauntga kira olardi.
//
// 24 soat — Telegram'ning o'z tavsiyasi. Undan qisqasi foydalanuvchini
// bezovta qiladi: Mini App ochiq turib qolsa `initData` yangilanmaydi.
// └───────────────────────────────────────────────────────────────────┘
const initDataMaxAge = 24 * time.Hour

// ValidateInitData — imzoni tekshiradi va foydalanuvchini qaytaradi.
//
// `botToken` bo'sh bo'lsa xato qaytariladi: sozlanmagan holatda
// "tekshirdim" deb o'tkazib yuborish — himoyani jimgina o'chirish
// bilan barobar.
func ValidateInitData(initData, botToken string, now time.Time) (*WebAppUser, error) {
	if strings.TrimSpace(botToken) == "" {
		return nil, errors.New("TELEGRAM_BOT_TOKEN sozlanmagan — initData tekshirib bo'lmaydi")
	}
	if strings.TrimSpace(initData) == "" {
		return nil, ErrInitDataInvalid
	}

	// ┌─ NEGA `url.ParseQuery` EMAS ──────────────────────────────────┐
	// `url.ParseQuery` — `application/x-www-form-urlencoded` parseri
	// va u `+` belgisini PROBELGA aylantiradi.
	//
	// Telegram esa `initData` ni `encodeURIComponent` bilan quradi:
	// u probelni `%20` qiladi va `+` ni O'ZGARISHSIZ qoldiradi.
	//
	// `query_id` — base64 satr va uning ichida `+` BO'LISHI MUMKIN.
	// O'shanda `ParseQuery` uni probelga aylantirib qiymatni buzardi
	// va imzo HECH QACHON mos kelmasdi — foydalanuvchi "Telegram
	// ma'lumoti tasdiqlanmadi" xabarini olardi (jonli qurilmada
	// aynan shu holat uchradi).
	//
	// Shuning uchun qo'lda ajratamiz va `url.PathUnescape` bilan
	// ochamiz — u `%XX` ni dekodlaydi, `+` ga esa TEGMAYDI.
	// └───────────────────────────────────────────────────────────────┘
	vals := make(map[string]string, 8)
	for _, pair := range strings.Split(initData, "&") {
		if pair == "" {
			continue
		}
		k, v, found := strings.Cut(pair, "=")
		if !found {
			continue
		}
		dk, err := url.PathUnescape(k)
		if err != nil {
			return nil, ErrInitDataInvalid
		}
		dv, err := url.PathUnescape(v)
		if err != nil {
			return nil, ErrInitDataInvalid
		}
		vals[dk] = dv
	}

	gotHash := vals["hash"]
	if gotHash == "" {
		return nil, ErrInitDataInvalid
	}

	// ┌─ TEKSHIRUV SATRINI QURISH ────────────────────────────────────┐
	// `hash` va `signature` CHIQARIB TASHLANADI, qolganlari kalit
	// bo'yicha ALIFBO tartibida `key=value` ko'rinishida `\n` bilan
	// birlashtiriladi. Tartib muhim — u buzilsa imzo mos kelmaydi.
	//
	// `signature` — Telegram'ning yangi Ed25519 imzosi uchun maydon;
	// u HMAC hisobiga KIRMAYDI.
	// └───────────────────────────────────────────────────────────────┘
	// secret_key = HMAC-SHA256(kalit: "WebAppData", ma'lumot: bot_token)
	//
	// Diqqat: kalit va ma'lumot ALMASHTIRILGAN ko'rinadi, lekin
	// Telegram spetsifikatsiyasi aynan shunday. Joyini almashtirish —
	// keng tarqalgan xato, natijada imzo hech qachon mos kelmaydi.
	mac := hmac.New(sha256.New, []byte("WebAppData"))
	mac.Write([]byte(botToken))
	secret := mac.Sum(nil)

	// ┌─ IKKI VARIANT SINALADI ───────────────────────────────────────┐
	// Telegram `signature` maydonini keyinroq qo'shdi (uchinchi
	// tomon uchun Ed25519 imzosi) va uni HMAC hisobidan chiqarish
	// bo'yicha hujjatlar versiyalari FARQ QILADI. Klient versiyasiga
	// qarab ikkala shakl ham uchraydi.
	//
	// Ikkalasini ham sinash XAVFSIZ: har ikkalasi ham bot tokeni
	// bilan HMAC, ya'ni hujumchi ularning BIRORTASINI ham sohta
	// yasay olmaydi. `signature` esa bizda umuman ishlatilmaydi.
	//
	// Faqat bittasini sinash esa haqiqiy foydalanuvchilarni rad
	// etishga olib kelardi — sabab ko'rinmaydigan holda.
	// └───────────────────────────────────────────────────────────────┘
	matched := false
	for _, skipSignature := range []bool{true, false} {
		keys := make([]string, 0, len(vals))
		for k := range vals {
			if k == "hash" {
				continue
			}
			if skipSignature && k == "signature" {
				continue
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var sb strings.Builder
		for i, k := range keys {
			if i > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(k)
			sb.WriteByte('=')
			sb.WriteString(vals[k])
		}

		mac2 := hmac.New(sha256.New, secret)
		mac2.Write([]byte(sb.String()))
		want := hex.EncodeToString(mac2.Sum(nil))

		// Doimiy vaqtli solishtirish: oddiy `==` birinchi farqli baytda
		// to'xtaydi va javob vaqti orqali imzoni bayt-bayt tanlash
		// imkonini berardi.
		if subtle.ConstantTimeCompare([]byte(want), []byte(gotHash)) == 1 {
			matched = true
			break
		}
		// `signature` umuman yo'q bo'lsa ikkinchi variant birinchisi
		// bilan bir xil — bekorga hisoblamaymiz.
		if _, has := vals["signature"]; !has {
			break
		}
	}
	if !matched {
		return nil, ErrInitDataInvalid
	}

	// ── Muddat (takroriy hujumga qarshi) ──
	authDateRaw := vals["auth_date"]
	if authDateRaw == "" {
		return nil, ErrInitDataInvalid
	}
	sec, err := strconv.ParseInt(authDateRaw, 10, 64)
	if err != nil {
		return nil, ErrInitDataInvalid
	}
	authDate := time.Unix(sec, 0)
	if now.Sub(authDate) > initDataMaxAge {
		return nil, ErrInitDataExpired
	}
	// Kelajakdagi sana ham rad etiladi (soat farqi uchun kichik zaxira).
	if authDate.Sub(now) > 5*time.Minute {
		return nil, ErrInitDataInvalid
	}

	// ── Foydalanuvchi ──
	userRaw := vals["user"]
	if userRaw == "" {
		return nil, ErrInitDataNoUser
	}
	var u WebAppUser
	if err := json.Unmarshal([]byte(userRaw), &u); err != nil {
		return nil, ErrInitDataNoUser
	}
	if u.ID == 0 {
		return nil, ErrInitDataNoUser
	}
	if u.IsBot {
		return nil, fmt.Errorf("bot akkaunti bilan kirish mumkin emas")
	}
	return &u, nil
}
