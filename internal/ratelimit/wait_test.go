package ratelimit

import (
	"testing"
	"time"
)

// Kutish vaqti FOYDALANUVCHIGA ko'rsatiladi. Noto'g'ri hisoblansa,
// odam aytilgan vaqtdan keyin urinib yana rad javobini oladi va
// ilovani buzuq deb o'ylaydi — ya'ni bu xato jimgina emas, ko'zga
// tashlanadigan bo'ladi.

func TestAllowWithWaitReportsRemaining(t *testing.T) {
	// Sekundiga 0.05 token => bitta token 20 soniyada.
	l := New(0.05, 3)
	for i := 0; i < 3; i++ {
		if ok, _ := l.AllowWithWait("k"); !ok {
			t.Fatalf("%d-urinish rad etildi, portlash 3 edi", i+1)
		}
	}
	ok, wait := l.AllowWithWait("k")
	if ok {
		t.Fatal("chelak bo'shagandan keyin ham ruxsat berildi")
	}
	// Chelak endigina bo'shadi => to'liq bitta token kerak => ~20s.
	if wait < 19*time.Second || wait > 21*time.Second {
		t.Fatalf("kutish vaqti noto'g'ri: %v (kutilgan ~20s)", wait)
	}
}

// Vaqt o'tgani sayin kutish vaqti KAMAYISHI kerak.
func TestWaitShrinksOverTime(t *testing.T) {
	l := New(0.05, 1)
	l.AllowWithWait("k") // yagona tokenni sarflaymiz

	_, first := l.AllowWithWait("k")
	time.Sleep(1200 * time.Millisecond)
	_, second := l.AllowWithWait("k")

	if second >= first {
		t.Fatalf("kutish vaqti kamaymadi: %v -> %v", first, second)
	}
}

// Ruxsat berilganda kutish vaqti bo'lmasligi kerak.
func TestNoWaitWhenAllowed(t *testing.T) {
	l := New(1, 5)
	if ok, wait := l.AllowWithWait("k"); !ok || wait != 0 {
		t.Fatalf("ruxsat berilganda kutish qaytdi: ok=%v wait=%v", ok, wait)
	}
}

// Juda kichik qoldiq "0 soniya kuting" bo'lib ko'rinmasligi kerak —
// bu foydalanuvchi uchun ma'nosiz javob bo'lardi.
func TestWaitNeverZeroWhenDenied(t *testing.T) {
	l := New(100, 1) // juda tez tiklanadi
	l.AllowWithWait("k")
	if ok, wait := l.AllowWithWait("k"); !ok && wait < time.Second {
		t.Fatalf("rad etilganda 1 soniyadan kam kutish qaytdi: %v", wait)
	}
}

// `Allow` eski xatti-harakatini saqlab qolishi kerak (u ko'p joyda
// ishlatiladi).
func TestAllowStillWorks(t *testing.T) {
	l := New(0.05, 2)
	first, second := l.Allow("k"), l.Allow("k")
	if !first || !second {
		t.Fatal("portlash ichidagi urinishlar rad etildi")
	}
	if l.Allow("k") {
		t.Fatal("chelak bo'shagandan keyin ruxsat berildi")
	}
}
