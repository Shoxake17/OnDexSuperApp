package users

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// Bu fayldagi testlar 2026-08-07 dagi xavfsizlik tekshiruvida topilgan
// va tuzatilgan kamchiliklarni QAYTIB KELISHIDAN himoya qiladi. Har
// biri aniq bir hujum stsenariysini takrorlaydi.

const okPassword = "juda-kuchli-parol-7"

func mustRegisterByPhone(t *testing.T, s *Service, phone, password string) string {
	t.Helper()
	code, err := s.Register(context.Background(), RegisterInput{
		Phone:           phone,
		FirstName:       "Test",
		LastName:        "Foydalanuvchi",
		Password:        password,
		PasswordConfirm: password,
	})
	if err != nil {
		t.Fatalf("Register: kutilmagan xato: %v", err)
	}
	return code
}

// TestRegisterByPhoneDoesNotStorePassword — telefon rejimida parol
// hash'i tasdiqlanmagan yozuvga YOZILMASLIGI kerak. Aynan shu yozuv
// akkaunt egallashning asosi edi.
func TestRegisterByPhoneDoesNotStorePassword(t *testing.T) {
	s := newTestService()
	mustRegisterByPhone(t, s, "+998901234567", okPassword)

	u, err := s.users.GetByPhone(context.Background(), "+998901234567")
	if err != nil {
		t.Fatalf("foydalanuvchi yaratilmadi: %v", err)
	}
	if u.PasswordHash != "" {
		t.Fatalf("tasdiqlanmagan yozuvda parol hash'i saqlanib qolgan: %q", u.PasswordHash)
	}
	if u.PhoneVerified {
		t.Fatal("yangi yozuv tasdiqlangan bo'lib chiqdi — token berilib ketishi mumkin")
	}
}

// TestAttackerCannotHijackAccountViaRegister — TO'LIQ hujum stsenariysi.
//
//	1. hujumchi qurbonning raqami bilan register qiladi va O'Z parolini
//	   qo'yadi; SMS qurbonning telefoniga boradi;
//	2. qurbon kodni kiritadi (o'zi kirmoqchi deb o'ylaydi);
//	3. hujumchi o'z paroli bilan kirishga uradi — RAD ETILISHI SHART.
//
// Tuzatishdan oldin 3-qadam muvaffaqiyatli token qaytarardi.
func TestAttackerCannotHijackAccountViaRegister(t *testing.T) {
	s := newTestService()
	const victim = "+998901112233"
	const attackerPassword = "hujumchining-paroli-9"

	code := mustRegisterByPhone(t, s, victim, attackerPassword)

	// Qurbon SMS kodni kiritadi va haqiqatan ham kiradi.
	if _, _, err := s.Verify(context.Background(), victim, code); err != nil {
		t.Fatalf("qurbon kira olmadi: %v", err)
	}

	// Hujumchi endi o'z paroli bilan urinadi.
	_, _, err := s.LoginWithPassword(context.Background(), victim, attackerPassword)
	if err == nil {
		t.Fatal("AKKAUNT EGALLANDI: hujumchining paroli bilan kirish muvaffaqiyatli bo'ldi")
	}
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("kutilgan ErrInvalidCredentials, olingan: %v", err)
	}
}

// TestVerifyClearsLegacyPasswordHash — tuzatishdan OLDIN yaratilgan
// yozuvlarda begona hash qolgan bo'lishi mumkin; tasdiqlash uni
// tiriltirmasligi kerak (ikkinchi himoya qatlami).
func TestVerifyClearsLegacyPasswordHash(t *testing.T) {
	s := newTestService()
	const phone = "+998905556677"
	ctx := context.Background()

	hash, err := HashPassword(tctx(),"eski-begona-parol-1")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	repo := s.users.(*fakeUserRepo)
	repo.data[phone] = &User{
		ID: "legacy1", Phone: phone, Role: RoleCustomer,
		PasswordHash: hash, PhoneVerified: false,
	}

	_, code, err := s.RequestCode(ctx, phone)
	if err != nil {
		t.Fatalf("RequestCode: %v", err)
	}
	if _, _, err := s.Verify(ctx, phone, code); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	u, _ := s.users.GetByPhone(ctx, phone)
	if u.PasswordHash != "" {
		t.Fatal("tasdiqlashdan keyin eski parol hash'i saqlanib qoldi")
	}
	if _, _, err := s.LoginWithPassword(ctx, phone, "eski-begona-parol-1"); err == nil {
		t.Fatal("eski begona parol bilan kirish mumkin bo'lib qoldi")
	}
}

// TestVerifyKeepsStaffAccountName — tasdiqlash xodim akkauntining
// NOMINI o'chirmasligi kerak.
//
// Admin yaratgan restoran/kuryer akkauntlarida `name` to'ldirilgan,
// `first_name`/`last_name` esa BO'SH bo'ladi. Parol hash'ini
// `UpdateProfile` orqali tozalash `name` ni first/last dan qayta
// hisoblab, uni bo'sh satrga aylantirib yuborardi — shuning uchun
// `ClearPassword` alohida metod sifatida qo'shildi.
func TestVerifyKeepsStaffAccountName(t *testing.T) {
	s := newTestService()
	ctx := context.Background()
	const phone = "+998900000010"

	repo := s.users.(*fakeUserRepo)
	repo.data[phone] = &User{
		ID: "u_rest1", Phone: phone, Name: "Chust Osh Markazi",
		Role: RoleRestaurant, EntityID: "r1", PhoneVerified: false,
	}

	_, code, err := s.RequestCode(ctx, phone)
	if err != nil {
		t.Fatalf("RequestCode: %v", err)
	}
	if _, _, err := s.Verify(ctx, phone, code); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	u, _ := s.users.GetByPhone(ctx, phone)
	if u.Name != "Chust Osh Markazi" {
		t.Fatalf("xodim akkaunti nomi buzildi: %q", u.Name)
	}
}

// TestRegisterDoesNotRevealTakenPhone — band raqam uchun ALOHIDA xato
// qaytarilmasligi kerak, aks holda `/auth/register` "bu raqam
// ro'yxatda bormi?" degan savolga bepul javob beruvchi vositaga
// aylanadi.
func TestRegisterDoesNotRevealTakenPhone(t *testing.T) {
	s := newTestService()
	const phone = "+998907778899"
	ctx := context.Background()

	code := mustRegisterByPhone(t, s, phone, okPassword)
	if _, _, err := s.Verify(ctx, phone, code); err != nil {
		t.Fatalf("birinchi tasdiqlash: %v", err)
	}

	// Endi raqam BAND va tasdiqlangan. Qayta register — xato YO'Q.
	if _, err := s.Register(ctx, RegisterInput{
		Phone: phone, FirstName: "Boshqa", LastName: "Odam",
		Password: okPassword, PasswordConfirm: okPassword,
	}); err != nil {
		t.Fatalf("band raqam oshkor bo'ldi: %v", err)
	}

	// Egasining profili BEGONA ma'lumot bilan almashib ketmasligi kerak.
	u, _ := s.users.GetByPhone(ctx, phone)
	if u.FirstName == "Boshqa" {
		t.Fatal("begona odam tasdiqlangan akkaunt profilini o'zgartirdi")
	}
}

// TestRegisterDoesNotRevealTakenEmail — email uchun ham xuddi shunday.
func TestRegisterDoesNotRevealTakenEmail(t *testing.T) {
	s := newTestService()
	ctx := context.Background()
	in := RegisterInput{
		Email: "test@example.com", FirstName: "Test",
		Password: okPassword, PasswordConfirm: okPassword,
	}
	if _, err := s.Register(ctx, in); err != nil {
		t.Fatalf("birinchi register: %v", err)
	}
	if _, err := s.Register(ctx, in); err != nil {
		t.Fatalf("band email oshkor bo'ldi: %v", err)
	}
}

// ---------- Email orqali ro'yxatdan o'tish / tasdiqlash ----------

// TestEmailRegisterDoesNotStorePassword — telefon oqimi bilan bir xil
// invariant: tasdiqlanmagan yozuvda parol hash'i BO'LMASLIGI kerak.
func TestEmailRegisterDoesNotStorePassword(t *testing.T) {
	s, mail := newTestServiceWithEmail()
	ctx := context.Background()
	const email = "yangi@example.com"

	code, err := s.Register(ctx, RegisterInput{
		Email: email, FirstName: "Test",
		Password: okPassword, PasswordConfirm: okPassword,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if code == "" {
		t.Fatal("dev kod qaytmadi — email yuborilmagan bo'lishi mumkin")
	}
	if mail.count() != 1 {
		t.Fatalf("bitta xat kutilgan, yuborilgan: %d", mail.count())
	}
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		t.Fatalf("akkaunt yaratilmadi: %v", err)
	}
	if u.PasswordHash != "" {
		t.Fatal("tasdiqlanmagan email yozuvida parol saqlanib qolgan")
	}
	if u.EmailVerified {
		t.Fatal("yangi yozuv tasdiqlangan bo'lib chiqdi")
	}
}

// TestEmailAccountCannotLoginBeforeVerify — tasdiqlanmagan email bilan
// parol orqali kirish MUMKIN EMAS.
func TestEmailAccountCannotLoginBeforeVerify(t *testing.T) {
	s, _ := newTestServiceWithEmail()
	ctx := context.Background()
	const email = "kutilmoqda@example.com"

	code, err := s.Register(ctx, RegisterInput{
		Email: email, FirstName: "Test",
		Password: okPassword, PasswordConfirm: okPassword,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	// Parol saqlanmagani uchun kirish baribir ishlamaydi, lekin
	// tasdiqdan keyin parol qo'yilgach ham tekshiruv ishlashi kerak.
	if _, _, err := s.LoginWithPassword(ctx, email, okPassword); err == nil {
		t.Fatal("tasdiqlanmagan akkauntga kirildi")
	}

	// Tasdiqlaymiz va parol qo'yamiz — endi kirish ochilishi kerak.
	if _, _, err := s.VerifyEmail(ctx, email, code); err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}
	u, _ := s.users.GetByEmail(ctx, email)
	if err := s.SetPassword(ctx, u.ID, "", okPassword, okPassword, false); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}
	if _, _, err := s.LoginWithPassword(ctx, email, okPassword); err != nil {
		t.Fatalf("tasdiqdan keyin kirib bo'lmadi: %v", err)
	}
}

// TestEmailAttackerCannotHijackAccount — telefon oqimidagi akkaunt
// egallashning EMAIL varianti: hujumchi qurbonning emaili bilan
// ro'yxatdan o'tib, o'z parolini qo'yib qo'ymasligi kerak.
func TestEmailAttackerCannotHijackAccount(t *testing.T) {
	s, _ := newTestServiceWithEmail()
	ctx := context.Background()
	const victim = "qurbon@example.com"
	const attackerPassword = "hujumchining-paroli-9"

	code, err := s.Register(ctx, RegisterInput{
		Email: victim, FirstName: "Hujumchi",
		Password: attackerPassword, PasswordConfirm: attackerPassword,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	// Qurbon o'z pochtasidagi kodni kiritadi.
	if _, _, err := s.VerifyEmail(ctx, victim, code); err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}
	if _, _, err := s.LoginWithPassword(ctx, victim, attackerPassword); err == nil {
		t.Fatal("AKKAUNT EGALLANDI: hujumchining paroli bilan kirildi")
	}
}

// TestVerifyEmailDoesNotCreateAccount — begona manzil uchun kod
// so'rab, tasdiqlab, akkaunt ochib olish mumkin bo'lmasligi kerak.
func TestVerifyEmailDoesNotCreateAccount(t *testing.T) {
	s, _ := newTestServiceWithEmail()
	ctx := context.Background()
	const email = "begona@example.com"

	_, code, err := s.RequestEmailCode(ctx, email)
	if err != nil {
		t.Fatalf("RequestEmailCode: %v", err)
	}
	if _, _, err := s.VerifyEmail(ctx, email, code); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("akkauntsiz tasdiqlash ErrInvalidCode berishi kerak, olindi: %v", err)
	}
	if _, err := s.users.GetByEmail(ctx, email); err == nil {
		t.Fatal("tasdiqlash begona manzilga akkaunt yaratdi")
	}
}

// TestEmailFlowsBlockedWhenSmtpMissing — SMTP ulanmagan bo'lsa email
// oqimlari JIMGINA muvaffaqiyat qaytarmasligi kerak.
func TestEmailFlowsBlockedWhenSmtpMissing(t *testing.T) {
	// atomic — sabab `internal/orders/service_test.go` dagi izohda.
	var n atomic.Int64
	s := NewService(
		&fakeUserRepo{data: make(map[string]*User)},
		&fakeCodeStore{data: make(map[string]*Code)},
		noopSms{},
		NewTokenIssuer("test-secret", time.Hour),
		func() string { return "id" + string(rune('0'+n.Add(1))) },
	) // WithEmail CHAQIRILMADI
	ctx := context.Background()

	if _, _, err := s.RequestEmailCode(ctx, "a@b.com"); !errors.Is(err, ErrEmailSendUnavailable) {
		t.Fatalf("kutilgan ErrEmailSendUnavailable, olindi: %v", err)
	}
	if _, err := s.Register(ctx, RegisterInput{
		Email: "a@b.com", FirstName: "A",
		Password: okPassword, PasswordConfirm: okPassword,
	}); !errors.Is(err, ErrEmailSendUnavailable) {
		t.Fatalf("email bilan register rad etilishi kerak edi: %v", err)
	}
}

// TestNormalizeEmail — CR/LF rad etilishi ALOHIDA muhim: manzil SMTP
// sarlavhasiga tushadi va yangi qator hujumchiga o'z sarlavhasini
// (masalan `Bcc:`) qo'shish imkonini berardi.
func TestNormalizeEmail(t *testing.T) {
	if got, err := NormalizeEmail("  Ali@Mail.UZ "); err != nil || got != "ali@mail.uz" {
		t.Fatalf("normalizatsiya noto'g'ri: %q, %v", got, err)
	}
	bad := []string{
		"", "notanemail", "a@b", "a b@c.uz",
		"a@b.uz\r\nBcc: victim@x.uz", "a@b.uz\nBcc: victim@x.uz",
	}
	for _, in := range bad {
		if _, err := NormalizeEmail(in); err == nil {
			t.Fatalf("qabul qilinmasligi kerak edi: %q", in)
		}
	}
}

// TestSetPasswordRequiresCurrent — parol allaqachon bo'lsa, uni
// bilmasdan almashtirib bo'lmasligi kerak (o'g'irlangan token bilan
// akkauntni butunlay egallab olishga qarshi).
func TestSetPasswordRequiresCurrent(t *testing.T) {
	s := newTestService()
	ctx := context.Background()
	repo := s.users.(*fakeUserRepo)
	hash, _ := HashPassword(tctx(),okPassword)
	repo.data["+998901010101"] = &User{
		ID: "u1", Phone: "+998901010101", PasswordHash: hash, PhoneVerified: true,
	}

	err := s.SetPassword(ctx, "u1", "", "yangi-parol-12345", "yangi-parol-12345", false)
	if !errors.Is(err, ErrCurrentPasswordWrong) {
		t.Fatalf("joriy parolsiz o'zgartirishga ruxsat berildi: %v", err)
	}
	err = s.SetPassword(ctx, "u1", "notogri-parol", "yangi-parol-12345", "yangi-parol-12345", false)
	if !errors.Is(err, ErrCurrentPasswordWrong) {
		t.Fatalf("noto'g'ri joriy parol qabul qilindi: %v", err)
	}
	if err := s.SetPassword(ctx, "u1", okPassword, "yangi-parol-12345", "yangi-parol-12345", false); err != nil {
		t.Fatalf("to'g'ri joriy parol bilan o'zgartirib bo'lmadi: %v", err)
	}
}

// TestSetPasswordWithFreshPhoneProof — "Parolni unutdingizmi?" oqimi:
// SMS kod bilan hozirgina kirgan foydalanuvchi joriy parolni bilmasa
// ham yangisini qo'ya oladi.
func TestSetPasswordWithFreshPhoneProof(t *testing.T) {
	s := newTestService()
	ctx := context.Background()
	repo := s.users.(*fakeUserRepo)
	hash, _ := HashPassword(tctx(),okPassword)
	repo.data["+998901010102"] = &User{
		ID: "u2", Phone: "+998901010102", PasswordHash: hash, PhoneVerified: true,
	}

	if err := s.SetPassword(ctx, "u2", "", "unutilgan-yangi-1", "unutilgan-yangi-1", true); err != nil {
		t.Fatalf("SMS tasdig'i bilan parol qo'yib bo'lmadi: %v", err)
	}
	if _, _, err := s.LoginWithPassword(ctx, "+998901010102", "unutilgan-yangi-1"); err != nil {
		t.Fatalf("yangi parol bilan kirib bo'lmadi: %v", err)
	}
}

// TestPhoneProofExpires — SMS tasdig'i abadiy emas: o'g'irlangan token
// 15 daqiqadan keyin parolni almashtira olmasligi kerak.
func TestPhoneProofExpires(t *testing.T) {
	u := &User{ID: "u3", Role: RoleCustomer}
	issuer := NewTokenIssuer("test-secret", 30*24*time.Hour)
	tok, err := issuer.IssuePhoneProven(u)
	if err != nil {
		t.Fatalf("IssuePhoneProven: %v", err)
	}
	c, err := issuer.Parse(tok)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !c.HasFreshPhoneProof(time.Now()) {
		t.Fatal("yangi berilgan token tasdiqlangan deb topilmadi")
	}
	if c.HasFreshPhoneProof(time.Now().Add(PhoneProofWindow + time.Minute)) {
		t.Fatal("muddati o'tgan tasdiq hali ham amal qilyapti")
	}
	// Oddiy (parol bilan berilgan) tokenda bu imtiyoz UMUMAN bo'lmasligi kerak.
	plain, _ := issuer.Issue(u)
	pc, _ := issuer.Parse(plain)
	if pc.HasFreshPhoneProof(time.Now()) {
		t.Fatal("oddiy token telefon tasdig'i deb qabul qilindi")
	}
}

// TestSetPasswordWhenNoneSet — ro'yxatdan o'tish oqimi aynan shunga
// tayanadi: tasdiqdan keyin parol joriy parolsiz o'rnatiladi.
func TestSetPasswordWhenNoneSet(t *testing.T) {
	s := newTestService()
	ctx := context.Background()
	const phone = "+998902020202"

	code := mustRegisterByPhone(t, s, phone, okPassword)
	if _, _, err := s.Verify(ctx, phone, code); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	u, _ := s.users.GetByPhone(ctx, phone)

	if err := s.SetPassword(ctx, u.ID, "", okPassword, okPassword, false); err != nil {
		t.Fatalf("tasdiqdan keyin parol o'rnatib bo'lmadi: %v", err)
	}
	if _, _, err := s.LoginWithPassword(ctx, phone, okPassword); err != nil {
		t.Fatalf("o'rnatilgan parol bilan kirib bo'lmadi: %v", err)
	}
}

// TestValidateRegisterInputIsCheap — handler bu tekshiruvni tezlik
// cheklovidan OLDIN chaqiradi, shuning uchun u bazaga ham, Argon2 ga
// ham murojaat qilmasligi kerak. Bu yerda faqat xatti-harakat
// tekshiriladi: noto'g'ri kiritma ValidationError beradi.
func TestValidateRegisterInputRejectsBadInput(t *testing.T) {
	cases := map[string]RegisterInput{
		"telefon ham, email ham yo'q": {FirstName: "A", Password: okPassword, PasswordConfirm: okPassword},
		"ism yo'q":                    {Phone: "+998901234567", Password: okPassword, PasswordConfirm: okPassword},
		"parollar mos emas":           {Phone: "+998901234567", FirstName: "A", Password: okPassword, PasswordConfirm: "boshqa-parol-1"},
		"parol qisqa":                 {Phone: "+998901234567", FirstName: "A", Password: "qisqa", PasswordConfirm: "qisqa"},
		"telefon formati":             {Phone: "12345", FirstName: "A", Password: okPassword, PasswordConfirm: okPassword},
		"email formati":               {Email: "notemail", FirstName: "A", Password: okPassword, PasswordConfirm: okPassword},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ValidateRegisterInput(in); err == nil {
				t.Fatal("noto'g'ri kiritma qabul qilindi")
			} else if !IsUserFacing(err) {
				t.Fatalf("xato mijozga ko'rsatish uchun belgilanmagan: %v", err)
			}
		})
	}
}

// TestIsUserFacingHidesInternalErrors — ro'yxatda yo'q xato mijozga
// CHIQMASLIGI kerak (baza drayveri matni, cheklov nomlari va h.k.).
func TestIsUserFacingHidesInternalErrors(t *testing.T) {
	if IsUserFacing(errors.New(`ERROR: duplicate key value violates unique constraint "users_email_lower_idx"`)) {
		t.Fatal("ichki baza xatosi mijozga ko'rsatiladigan deb belgilandi")
	}
	if !IsUserFacing(ErrInvalidCredentials) {
		t.Fatal("domen xatosi mijozdan yashirildi")
	}
}

// TestNeedsRehash — hash eski/zaif parametrlar bilan bo'lsa aniqlansin.
func TestNeedsRehash(t *testing.T) {
	current, err := HashPassword(tctx(),okPassword)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if NeedsRehash(current) {
		t.Fatal("hozirgi parametrlar bilan yaratilgan hash eski deb topildi")
	}
	// Zaifroq xotira bilan yasalgan hash — qayta hash talab qilinadi.
	weak := "$argon2id$v=19$m=1024,t=1,p=1$c29tZXNhbHRzb21lc2E$c29tZWtleXNvbWVrZXlzb21la2V5c29tZWtleXM"
	if !NeedsRehash(weak) {
		t.Fatal("zaif parametrli hash yangilanishi kerak deb topilmadi")
	}
	if !NeedsRehash("buzilgan-hash") {
		t.Fatal("buzilgan hash yangilanishi kerak deb topilmadi")
	}
}
