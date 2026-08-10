package users

import (
	"context"
	"testing"
)

// ── EMAIL KIMLIK SIFATIDA: TASDIQLANMAGAN MANZIL ─────────────────────
//
// Bu fayldagi testlar 2026-08-10 dagi tekshiruvda topilgan zaiflikni
// qamrab oladi.
//
// HUJUM (uch eshikda ham bir xil):
//
//	1. hujumchi O'Z RAQAMI bilan register qiladi, lekin `email`
//	   maydoniga QURBONNING manzilini yozadi;
//	2. o'z raqamiga kelgan SMS kodni kiritadi -> yozuv tasdiqlanadi.
//	   Endi bazada: hujumchining telefoni + qurbonning emaili,
//	   `email_verified = false`;
//	3. qurbon o'sha manzil bilan Google orqali kiradi (yoki emailga
//	   kod so'raydi, yoki email bilan ro'yxatdan o'tmoqchi bo'ladi).
//
// Tuzatishdan OLDIN 3-qadamda `GetByEmail` hujumchining yozuvini
// topardi, `!EmailVerified` shoxobchasi uni "tugallanmagan ro'yxatdan
// o'tish" deb hisoblab, manzilni TASDIQLANGAN qilardi va tokenni SHU
// yozuvga berardi. Natijada qurbon hujumchining akkauntiga tushardi;
// hujumchi esa o'z telefoni orqali unga kirib turaverardi va qurbonning
// buyurtmalari, manzillari, raqamini ko'rardi.
//
// ILDIZ SABAB: `email` ustuni IKKI XIL ma'noda ishlatilardi —
// (a) kimlik, (b) profildagi bog'lanish ma'lumoti. Tasdiqlanmagan
// manzil FAQAT (b) bo'lishi mumkin.

// pinEmailToVerifiedPhoneAccount — yuqoridagi 1-2 qadamlarni bajaradi
// va hujumchining foydalanuvchi ID sini qaytaradi.
func pinEmailToVerifiedPhoneAccount(t *testing.T, s *Service, phone, email string) string {
	t.Helper()
	ctx := context.Background()
	code, err := s.Register(ctx, RegisterInput{
		Phone: phone, Email: email,
		FirstName: "Hujumchi",
		Password:  okPassword, PasswordConfirm: okPassword,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if _, _, err := s.Verify(ctx, phone, code); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	u, err := s.users.GetByPhone(ctx, phone)
	if err != nil {
		t.Fatalf("GetByPhone: %v", err)
	}
	if u.Email != email || u.EmailVerified {
		t.Fatalf("kutilgan holat yaratilmadi: email=%q verified=%v", u.Email, u.EmailVerified)
	}
	return u.ID
}

// TestGoogleDoesNotAdoptEmailPinnedToPhoneAccount — Google orqali
// kirish begonaning yozuviga TUSHMASLIGI kerak.
func TestGoogleDoesNotAdoptEmailPinnedToPhoneAccount(t *testing.T) {
	s := newTestService()
	const attackerPhone = "+998901112233"
	const victimEmail = "qurbon@example.com"

	attackerID := pinEmailToVerifiedPhoneAccount(t, s, attackerPhone, victimEmail)

	_, u, err := s.LoginWithGoogle(context.Background(), victimEmail, "Qurbon Familiya")
	if err != nil {
		t.Fatalf("LoginWithGoogle: %v", err)
	}
	if u.ID == attackerID {
		t.Fatal("AKKAUNT ARALASHDI: Google qurbonni hujumchining yozuviga kiritdi")
	}
	if u.Phone == attackerPhone {
		t.Fatal("AKKAUNT ARALASHDI: qurbonning yozuvida hujumchining raqami turibdi")
	}
	if !u.EmailVerified {
		t.Fatal("Google bilan kirgan yozuvda email tasdiqlanmagan bo'lib qoldi")
	}
}

// TestEmailCodeDoesNotAdoptEmailPinnedToPhoneAccount — emailga kelgan
// kod ham xuddi shunday: u faqat MANZIL egaligini isbotlaydi, begona
// telefon yozuviga kirish huquqini emas.
func TestEmailCodeDoesNotAdoptEmailPinnedToPhoneAccount(t *testing.T) {
	s := newTestService()
	const attackerPhone = "+998901112244"
	const victimEmail = "qurbon2@example.com"

	attackerID := pinEmailToVerifiedPhoneAccount(t, s, attackerPhone, victimEmail)

	ctx := context.Background()
	_, code, err := s.RequestEmailCode(ctx, victimEmail)
	if err != nil {
		t.Fatalf("RequestEmailCode: %v", err)
	}
	_, u, err := s.VerifyEmail(ctx, victimEmail, code)
	if err == nil && u.ID == attackerID {
		t.Fatal("AKKAUNT ARALASHDI: email kodi hujumchining yozuviga token berdi")
	}
}

// TestVictimCanStillRegisterWithOwnEmail — tuzatish qurbonni O'Z
// manzilidan mahrum qilmasligi kerak: begonaning tasdiqlanmagan
// da'vosi uni abadiy band qilib qo'ymasin.
func TestVictimCanStillRegisterWithOwnEmail(t *testing.T) {
	s := newTestService()
	const attackerPhone = "+998901112255"
	const victimEmail = "qurbon3@example.com"

	attackerID := pinEmailToVerifiedPhoneAccount(t, s, attackerPhone, victimEmail)

	ctx := context.Background()
	code, err := s.Register(ctx, RegisterInput{
		Email: victimEmail, FirstName: "Qurbon",
		Password: okPassword, PasswordConfirm: okPassword,
	})
	if err != nil {
		t.Fatalf("qurbon o'z manzili bilan ro'yxatdan o'ta olmadi: %v", err)
	}
	if code == "" {
		t.Fatal("kod yuborilmadi — manzil begona yozuv tomonidan band qilib qo'yilgan")
	}
	_, u, err := s.VerifyEmail(ctx, victimEmail, code)
	if err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}
	if u.ID == attackerID {
		t.Fatal("AKKAUNT ARALASHDI: qurbon hujumchining yozuviga tushdi")
	}
}
