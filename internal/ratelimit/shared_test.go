package ratelimit

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeBackend struct {
	mu    sync.Mutex
	keys  map[string]int
	fail  bool
	calls int
}

func (f *fakeBackend) Take(_ context.Context, key string, _, capacity float64) (bool, time.Duration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.fail {
		return false, 0, errors.New("redis o'chgan")
	}
	if f.keys == nil {
		f.keys = map[string]int{}
	}
	if float64(f.keys[key]) >= capacity {
		return false, 2500 * time.Millisecond, nil
	}
	f.keys[key]++
	return true, 0, nil
}

func TestSharedUsesBackendAndHashesKeys(t *testing.T) {
	fb := &fakeBackend{}
	UseBackend(fb)
	t.Cleanup(func() { UseBackend(nil) })
	a := New(1, 2).Shared("testShared1")
	b := New(1, 2).Shared("testShared1") // ikkinchi "nusxa"
	if !a.Allow("+998901112233") || !b.Allow("+998901112233") {
		t.Fatal("birinchi ikki so'rov o'tishi kerak")
	}
	ok, wait := a.AllowWithWait("+998901112233")
	if ok || wait != 3*time.Second {
		t.Fatalf("umumiy hisob ishlamadi: %v %v", ok, wait)
	}
	for k := range fb.keys {
		if len(k) > 64 || containsDigits(k, "998901112233") {
			t.Fatalf("XAVFSIZLIK: telefon Redis kalitida ochiq: %q", k)
		}
	}
}

func containsDigits(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Backend o'chsa — lokal cheklov (fail-open EMAS).
func TestSharedFallsBackToLocal(t *testing.T) {
	fb := &fakeBackend{fail: true}
	UseBackend(fb)
	t.Cleanup(func() { UseBackend(nil) })
	l := New(0.001, 3).Shared("testShared2")
	for i := 0; i < 3; i++ {
		if !l.Allow("ip") {
			t.Fatalf("%d-so'rov rad etildi", i)
		}
	}
	if l.Allow("ip") {
		t.Fatal("XAVFSIZLIK: backend o'chganda cheklov yo'qoldi")
	}
	if fb.calls == 0 {
		t.Fatal("backend chaqirilmadi")
	}
}

func TestUnnamedLimiterStaysLocal(t *testing.T) {
	fb := &fakeBackend{}
	UseBackend(fb)
	t.Cleanup(func() { UseBackend(nil) })
	l := New(1, 1)
	l.Allow("x")
	if fb.calls != 0 {
		t.Fatal("nomsiz limiter backend'ga bordi")
	}
}

func TestResetAllClearsSharedBuckets(t *testing.T) {
	l := New(0.001, 1).Shared("testShared3")
	l.Allow("k")
	if l.Allow("k") {
		t.Fatal("cheklov ishlamadi")
	}
	ResetAll()
	if !l.Allow("k") {
		t.Fatal("ResetAll chelakni tozalamadi")
	}
}

func TestSharedConflictingLimitsPanics(t *testing.T) {
	New(1, 5).Shared("testShared4")
	defer func() {
		if recover() == nil {
			t.Fatal("bir nom — ikki xil chegara qabul qilindi")
		}
	}()
	New(2, 5).Shared("testShared4")
}
