package safego

import (
	"sync"
	"testing"
	"time"
)

// `safego` testlari (bug.md 44-band).
//
// ┌─ NEGA BU PAKET SINALADI ───────────────────────────────────────────┐
// Butun tuzatishning ma'nosi bitta da'voda: goroutine ichidagi panic
// jarayonni O'LDIRMAYDI. Agar `recover()` noto'g'ri joyda tursa
// (masalan `defer` ichida emas), kod kompilyatsiya bo'ladi-yu himoya
// ishlamaydi — va buni faqat production'da, server qulaganda bilib
// olardik.
//
// Test panic bo'lsa YIQILADI (panic test jarayonini o'ldiradi), ya'ni
// muvaffaqiyatli tugash — himoyaning isboti.
// └────────────────────────────────────────────────────────────────────┘

func TestGoRecoversPanic(t *testing.T) {
	done := make(chan struct{})
	Go("sinov", func() {
		defer close(done)
		panic("ataylab")
	})

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("goroutine tugamadi")
	}
	// Bu yergacha yetish — panic jarayonni o'ldirmaganining isboti.
}

// Nil-pointer kabi RUNTIME panic ham ushlanishi kerak (faqat
// `panic()` chaqiruvi emas): amaldagi nosozliklar aynan shunday
// bo'ladi.
func TestGoRecoversRuntimePanic(t *testing.T) {
	done := make(chan struct{})
	Go("nil-pointer", func() {
		defer close(done)
		var m map[string]int
		m["yozib bo'lmaydi"] = 1 //nolint:staticcheck // ataylab: nil map panikasi (runtime xato) sinaladi
	})

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("goroutine tugamadi")
	}
}

// Panic BO'LMAGANDA funksiya oddiy bajarilishi kerak.
func TestGoRunsNormalFunction(t *testing.T) {
	var mu sync.Mutex
	got := 0
	done := make(chan struct{})
	Go("oddiy", func() {
		mu.Lock()
		got = 42
		mu.Unlock()
		close(done)
	})

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("goroutine tugamadi")
	}
	mu.Lock()
	defer mu.Unlock()
	if got != 42 {
		t.Fatalf("funksiya bajarilmadi: got=%d", got)
	}
}

// `Run` — sinxron variant: joriy goroutine'da ishlaydi va panic'ni
// baribir ushlaydi. Chaqiruvchi undan KEYIN davom etadi.
func TestRunIsSynchronousAndRecovers(t *testing.T) {
	// `Run` panic'ni o'tkazib yuborsa, test AYNAN shu qatorda o'ladi.
	// Pastdagi kodgacha yetish — himoyaning isboti.
	Run("sinxron", func() { panic("ataylab") })

	// Sinxronligi: `Run` qaytgach ish TUGAGAN bo'lishi kerak
	// (goroutine ochilmaydi).
	bajarildi := false
	Run("sinxron-2", func() { bajarildi = true })
	if !bajarildi {
		t.Fatal("Run funksiyani sinxron bajarmadi")
	}
}

// `Run` dan keyin chaqiruvchining `defer` lari ishlashi kerak —
// `routes_assistant_live.go` aynan shunga tayanadi (`wg.Done()` va
// `cancel()` panic bo'lganda ham bajarilishi SHART).
func TestRunLetsCallerDefersRun(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		Run("panic bilan", func() { panic("ataylab") })
	}()

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("wg.Done() bajarilmadi — chaqiruvchi abadiy osilib qolardi")
	}
}
