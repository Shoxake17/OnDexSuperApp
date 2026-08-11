// Paket cache — Redis ustidagi yupqa keshlash qatlami: o'qish ko'p,
// o'zgarish kam ma'lumotlarni (restoranlar ro'yxati, menyu) qisqa muddatga
// keshlaydi.
package cache

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Connect — Redis'ga ulanishga urinadi. Manzil bo'sh bo'lsa yoki Redis
// javob bermasa (u ixtiyoriy komponent — Mongo/R2 kabi), nil qaytaradi.
// Chaqiruvchi kod har doim shu holatni tekshirib, kesh mavjud bo'lmaganda
// to'g'ridan-to'g'ri bazaga tushishi kerak (graceful degradation — Redis
// o'chib qolishi/ulanmasligi ilovani HECH QACHON to'xtatmasligi kerak,
// faqat javoblar biroz sekinroq bo'ladi).
//
// ┌─ PAROL ────────────────────────────────────────────────────────────┐
// `REDIS_PASSWORD` muhit o'zgaruvchisidan olinadi. Bo'sh bo'lsa parolsiz
// ulaniladi — mahalliy ishlab chiqishda Redis parolsiz turadi.
//
// Production'da u MAJBURIY: Redis ichki tarmoqda bo'lsa ham, boshqa
// konteyner buzilsa (masalan Next.js'da RCE) parolsiz Redis darhol
// OTP kodlar va sessiya keshiga to'liq kirish beradi. Tarmoq izolyatsiyasi
// — birinchi qatlam, parol — ikkinchisi.
//
// DIQQAT: parol NOTO'G'RI bo'lsa `Ping` xato beradi va bu funksiya `nil`
// qaytaradi — ya'ni ilova ishlashda davom etadi, lekin keshsiz va OTP
// xotirada. Shuning uchun xato matni logda ANIQ ajratiladi.
// └────────────────────────────────────────────────────────────────────┘
func Connect(addr string) *redis.Client {
	return ConnectWithPassword(addr, os.Getenv("REDIS_PASSWORD"))
}

// ConnectWithPassword — `Connect` ning aniq parol beriladigan shakli
// (testlar va bir nechta Redis ishlatilishi mumkin bo'lgan holatlar uchun).
func ConnectWithPassword(addr, password string) *redis.Client {
	if addr == "" {
		return nil
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr, Password: password})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		// NOAUTH/WRONGPASS ni alohida ajratamiz: "ulanib bo'lmadi" degan
		// umumiy xabar bilan bu sozlama xatosini tarmoq muammosidan
		// farqlash mumkin emas edi va uni topish soatlab vaqt olardi.
		msg := "Redis'ga ulanib bo'lmadi — kesh o'chirilgan holda davom etiladi"
		if s := err.Error(); strings.Contains(s, "NOAUTH") ||
			strings.Contains(s, "WRONGPASS") || strings.Contains(s, "invalid password") {
			msg = "Redis PAROLI noto'g'ri yoki berilmagan (REDIS_PASSWORD) — kesh va OTP o'chirilgan holda davom etiladi"
		}
		slog.Warn(msg, "addr", addr, "err", err)
		return nil
	}
	slog.Info("rejim: Redis kesh yoqildi", "addr", addr)
	return rdb
}

// Cache — nil-xavfsiz keshlash wrapper'i. nil qiymatda (Redis ulanmagan)
// barcha metodlar xavfsiz no-op: GetJSON doim "topilmadi" qaytaradi,
// SetJSON/Del hech narsa qilmaydi. Shuning uchun chaqiruvchi kodda
// "Redis bormi yo'qmi" deb alohida tekshirish shart emas.
type Cache struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Cache {
	if rdb == nil {
		return nil
	}
	return &Cache{rdb: rdb}
}

// GetJSON — kalitni o'qib dest'ga JSON sifatida yozadi. Topilmasa, Redis
// o'zi mavjud bo'lmasa yoki biror xato bo'lsa — false qaytaradi va
// chaqiruvchi ODATDAGIDEK bazaga murojaat qilishi kerak.
func (c *Cache) GetJSON(ctx context.Context, key string, dest any) bool {
	if c == nil {
		return false
	}
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return false
	}
	return json.Unmarshal(data, dest) == nil
}

// SetJSON — natijani berilgan muddatga keshlaydi. Xato jim log qilinadi —
// kesh faqat tezlik uchun, u ishlamasa ham asosiy so'rov buzilmasligi kerak.
func (c *Cache) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) {
	if c == nil {
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	if err := c.rdb.Set(ctx, key, data, ttl).Err(); err != nil {
		slog.Warn("keshga yozib bo'lmadi", "key", key, "err", err)
	}
}

// Del — mutatsiyadan keyin (yaratish/tahrirlash/o'chirish) eskirgan
// yozuvlarni keshdan chiqarib tashlaydi.
func (c *Cache) Del(ctx context.Context, keys ...string) {
	if c == nil || len(keys) == 0 {
		return
	}
	if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
		slog.Warn("keshni tozalab bo'lmadi", "keys", keys, "err", err)
	}
}
