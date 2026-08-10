package storage

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"chustapp/internal/users"
)

// RedisCodeStore — SMS tasdiqlash kodlarini Redis'da saqlaydi. Bu vazifaga
// Redis tabiiy mos keladi: kod qisqa umr ko'radi (5 daqiqa, users.codeTTL)
// va odatda bir marta o'qiladi — Redis'ning o'zi TTL bilan avtomatik
// tozalaydi. Solishtirish uchun: PgCodeStore'da bunday avtomatik tozalash
// yo'q edi (eskirgan qatorlar phone_codes jadvalida qolib ketaveradi, hech
// kim o'chirmaydi) — Redis versiyasi bu jihatdan ham to'g'riroq.
type RedisCodeStore struct {
	rdb *redis.Client
}

func NewRedisCodeStore(rdb *redis.Client) *RedisCodeStore {
	return &RedisCodeStore{rdb: rdb}
}

func codeKey(phone string) string { return "otp:" + phone }

func (s *RedisCodeStore) Save(ctx context.Context, c *users.Code) error {
	key := codeKey(c.Target)
	ttl := time.Until(c.ExpiresAt)
	if ttl <= 0 {
		ttl = time.Second
	}
	// Oddiy Pipeline (MULTI/EXEC emas) — ikkita buyruq (HSET+EXPIRE) bitta
	// tarmoq round-trip'da yuboriladi, lekin Redis'ning MULTI/EXEC
	// tranzaksiya qatlamiga o'ralmaydi (bizga bu yerda qat'iy atomiklik
	// shart emas — faqat ikkala buyruq ketma-ket, tez yetib borsa kifoya).
	pipe := s.rdb.Pipeline()
	// Xarita (map) o'rniga aniq kalit-qiymat juftliklari — HSet'ning
	// map argumentini "yozib" (unroll) berishi go-redis versiyalari orasida
	// izchil emasligi aniqlandi (jonli sinovda "wrong number of arguments"
	// xatosi berdi); aniq juftliklar har doim to'g'ri ishlaydi.
	pipe.HSet(ctx, key,
		"target", c.Target,
		"code_hash", c.CodeHash,
		"expires_at", c.ExpiresAt.Format(time.RFC3339Nano),
		"created_at", c.CreatedAt.Format(time.RFC3339Nano),
		"attempts", 0,
	)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisCodeStore) Get(ctx context.Context, phone string) (*users.Code, error) {
	res, err := s.rdb.HGetAll(ctx, codeKey(phone)).Result()
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, users.ErrInvalidCode
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, res["expires_at"])
	if err != nil {
		return nil, err
	}
	createdAt, err := time.Parse(time.RFC3339Nano, res["created_at"])
	if err != nil {
		return nil, err
	}
	attempts, _ := strconv.Atoi(res["attempts"])
	return &users.Code{
		Target:    res["target"],
		CodeHash:  res["code_hash"],
		ExpiresAt: expiresAt,
		CreatedAt: createdAt,
		Attempts:  attempts,
	}, nil
}

// IncrementAttempts — `HIncrBy` atomik va YANGI qiymatni qaytaradi,
// shuning uchun chegara tekshiruvi poyga (race) holatiga tushmaydi.
func (s *RedisCodeStore) IncrementAttempts(ctx context.Context, phone string) (int, error) {
	key := codeKey(phone)
	// Muddati o'tib ketgan kalitni HIncrBy TTL'siz qayta yaratib qo'yishi
	// mumkin (u holda abadiy Redis'da qolib ketardi) — shuning uchun avval
	// mavjudligini tekshiramiz; yo'q bo'lsa jim o'tkazib yuboramiz (xuddi
	// Postgres'dagi "WHERE phone=$1" 0 qator yangilagani kabi — xato emas).
	exists, err := s.rdb.Exists(ctx, key).Result()
	if err != nil || exists == 0 {
		return 0, err
	}
	n, err := s.rdb.HIncrBy(ctx, key, "attempts", 1).Result()
	return int(n), err
}

func (s *RedisCodeStore) Delete(ctx context.Context, phone string) error {
	return s.rdb.Del(ctx, codeKey(phone)).Err()
}
