package storage

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR berilmagan — Redis testi o'tkazib yuborildi")
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

// Token-bucket: sig'im bir zumda, keyin rad; ikki "nusxa" bitta hisobni ko'radi.
func TestRedisRateBackendSharedBucket(t *testing.T) {
	rdb := testRedis(t)
	ctx := context.Background()
	key := fmt.Sprintf("test:%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = rdb.Del(ctx, "rl:"+key).Err() })
	a, b := NewRedisRateBackend(rdb), NewRedisRateBackend(rdb)
	var allowed atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			be := a
			if i%2 == 1 {
				be = b
			}
			ok, _, err := be.Take(ctx, key, 0.001, 5)
			if err != nil {
				t.Error(err)
			}
			if ok {
				allowed.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if n := allowed.Load(); n != 5 {
		t.Fatalf("XAVFSIZLIK: 50 parallel so'rovdan %d tasi o'tdi, 5 kutilgan", n)
	}
	ok, wait, err := a.Take(ctx, key, 0.001, 5)
	if ok || err != nil || wait < 500*time.Second {
		t.Fatalf("rad etilgan so'rov kutish vaqti noto'g'ri: %v %v %v", ok, wait, err)
	}
	if ttl := rdb.TTL(ctx, "rl:"+key).Val(); ttl <= 0 {
		t.Fatalf("kalitda TTL yo'q: %v", ttl)
	}
}

// Xato urinishlar hisobi: 200 parallel Begin — faqat max tasi o'tadi;
// muvaffaqiyat hisobni tozalaydi; Begin'siz Fail ham sanaladi.
func TestRedisGuardStore(t *testing.T) {
	rdb := testRedis(t)
	ctx := context.Background()
	s := NewRedisGuardStore(rdb)
	key := fmt.Sprintf("u-%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = rdb.Del(ctx, guardKeys(key)...).Err() })

	var began atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, err := s.Begin(ctx, key, 5, time.Minute); ok && err == nil {
				began.Add(1)
			}
		}()
	}
	wg.Wait()
	if n := began.Load(); n != 5 {
		t.Fatalf("XAVFSIZLIK: %d ta parallel urinish boshlandi, 5 kutilgan", n)
	}
	for i := 0; i < 5; i++ {
		if err := s.Finish(ctx, key, "fail", time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if n, _ := s.Count(ctx, key); n != 5 {
		t.Fatalf("hisob %d, 5 kutilgan", n)
	}
	if ok, _ := s.Begin(ctx, key, 5, time.Minute); ok {
		t.Fatal("XAVFSIZLIK: 5 xatodan keyin urinish boshlandi")
	}
	if ttl := rdb.TTL(ctx, guardKeys(key)[0]).Val(); ttl <= 0 || ttl > time.Minute {
		t.Fatalf("xatolar oynasi TTL noto'g'ri: %v", ttl)
	}
	if err := s.Reset(ctx, key); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.Begin(ctx, key, 5, time.Minute); !ok {
		t.Fatal("Reset'dan keyin urinish boshlanmadi")
	}
	_ = s.Finish(ctx, key, "ok", time.Minute)
	if n, _ := s.Count(ctx, key); n != 0 {
		t.Fatalf("muvaffaqiyatdan keyin hisob %d", n)
	}
	_ = s.Fail(ctx, key, time.Minute)
	if n, _ := s.Count(ctx, key); n != 1 {
		t.Fatalf("Begin'siz Fail sanalmadi: %d", n)
	}
}
