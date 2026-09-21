package users

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Bu testlar `test_login.go` dagi xavfsizlik chegaralarini BUZISHGA
// urinadi: sobit kod faqat bayroq yoqilganda va faqat raqamga
// biriktirilgan rolni (888 — kuryer, 999 — affitsiant) ochishi kerak.

const (
	goPhone  = "+998888888888" // OnDexGO — kuryer
	proPhone = "+998999999999" // OnDexPro — affitsiant
)

// testLoginService — SMS kanali o'chirilgan (production holati) servis va
// `phone` dagi akkaunt. `role` bo'sh bo'lsa akkaunt yaratilmaydi.
func testLoginService(t *testing.T, phone string, role Role, enable bool) (*Service, *fakeUserRepo, *countingSms) {
	t.Helper()
	sms := &countingSms{}
	s := newServiceWithSms(sms).WithoutSms()
	repo := s.users.(*fakeUserRepo)
	if role != "" {
		_ = repo.Create(context.Background(), &User{
			ID: "test-user", Phone: phone, Role: role,
			EntityID: "staff-1", PhoneVerified: true, CreatedAt: time.Now(),
		})
	}
	if enable {
		var err error
		if s, err = s.WithTestLogin(TestLogins, TestLoginCode); err != nil {
			t.Fatal(err)
		}
	}
	return s, repo, sms
}

// Ro'yxat ilovalarga mos: OnDexGO — kuryer, OnDexPro — affitsiant.
func TestTestLoginsMapping(t *testing.T) {
	want := map[string]Role{goPhone: RoleCourier, proPhone: RoleWaiter}
	if len(TestLogins) != len(want) {
		t.Fatalf("%d ta test raqami, kutilgan %d", len(TestLogins), len(want))
	}
	for _, l := range TestLogins {
		if want[l.Phone] != l.Role {
			t.Errorf("%s → %q, kutilgan %q", l.Phone, l.Role, want[l.Phone])
		}
	}
}

// Asosiy oqim: har bir ilova sharhlovchisi SMS'siz, Telegram'siz kiradi.
func TestTestLoginOpensStaffAccounts(t *testing.T) {
	for _, c := range []struct {
		name  string
		phone string
		role  Role
		typed string // foydalanuvchi yozgan shakl
	}{
		{"OnDexGO", goPhone, RoleCourier, "+998 88 888-88-88"},
		{"OnDexPro", proPhone, RoleWaiter, "998999999999"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, _, sms := testLoginService(t, c.phone, c.role, true)
			ctx := context.Background()

			if !s.TestLoginActive(ctx, c.typed) {
				t.Fatal("test raqami (har qanday yozilishda) faol bo'lishi kerak edi")
			}
			phone, code, err := s.RequestCode(ctx, c.typed)
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
			if token == "" || u.Role != c.role {
				t.Fatalf("token=%q rol=%q, kutilgan rol %q", token, u.Role, c.role)
			}
			// Bir martalik: o'sha kod ikkinchi marta o'tmaydi.
			if _, _, err := s.Verify(ctx, phone, TestLoginCode); !errors.Is(err, ErrInvalidCode) {
				t.Fatalf("kod qayta ishlatildi: %v", err)
			}
		})
	}
}

// Har raqam FAQAT o'z rolini ochadi: 888 affitsiant bo'lsa ham, 999 kuryer
// bo'lsa ham sobit kod ishlamaydi.
func TestTestLoginPhoneBoundToItsRole(t *testing.T) {
	for _, c := range []struct {
		phone string
		role  Role
	}{
		{goPhone, RoleWaiter},
		{proPhone, RoleCourier},
	} {
		s, _, _ := testLoginService(t, c.phone, c.role, true)
		ctx := context.Background()
		if s.TestLoginActive(ctx, c.phone) {
			t.Errorf("%s (%s): boshqa rol uchun test kirishi faol bo'lmasligi kerak", c.phone, c.role)
		}
		phone, code, err := s.IssueCode(ctx, c.phone)
		if err != nil {
			t.Fatal(err)
		}
		if code == TestLoginCode {
			t.Errorf("%s (%s): sobit kod berildi", c.phone, c.role)
		}
		if _, _, err := s.Verify(ctx, phone, TestLoginCode); err == nil {
			t.Errorf("%s (%s): sobit kod boshqa rolni ochdi", c.phone, c.role)
		}
	}
}

// "+998 88" va "+998 99" — haqiqiy operator kodlari: raqam tirik odamniki
// bo'lishi mumkin. Uning mijoz akkaunti (hamyon!), restoran yoki admin
// akkaunti sobit kod bilan HECH QACHON ochilmasligi kerak.
func TestTestLoginNeverOpensPrivilegedOrCustomerAccounts(t *testing.T) {
	for _, phone := range []string{goPhone, proPhone} {
		for _, role := range []Role{RoleCustomer, RoleRestaurant, RoleAdmin} {
			t.Run(phone+"/"+string(role), func(t *testing.T) {
				s, _, _ := testLoginService(t, phone, role, true)
				ctx := context.Background()

				if s.TestLoginActive(ctx, phone) {
					t.Fatal("bu rol uchun test kirishi faol bo'lmasligi kerak")
				}
				// Oddiy raqamdek: SMS o'chiq — kod umuman berilmaydi.
				if _, _, err := s.RequestCode(ctx, phone); !errors.Is(err, ErrSmsSendUnavailable) {
					t.Fatalf("ErrSmsSendUnavailable kutilgandi, olindi: %v", err)
				}
				// Telegram yo'li (IssueCode) tasodifiy kod beradi — sobit EMAS.
				p, code, err := s.IssueCode(ctx, phone)
				if err != nil {
					t.Fatal(err)
				}
				if code == TestLoginCode {
					t.Fatal("imtiyozli/mijoz akkauntiga sobit kod berildi")
				}
				if _, _, err := s.Verify(ctx, p, TestLoginCode); err == nil {
					t.Fatal("sobit kod imtiyozli/mijoz akkauntini ochdi")
				}
				// Haqiqiy egasi esa o'z kodini (Telegram'dan) kiritib kira oladi.
				if _, _, err := s.Verify(ctx, p, code); err != nil {
					t.Fatalf("haqiqiy kod ishlashi kerak edi: %v", err)
				}
			})
		}
	}
}

// Akkaunt yo'q bo'lsa test kirishi uni YARATMAYDI (aks holda sobit kod
// bilan mijoz akkaunti paydo bo'lardi).
func TestTestLoginDoesNotCreateAccounts(t *testing.T) {
	s, repo, _ := testLoginService(t, goPhone, "", true)
	ctx := context.Background()

	if s.TestLoginActive(ctx, goPhone) {
		t.Fatal("akkaunt yo'q — test kirishi faol bo'lmasligi kerak")
	}
	if _, _, err := s.Verify(ctx, goPhone, TestLoginCode); err == nil {
		t.Fatal("kod berilmagan raqamga kirildi")
	}
	if _, err := repo.GetByPhone(ctx, goPhone); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("akkaunt yaratilmasligi kerak edi: %v", err)
	}
}

// O'chirilgan kuryer akkaunti sobit kod bilan tiklanmaydi.
func TestTestLoginSkipsDeletedAccount(t *testing.T) {
	s, repo, _ := testLoginService(t, goPhone, RoleCourier, true)
	ctx := context.Background()
	u, _ := repo.GetByPhone(ctx, goPhone)
	now := time.Now()
	u.DeletedAt = &now

	if s.TestLoginActive(ctx, goPhone) {
		t.Fatal("o'chirilgan akkaunt uchun test kirishi faol bo'lmasligi kerak")
	}
}

// Kod berilgandan keyin rol o'zgarsa (masalan akkaunt restoran egasiga
// aylantirildi) — sobit kod rad etiladi va o'chiriladi.
func TestTestLoginRecheckedAtVerify(t *testing.T) {
	s, repo, _ := testLoginService(t, goPhone, RoleCourier, true)
	ctx := context.Background()

	phone, _, err := s.RequestCode(ctx, goPhone)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := repo.GetByPhone(ctx, goPhone)
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

// Bayroq o'chiq (standart) — test raqamlari oddiy raqam.
func TestTestLoginDisabledByDefault(t *testing.T) {
	s, _, _ := testLoginService(t, goPhone, RoleCourier, false)
	ctx := context.Background()

	if s.TestLoginActive(ctx, goPhone) {
		t.Fatal("TEST_OTP o'chiq — test kirishi faol bo'lmasligi kerak")
	}
	if _, _, err := s.RequestCode(ctx, goPhone); !errors.Is(err, ErrSmsSendUnavailable) {
		t.Fatalf("ErrSmsSendUnavailable kutilgandi, olindi: %v", err)
	}
	if _, _, err := s.Verify(ctx, goPhone, TestLoginCode); err == nil {
		t.Fatal("bayroq o'chiq bo'lsa sobit kod ishlamasligi kerak")
	}
}

// Boshqa raqamlarga ta'sir yo'q.
func TestTestLoginDoesNotAffectOtherPhones(t *testing.T) {
	s, _, _ := testLoginService(t, goPhone, RoleCourier, true)
	if _, _, err := s.RequestCode(context.Background(), "+998901234567"); !errors.Is(err, ErrSmsSendUnavailable) {
		t.Fatalf("ErrSmsSendUnavailable kutilgandi, olindi: %v", err)
	}
}

// Urinishlar cheklovi test kodida ham ishlaydi.
func TestTestLoginKeepsAttemptLimit(t *testing.T) {
	s, _, _ := testLoginService(t, goPhone, RoleCourier, true)
	ctx := context.Background()
	phone, _, err := s.RequestCode(ctx, goPhone)
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
	ok := []TestLogin{{Phone: goPhone, Role: RoleCourier}}
	bad := map[string]struct {
		logins []TestLogin
		code   string
	}{
		"bo'sh ro'yxat":   {nil, TestLoginCode},
		"noto'g'ri raqam": {[]TestLogin{{Phone: "999", Role: RoleCourier}}, TestLoginCode},
		"mijoz roli":      {[]TestLogin{{Phone: goPhone, Role: RoleCustomer}}, TestLoginCode},
		"restoran roli":   {[]TestLogin{{Phone: goPhone, Role: RoleRestaurant}}, TestLoginCode},
		"admin roli":      {[]TestLogin{{Phone: goPhone, Role: RoleAdmin}}, TestLoginCode},
		"takroriy raqam":  {[]TestLogin{{Phone: goPhone, Role: RoleCourier}, {Phone: "+998 88 888 88 88", Role: RoleWaiter}}, TestLoginCode},
		"bo'sh kod":       {ok, ""},
		"5 xonali kod":    {ok, "12345"},
		"7 xonali kod":    {ok, "1234567"},
		"harfli kod":      {ok, "abcdef"},
	}
	for name, c := range bad {
		s := newServiceWithSms(&countingSms{})
		if _, err := s.WithTestLogin(c.logins, c.code); err == nil {
			t.Errorf("%s: qabul qilindi", name)
		}
	}
}
