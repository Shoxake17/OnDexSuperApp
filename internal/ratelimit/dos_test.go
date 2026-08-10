package ratelimit

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// Bu fayldagi testlar cheklovchining O'ZI DoS quroliga aylanishiga
// qarshi (limiter.go boshidagi izohga qarang).

// Xotira chegarasi: hujumchi cheksiz kalit yarata olmasligi kerak.
func TestBucketsAreBounded(t *testing.T) {
	l := New(1, 5)
	for i := 0; i < maxBuckets+5_000; i++ {
		l.Allow(fmt.Sprintf("10.0.%d.%d", i/256%256, i%256) + fmt.Sprint(i))
	}
	if got := l.Len(); got > maxBuckets {
		t.Fatalf("chelaklar soni chegaradan oshdi: %d > %d", got, maxBuckets)
	}
}

// Chegaraga yetganda yangi kalitlar RAD ETILADI (fail-closed), ya'ni
// hujumchi ko'p kalit yaratib cheklovdan qutulib qololmaydi.
func TestOverflowFailsClosed(t *testing.T) {
	l := New(1, 5)
	for i := 0; i < maxBuckets; i++ {
		l.Allow(fmt.Sprint("kalit-", i))
	}
	if ok, wait := l.AllowWithWait("butunlay-yangi-kalit"); ok {
		t.Fatal("chegara to'lganda ham yangi kalit o'tkazildi")
	} else if wait <= 0 {
		t.Fatal("kutish vaqti berilmadi")
	}
}

// ★ ASOSIY TEST: tozalash HAR SO'ROVDA bajarilmasligi kerak.
//
// Avval `sweepLocked` har yangi kalit qo'shilganda butun xaritani
// aylanardi — 1 mln yozuvda har so'rov 1 mln elementni skanerlardi va
// yuk KVADRATIK o'sardi (hammasi bitta mutex ostida). Bu test buni
// vaqt bo'yicha ushlaydi: katta xaritada ham yangi kalitlar qo'shish
// chiziqli qolishi kerak.
func TestSweepIsNotPerRequest(t *testing.T) {
	l := New(1, 5)
	// Xaritani to'ldiramiz (chegaradan sal past).
	const n = 20_000
	for i := 0; i < n; i++ {
		l.Allow(fmt.Sprint("k", i))
	}

	start := time.Now()
	for i := 0; i < 2_000; i++ {
		l.Allow(fmt.Sprint("yangi", i))
	}
	elapsed := time.Since(start)

	// 2000 ta qo'shish O(n) tozalashsiz millisekundlar oladi. Har
	// qo'shishda 20k elementni skanerlash 40 mln amal bo'lardi.
	if elapsed > 2*time.Second {
		t.Fatalf("katta xaritada qo'shish juda sekin (%v) — "+
			"tozalash har so'rovda bajarilyapti", elapsed)
	}
}

// Juda uzun kalit xotirani yeb qo'ymasligi kerak: u hash bilan
// almashtiriladi.
func TestLongKeysAreClamped(t *testing.T) {
	huge := strings.Repeat("A", 1<<20) // 1 MB
	if got := clampKey(huge); len(got) > maxKeyLen {
		t.Fatalf("uzun kalit qisqartirilmadi: %d bayt", len(got))
	}
	// Turli uzun kalitlar TURLICHA qolishi kerak (to'qnashuv bo'lmasin).
	a := clampKey(strings.Repeat("A", 5000))
	b := clampKey(strings.Repeat("B", 5000))
	if a == b {
		t.Fatal("turli uzun kalitlar bitta chelakka tushdi")
	}
}

// Parallel yuk ostida ham holat buzilmasligi kerak (poyga yo'q).
func TestConcurrentAccessIsSafe(t *testing.T) {
	l := New(100, 50)
	var wg sync.WaitGroup
	for w := 0; w < 32; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				l.Allow(fmt.Sprint("ip-", (w*500+i)%1000))
			}
		}(w)
	}
	wg.Wait()
	if l.Len() == 0 {
		t.Fatal("hech qanday chelak yaratilmadi")
	}
}
