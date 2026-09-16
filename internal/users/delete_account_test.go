package users

import (
	"context"
	"errors"
	"testing"
)

// Bu fayldagi testlar "Akkauntni o'chirish" (yumshoq o'chirish,
// migration 0056) uchun: ma'lumot saqlanishi, kirish yopilishi va
// egalik QAYTADAN isbotlanganda avtomatik tiklanishi.

// TestDeleteAccountBlocksPasswordLogin — o'chirilgan akkauntga parol
// TO'G'RI bo'lsa ham kirib bo'lmasligi kerak.
func TestDeleteAccountBlocksPasswordLogin(t *testing.T) {
	s := newTestService()
	const phone = "+998901234567"
	code := mustRegisterByPhone(t, s, phone, okPassword)
	if _, _, err := s.Verify(context.Background(), phone, code); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	u, err := s.users.GetByPhone(context.Background(), phone)
	if err != nil {
		t.Fatalf("GetByPhone: %v", err)
	}
	// Telefon rejimida `Register` parolni SAQLAMAYDI (akkaunt egallashga
	// qarshi chora) — u ilova tomonidan tasdiqdan KEYIN, shu chaqiruv
	// bilan o'rnatiladi (`Register` izohiga qarang).
	if err := s.SetPassword(context.Background(), u.ID, "", okPassword, okPassword, true); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}

	// SMS bilan HOZIRGINA kirgan (phoneProven=true), shuning uchun
	// joriy parolsiz o'chirish o'tishi kerak — `SetPassword` bilan bir
	// xil qoida.
	if err := s.DeleteAccount(context.Background(), u.ID, "", true); err != nil {
		t.Fatalf("DeleteAccount: kutilmagan xato: %v", err)
	}

	_, _, err = s.LoginWithPassword(context.Background(), phone, okPassword)
	if err == nil {
		t.Fatal("o'chirilgan akkauntga TO'G'RI parol bilan kirish muvaffaqiyatli bo'ldi")
	}
	if !errors.Is(err, ErrAccountDeleted) {
		t.Fatalf("kutilgan ErrAccountDeleted, olingan: %v", err)
	}
}

// TestDeleteAccountRequiresCurrentPassword — parol bor va token
// YANGI tasdiqlanmagan bo'lsa, joriy parolsiz o'chirib bo'lmaydi.
// Aks holda o'g'irlangan/qulfsiz qolgan telefon egasining
// ma'lumotlariga kirishni butunlay yopib qo'yishi mumkin edi.
func TestDeleteAccountRequiresCurrentPassword(t *testing.T) {
	s := newTestService()
	const phone = "+998901234599"
	code := mustRegisterByPhone(t, s, phone, okPassword)
	if _, _, err := s.Verify(context.Background(), phone, code); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	u, err := s.users.GetByPhone(context.Background(), phone)
	if err != nil {
		t.Fatalf("GetByPhone: %v", err)
	}
	if err := s.SetPassword(context.Background(), u.ID, "", okPassword, okPassword, true); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}

	// phoneProven=false, joriy parol berilmagan — rad etilishi shart.
	err = s.DeleteAccount(context.Background(), u.ID, "", false)
	if !errors.Is(err, ErrCurrentPasswordWrong) {
		t.Fatalf("kutilgan ErrCurrentPasswordWrong, olingan: %v", err)
	}
	if u2, _ := s.users.GetByID(context.Background(), u.ID); u2.IsDeleted() {
		t.Fatal("parol tekshirilmasdan akkaunt o'chirilib ketdi")
	}

	// TO'G'RI joriy parol bilan — o'tishi kerak.
	if err := s.DeleteAccount(context.Background(), u.ID, okPassword, false); err != nil {
		t.Fatalf("to'g'ri parol bilan DeleteAccount muvaffaqiyatsiz: %v", err)
	}
}

// TestReactivateOnPhoneVerify — o'chirilgan akkauntga O'SHA telefon
// raqami bilan qaytadan tasdiqlansa (SMS kod), akkaunt tiklanadi va
// ESKI ma'lumot (masalan ism) saqlanib qoladi.
func TestReactivateOnPhoneVerify(t *testing.T) {
	s := newTestService()
	const phone = "+998901234511"
	code := mustRegisterByPhone(t, s, phone, okPassword)
	if _, _, err := s.Verify(context.Background(), phone, code); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	u, err := s.users.GetByPhone(context.Background(), phone)
	if err != nil {
		t.Fatalf("GetByPhone: %v", err)
	}
	if err := s.DeleteAccount(context.Background(), u.ID, "", true); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if u2, _ := s.users.GetByID(context.Background(), u.ID); !u2.IsDeleted() {
		t.Fatal("akkaunt o'chirilmagan bo'lib chiqdi")
	}

	// Qaytadan ro'yxatdan o'tish/kirish oqimi: Register bir xil
	// telefon uchun tasdiqlangan yozuvni qayta ishlatadi (enumeration
	// himoyasi), keyin Verify kod bilan tasdiqlaydi.
	code2, err := s.Register(context.Background(), RegisterInput{
		Phone: phone, FirstName: "Test", LastName: "Foydalanuvchi",
		Password: okPassword, PasswordConfirm: okPassword,
	})
	if err != nil {
		t.Fatalf("qayta Register: %v", err)
	}
	token, u3, err := s.Verify(context.Background(), phone, code2)
	if err != nil {
		t.Fatalf("qayta Verify: %v", err)
	}
	if token == "" {
		t.Fatal("token bo'sh")
	}
	if u3.IsDeleted() {
		t.Fatal("akkaunt qaytadan tasdiqlangandan keyin ham o'chirilgan bo'lib qoldi")
	}
	if u3.ID != u.ID {
		t.Fatalf("YANGI akkaunt yaratilib ketdi (ID %q), eskisi (ID %q) tiklanishi kerak edi",
			u3.ID, u.ID)
	}
	if u3.Name != "Test Foydalanuvchi" {
		t.Fatalf("eski ma'lumot (ism) saqlanmagan: %q", u3.Name)
	}

	// Ma'lumot bazada ham tiklangan holatda qolgani tasdiqlanadi.
	u4, err := s.users.GetByID(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if u4.IsDeleted() {
		t.Fatal("bazadagi yozuv hali ham o'chirilgan deb belgilangan")
	}
}

// TestDeletedAccountBlocksTelegramIDLogin — eski Telegram sessiyasi
// (initData) o'chirilgan akkauntni AVTOMATIK tiklamasligi kerak —
// faqat YANGI dalil (kontakt ulashish, SMS, email, Google) tiklaydi.
func TestDeletedAccountBlocksTelegramIDLogin(t *testing.T) {
	s := newTestService()
	const phone = "+998901234522"
	const tgID = int64(555)

	u, err := s.LinkTelegramPhone(context.Background(), tgID, phone)
	if err != nil {
		t.Fatalf("LinkTelegramPhone: %v", err)
	}
	if err := s.DeleteAccount(context.Background(), u.ID, "", true); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}

	_, _, err = s.LoginWithTelegramID(context.Background(), tgID)
	if !errors.Is(err, ErrAccountDeleted) {
		t.Fatalf("kutilgan ErrAccountDeleted, olingan: %v", err)
	}

	// Kontakt QAYTA ulashilsa (yangi dalil) — tiklanadi.
	u2, err := s.LinkTelegramPhone(context.Background(), tgID, phone)
	if err != nil {
		t.Fatalf("qayta LinkTelegramPhone: %v", err)
	}
	if u2.IsDeleted() {
		t.Fatal("kontakt qayta ulashilgandan keyin ham o'chirilgan bo'lib qoldi")
	}
	if _, _, err := s.LoginWithTelegramID(context.Background(), tgID); err != nil {
		t.Fatalf("tiklangandan keyin LoginWithTelegramID muvaffaqiyatsiz: %v", err)
	}
}
