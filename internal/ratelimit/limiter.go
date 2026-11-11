// Package ratelimit — oddiy, tashqi bog'liqliksiz token-bucket
// cheklovchi.
//
// NEGA KERAK: kod bazasida hech qanday tezlik cheklovi yo'q edi.
// Natijada:
//   - `/geocode/*` — har chaqiruv PULLIK (Google/Yandex/2GIS). Bitta
//     oddiy mijoz akkaunti cheksiz sikllab, hisobni ko'tarishi mumkin
//     edi (moliyaviy DoS).
//   - `/auth/request-code` — faqat per-telefon 60s pauza bor edi;
//     skript turli raqamlarni aylanib cheksiz HAQIQIY SMS jo'natardi.
//   - dispatch — qabul qilinmagan buyurtma pullik Distance Matrix'ni
//     abadiy urardi.
//
// Ikki qatlam (SSO-Redis):
//   - har limiter xotirada token-bucket (har doim ishlaydi);
//   - `Shared(name)` bilan nomlangan limiter `UseBackend` berilganda
//     TAQSIMLANGAN (Redis): bir nechta API nusxasi bitta hisobni bo'lishadi.
//     Backend xato bersa (Redis o'chgan) — xotiradagi chelak ishlatiladi
//     (fail-open EMAS: lokal cheklov baribir bor).
package ratelimit

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// ┌─ XOTIRA VA CPU CHEGARALARI (DoS'ga qarshi) ───────────────────────┐
// Chelaklar xaritasi HUJUMCHI boshqaradigan kalitlar bilan to'ladi:
// har yangi IP (yoki `/auth/login` da har yangi "login" qiymati) yangi
// yozuv yaratadi. Ikki xavf bor edi:
//
//	XOTIRA — yozuvlar faqat 10 daqiqalik TTL bo'yicha o'chirilardi.
//	         Taqsimlangan hujumda 10 daqiqada millionlab yozuv
//	         to'planib, serverni xotiradan chiqarardi.
//
//	CPU    — tozalash HAR YANGI KALIT qo'shilganda, MUTEX OSTIDA va
//	         butun xaritani aylanib bajarilardi. 1 mln yozuvda har
//	         yangi so'rov 1 mln elementni skanerlardi, ya'ni yuk
//	         kvadratik o'sardi va BARCHA so'rovlar bitta qulfda
//	         to'xtardi. Bu cheklovchining o'zini DoS quroliga
//	         aylantirardi.
//
// Endi tozalash VAQT bo'yicha (ko'pi bilan `sweepEvery` da bir marta),
// xotira esa `maxBuckets` bilan chegaralangan.
// └───────────────────────────────────────────────────────────────────┘
const (
	// Xotiradagi chelaklar chegarasi. ~50k × ~150 bayt ≈ 7 MB.
	maxBuckets = 50_000
	// Tozalash oralig'i — undan tez-tez skanerlash foydasiz.
	sweepEvery = 30 * time.Second
	// Chegaraga yetganda TTL shu qiymatgacha qisqartiriladi.
	pressureTTL = 60 * time.Second
	// Kalit uzunligi chegarasi (`clampKey`).
	maxKeyLen = 128
)

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// clampKey — juda uzun kalitni qat'iy uzunlikdagi hash bilan
// almashtiradi.
//
// NEGA: `/auth/login` cheklovi kaliti — foydalanuvchi YUBORGAN "login"
// satri. Tana 1 MB gacha bo'lishi mumkin, ya'ni bitta so'rov 1 MB lik
// xarita kalitini yaratardi. Bir necha o'nlab shunday so'rov
// cheklovchining o'zini xotira yeguvchiga aylantirardi.
func clampKey(k string) string {
	if len(k) <= maxKeyLen {
		return k
	}
	sum := sha256.Sum256([]byte(k))
	return hex.EncodeToString(sum[:])
}

// Limiter — kalit (masalan foydalanuvchi ID yoki IP) bo'yicha
// token-bucket. Nol qiymatli Limiter ishlatilmaydi — `New` ishlating.
type Limiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     float64 // sekundiga to'ldiriladigan token
	capacity float64 // maksimal to'plangan token (portlash imkoniyati)
	ttl      time.Duration
	// lastSweep — oxirgi tozalash vaqti (yuqoridagi izohga qarang).
	lastSweep time.Time
	// name — `Shared` bilan berilgan barqaror nom (taqsimlangan kalit).
	name string
}

// New — `capacity` ta so'rovga bir zumda ruxsat beradi, keyin
// sekundiga `perSecond` tezlikda tiklanadi.
func New(perSecond, capacity float64) *Limiter {
	l := &Limiter{
		buckets:  make(map[string]*bucket),
		rate:     perSecond,
		capacity: capacity,
		// Ishlatilmayotgan kalitlar tozalanadi — aks holda xotira
		// cheksiz o'sardi (har yangi IP uchun yangi yozuv).
		ttl: 10 * time.Minute,
	}
	return l
}

// Allow — kalit uchun bitta token ajratadi. `false` — cheklov oshgan.
func (l *Limiter) Allow(key string) bool {
	ok, _ := l.AllowWithWait(key)
	return ok
}

// AllowWithWait — `Allow` bilan bir xil, lekin RAD ETILGANDA yana
// qancha kutish kerakligini ham qaytaradi.
//
// NEGA KERAK: "juda ko'p urinish — biroz kuting" foydalanuvchiga hech
// narsa aytmaydi. U 5 soniyadan keyin ham, 10 daqiqadan keyin ham
// qayta urinib ko'radi va har safar o'sha xabarni oladi — natijada
// ilova buzuq deb o'ylaydi. Aniq vaqt ko'rsatilsa, u shunchaki kutadi.
//
// Qaytariladigan muddat — chelakda BUTUN bitta token paydo bo'lishi
// uchun kerak bo'lgan vaqt.
func (l *Limiter) AllowWithWait(key string) (bool, time.Duration) {
	if ok, wait, done := l.distributed(key); done {
		return ok, wait
	}
	return l.allowLocal(key)
}

// allowLocal — xotiradagi token-bucket.
func (l *Limiter) allowLocal(key string) (bool, time.Duration) {
	key = clampKey(key)
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	// Tozalash HAR SO'ROVDA emas, vaqt bo'yicha (yuqoridagi izoh).
	l.sweepLocked(now)

	b, ok := l.buckets[key]
	if !ok {
		// ┌─ XARITA TO'LGANDA: RAD EMAS, SIQIB CHIQARISH (bug.md 49) ──────┐
		// Ilgari yangi kalit RAD ETILARDI (fail-closed). Lekin kalitlar
		// hujumchi nazoratida (login satri, IP): 50 000 ta soxta kalit
		// yuborib xaritani to'ldirsa, keyin HAR BIR halol foydalanuvchi
		// 429 olardi — butun kirish o'chardi.
		//
		// Endi eng uzoq ishlatilmagan chelak (taxminiy LRU, tasodifiy
		// tanlanma) chiqariladi. Fail-open EMAS: yangi kalit baribir
		// chelak oladi va cheklanadi. Narxi — chiqarilgan kalit yangi
		// chelak bilan qaytadi; uni qayta "tozalash" uchun hujumchi
		// yana ~maxBuckets so'rov yuborishi kerak (o'zi esa IP
		// cheklovchisiga uriladi) — bitta nishonga qo'shimcha urinish
		// juda qimmat, umumiy rad etish esa endi imkonsiz.
		// └────────────────────────────────────────────────────────────────┘
		if len(l.buckets) >= maxBuckets {
			l.evictOneLocked()
		}
		// Yangi kalit — to'la chelak, bittasi darhol sarflanadi.
		l.buckets[key] = &bucket{tokens: l.capacity - 1, lastSeen: now}
		return true, 0
	}

	elapsed := now.Sub(b.lastSeen).Seconds()
	b.tokens += elapsed * l.rate
	if b.tokens > l.capacity {
		b.tokens = l.capacity
	}
	b.lastSeen = now

	if b.tokens < 1 {
		return false, l.waitFor(1 - b.tokens)
	}
	b.tokens--
	return true, 0
}

// waitFor — `missing` ta token to'planishi uchun kerak bo'lgan vaqt.
// Yuqoriga yaxlitlanadi: 0.2 soniya "0 soniya" bo'lib ko'rinmasin.
func (l *Limiter) waitFor(missing float64) time.Duration {
	if l.rate <= 0 {
		// Tiklanmaydigan cheklov — amalda ishlatilmaydi, lekin nolga
		// bo'lishdan himoya.
		return time.Hour
	}
	d := time.Duration(missing / l.rate * float64(time.Second))
	if d < time.Second {
		return time.Second
	}
	return d.Round(time.Second)
}

// sweepLocked — eskirgan yozuvlarni olib tashlaydi. Chaqiruvchi
// mutex'ni ushlab turishi shart.
//
// Ko'pi bilan `sweepEvery` da bir marta to'liq skanerlaydi — aks holda
// katta xaritada har so'rov O(n) ish qilib, yukni kvadratik o'stirardi
// (fayl boshidagi izohga qarang).
func (l *Limiter) sweepLocked(now time.Time) {
	over := len(l.buckets) >= maxBuckets
	if !over && now.Sub(l.lastSweep) < sweepEvery {
		return
	}
	if len(l.buckets) == 0 {
		l.lastSweep = now
		return
	}
	l.lastSweep = now

	ttl := l.ttl
	if over {
		// Xotira bosimi — ancha agressiv tozalaymiz.
		ttl = pressureTTL
	}
	for k, b := range l.buckets {
		if now.Sub(b.lastSeen) > ttl {
			delete(l.buckets, k)
		}
	}
}

// evictSample — siqib chiqarishda ko'rib chiqiladigan yozuvlar soni.
// Go map iteratsiyasi tasodifiy boshlanadi, shuning uchun bu
// "eng eskisi"ga yaqin (Redis'dagi taxminiy LRU kabi), O(evictSample).
const evictSample = 64

// evictOneLocked — tanlanmadagi eng uzoq ishlatilmagan chelakni
// o'chiradi. Chaqiruvchi mutex'ni ushlab turishi shart.
func (l *Limiter) evictOneLocked() {
	var victim string
	var oldest time.Time
	n := 0
	for k, b := range l.buckets {
		if n == 0 || b.lastSeen.Before(oldest) {
			victim, oldest = k, b.lastSeen
		}
		n++
		if n >= evictSample {
			break
		}
	}
	if n > 0 {
		delete(l.buckets, victim)
	}
}

// Len — kuzatilayotgan kalitlar soni (testlar va diagnostika uchun).
func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
