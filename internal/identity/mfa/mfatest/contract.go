// Package mfatest — `mfa.Store` implementatsiyalari uchun umumiy shartnoma.
package mfatest

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"chustapp/internal/identity/mfa"
)

var base = time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

type contract struct {
	ctx      context.Context
	newStore func(t *testing.T) mfa.Store
	userID   string
}

// RunStoreContract — `newStore` har chaqirilganda shu foydalanuvchi uchun
// TOZA ombor qaytarsin. `userID` users jadvalida MAVJUD bo'lishi shart.
func RunStoreContract(t *testing.T, newStore func(t *testing.T) mfa.Store, userID string) {
	c := &contract{ctx: context.Background(), newStore: newStore, userID: userID}
	t.Run("ulash: pending -> confirm, ikkinchi ulash rad", c.enroll)
	t.Run("qadam faqat oldinga; parallel bitta qadam bir marta", c.steps)
	t.Run("tiklash kodi bir martalik", c.recovery)
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func hashes(t *testing.T) [][]byte {
	t.Helper()
	_, h, err := mfa.NewRecoveryCodes()
	must(t, err)
	return h
}

func (c *contract) enroll(t *testing.T) {
	s, uid := c.newStore(t), c.userID
	if _, err := s.Confirmed(c.ctx, uid); !errors.Is(err, mfa.ErrNotFound) {
		t.Fatalf("yangi akkauntda TOTP: %v", err)
	}
	must(t, s.SavePending(c.ctx, &mfa.Factor{ID: "mfat_f1", UserID: uid, SecretEnc: "enc1", CreatedAt: base}))
	// qayta ulashni boshlash eski tasdiqlanmaganni almashtiradi
	must(t, s.SavePending(c.ctx, &mfa.Factor{ID: "mfat_f2", UserID: uid, SecretEnc: "enc2", CreatedAt: base.Add(time.Minute)}))
	p, err := s.Pending(c.ctx, uid)
	must(t, err)
	if p.ID != "mfat_f2" || p.SecretEnc != "enc2" {
		t.Fatalf("pending: %+v", p)
	}
	if err := s.Confirm(c.ctx, "mfat_f1", 100, base, hashes(t)); !errors.Is(err, mfa.ErrNotFound) {
		t.Fatalf("almashtirilgan faktor tasdiqlandi: %v", err)
	}
	must(t, s.Confirm(c.ctx, "mfat_f2", 100, base, hashes(t)))
	f, err := s.Confirmed(c.ctx, uid)
	must(t, err)
	if f.ID != "mfat_f2" || f.LastUsedStep != 100 || f.ConfirmedAt == nil {
		t.Fatalf("confirmed: %+v", f)
	}
	if err := s.SavePending(c.ctx, &mfa.Factor{ID: "mfat_f3", UserID: uid, SecretEnc: "enc3"}); !errors.Is(err, mfa.ErrAlreadyEnrolled) {
		t.Fatalf("tasdiqlangan bor-u yangi ulash: %v", err)
	}
	if err := s.Confirm(c.ctx, "mfat_f2", 101, base, hashes(t)); !errors.Is(err, mfa.ErrNotFound) {
		t.Fatalf("ikki marta tasdiqlash: %v", err)
	}
	if n, _ := s.RecoveryLeft(c.ctx, uid); n != mfa.RecoveryCount {
		t.Fatalf("tiklash kodlari %d", n)
	}
}

func (c *contract) steps(t *testing.T) {
	s := c.newStore(t)
	must(t, s.SavePending(c.ctx, &mfa.Factor{ID: "mfat_s1", UserID: c.userID, SecretEnc: "enc"}))
	must(t, s.Confirm(c.ctx, "mfat_s1", 100, base, hashes(t)))
	for _, step := range []int64{100, 99} {
		if ok, err := s.UseStep(c.ctx, "mfat_s1", step); err != nil || ok {
			t.Fatalf("eski qadam %d qabul qilindi: %v %v", step, ok, err)
		}
	}
	var wins atomic.Int32
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, err := s.UseStep(c.ctx, "mfat_s1", 101); err == nil && ok {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("bitta kod %d marta qabul qilindi", wins.Load())
	}
	if _, err := s.UseStep(c.ctx, "mfat_yoq", 200); !errors.Is(err, mfa.ErrNotFound) {
		t.Fatalf("mavjud bo'lmagan faktor: %v", err)
	}
}

func (c *contract) recovery(t *testing.T) {
	s, uid := c.newStore(t), c.userID
	plain, h, err := mfa.NewRecoveryCodes()
	must(t, err)
	must(t, s.SavePending(c.ctx, &mfa.Factor{ID: "mfat_r1", UserID: uid, SecretEnc: "enc"}))
	must(t, s.Confirm(c.ctx, "mfat_r1", 1, base, h))
	code := plain[3]
	if ok, err := s.UseRecovery(c.ctx, uid, mfa.HashRecovery(code), base); err != nil || !ok {
		t.Fatalf("to'g'ri tiklash kodi: %v %v", ok, err)
	}
	if ok, _ := s.UseRecovery(c.ctx, uid, mfa.HashRecovery(code), base); ok {
		t.Fatal("tiklash kodi ikki marta ishladi")
	}
	if ok, _ := s.UseRecovery(c.ctx, uid, mfa.HashRecovery("AAAAA-BBBBB"), base); ok {
		t.Fatal("soxta kod qabul qilindi")
	}
	if n, _ := s.RecoveryLeft(c.ctx, uid); n != mfa.RecoveryCount-1 {
		t.Fatalf("qolgan kodlar %d", n)
	}
}
