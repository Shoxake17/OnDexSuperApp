package identity_test

import (
	"errors"
	"testing"

	"chustapp/internal/identity"
	"chustapp/internal/identity/mfa"
	"chustapp/internal/users"
)

// Akkaunt sahifasidan parol almashtirish (Plane 270, step-up 135):
// joriy parol + Authenticator kodi; so'ng BARCHA sessiyalar yopiladi.
func TestAccount_PasswordChangeNeedsStepUpAndEndsAllSessions(t *testing.T) {
	f := newMerchantFixture(t)
	f.user(t, "u_acc01", users.RoleRestaurant)
	phone := f.phoneOf(t, "u_acc01")
	f.setPassword(t, phone, "parol-123")
	secret, recovery := f.enroll(t, "u_acc01")

	other, err := f.svc.Create(f.ctx, identity.CreateInput{UserID: "u_acc01", ClientID: "merchant-desktop", AMR: []string{"otp"}})
	if err != nil {
		t.Fatal(err)
	}

	if err := f.m.ChangePassword(f.ctx, "u_acc01", "parol-123", "Yangi-Parol-77", "Yangi-Parol-77", "", meta); !errors.Is(err, identity.ErrStepUpRequired) {
		t.Fatalf("Authenticator kodisiz parol almashdi: %v", err)
	}
	f.clk.add(mfa.Period)
	code, _ := mfa.Code(secret, f.clk.now())
	if err := f.m.ChangePassword(f.ctx, "u_acc01", "xato-parol", "Yangi-Parol-77", "Yangi-Parol-77", code, meta); !errors.Is(err, users.ErrCurrentPasswordWrong) {
		t.Fatalf("noto'g'ri joriy parol: %v", err)
	}
	// O'sha TOTP qadami ishlatildi — qayta o'tmaydi.
	if err := f.m.ChangePassword(f.ctx, "u_acc01", "parol-123", "Yangi-Parol-77", "Yangi-Parol-77", code, meta); !errors.Is(err, identity.ErrMFAInvalid) {
		t.Fatalf("takroriy TOTP qabul qilindi: %v", err)
	}
	// Tiklash kodi ham step-up uchun yaraydi (telefon yo'qolgan holat).
	if err := f.m.ChangePassword(f.ctx, "u_acc01", "parol-123", "Yangi-Parol-77", "Yangi-Parol-77", recovery[0], meta); err != nil {
		t.Fatalf("parol almashmadi: %v", err)
	}
	if _, err := f.svc.Authenticate(f.ctx, other.AccessToken, users.RealmMerchant); err == nil {
		t.Fatal("XAVFSIZLIK: parol almashgach eski sessiya tirik qoldi")
	}
	if _, err := f.m.LoginPassword(f.ctx, phone, "parol-123", "merchant-web", meta); err == nil {
		t.Fatal("eski parol ishlayapti")
	}
	if out, err := f.m.LoginPassword(f.ctx, phone, "Yangi-Parol-77", "merchant-web", meta); err != nil || out.MFA == nil || out.MFA.Method != "totp" {
		t.Fatalf("yangi parol: %+v %v", out, err)
	}
}

// Step-up orqali TOTP'ni terib chiqib bo'lmaydi; TOTP'siz xodimda talab yo'q.
func TestAccount_StepUpBruteForceAndStaff(t *testing.T) {
	f := newMerchantFixture(t)
	f.user(t, "u_acc02", users.RoleRestaurant)
	f.setPassword(t, f.phoneOf(t, "u_acc02"), "parol-123")
	secret, _ := f.enroll(t, "u_acc02")

	var err error
	for i := 0; i < 10; i++ {
		if err = f.m.StepUp(f.ctx, "u_acc02", "000000", "test", meta); errors.Is(err, identity.ErrMFATooMany) {
			break
		}
	}
	if !errors.Is(err, identity.ErrMFATooMany) {
		t.Fatalf("cheksiz urinish: %v", err)
	}
	f.clk.add(mfa.Period)
	good, _ := mfa.Code(secret, f.clk.now())
	if err := f.m.StepUp(f.ctx, "u_acc02", good, "test", meta); !errors.Is(err, identity.ErrMFATooMany) {
		t.Fatalf("blok paytida to'g'ri kod ham o'tmasligi kerak: %v", err)
	}

	f.user(t, "u_acc03", users.RoleWaiter)
	if err := f.m.StepUp(f.ctx, "u_acc03", "", "test", meta); err != nil {
		t.Fatalf("TOTP'siz xodim: %v", err)
	}
	if ok, err := f.m.HasTOTP(f.ctx, "u_acc02"); err != nil || !ok {
		t.Fatalf("HasTOTP: %v %v", ok, err)
	}
}
