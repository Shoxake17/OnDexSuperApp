package ttlcache

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCacheHitAndExpiry(t *testing.T) {
	c := New[int](time.Second, 10)
	now := time.Unix(1000, 0)
	c.now = func() time.Time { return now }

	var loads int
	load := func() (int, error) { loads++; return loads, nil }

	for i := 0; i < 5; i++ {
		if v, _ := c.Get("k", load); v != 1 {
			t.Fatalf("keshdan %d, 1 kutilgan", v)
		}
	}
	if loads != 1 {
		t.Fatalf("yuklash %d marta, 1 kutilgan", loads)
	}
	now = now.Add(1500 * time.Millisecond) // TTL o'tdi
	if v, _ := c.Get("k", load); v != 2 || loads != 2 {
		t.Fatalf("TTL'dan keyin qayta yuklash kerak: v=%d loads=%d", v, loads)
	}
}

func TestCacheDoesNotCacheErrors(t *testing.T) {
	c := New[int](time.Minute, 10)
	boom := errors.New("topilmadi")
	var loads int
	for i := 0; i < 3; i++ {
		if _, err := c.Get("k", func() (int, error) { loads++; return 0, boom }); !errors.Is(err, boom) {
			t.Fatalf("xato kutilgan: %v", err)
		}
	}
	if loads != 3 {
		t.Fatalf("xato keshlangan: yuklash %d marta, 3 kutilgan", loads)
	}
	// Keyin yozuv paydo bo'lsa DARHOL ko'rinadi.
	if v, err := c.Get("k", func() (int, error) { return 7, nil }); err != nil || v != 7 {
		t.Fatalf("yangi qiymat: %v %v", v, err)
	}
}

func TestCacheInvalidateReloads(t *testing.T) {
	c := New[string](time.Minute, 10)
	val := "eski"
	get := func() string { v, _ := c.Get("k", func() (string, error) { return val, nil }); return v }
	if get() != "eski" {
		t.Fatal("boshlang'ich")
	}
	val = "yangi"
	if get() != "eski" {
		t.Fatal("kesh ichida eski qiymat kutilgan (bekor qilinmagan)")
	}
	c.Invalidate("k")
	if get() != "yangi" {
		t.Fatal("bekor qilingach yangi qiymat kutilgan")
	}
}

// POYGA: o'qish boshlangan, KEYIN bekor qilingan — eskirgan natija keshga yozilmaydi.
func TestCacheInFlightResultDiscardedAfterInvalidate(t *testing.T) {
	c := New[string](time.Minute, 10)
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan string)

	go func() {
		v, _ := c.Get("k", func() (string, error) {
			close(started)
			<-release // "eski" qiymatni o'qib, kutib turadi
			return "eski", nil
		})
		done <- v
	}()
	<-started
	c.Invalidate("k") // shu orada yozuv bo'ldi
	close(release)
	if v := <-done; v != "eski" {
		t.Fatalf("o'qigan chaqiruvchi o'z natijasini olishi kerak: %q", v)
	}
	// Keshda eski qiymat QOLMAGAN: keyingi o'qish yangisini yuklaydi.
	v, _ := c.Get("k", func() (string, error) { return "yangi", nil })
	if v != "yangi" {
		t.Fatalf("eskirgan natija keshga yozilgan: %q", v)
	}
}

// Bekor qilingandan KEYIN kelgan chaqiruvchi eski (uchib ketayotgan) yuklashga qo'shilmaydi.
func TestCacheNewArrivalAfterInvalidateDoesNotJoinOldFlight(t *testing.T) {
	c := New[string](time.Minute, 10)
	started := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_, _ = c.Get("k", func() (string, error) { close(started); <-release; return "eski", nil })
	}()
	<-started
	c.Invalidate("k")
	v, _ := c.Get("k", func() (string, error) { return "yangi", nil })
	close(release)
	if v != "yangi" {
		t.Fatalf("bekor qilingandan keyingi chaqiruv eski yuklashga qo'shildi: %q", v)
	}
}

func TestCacheSingleflight(t *testing.T) {
	c := New[int](time.Minute, 10)
	var loads atomic.Int32
	gate := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, _ := c.Get("k", func() (int, error) {
				loads.Add(1)
				<-gate
				return 42, nil
			})
			if v != 42 {
				t.Errorf("v=%d", v)
			}
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(gate)
	wg.Wait()
	if n := loads.Load(); n != 1 {
		t.Fatalf("bir vaqtdagi 100 so'rov uchun %d ta yuklash, 1 kutilgan", n)
	}
}

func TestCacheBoundedSize(t *testing.T) {
	c := New[int](time.Minute, 3)
	for i := 0; i < 10; i++ {
		k := string(rune('a' + i))
		_, _ = c.Get(k, func() (int, error) { return i, nil })
		c.mu.Lock()
		n := len(c.data)
		c.mu.Unlock()
		if n > 3 {
			t.Fatalf("kesh chegaradan oshdi: %d", n)
		}
	}
}
