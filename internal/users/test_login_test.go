package users

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Bu testlar `test_login.go` dagi xavfsizlik chegaralarini BUZISHGA
// urinadi: sobit kod faqat bayroq yoqilganda va faqat kuryer/affitsiant
// akkauntini ochishi kerak.

// testLoginService — SMS kanali o'chirilgan (production holati) servis va
// test raqamidagi akkaunt. `role` bo'sh bo'lsa akkaunt yaratilmaydi.
func testLoginService(t *testing.T, role Role, enable bool) (*Service, *fakeUserRepo, *countingSms) {
	t.Helper()
	sms := &countingSms{}
	s := newServiceWithSms(sms).WithoutSms()
	repo := s.users.(*fakeUserRepo)
	if role != "" {
		_ = repo.Create(context.Background(), &User{
			ID: "test-user", Phone: TestLoginPhone, Role: role,
			EntityID: "staff-1", PhoneVerified: true, CreatedAt: time.Now(),
		})
	}
	if enable {
		var err error
		if s, err = s.WithTestLogin(TestLoginPhone, TestLoginCode); err != nil {
			t.Fatal(err)
		}
	}
	return s, repo, sms
}

// Asosiy oqim: OnDexGO (kuryer) va OnDexPro (affitsiant) sharhlovchisi
// SMS'siz, Telegram'siz kiradi.
func TestTestLoginOpensStaffAccounts(t *testing.T) {
	for _, role := range []Role{RoleCourier, RoleWaiter} {
		t.Run(string(role), func(t *testing.T) {
			s, _, sms := testLoginService(t, role, true)
			ctx := context.Background()

			if !s.TestLoginActive(ctx, "+998 99 999-99-99") {
				t.Fatal("test raqami (har qanday yozilishda) faol bo'lishi kerak edi")
			}
			phone, code, err := s.RequestCode(ctx, TestLoginPhone)
			if err != nil {
				t.Fatalf("SMS o'chiq bo'lsa ham test kodi berilishi kerak: %v", err)
			}
			if code != TestLoginCode {
				t.Fatalf("kod = %q, kutilgan sobit %q", code, TestLoginCode)
			}
			if got := sms.calls.Load(); got != 0 {
				t.Errorf("test raqamiga SMS yuborilmasligi kerak, chaqiruvlar: %d", got)
			}
			token, u, err := s.Verify(ctx, phone, TestLoginCode)
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if token == "" || u.Role != role {
				t.Fatalf("token=%q rol=%q, kutilgan rol %q", token, u.Role, role)
			}
			// Bir martalik: o'sha kod ikkinchi marta o'tmaydi.
			if _, _, err := s.Verify(ctx, phone, TestLoginCode); !errors.Is(err, ErrInvalidCode) {
				t.Fatalf("kod qayta ishlatildi: %v", err)
			}
		})
	}
}

// "+998 99" — haqiqiy operator kodi: raqam tirik odamniki bo'lishi
// mumkin. Uning mijoz akkaunti (hamyon!), restoran yoki admin akkaunti
// sobit kod bilan HECH QACHON ochilmasligi kerak.
func TestTestLoginNeverOpensPrivilegedOrCustomerAccounts(t *testing.T) {
	for _, role := range []Role{RoleCustomer, RoleRestaurant, RoleAdmin} {
		t.Run(string(role), func(t *testing.T) {
			s, _, _ := testLoginService(t, role, true)
			ctx := context.Background()

			if s.TestLoginActive(ctx, TestLoginPhone) {
				t.Fatal("bu rol uchun test kirishi faol bo'lmasligi kerak")
			}
			// Oddiy raqamdek: SMS o'chiq — kod umuman berilmaydi.
			if _, _, err := s.RequestCode(ctx, TestLoginPhone); !errors.Is(err, ErrSmsSendUnavailable) {
				t.Fatalf("ErrSmsSendUnavailable kutilgandi, olindi: %v", err)
			}
			// Telegram yo'li (IssueCode) tasodifiy kod beradi — sobit EMAS.
			phone, code, err := s.IssueCode(ctx, TestLoginPhone)
			if err != nil {
				t.Fatal(err)
			}
			if code == TestLoginCode {
				t.Fatal("imtiyozli/mijoz akkauntiga sobit kod berildi")
			}
			if _, _, err := s.Verify(ctx, phone, TestLoginCode); err == nil {
				t.Fatal("sobit kod imtiyozli/mijoz akkauntini ochdi")
			}
			// Haqiqiy egasi esa o'z kodini (Telegram'dan) kiritib kira oladi.
			if _, _, err := s.Verify(ctx, phone, code); err != nil {
				t.Fatalf("haqiqiy kod ishlashi kerak edi: %v", err)
			}
		})
	}
}

// Akkaunt yo'q bo'lsa test kirishi uni YARATMAYDI (aks holda sobit kod
// bilan mijoz akkaunti paydo bo'lardi).
func TestTestLoginDoesNotCreateAccounts(t *testing.T) {
	s, repo, _ := testLoginService(t, "", true)
	ctx := context.Background()

	if s.TestLoginActive(ctx, TestLoginPhone) {
		t.Fatal("akkaunt yo'q — test kirishi faol bo'lmasligi kerak")
	}
	if _, _, err := s.Verify(ctx, TestLoginPhone, TestLoginCode); err == nil {
		t.Fatal("kod berilmagan raqamga kirildi")
	}
	if _, err := repo.GetByPhone(ctx, TestLoginPhone); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("akkaunt yaratilmasligi kerak edi: %v", err)
	}
}

// O'chirilgan kuryer akkaunti sobit kod bilan tiklanmaydi.
func TestTestLoginSkipsDeletedAccount(t *testing.T) {
	s, repo, _ := testLoginService(t, RoleCourier, true)
	ctx := context.Background()
	u, _ := repo.GetByPhone(ctx, TestLoginPhone)
	now := time.Now()
	u.DeletedAt = &now

	if s.TestLoginActive(ctx, TestLoginPhone) {
		t.Fatal("o'chirilgan akkaunt uchun test kirishi faol bo'lmasligi kerak")
	}
}

// Kod berilgandan keyin rol o'zgarsa (masalan akkaunt restoran egasiga
// aylantirildi) — sobit kod rad etiladi va o'chiriladi.
func TestTestLoginRecheckedAtVerify(t *testing.T) {
	s, repo, _ := testLoginService(t, RoleCourier, true)
	ctx := context.Background()

	phone, _, err := s.RequestCode(ctx, TestLoginPhone)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := repo.GetByPhone(ctx, TestLoginPhone)
	u.Role = RoleRestaurant

	if _, _, err := s.Verify(ctx, phone, TestLoginCode); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("ErrInvalidCode kutilgandi, olindi: %v", err)
	}
	// Kod o'chirilgan — rol qaytarilsa ham o'sha kod endi o'tmaydi.
	u.Role = RoleCourier
	if _, _, err := s.Verify(ctx, phone, TestLoginCode); err == nil {
		t.Fatal("rad etilgan kod qayta ishladi")
	}
}

// Bayroq o'chiq (standart) — test raqami oddiy raqam.
func TestTestLoginDisabledByDefault(t *testing.T) {
	s, _, _ := testLoginService(t, RoleCourier, false)
	ctx := context.Background()

	if s.TestLoginActive(ctx, TestLoginPhone) {
		t.Fatal("TEST_OTP o'chiq — test kirishi faol bo'lmasligi kerak")
	}
	if _, _, err := s.RequestCode(ctx, TestLoginPhone); !errors.Is(err, ErrSmsSendUnavailable) {
		t.Fatalf("ErrSmsSendUnavailable kutilgandi, olindi: %v", err)
	}
	if _, _, err := s.Verify(ctx, TestLoginPhone, TestLoginCode); err == nil {
		t.Fatal("bayroq o'chiq bo'lsa sobit kod ishlamasligi kerak")
	}
}

// Boshqa raqamlarga ta'sir yo'q.
func TestTestLoginDoesNotAffectOtherPhones(t *testing.T) {
	s, _, _ := testLoginService(t, RoleCourier, true)
	if _, _, err := s.RequestCode(context.Background(), "+998901234567"); !errors.Is(err, ErrSmsSendUnavailable) {
		t.Fatalf("ErrSmsSendUnavailable kutilgandi, olindi: %v", err)
	}
}

// Urinishlar cheklovi test kodida ham ishlaydi.
func TestTestLoginKeepsAttemptLimit(t *testing.T) {
	s, _, _ := testLoginService(t, RoleCourier, true)
	ctx := context.Background()
	phone, _, err := s.RequestCode(ctx, TestLoginPhone)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxAttempts; i++ {
		if _, _, err := s.Verify(ctx, phone, "000000"); !errors.Is(err, ErrInvalidCode) {
			t.Fatalf("%d-urinish: ErrInvalidCode kutilgandi, olindi: %v", i+1, err)
		}
	}
	if _, _, err := s.Verify(ctx, phone, TestLoginCode); !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("urinishlar tugagach ErrTooManyAttempts kutilgandi, olindi: %v", err)
	}
}

func TestWithTestLoginValidates(t *testing.T) {
	s := newServiceWithSms(&countingSms{})
	if _, err := s.WithTestLogin("999", TestLoginCode); err == nil {
		t.Error("noto'g'ri raqam qabul qilindi")
	}
	for _, bad := range []string{"", "12345", "1234567", "abcdef"} {
		if _, err := s.WithTestLogin(TestLoginPhone, bad); err == nil {
			t.Errorf("noto'g'ri kod qabul qilindi: %q", bad)
		}
	}
}
