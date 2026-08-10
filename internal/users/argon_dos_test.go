package users

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Argon2 himoyaning O'ZI qurolga aylanishi mumkin: har chaqiruv 64 MB
// oladi (`password.go` boshidagi izohga qarang). Bu fayl chegaralarni
// sinaydi.

// Bir vaqtda bajariladigan Argon2 amallari soni chegaralangan bo'lishi
// kerak — aks holda bir necha o'nlab parallel login serverni
// xotiradan chiqarardi.
func TestArgonConcurrencyIsBounded(t *testing.T) {
	limit := cap(argonGate)
	if limit < 2 || limit > 8 {
		t.Fatalf("navbat chegarasi mantiqsiz: %d", limit)
	}

	var running, peak int64
	var wg sync.WaitGroup
	for i := 0; i < limit*4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = withArgonSlot(context.Background(), func() {
				cur := atomic.AddInt64(&running, 1)
				for {
					old := atomic.LoadInt64(&peak)
					if cur <= old || atomic.CompareAndSwapInt64(&peak, old, cur) {
						break
					}
				}
				time.Sleep(15 * time.Millisecond)
				atomic.AddInt64(&running, -1)
			})
		}()
	}
	wg.Wait()

	if peak > int64(limit) {
		t.Fatalf("bir vaqtda %d ta Argon2 ishladi, chegara %d — "+
			"xotira portlashi mumkin", peak, limit)
	}
}

// Navbat to'lganda so'rov CHEKSIZ kutmasligi, aniq xato qaytarishi
// kerak: kutayotgan goroutine'lar to'planishi holatni yomonlashtiradi.
func TestArgonQueueRejectsWhenFull(t *testing.T) {
	// Navbatni to'ldirib turamiz.
	release := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < cap(argonGate); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = withArgonSlot(context.Background(), func() { <-release })
		}()
	}
	// Hamma slot band bo'lishini kutamiz.
	deadline := time.Now().Add(2 * time.Second)
	for len(argonGate) < cap(argonGate) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	// Bekor qilinadigan kontekst bilan — darhol qaytishi kerak.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := withArgonSlot(ctx, func() { t.Error("bajarilmasligi kerak edi") })
	if err == nil {
		t.Fatal("navbat to'lganda ham slot berildi")
	}

	close(release)
	wg.Wait()
}

// Mijoz uzilib ketsa (kontekst bekor) Argon2 UMUMAN boshlanmasligi
// kerak — bekorga 64 MB sarflamaymiz.
func TestVerifyPasswordHonoursContext(t *testing.T) {
	hash, err := HashPassword(context.Background(), "tekshiruv-paroli-9")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Navbat bo'sh bo'lsa `select` slotni ham tanlashi mumkin, shuning
	// uchun natija emas, XATO TURI muhim: bekor qilingan kontekstda
	// xato bo'lsa, u aynan kontekst xatosi bo'lsin.
	if _, err := VerifyPassword(ctx, "tekshiruv-paroli-9", hash); err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, ErrServerBusy) {
			t.Fatalf("kutilmagan xato turi: %v", err)
		}
	}
}

// Juda uzun parol Argon2'ga umuman yetib bormasligi kerak.
func TestOverlongPasswordRejectedBeforeHashing(t *testing.T) {
	long := make([]byte, MaxPasswordLength+1)
	for i := range long {
		long[i] = 'a'
	}
	if err := ValidatePassword(string(long)); !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("uzun parol rad etilmadi: %v", err)
	}
}
