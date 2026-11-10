package users

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Sutkalik chegara: kod terib topishga qarshi (SSO auditi).
func TestCodeGuard_DailyLimits(t *testing.T) {
	s := newTestService()
	ctx := context.Background()
	clock := time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return clock }
	const phone = "+998901112299"

	// 10 ta kod — keyingisi rad (oraliq 60 s cheklovidan o'tib).
	for i := 0; i < codesPerDay; i++ {
		if _, _, err := s.IssueCode(ctx, phone); err != nil {
			t.Fatalf("%d-kod: %v", i+1, err)
		}
		clock = clock.Add(61 * time.Second)
	}
	if _, _, err := s.IssueCode(ctx, phone); !errors.Is(err, ErrTooManyCodes) {
		t.Fatalf("11-kod berildi: %v", err)
	}

	// Ertaga — yana mumkin; 20 ta xato taxmindan keyin TO'G'RI kod ham o'tmaydi.
	clock = clock.Add(25 * time.Hour)
	for fails := 0; fails < failuresPerDay; {
		_, code, err := s.IssueCode(ctx, phone)
		if err != nil {
			t.Fatalf("kod: %v", err)
		}
		for j := 0; j < 4 && fails < failuresPerDay; j++ {
			wrong := "000000"
			if code == wrong {
				wrong = "111111"
			}
			if _, err := s.VerifyPhoneCode(ctx, phone, wrong); !errors.Is(err, ErrInvalidCode) {
				t.Fatalf("xato kod: %v", err)
			}
			fails++
		}
		clock = clock.Add(61 * time.Second)
	}
	_, code, err := s.IssueCode(ctx, phone)
	if err != nil {
		t.Fatalf("kod: %v", err)
	}
	if _, err := s.VerifyPhoneCode(ctx, phone, code); !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("XAVFSIZLIK: chegaradan keyin kod qabul qilindi: %v", err)
	}
	if !IsUserFacing(ErrTooManyCodes) {
		t.Fatal("ErrTooManyCodes foydalanuvchiga ko'rinadigan bo'lishi kerak")
	}
}
