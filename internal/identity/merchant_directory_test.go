package identity_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"chustapp/internal/identity"
	"chustapp/internal/identity/authz"
	"chustapp/internal/users"
)

func tpl(t authz.BusinessType, key string) string { return authz.Template{Type: t, Key: key}.ID() }

// signupOwner — yangi egasi (restoran) va uning biznes ID'si.
func (f *merchantFixture) signupOwner(t *testing.T, phone string) (ownerID, bizID string) {
	t.Helper()
	out, err := f.m.Signup(f.ctx, identity.SignupInput{Phone: phone, Code: f.fu.sendCode(phone),
		Password: "Kuchli-parol-1", PasswordConfirm: "Kuchli-parol-1", FirstName: "Shox", LastName: "T",
		OrgName: "Book Cafe MCHJ", BusinessType: authz.TypeRestaurant, BusinessName: "Book Cafe",
		ClientID: "merchant-web"}, meta)
	if err != nil || out.MFA == nil || out.MFA.Method != "totp_enroll" {
		t.Fatalf("Signup: %+v %v", out, err)
	}
	u, err := f.users.GetByPhoneRealm(f.ctx, users.RealmMerchant, phone)
	if err != nil {
		t.Fatal(err)
	}
	me, err := f.m.Me(f.ctx, u.ID)
	if err != nil || len(me.Businesses) != 1 {
		t.Fatalf("Me: %+v %v", me, err)
	}
	return u.ID, me.Businesses[0].ID
}

func TestDirectory_SignupSeparateFromCustomer(t *testing.T) {
	f := newMerchantFixture(t)
	const phone = "+998901112233"
	if err := f.users.Create(f.ctx, &users.User{ID: "cust1", Phone: phone, Role: users.RoleCustomer,
		PhoneVerified: true, CreatedAt: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	ownerID, bizID := f.signupOwner(t, phone)
	if ownerID == "cust1" {
		t.Fatal("mijoz akkaunti biznes egasiga aylantirildi")
	}
	if c, _ := f.users.GetByID(f.ctx, "cust1"); c.Role != users.RoleCustomer {
		t.Fatalf("mijoz akkaunti o'zgardi: %+v", c)
	}
	if u, _ := f.users.GetByPhone(f.ctx, phone); u.ID != "cust1" {
		t.Fatal("eski qidiruv endi mijozni emas, yangi akkauntni qaytaryapti")
	}
	me, _ := f.m.Me(f.ctx, ownerID)
	b := me.Businesses[0]
	if b.ID != bizID || !b.IsOwner || b.OrgStatus != authz.OrgPending || b.Type != authz.TypeRestaurant ||
		!slices.Contains(b.Permissions, authz.PermPayoutsManage) || me.MFAEnabled {
		t.Fatalf("me: %+v", me)
	}
	_, err := f.m.Signup(f.ctx, identity.SignupInput{Phone: phone, Code: f.fu.sendCode(phone), Password: "Kuchli-parol-1",
		PasswordConfirm: "Kuchli-parol-1", FirstName: "X", OrgName: "Ikkinchi", BusinessType: authz.TypeStore,
		BusinessName: "D", ClientID: "merchant-web"}, meta)
	if !errors.Is(err, identity.ErrAccountExists) {
		t.Fatalf("ikkinchi ro'yxatdan o'tish: %v", err)
	}
	// forma xatosi kodni YOQMAYDI
	f.fu.sendCode("+998901112299")
	_, err = f.m.Signup(f.ctx, identity.SignupInput{Phone: "+998901112299", Code: "424242", Password: "a",
		PasswordConfirm: "a", FirstName: "X", OrgName: "O", BusinessType: authz.TypeStore, BusinessName: "D",
		ClientID: "merchant-web"}, meta)
	if err == nil {
		t.Fatal("zaif parol qabul qilindi")
	}
	if _, ok := f.fu.codes["+998901112299"]; !ok {
		t.Fatal("forma xatosida telefon kodi ishlatilib yuborildi")
	}
	if _, err := f.m.Signup(f.ctx, identity.SignupInput{ClientID: "ondex-pro"}, meta); !errors.Is(err, authz.ErrInvalid) {
		t.Fatalf("OnDexPro'dan ro'yxatdan o'tish: %v", err)
	}
}

func TestDirectory_InviteAcceptDismiss(t *testing.T) {
	f := newMerchantFixture(t)
	ownerID, bizID := f.signupOwner(t, "+998901112201")
	const staffPhone = "+998901112202"
	// xodim raqamida mijoz akkaunti bor — tegilmaydi
	_ = f.users.Create(f.ctx, &users.User{ID: "cust2", Phone: staffPhone, Role: users.RoleCustomer, CreatedAt: time.Now()})

	sv, err := f.m.AddStaff(f.ctx, ownerID, bizID, identity.AddStaffInput{Phone: staffPhone, FirstName: "Ali",
		RoleID: tpl(authz.TypeRestaurant, "waiter")}, meta)
	if err != nil || sv.Status != authz.MemberInvited || !slices.Equal(sv.Apps, []string{authz.AppPro}) || sv.RoleKey != "waiter" {
		t.Fatalf("AddStaff: %+v %v", sv, err)
	}
	if sv.UserID == "cust2" {
		t.Fatal("xodim mijoz akkauntiga biriktirildi")
	}
	// taklif qabul qilinmaguncha huquq yo'q
	if _, err := f.m.LoginCode(f.ctx, staffPhone, "000000", "ondex-pro", meta); err == nil {
		t.Fatal("noto'g'ri kod")
	}
	out, err := f.m.LoginCode(f.ctx, staffPhone, f.fu.sendCode(staffPhone), "ondex-pro", meta)
	if err != nil || out.Tokens == nil {
		t.Fatalf("xodim taklifni qabul qilib kira olmadi: %+v %v", out, err)
	}
	list, _ := f.m.ListStaff(f.ctx, ownerID, bizID)
	if len(list) != 1 || list[0].Status != authz.MemberActive || list[0].Phone != staffPhone {
		t.Fatalf("ro'yxat: %+v", list)
	}
	if _, err := f.m.AddStaff(f.ctx, ownerID, bizID, identity.AddStaffInput{Phone: staffPhone, FirstName: "Ali",
		RoleID: tpl(authz.TypeRestaurant, "cook")}, meta); !errors.Is(err, authz.ErrConflict) {
		t.Fatalf("takroriy xodim: %v", err)
	}

	// bo'shatish: sessiya DARHOL o'ladi
	if err := f.m.DismissStaff(f.ctx, ownerID, bizID, sv.MembershipID, meta); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Authenticate(f.ctx, out.Tokens.AccessToken); err == nil {
		t.Fatal("bo'shatilgan xodimning sessiyasi ishlayapti")
	}
	if _, err := f.m.LoginCode(f.ctx, staffPhone, f.fu.sendCode(staffPhone), "ondex-pro", meta); err == nil {
		t.Fatal("bo'shatilgan xodim qayta kirdi")
	}
	// qayta taklif -> yana kira oladi
	if _, err := f.m.ReinviteStaff(f.ctx, ownerID, bizID, sv.MembershipID, meta); err != nil {
		t.Fatal(err)
	}
	if out, err := f.m.LoginCode(f.ctx, staffPhone, f.fu.sendCode(staffPhone), "ondex-pro", meta); err != nil || out.Tokens == nil {
		t.Fatalf("qayta taklifdan keyin: %v", err)
	}
}

func TestDirectory_InviteExpires(t *testing.T) {
	f := newMerchantFixture(t)
	ownerID, bizID := f.signupOwner(t, "+998901112211")
	const p = "+998901112212"
	sv, err := f.m.AddStaff(f.ctx, ownerID, bizID, identity.AddStaffInput{Phone: p, FirstName: "Vali",
		RoleID: tpl(authz.TypeRestaurant, "cook"), Apps: []string{authz.AppPanel}}, meta)
	if err != nil {
		t.Fatal(err)
	}
	f.clk.add(authz.InviteTTL + time.Minute)
	if _, err := f.m.LoginCode(f.ctx, p, f.fu.sendCode(p), "merchant-web", meta); !errors.Is(err, users.ErrInvalidCredentials) {
		t.Fatalf("eskirgan taklif bilan kirish: %v", err)
	}
	list, _ := f.m.ListStaff(f.ctx, ownerID, bizID)
	if !list[0].InviteExpired {
		t.Fatal("eskirgan taklif belgilanmagan")
	}
	if _, err := f.m.ReinviteStaff(f.ctx, ownerID, bizID, sv.MembershipID, meta); err != nil {
		t.Fatal(err)
	}
	if out, err := f.m.LoginCode(f.ctx, p, f.fu.sendCode(p), "merchant-web", meta); err != nil || out.Tokens == nil {
		t.Fatalf("yangilangan taklif: %v", err)
	}
}

// Menejer huquqini oshira olmaydi; begona egasi biznesni ko'rmaydi.
func TestDirectory_ManagerCannotEscalate(t *testing.T) {
	f := newMerchantFixture(t)
	ownerID, bizID := f.signupOwner(t, "+998901112221")
	boss, err := f.m.CreateRole(f.ctx, ownerID, bizID, identity.CreateRoleInput{Key: "boss", Name: "Boshliq",
		Permissions: []authz.Permission{authz.PermStaffManage, authz.PermStaffView, authz.PermMenuView,
			authz.PermOrdersView, authz.PermKitchenView}}, meta)
	if err != nil {
		t.Fatal(err)
	}
	add := func(actor, phone, role string, apps []string) (*identity.StaffView, error) {
		return f.m.AddStaff(f.ctx, actor, bizID, identity.AddStaffInput{Phone: phone, FirstName: "X", RoleID: role, Apps: apps}, meta)
	}
	mgr, err := add(ownerID, "+998901112222", boss.ID, []string{authz.AppPanel})
	if err != nil {
		t.Fatal(err)
	}
	waiter, err := add(ownerID, "+998901112223", tpl(authz.TypeRestaurant, "waiter"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := f.m.LoginCode(f.ctx, "+998901112222", f.fu.sendCode("+998901112222"), "merchant-web", meta); err != nil || out.Tokens == nil {
		t.Fatalf("menejer kirishi: %v", err)
	}
	mgrID := mgr.UserID

	cases := map[string]error{}
	_, cases["o'zida yo'q ruxsatli rol (kassir)"] = add(mgrID, "+998901112224", tpl(authz.TypeRestaurant, "cashier"), nil)
	_, cases["o'ziga rol o'zgartirish"] = f.m.UpdateStaff(f.ctx, mgrID, bizID, mgr.MembershipID, identity.UpdateStaffInput{RoleID: tpl(authz.TypeRestaurant, "cook")}, meta)
	cases["o'zidan yuqori xodimni bo'shatish"] = f.m.DismissStaff(f.ctx, mgrID, bizID, waiter.MembershipID, meta)
	_, cases["xavfli ruxsatli rol yaratish"] = f.m.CreateRole(f.ctx, mgrID, bizID, identity.CreateRoleInput{Key: "x", Name: "X",
		Permissions: []authz.Permission{authz.PermStaffManage}}, meta)
	for name, err := range cases {
		if !errors.Is(err, authz.ErrForbidden) {
			t.Errorf("%s: %v, kutilgan ErrForbidden", name, err)
		}
	}
	// o'z ruxsatlari ichida — mumkin
	cook, err := add(mgrID, "+998901112225", tpl(authz.TypeRestaurant, "cook"), nil)
	if err != nil {
		t.Fatalf("menejer oshpaz qo'sha olmadi: %v", err)
	}
	if err := f.m.DismissStaff(f.ctx, mgrID, bizID, cook.MembershipID, meta); err != nil {
		t.Fatalf("menejer oshpazni bo'shata olmadi: %v", err)
	}

	// begona egasi: biznes umuman ko'rinmaydi (404)
	otherID, otherBiz := f.signupOwner(t, "+998901112231")
	if _, err := f.m.ListStaff(f.ctx, otherID, bizID); !errors.Is(err, authz.ErrNotFound) {
		t.Fatalf("begona biznes xodimlari: %v", err)
	}
	if _, err := f.m.UpdateStaff(f.ctx, otherID, otherBiz, waiter.MembershipID, identity.UpdateStaffInput{RoleID: tpl(authz.TypeRestaurant, "cook")}, meta); !errors.Is(err, authz.ErrNotFound) {
		t.Fatalf("begona a'zolikni o'z biznesi orqali o'zgartirish: %v", err)
	}
}
