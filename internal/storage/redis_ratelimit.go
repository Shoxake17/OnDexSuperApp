package storage

import (
	"context"
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// ┌─ TAQSIMLANGAN CHEKLOV (SSO-Redis) ─────────────────────────────────────┐
// Ilgari har limiter va har "xato urinishlar" hisobi faqat JARAYON
// xotirasida edi: ikkita API nusxasida hujumchi ikki baravar urinish
// olardi, restartda esa hisob nolga tushardi. Endi hisob Redis'da.
//
// Vaqt Redis'ning `TIME` buyrug'idan — API nusxalari soati farq qilsa ham
// hisob buzilmaydi. Har kalitda TTL bor (`noeviction` siyosati bilan ham
// xotira cheksiz o'smaydi).
// └────────────────────────────────────────────────────────────────────────┘

// RedisRateBackend — `ratelimit.Backend`.
type RedisRateBackend struct{ rdb redis.UniversalClient }

// NewRedisRateBackend — rdb nil bo'lsa nil.
func NewRedisRateBackend(rdb redis.UniversalClient) *RedisRateBackend {
	if rdb == nil {
		return nil
	}
	return &RedisRateBackend{rdb: rdb}
}

// tokenBucket — KEYS[1]; ARGV: rate, capacity, ttl. Qaytaradi {ok, wait_s}.
var tokenBucket = redis.NewScript(`
local rate = tonumber(ARGV[1])
local cap = tonumber(ARGV[2])
local t = redis.call('TIME')
local now = tonumber(t[1]) + tonumber(t[2]) / 1000000
local b = redis.call('HMGET', KEYS[1], 't', 's')
local tokens = tonumber(b[1])
local ts = tonumber(b[2])
if tokens == nil or ts == nil then
  tokens = cap
  ts = now
end
if now > ts then
  tokens = math.min(cap, tokens + (now - ts) * rate)
end
local ok = 0
local wait = 0
if tokens >= 1 then
  tokens = tokens - 1
  ok = 1
elseif rate > 0 then
  wait = (1 - tokens) / rate
else
  wait = 3600
end
redis.call('HSET', KEYS[1], 't', tostring(tokens), 's', tostring(now))
redis.call('EXPIRE', KEYS[1], tonumber(ARGV[3]))
return {ok, tostring(wait)}
`)

// Take — bitta token.
func (b *RedisRateBackend) Take(ctx context.Context, key string, perSecond, capacity float64) (bool, time.Duration, error) {
	// Chelak to'lishi uchun kerak bo'lgan vaqt + zaxira; kamida 1 daqiqa,
	// ko'pi bilan 1 kun.
	ttl := 60.0
	if perSecond > 0 {
		ttl = math.Ceil(capacity/perSecond) + 60
	}
	ttl = math.Min(math.Max(ttl, 60), 86400)
	res, err := tokenBucket.Run(ctx, b.rdb, []string{"rl:" + key},
		strconv.FormatFloat(perSecond, 'f', -1, 64), strconv.FormatFloat(capacity, 'f', -1, 64), int64(ttl)).Slice()
	if err != nil {
		return false, 0, err
	}
	if len(res) != 2 {
		return false, 0, errors.New("ratelimit: Redis javobi noto'g'ri")
	}
	ok, _ := res[0].(int64)
	ws, _ := res[1].(string)
	wait, _ := strconv.ParseFloat(ws, 64)
	return ok == 1, time.Duration(wait * float64(time.Second)), nil
}

// ── Xato urinishlar hisobi (identity.GuardStore) ─────────────────────────

// RedisGuardStore — akkaunt bo'yicha xato urinishlar (TOTP, parol, kod):
// `fg:<kalit>:n` — xatolar (oyna TTL, birinchi xatodan), `fg:<kalit>:i` —
// bajarilayotgan urinishlar (jarayon yiqilsa 2 daqiqada o'zi tozalanadi).
type RedisGuardStore struct{ rdb redis.UniversalClient }

// NewRedisGuardStore — rdb nil bo'lsa nil.
func NewRedisGuardStore(rdb redis.UniversalClient) *RedisGuardStore {
	if rdb == nil {
		return nil
	}
	return &RedisGuardStore{rdb: rdb}
}

const inflightTTL = 120 // soniya

func guardKeys(key string) []string { return []string{"fg:" + key + ":n", "fg:" + key + ":i"} }

var guardBegin = redis.NewScript(`
local n = tonumber(redis.call('GET', KEYS[1]) or '0')
local i = tonumber(redis.call('GET', KEYS[2]) or '0')
if n + i >= tonumber(ARGV[1]) then return 0 end
redis.call('INCR', KEYS[2])
redis.call('EXPIRE', KEYS[2], tonumber(ARGV[2]))
return 1
`)

// ARGV[1]: "ok" | "fail" | "abort" | "fail-only" (Begin'siz Fail); ARGV[2]: oyna (s).
var guardFinish = redis.NewScript(`
if ARGV[1] ~= 'fail-only' then
  local i = tonumber(redis.call('GET', KEYS[2]) or '0')
  if i > 0 then redis.call('DECR', KEYS[2]) end
end
if ARGV[1] == 'ok' then
  redis.call('DEL', KEYS[1])
elseif ARGV[1] == 'fail' or ARGV[1] == 'fail-only' then
  local n = redis.call('INCR', KEYS[1])
  if n == 1 then redis.call('EXPIRE', KEYS[1], tonumber(ARGV[2])) end
end
return 1
`)

// Begin — `max` ga yetmagan bo'lsa urinishni boshlaydi (atomik).
func (s *RedisGuardStore) Begin(ctx context.Context, key string, max int, _ time.Duration) (bool, error) {
	n, err := guardBegin.Run(ctx, s.rdb, guardKeys(key), max, inflightTTL).Int()
	return n == 1, err
}

// Finish — `Begin` natijasi: "ok", "fail", "abort".
func (s *RedisGuardStore) Finish(ctx context.Context, key, result string, window time.Duration) error {
	return guardFinish.Run(ctx, s.rdb, guardKeys(key), result, int64(window/time.Second)).Err()
}

// Fail — `Begin`'siz xato.
func (s *RedisGuardStore) Fail(ctx context.Context, key string, window time.Duration) error {
	return guardFinish.Run(ctx, s.rdb, guardKeys(key), "fail-only", int64(window/time.Second)).Err()
}

// Reset — muvaffaqiyat (`Begin`'siz yo'llar).
func (s *RedisGuardStore) Reset(ctx context.Context, key string) error {
	return s.rdb.Del(ctx, guardKeys(key)[0]).Err()
}

// Count — xatolar + bajarilayotgan urinishlar.
func (s *RedisGuardStore) Count(ctx context.Context, key string) (int, error) {
	vals, err := s.rdb.MGet(ctx, guardKeys(key)...).Result()
	if err != nil {
		return 0, err
	}
	total := 0
	for _, v := range vals {
		if sv, ok := v.(string); ok {
			n, _ := strconv.Atoi(sv)
			total += n
		}
	}
	return total, nil
}
