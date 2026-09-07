package users

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// Bu fayl bitta hodisani qo'riqlaydi: desktop brauzerdan "Telegram
// bilan kirish" bosilganda bot "Hozir kod yuborib bo'lmadi" deb
// javob berardi. Sabab kod yaratishda emas edi — kod yaratilib
// saqlangan, keyin `RequestCode` MAJBURAN SMS yuborgan va Eskiz
// yiqilgani uchun butun funksiya xato qaytargan. Ya'ni ishlaydigan
// kanal (Telegram) ishlamaydigan kanal (SMS) tufayli o'lgan.

// errSms — har doim yiqiladigan SMS kanali: Eskiz sozlanmagan yoki
// jo'natuvchi nomi tasdiqlanmagan holat.
type errSms struct{ calls atomic.Int64 }

func (e *errSms) Send(phone, text string) error {
	e.calls.Add(1)
	return errors.New("eskiz: jo'natuvchi nomi tasdiqlanmagan")
}

func newServiceWithSms(sms SmsSender) *Service {
	var n atomic.Int64
	return NewService(
		&fakeUserRepo{data: make(map[string]*User)},
		&fakeCodeStore{data: make(map[string]*Code)},
		sms,
		NewTokenIssuer("test-secret", time.Hour),
		func() string { return "id" + string(rune('0'+n.Add(1))) },
	).WithEmail(&noopEmail{}, true)
}

// SMS butunlay ishlamasa ham Telegram orqali kirish ishlashi SHART.
func TestIssueCodeWorksWhenSmsIsDown(t *testing.T) {
	sms := &errSms{}
	s := newServiceWithSms(sms)
	ctx := context.Background()

	phone, code, err := s.IssueCode(ctx, "+998901234567")
	if err != nil {
		t.Fatalf("SMS ishlamasa ham kod berilishi kerak edi: %v", err)
	}
	if got := sms.calls.Load(); got != 0 {
		t.Errorf("IssueCode SMS yubormasligi kerak, %d marta chaqirildi", got)
	}

	// Kod haqiqatan yaroqli — ya'ni bot uni chatga yozsa kirish tugaydi.
	if _, _, err := s.Verify(ctx, phone, code); err != nil {
		t.Fatalf("IssueCode bergan kod yaroqli bo'lishi kerak: %v", err)
	}
}

// SMS yo'li esa AVVALGIDEK yiqilishi kerak: kod ketmagan bo'lsa
// foydalanuvchiga "yuborildi" deb aytilmasin.
func TestRequestCodeStillFailsWhenSmsIsDown(t *testing.T) {
	sms := &errSms{}
	s := newServiceWithSms(sms)

	if _, _, err := s.RequestCode(context.Background(), "+998901234567"); err == nil {
		t.Fatal("SMS yiqilganda RequestCode xato qaytarishi kerak")
	}
	if got := sms.calls.Load(); got != 1 {
		t.Errorf("RequestCode SMS yuborishga urinishi kerak, chaqiruvlar: %d", got)
	}
}

// SMS ishlayotganda RequestCode avvalgidek kodni yetkazadi.
func TestRequestCodeSendsSmsWhenHealthy(t *testing.T) {
	sms := &countingSms{}
	s := newServiceWithSms(sms)
	ctx := context.Background()

	phone, code, err := s.RequestCode(ctx, "+998901234567")
	if err != nil {
		t.Fatalf("RequestCode: %v", err)
	}
	if got := sms.calls.Load(); got != 1 {
		t.Errorf("aynan bitta SMS kutilgandi, olindi %d", got)
	}
	if _, _, err := s.Verify(ctx, phone, code); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

type countingSms struct{ calls atomic.Int64 }

func (c *countingSms) Send(phone, text string) error {
	c.calls.Add(1)
	return nil
}
