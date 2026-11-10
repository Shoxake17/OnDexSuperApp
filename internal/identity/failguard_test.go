package identity

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// AUD-14: parallel urinishlar chegaradan oshmaydi (TOCTOU).
func TestFailGuardParallelAttemptsCapped(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	g := newFailGuard(5, 15*time.Minute, func() time.Time { return now })
	var started atomic.Int32
	var wg sync.WaitGroup
	gate := make(chan struct{})
	for range 200 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			end, ok := g.Begin("u1")
			if !ok {
				return
			}
			started.Add(1)
			<-gate // hammasi bir vaqtda "tekshirilmoqda"
			end(attemptFail)
		}()
	}
	// Barcha goroutine Begin'dan o'tguncha kutamiz.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		g.mu.Lock()
		e := g.byKey["u1"]
		done := e != nil && e.inflight == 5
		g.mu.Unlock()
		if done {
			break
		}
		time.Sleep(time.Millisecond)
	}
	close(gate)
	wg.Wait()
	if n := started.Load(); n != 5 {
		t.Fatalf("XAVFSIZLIK: %d ta parallel urinish o'tdi, 5 kutilgan", n)
	}
	if _, ok := g.Begin("u1"); ok {
		t.Fatal("5 xatodan keyin blok yo'q")
	}
	if !g.Blocked("u1") {
		t.Fatal("Blocked noto'g'ri")
	}
	now = now.Add(15 * time.Minute)
	end, ok := g.Begin("u1")
	if !ok {
		t.Fatal("oyna o'tgach blok yechilmadi")
	}
	end(attemptOK)
}

func TestFailGuardOutcomes(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	g := newFailGuard(3, time.Minute, func() time.Time { return now })
	for range 10 { // tekshiruvgacha yetmagan urinishlar sanalmaydi
		end, ok := g.Begin("k")
		if !ok {
			t.Fatal("abort hisoblandi")
		}
		end(attemptAbort)
		end(attemptFail) // ikkinchi chaqiruv e'tiborsiz
	}
	if len(g.byKey) != 0 {
		t.Fatalf("yozuv qoldi: %+v", g.byKey["k"])
	}
	for range 2 {
		end, _ := g.Begin("k")
		end(attemptFail)
	}
	end, _ := g.Begin("k")
	end(attemptOK) // muvaffaqiyat tozalaydi
	for range 2 {
		end, ok := g.Begin("k")
		if !ok {
			t.Fatal("muvaffaqiyatdan keyin hisob tozalanmadi")
		}
		end(attemptFail)
	}
	g.Fail("k")
	if !g.Blocked("k") {
		t.Fatal("Fail hisoblanmadi")
	}
}
