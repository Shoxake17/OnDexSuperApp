package ratelimit

import (
	"sync"
	"testing"
)

func TestAllowsBurstThenBlocks(t *testing.T) {
	// 3 ta portlash, keyin sekundiga 1 ta.
	l := New(1, 3)
	for i := 0; i < 3; i++ {
		if !l.Allow("a") {
			t.Fatalf("%d-so'rov ruxsat etilishi kerak edi", i+1)
		}
	}
	if l.Allow("a") {
		t.Error("4-so'rov RAD ETILISHI kerak edi (chelak bo'sh)")
	}
}

func TestKeysAreIndependent(t *testing.T) {
	l := New(1, 1)
	if !l.Allow("a") {
		t.Fatal("a: birinchi so'rov o'tishi kerak")
	}
	if l.Allow("a") {
		t.Error("a: ikkinchi so'rov rad etilishi kerak")
	}
	// Boshqa kalit boshqa akkaunt/IP — ta'sirlanmasligi shart.
	if !l.Allow("b") {
		t.Error("b: mustaqil kalit o'tishi kerak edi")
	}
}

// Parallel chaqiruvlarda umumiy chegara oshib ketmasligi kerak —
// aks holda cheklov ko'p goroutine ostida ma'nosiz bo'lardi.
func TestConcurrentDoesNotExceedCapacity(t *testing.T) {
	const capacity = 50
	l := New(0, capacity) // to'ldirilmaydi: umumiy ruxsat aynan capacity
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l.Allow("shared") {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if allowed > capacity {
		t.Errorf("ruxsat etilgan: %d, maksimal: %d", allowed, capacity)
	}
}
