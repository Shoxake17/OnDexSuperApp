// Package revoke — chiqarilgan JWT'larni MUDDATIDAN OLDIN bekor qilish.
//
// MUAMMO (tuzatilgan zaiflik): tokenlar imzo bo'yicha tekshirilardi,
// xolos. Ya'ni:
//   - "Chiqish" (logout) tugmasi faqat qurilmadagi nusxani o'chirardi —
//     token o'g'irlangan bo'lsa, u to'liq amal muddatigacha (30 kun)
//     ishlashda davom etardi;
//   - superadmin restoranni o'chirganda uning xodim akkaunti bazadan
//     o'chirilardi, LEKIN qo'lidagi token hamon ishlayverardi (auth
//     bazaga umuman qaramaydi);
//   - kuryerning tasdig'i bekor qilinganda ham xuddi shunday.
//
// YECHIM: har bir foydalanuvchi uchun "shu vaqtdan oldin chiqarilgan
// barcha tokenlar yaroqsiz" belgisi saqlanadi. Token tekshirilayotganda
// uning `iat` (chiqarilgan vaqti) shu belgidan oldin bo'lsa — rad
// etiladi.
//
// TEZLIK: belgi RAM'da (map) saqlanadi, shuning uchun har bir so'rovda
// hech qanday tarmoq/DB murojaati YO'Q. Redis faqat serverni qayta
// ishga tushirishdan omon qolish uchun ishlatiladi (ishga tushishda bir
// marta o'qiladi). Redis ulanmagan bo'lsa ham bekor qilish ishlaydi —
// faqat server restart bo'lsa unutiladi.
//
// CHEKLOV (ataylab): bir nechta API nusxasi (replica) ishlatilsa, bekor
// qilish faqat so'rovni qabul qilgan nusxada darhol ta'sir qiladi;
// qolganlari keyingi restartda ko'radi. Hozirgi joylashtirish — bitta
// Docker xizmati, shuning uchun bu yetarli. Ko'p nusxaga o'tilganda
// Redis pub/sub qo'shish kifoya.
package revoke

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "revoked:"

type Store struct {
	mu sync.RWMutex
	at map[string]time.Time // userID -> shu vaqtdan oldingi tokenlar yaroqsiz

	rdb *redis.Client // nil bo'lishi mumkin (Redis ixtiyoriy)
	ttl time.Duration // token amal muddati — belgi shundan uzoq saqlanmaydi
}

// New — bo'sh do'kon yaratadi va (Redis bo'lsa) saqlangan belgilarni
// yuklaydi. `tokenTTL` — chiqariladigan JWT'ning amal muddati: undan
// eski belgini saqlashning ma'nosi yo'q, chunki bunday tokenlar
// baribir o'z-o'zidan yaroqsiz bo'ladi.
func New(rdb *redis.Client, tokenTTL time.Duration) *Store {
	s := &Store{at: make(map[string]time.Time), rdb: rdb, ttl: tokenTTL}
	s.load()
	return s
}

func (s *Store) load() {
	if s.rdb == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var cursor uint64
	loaded := 0
	for {
		keys, next, err := s.rdb.Scan(ctx, cursor, keyPrefix+"*", 500).Result()
		if err != nil {
			slog.Warn("bekor qilingan tokenlar ro'yxatini o'qib bo'lmadi", "err", err)
			return
		}
		for _, k := range keys {
			raw, err := s.rdb.Get(ctx, k).Result()
			if err != nil {
				continue
			}
			sec, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				continue
			}
			s.at[k[len(keyPrefix):]] = time.Unix(sec, 0)
			loaded++
		}
		if next == 0 {
			break
		}
		cursor = next
	}
	if loaded > 0 {
		slog.Info("bekor qilingan sessiyalar tiklandi", "count", loaded)
	}
}

// Revoke — shu foydalanuvchining HOZIRGACHA chiqarilgan barcha
// tokenlarini yaroqsiz qiladi.
//
// BELGI SONIYAGACHA YAXLITLANADI. Sabab: JWT `iat` maydoni soniya
// aniqligida saqlanadi, `time.Now()` esa nanosekundgacha. Yaxlitlashsiz
// 10:00:00.500 da qo'yilgan belgi AYNAN o'sha soniyada chiqarilgan
// tokenni ham (iat = 10:00:00) "eski" deb rad etardi — ya'ni
// bekor qilishdan keyin DARHOL beriladigan yangi token o'zi
// yaroqsiz bo'lib tug'ilardi (`POST /me/password` shunday ishlaydi).
// `IsRevoked` izohida ta'riflangan xatti-harakat aslida faqat shu
// yaxlitlash bilan to'g'ri bo'ladi.
//
// QOLADIGAN YON TA'SIR: bekor qilish bilan BIR XIL soniyada
// chiqarilgan token yaroqli qoladi (eng ko'pi 1 soniyalik oyna).
// Bu ataylab qabul qilingan — hujumchi qurbon parolini qachon
// o'zgartirishini bilmagani uchun bu oynaga tushishni rejalashtira
// olmaydi, halol qayta-login esa har doim ishlashi kerak.
func (s *Store) Revoke(ctx context.Context, userID string) {
	if userID == "" {
		return
	}
	now := time.Now().Truncate(time.Second)

	s.mu.Lock()
	s.at[userID] = now
	s.mu.Unlock()

	if s.rdb != nil {
		if err := s.rdb.Set(ctx, keyPrefix+userID,
			strconv.FormatInt(now.Unix(), 10), s.ttl).Err(); err != nil {
			// Redis'ga yozib bo'lmasa ham bekor qilish SHU NUSXADA
			// kuchga kiradi — faqat restartdan keyin unutiladi.
			slog.Warn("bekor qilishni saqlab bo'lmadi", "user", userID, "err", err)
		}
	}
}

// IsRevoked — `iat` (token chiqarilgan vaqt) bekor qilish belgisidan
// oldinmi. Belgi yo'q bo'lsa — false (tez yo'l, har bir so'rovda shu).
func (s *Store) IsRevoked(userID string, issuedAt time.Time) bool {
	s.mu.RLock()
	t, ok := s.at[userID]
	s.mu.RUnlock()
	if !ok {
		return false
	}
	// `Before` — aynan bekor qilish soniyasida chiqarilgan token
	// yaroqli qoladi (masalan "chiqish" bosilgan zahoti qayta login
	// qilingan holat). JWT `iat` soniya aniqligida bo'lgani uchun bu
	// ataylab tanlangan: aks holda halol qayta-login rad etilardi.
	return issuedAt.Before(t)
}
