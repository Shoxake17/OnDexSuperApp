package identity_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"chustapp/internal/identity"
	"chustapp/internal/identity/authz"
	"chustapp/internal/identity/mfa"
	"chustapp/internal/users"
)

// Taklif xabari (135): AddStaff va qayta taklifda xodimga; matnda biznes,
// lavozim, ilova; havola yo'q.
func TestDirectory_InviteNotifiesStaff(t *testing.T) {
	f := newMerchantFixture(t)
	got := make(chan identity.Invite, 4)
	f.m.OnInvite(func(_ context.Context, inv identity.Invite) { got <- inv })
	owner, biz := f.signupOwner(t, "+998901113311")
	w, err := f.m.AddStaff(f.ctx, owner, biz, identity.AddStaffInput{Phone: "+998901113312", FirstName: "W",
		RoleID: tpl(authz.TypeRestaurant, "waiter")}, meta)
	if err != nil {
		t.Fatal(err)
	}
	var inv identity.Invite
	select {
	case inv = <-got:
	case <-time.After(2 * time.Second):
		t.Fatal("taklif xabari chaqirilmadi")
	}
	if inv.Phone != "+998901113312" || inv.BusinessName != "Book Cafe" || inv.UserID != w.UserID {
		t.Fatalf("taklif: %+v", inv)
	}
	txt := identity.InviteText(inv)
	for _, want := range []string{"«Book Cafe»", "OnDexPro", "24 soat"} {
		if !strings.Contains(txt, want) {
			t.Errorf("matnda %q yo'q:\n%s", want, txt)
		}
	}
	if strings.Contains(txt, "http") {
		t.Error("taklif xabarida havola bo'lmasligi kerak")
	}
	if err := f.m.DismissStaff(f.ctx, owner, biz, w.MembershipID, meta); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.ReinviteStaff(f.ctx, owner, biz, w.MembershipID, meta); err != nil {
		t.Fatal(err)
	}
	select {
	case <-got:
	case <-time.After(2 * time.Second):
		t.Fatal("qayta taklif xabari chaqirilmadi")
	}
}

// Xavfli ruxsat (staff.manage, payouts.manage, ...) berish — egasining
// Authenticator kodi bilan qayta tasdiq (Plane 135). Oddiy rol — kodsiz.
func TestDirectory_DangerousGrantNeedsStepUp(t *testing.T) {
	f := newMerchantFixture(t)
	const phone = "+998901113301"
	out, err := f.m.Signup(f.ctx, identity.SignupInput{Phone: phone, Code: f.fu.sendCode(phone),
		Password: "Kuchli-parol-1", PasswordConfirm: "Kuchli-parol-1", FirstName: "Shox",
		OrgName: "Step MCHJ", BusinessType: authz.TypeRestaurant, BusinessName: "Step Cafe", ClientID: "merchant-web"}, meta)
	if err != nil || out.MFA == nil {
		t.Fatalf("Signup: %+v %v", out, err)
	}
	en, err := f.m.EnrollStart(f.ctx, out.MFA.Token)
	if err != nil {
		t.Fatal(err)
	}
	c0, _ := mfa.Code(en.Secret, f.clk.now())
	if _, _, err := f.m.EnrollConfirm(f.ctx, out.MFA.Token, c0, meta); err != nil {
		t.Fatal(err)
	}
	u, err := f.users.GetByPhoneRealm(f.ctx, users.RealmMerchant, phone)
	if err != nil {
		t.Fatal(err)
	}
	me, err := f.m.Me(f.ctx, u.ID)
	if err != nil || len(me.Businesses) != 1 {
		t.Fatalf("Me: %v", err)
	}
	owner, biz := u.ID, me.Businesses[0].ID

	danger := identity.CreateRoleInput{Key: "boss", Name: "Boshliq",
		Permissions: []authz.Permission{authz.PermStaffManage, authz.PermStaffView}}
	if _, err := f.m.CreateRole(f.ctx, owner, biz, danger, meta); !errors.Is(err, identity.ErrStepUpRequired) {
		t.Fatalf("kodsiz xavfli rol yaratildi: %v", err)
	}
	danger.StepUpCode = "000000"
	if _, err := f.m.CreateRole(f.ctx, owner, biz, danger, meta); !errors.Is(err, identity.ErrMFAInvalid) {
		t.Fatalf("noto'g'ri kod: %v", err)
	}
	f.clk.add(mfa.Period)
	danger.StepUpCode, _ = mfa.Code(en.Secret, f.clk.now())
	boss, err := f.m.CreateRole(f.ctx, owner, biz, danger, meta)
	if err != nil {
		t.Fatalf("to'g'ri kod bilan: %v", err)
	}

	// Xavfli rolni xodimga berish — yana kod; oddiy rol (ofitsiant) — kodsiz.
	if _, err := f.m.AddStaff(f.ctx, owner, biz, identity.AddStaffInput{Phone: "+998901113302", FirstName: "M",
		RoleID: boss.ID, Apps: []string{authz.AppPanel}}, meta); !errors.Is(err, identity.ErrStepUpRequired) {
		t.Fatalf("kodsiz menejer qo'shildi: %v", err)
	}
	w, err := f.m.AddStaff(f.ctx, owner, biz, identity.AddStaffInput{Phone: "+998901113303", FirstName: "W",
		RoleID: tpl(authz.TypeRestaurant, "waiter")}, meta)
	if err != nil {
		t.Fatalf("ofitsiant (oddiy rol) kodsiz qo'shilishi kerak: %v", err)
	}
	if _, err := f.m.UpdateStaff(f.ctx, owner, biz, w.MembershipID, identity.UpdateStaffInput{RoleID: boss.ID}, meta); !errors.Is(err, identity.ErrStepUpRequired) {
		t.Fatalf("kodsiz menejerga ko'tarildi: %v", err)
	}
	f.clk.add(mfa.Period)
	code, _ := mfa.Code(en.Secret, f.clk.now())
	if _, err := f.m.UpdateStaff(f.ctx, owner, biz, w.MembershipID, identity.UpdateStaffInput{RoleID: boss.ID, StepUpCode: code}, meta); err != nil {
		t.Fatalf("kod bilan: %v", err)
	}
}
