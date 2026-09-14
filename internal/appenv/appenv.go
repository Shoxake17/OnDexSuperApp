// Package appenv — API server o'qiydigan BARCHA muhit
// o'zgaruvchilarining yagona reyestri.
//
// ┌─ NEGA BU PAKET BOR (bug.md 42, 52, 53, 99-bandlar) ────────────────┐
// Loyihada bir xil xato KAMIDA TO'RT marta takrorlangan: kod
// `os.Getenv("X")` ni o'qiydi, `deploy/docker-compose.prod.yml` ning
// `environment:` ro'yxatida esa `X` yo'q. Docker Compose `.env`
// dagi o'zgaruvchini konteynerga AVTOMATIK uzatmaydi — u faqat
// compose faylining o'zida almashtirish uchun ishlatiladi.
//
// Natija har safar bir xil: lokalda hammasi ishlaydi, production'da
// funksiya JIMGINA o'chadi. Xato xabari yo'q, log yo'q — kod
// shunchaki bo'sh satr oladi va "o'chirilgan" shoxga tushadi.
//
//	R2_*                 → rasmlar konteyner ichida qoldi (har deploy'da yo'qoldi)
//	GOOGLE_MAPS_API_KEY  → panelda "config/maps -> 503"
//	OCTO_*               → karta to'lovi umuman yoqilmagan
//	PUBLIC_API_URL       → Octo callback manzili "/payments/octo/callback"
//	SHADDIY_*, GEMINI_*  → AI va ovozli rejim o'chiq
//
// Compose faylidagi izoh bu tuzoq haqida IKKI MARTA ogohlantiradi va
// baribir takrorlandi. Sabab tizimli: **ro'yxat qo'lda yuritiladi va
// uni hech narsa tekshirmaydi.**
//
// Shuning uchun reyestr shu yerda, KODDA turadi va ikki tomondan
// qulflangan:
//
//  1. `appenv_test.go` manba kodini skanerlab, `os.Getenv("X")`
//     bilan o'qilgan HAR BIR nom shu reyestrda borligini tekshiradi.
//     Yangi o'zgaruvchi qo'shgan odam reyestrni to'ldirmasa, CI
//     yiqiladi.
//  2. O'sha test `deploy/docker-compose.prod.yml` mavjud bo'lsa,
//     `DeployRequired` deb belgilangan har bir nom uning ichida
//     borligini ham tekshiradi.
//
// Va `Report()` server ishga tushganda yo'q o'zgaruvchilarni ULAR
// NIMANI O'CHIRISHI bilan birga logga chiqaradi — ya'ni "jimgina
// o'chgan funksiya" endi jim emas.
// └────────────────────────────────────────────────────────────────────┘
package appenv

import (
	"log/slog"
	"os"
	"sort"
	"strings"
)

// Var — bitta muhit o'zgaruvchisi haqidagi yozuv.
type Var struct {
	// Name — `os.Getenv` ga beriladigan nom.
	Name string

	// Effect — bu o'zgaruvchi BO'LMASA nima ishlamaydi. Log xabarida
	// aynan shu matn ko'rinadi, shuning uchun u "sozlanmagan" emas,
	// FUNKSIYA tilida yozilishi kerak.
	Effect string

	// ProdRequired — production'da yo'qligi server ishga tushishini
	// TO'XTATADI (kod o'zi `os.Exit(1)` qiladi). Bu yerda faqat
	// hujjat sifatida belgilanadi.
	ProdRequired bool

	// DeployRequired — `deploy/docker-compose.prod.yml` ning
	// `environment:` ro'yxatida SANAB CHIQILGAN bo'lishi shart.
	//
	// `false` bo'lgan yagona holat — o'zgaruvchi faqat DEV uchun
	// (masalan `OCTO_TRUST_CALLBACK_DEV`) yoki compose'da boshqa
	// mexanizm bilan beriladi.
	DeployRequired bool
}

// Registry — API server o'qiydigan barcha o'zgaruvchilar.
//
// Tartib: mavzular bo'yicha, compose faylining tuzilishiga mos —
// ikkalasini yonma-yon solishtirish oson bo'lsin.
var Registry = []Var{
	// ---------- Asosiy ----------
	{Name: "APP_ENV", Effect: "dev rejim yoqilmaydi (production standarti)", DeployRequired: true},
	{Name: "JWT_SECRET", Effect: "server ishga tushmaydi", ProdRequired: true, DeployRequired: true},
	{Name: "ALLOWED_ORIGINS", Effect: "server ishga tushmaydi", ProdRequired: true, DeployRequired: true},
	{Name: "TRUSTED_PROXIES", Effect: "IP cheklovlari proksi ortida NOTO'G'RI ishlaydi (bug.md 18-band)", DeployRequired: true},

	// ---------- Manzillar ----------
	// ┌─ IKKI O'XSHASH NOM ───────────────────────────────────────────┐
	// `PUBLIC_BASE_URL` — Telegram "qaytish" havolasi uchun.
	// `PUBLIC_API_URL`  — Octo to'lov callback'i uchun.
	// Compose'da birinchisi BOR edi, ikkinchisi YO'Q — va aynan
	// ikkinchisi pul oqimida turadi (bug.md 42-band).
	// └───────────────────────────────────────────────────────────────┘
	{Name: "PUBLIC_BASE_URL", Effect: "Telegram orqali kirish havolasi buziladi", DeployRequired: true},
	{Name: "PUBLIC_API_URL", Effect: "KARTA TO'LOVI O'LIK: Octo callback'i hech qachon kelmaydi, buyurtma 30 daqiqadan keyin bekor bo'ladi", DeployRequired: true},
	{Name: "WEB_PUBLIC_BASE_URL", Effect: "brauzerdan Telegram bilan kirish yakunlanmaydi", DeployRequired: true},

	// ---------- Ma'lumot omborlari ----------
	{Name: "DATABASE_URL", Effect: "server ishga tushmaydi", ProdRequired: true, DeployRequired: true},
	{Name: "REDIS_ADDR", Effect: "kesh va tezlik cheklovi o'chadi", DeployRequired: true},
	{Name: "REDIS_PASSWORD", Effect: "Redis'ga ulanib bo'lmaydi", DeployRequired: true},
	{Name: "MONGODB_URI", Effect: "server ishga tushmaydi (katalog faqat Mongo'da)", ProdRequired: true, DeployRequired: true},
	{Name: "MONGODB_DB", Effect: "standart baza nomi ishlatiladi", DeployRequired: true},

	// ---------- Media (R2) ----------
	{Name: "R2_BUCKET", Effect: "server ishga tushmaydi (media ombori yo'q)", ProdRequired: true, DeployRequired: true},
	{Name: "R2_ACCOUNT_ID", Effect: "R2 sozlanmaydi", DeployRequired: true},
	{Name: "R2_ACCESS_KEY_ID", Effect: "R2 sozlanmaydi", DeployRequired: true},
	{Name: "R2_SECRET_ACCESS_KEY", Effect: "R2 sozlanmaydi", DeployRequired: true},
	{Name: "R2_PUBLIC_URL", Effect: "R2 sozlanmaydi", DeployRequired: true},
	// 3D maketlar uchun ALOHIDA, ommaviy BO'LMAGAN bucket. Bo'sh
	// bo'lsa maket ommaviy bucket'dan beriladi — havola faqat kirgan
	// foydalanuvchiga ko'rsatiladi, lekin muddatsiz bo'ladi.
	{Name: "R2_SCENES_BUCKET", Effect: "3D maket ommaviy bucket'dan, MUDDATSIZ havola bilan beriladi", DeployRequired: true},
	// Ixtiyoriy: maketlar uchun ALOHIDA, faqat-o'qish tokeni. Berilmasa
	// rasmlar tokeni ishlatiladi — u ishlaydi, lekin yozish huquqiga
	// ham ega (eng kam huquq tamoyiliga zid).
	{Name: "R2_SCENES_ACCESS_KEY_ID", Effect: "maket uchun rasmlar tokeni ishlatiladi (yozish huquqi bilan)", DeployRequired: true},
	{Name: "R2_SCENES_SECRET_ACCESS_KEY", Effect: "maket uchun rasmlar tokeni ishlatiladi (yozish huquqi bilan)", DeployRequired: true},

	// ---------- Push (FCM) ----------
	{Name: "FIREBASE_SERVICE_ACCOUNT_FILE", Effect: "push bildirishnomalar o'chadi", DeployRequired: true},
	{Name: "FIREBASE_SERVICE_ACCOUNT_JSON", Effect: "push uchun muqobil manba (FILE afzal)", DeployRequired: false},
	{Name: "FIREBASE_PROJECT_ID", Effect: "push bildirishnomalar o'chadi", DeployRequired: true},

	// ---------- Telegram ----------
	{Name: "TELEGRAM_BOT_TOKEN", Effect: "Telegram orqali kirish va bot bildirishnomalari o'chadi", DeployRequired: true},
	{Name: "TELEGRAM_MINIAPP_SHORT_NAME", Effect: "stol QR havolalari standart nomga tushadi", DeployRequired: true},

	// ---------- Email ----------
	{Name: "SMTP_HOST", Effect: "email yuborilmaydi", DeployRequired: true},
	{Name: "SMTP_PORT", Effect: "standart port ishlatiladi", DeployRequired: true},
	{Name: "SMTP_USERNAME", Effect: "email yuborilmaydi", DeployRequired: true},
	{Name: "SMTP_PASSWORD", Effect: "email yuborilmaydi", DeployRequired: true},
	// SMTP_FROM yo'q bo'lsa kod SMTP_USERNAME ga tushadi, u esa
	// Resend'da aynan "resend" — email manzili EMAS. Natijada
	// `From: OnDex <resend>` yaroqsiz bo'ladi va xatlar rad etiladi.
	{Name: "SMTP_FROM", Effect: "jo'natuvchi manzili YAROQSIZ bo'ladi va xatlar rad etiladi/spamga tushadi", DeployRequired: true},
	{Name: "SMTP_FROM_NAME", Effect: "jo'natuvchi nomi ko'rsatilmaydi", DeployRequired: true},
	{Name: "EMAIL_LOGIN_ENABLED", Effect: "email bilan kirish o'chiq qoladi", DeployRequired: true},

	// ---------- Xarita va geo ----------
	{Name: "GOOGLE_MAPS_API_KEY", Effect: "panelda xarita ochilmaydi (/config/maps → 503)", DeployRequired: true},
	{Name: "GOOGLE_GEOCODING_API_KEY", Effect: "manzil aniqlash va masofa hisobi o'chadi", DeployRequired: true},
	{Name: "YANDEX_GEOCODER_API_KEY", Effect: "manzil aniqlash faqat Google'ga tayanadi", DeployRequired: true},
	{Name: "DGIS_API_KEY", Effect: "manzil aniqlash faqat Google'ga tayanadi", DeployRequired: true},

	// ---------- SMS ----------
	{Name: "ESKIZ_EMAIL", Effect: "SMS yuborilmaydi — OTP kodlar LOGGA tushadi (bug.md 47-band)", DeployRequired: true},
	{Name: "ESKIZ_PASSWORD", Effect: "SMS yuborilmaydi", DeployRequired: true},
	{Name: "ESKIZ_FROM", Effect: "SMS sinov jo'natuvchisidan ketadi", DeployRequired: true},
	// `ESKIZ_ENABLED=false` — SMS kanalini ATAYLAB o'chirish. Busiz
	// production'da `ESKIZ_*` sozlanmagan bo'lsa server to'xtaydi
	// (47-band). Diqqat: bu o'zgaruvchi `parseEnvBool` orqali
	// o'qiladi, ya'ni manba skaneri (`appenv_test.go`) uni `os.Getenv`
	// literali sifatida TOPMAYDI — qo'lda qo'shildi.
	{Name: "ESKIZ_ENABLED", Effect: "SMS kanali yoqiq; `false` bo'lsa kod faqat Telegram/Firebase orqali ketadi", DeployRequired: true},
	{Name: "ESKIZ_BASE_URL", Effect: "standart Eskiz manzili ishlatiladi", DeployRequired: true},

	// ---------- To'lov (Octo) ----------
	{Name: "OCTO_SHOP_ID", Effect: "karta orqali to'lov O'CHIQ — faqat naqd", DeployRequired: true},
	{Name: "OCTO_SECRET", Effect: "karta orqali to'lov o'chiq", DeployRequired: true},
	{Name: "OCTO_SIGNATURE_KEY", Effect: "callback imzosi tekshirilmaydi", DeployRequired: true},
	{Name: "OCTO_TEST", Effect: "to'lov SINOV rejimida qoladi — haqiqiy pul olinmaydi (bug.md 43-band)", DeployRequired: true},
	{Name: "OCTO_RETURN_URL", Effect: "to'lovdan keyin mijoz ilovaga qaytmaydi", DeployRequired: true},
	// FAQAT DEV: production'da bu bayroq YOQILMASLIGI kerak, shuning
	// uchun compose'da ataylab yo'q.
	{Name: "OCTO_TRUST_CALLBACK_DEV", Effect: "faqat dev: imzosiz callback qabul qilinadi", DeployRequired: false},

	// ---------- AI (Shaddiy + Gemini) ----------
	{Name: "SHADDIY_AI_URL", Effect: "AI yordamchisi BUTUNLAY o'chiq (/ai/* → 503)", DeployRequired: true},
	{Name: "SHADDIY_API_KEY", Effect: "AI yordamchisi butunlay o'chiq", DeployRequired: true},
	{Name: "GEMINI_API_KEY", Effect: "ovozli rejim o'chiq", DeployRequired: true},
	{Name: "GEMINI_LIVE_MODEL", Effect: "standart Gemini modeli ishlatiladi", DeployRequired: true},

	// ---------- 3D model (Tripo) ----------
	{Name: "TRIPO_API_KEY", Effect: "rasmdan 3D model yaratish o'chiq (endpointlar 503)", DeployRequired: true},
	{Name: "TRIPO_BASE_URL", Effect: "standart Tripo manzili ishlatiladi", DeployRequired: true},
	{Name: "TRIPO_MODEL_VERSION", Effect: "standart model versiyasi ishlatiladi", DeployRequired: true},

	// ---------- Qidiruv (MeiliSearch) ----------
	// Ikkalasi ham FAQAT DEV: hozircha `docker-compose.yml`da (lokal
	// dev) turibdi, `deploy/docker-compose.prod.yml`da YO'Q. Production
	// ishga tushirilganda shu ikkovi ham compose'ga qo'shiladi va
	// `DeployRequired: true` ga o'zgartiriladi.
	{Name: "MEILI_HOST", Effect: "tezkor qidiruv o'chiq — eski Mongo substring skaneri ishlatiladi", DeployRequired: false},
	{Name: "MEILI_API_KEY", Effect: "MeiliSearch'ga master-key'siz so'raladi (faqat lokal, 127.0.0.1 portida xavfsiz)", DeployRequired: false},

	// ---------- Boshqaruv ----------
	{Name: "BOOTSTRAP_ADMIN_PHONE", Effect: "yangi bazada hech kim ADMIN bo'la olmaydi", DeployRequired: true},
	{Name: "ARGON_MAX_CONCURRENCY", Effect: "parol xeshlash cheklovi standart qiymatda", DeployRequired: true},
}

// Missing — hozir o'rnatilmagan (bo'sh) o'zgaruvchilar.
func Missing() []Var {
	var out []Var
	for _, v := range Registry {
		if strings.TrimSpace(os.Getenv(v.Name)) == "" {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Report — server ishga tushganda yo'q o'zgaruvchilarni ULAR NIMANI
// O'CHIRISHI bilan birga logga chiqaradi.
//
// Dev'da bu shovqin bo'lardi (u yerda ko'p narsa ataylab yo'q),
// shuning uchun faqat production'da to'liq chiqadi.
func Report(devMode bool) {
	missing := Missing()
	if len(missing) == 0 {
		slog.Info("muhit: barcha ma'lum o'zgaruvchilar o'rnatilgan", "jami", len(Registry))
		return
	}
	if devMode {
		names := make([]string, len(missing))
		for i, v := range missing {
			names[i] = v.Name
		}
		slog.Info("muhit: o'rnatilmagan o'zgaruvchilar (dev — odatiy holat)",
			"soni", len(missing), "nomlar", strings.Join(names, ", "))
		return
	}
	// Production: har birini ALOHIDA qatorda, oqibati bilan. Bu
	// xabarlar aynan "jimgina o'chgan funksiya" ni ko'rinadigan
	// qiladi — 42 va 99-bandlardagi beshta funksiya shu sababdan
	// oylab o'lik turgan edi.
	slog.Warn("muhit: o'rnatilmagan o'zgaruvchilar bor — quyidagi funksiyalar ISHLAMAYDI",
		"soni", len(missing))
	for _, v := range missing {
		slog.Warn("muhit: o'zgaruvchi yo'q", "nom", v.Name, "oqibati", v.Effect)
	}
}
