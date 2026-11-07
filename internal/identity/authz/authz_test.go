package authz_test

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"testing"

	"chustapp/internal/identity/authz"
	"chustapp/internal/identity/authz/authztest"
)

func TestMemStoreContract(t *testing.T) {
	authztest.RunStoreContract(t, func(t *testing.T) *authztest.Env {
		s := authz.NewMemStore()
		return &authztest.Env{Store: s, Owner: "u_owner", Staff: "u_staff", Other: "u_other",
			SetOrgStatus: s.SetOrganizationStatus, SetBusinessStatus: s.SetBusinessStatus}
	})
}

var permRe = regexp.MustCompile(`^[a-z]+\.[a-z]+$`)

// Katalog o'zi izchil: format, takrorsiz, shablonlar faqat o'z turi
// ruxsatlaridan, shablon kaliti bazadagi CHECK'ga mos, xavfli ruxsat
// hech bir shablonda yo'q (xodimga faqat egasi alohida beradi).
func TestCatalogConsistency(t *testing.T) {
	keyRe := regexp.MustCompile(`^[a-z][a-z0-9_]{1,40}$`)
	for _, bt := range authz.BusinessTypes() {
		seen := map[authz.Permission]bool{}
		for _, p := range authz.Catalog(bt) {
			if !permRe.MatchString(string(p.Key)) {
				t.Errorf("%s: ruxsat formati %q", bt, p.Key)
			}
			if seen[p.Key] {
				t.Errorf("%s: takroriy ruxsat %q", bt, p.Key)
			}
			seen[p.Key] = true
		}
	}
	ids := map[string]bool{}
	for _, tpl := range authz.Templates() {
		if !keyRe.MatchString(tpl.Key) || ids[tpl.ID()] {
			t.Errorf("shablon kaliti/ID: %s", tpl.ID())
		}
		ids[tpl.ID()] = true
		if err := authz.ValidatePermissions(tpl.Type, tpl.Permissions); err != nil {
			t.Errorf("%s: %v", tpl.ID(), err)
		}
		for _, p := range tpl.Permissions {
			if authz.IsDangerous(tpl.Type, p) {
				t.Errorf("%s: xavfli ruxsat %q shablonda", tpl.ID(), p)
			}
		}
		for _, a := range tpl.Apps {
			if !authz.ValidApp(a) {
				t.Errorf("%s: ilova %q", tpl.ID(), a)
			}
		}
	}
	if authz.Catalog("noma_lum") != nil {
		t.Error("noma'lum tur katalogi bo'sh bo'lishi kerak")
	}
}

// Huquqni oshirishga qarshi: menejer o'zida yo'q yoki xavfli ruxsatni bera
// olmaydi; staff.manage'siz umuman bera olmaydi; egasi hammasini beradi.
func TestCanGrant(t *testing.T) {
	ctx := context.Background()
	s := authz.NewMemStore()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(authz.EnsureTemplates(ctx, s))
	must(s.CreateOrganization(ctx, &authz.Organization{ID: "o", Name: "O", OwnerUserID: "owner"}))
	must(s.CreateBusiness(ctx, &authz.Business{ID: "b", OrgID: "o", Type: authz.TypeRestaurant, Name: "B"}))
	must(s.CreateMembership(ctx, &authz.Membership{ID: "m0", UserID: "owner", OrgID: "o", IsOwner: true, Status: authz.MemberActive}))
	must(s.CreateRole(ctx, &authz.Role{ID: "r_mgr", OrgID: "o", BusinessType: authz.TypeRestaurant, Key: "boss",
		Name: "Boshqaruvchi", Permissions: []authz.Permission{authz.PermStaffManage, authz.PermMenuView, authz.PermOrdersView}}))
	must(s.CreateMembership(ctx, &authz.Membership{ID: "m1", UserID: "mgr", OrgID: "o", BusinessID: "b", RoleID: "r_mgr", Status: authz.MemberActive}))
	must(s.CreateMembership(ctx, &authz.Membership{ID: "m2", UserID: "cashier", OrgID: "o", BusinessID: "b",
		RoleID: authz.Template{Type: authz.TypeRestaurant, Key: "cashier"}.ID(), Status: authz.MemberActive}))
	a := authz.New(s)
	acc := func(u string) *authz.Access {
		x, err := a.Check(ctx, u, "b")
		must(err)
		return x
	}
	owner, mgr, cashier := acc("owner"), acc("mgr"), acc("cashier")

	if err := authz.CanGrant(owner, []authz.Permission{authz.PermPayoutsManage, authz.PermStaffManage}); err != nil {
		t.Errorf("egasi xavfli ruxsat bera olmadi: %v", err)
	}
	if err := authz.CanGrant(mgr, []authz.Permission{authz.PermMenuView}); err != nil {
		t.Errorf("menejer o'z ruxsatini bera olmadi: %v", err)
	}
	for name, perms := range map[string][]authz.Permission{
		"o'zida yo'q":         {authz.PermMenuManage},
		"xavfli (o'zida bor)": {authz.PermStaffManage},
		"xavfli":              {authz.PermRefundsCreate},
	} {
		if err := authz.CanGrant(mgr, perms); !errors.Is(err, authz.ErrForbidden) {
			t.Errorf("menejer %s ruxsatni berdi: %v", name, err)
		}
	}
	if err := authz.CanGrant(cashier, []authz.Permission{authz.PermPOSSell}); !errors.Is(err, authz.ErrForbidden) {
		t.Errorf("staff.manage'siz kassir rol berdi: %v", err)
	}
	if err := authz.CanGrant(owner, []authz.Permission{"payouts.steal"}); !errors.Is(err, authz.ErrInvalid) {
		t.Errorf("noma'lum ruxsat: %v", err)
	}
	if err := authz.CanGrant(owner, []authz.Permission{authz.PermProductsManage}); !errors.Is(err, authz.ErrInvalid) {
		t.Errorf("do'kon ruxsati restoranga: %v", err)
	}
	if err := authz.CanGrant(nil, nil); !errors.Is(err, authz.ErrForbidden) {
		t.Errorf("nil berayotgan: %v", err)
	}
}

// Bazadagi rolda katalogda yo'q satr bo'lsa — huquq bermaydi (yozishni
// chetlab o'tgan eski/qo'lda kiritilgan qiymat).
func TestUnknownPermissionInRoleIgnored(t *testing.T) {
	ctx := context.Background()
	s := authz.NewMemStore()
	_ = s.CreateOrganization(ctx, &authz.Organization{ID: "o", Name: "O", OwnerUserID: "w"})
	_ = s.CreateBusiness(ctx, &authz.Business{ID: "b", OrgID: "o", Type: authz.TypeStore, Name: "B"})
	_ = s.CreateRole(ctx, &authz.Role{ID: "r", OrgID: "o", BusinessType: authz.TypeStore, Key: "odd", Name: "Odd",
		Permissions: []authz.Permission{"payouts.steal", authz.PermPOSSell, authz.PermMenuManage}})
	if err := s.CreateMembership(ctx, &authz.Membership{ID: "m", UserID: "x", OrgID: "o", BusinessID: "b", RoleID: "r",
		Status: authz.MemberActive}); err != nil {
		t.Fatal(err)
	}
	acc, err := authz.New(s).Check(ctx, "x", "b")
	if err != nil {
		t.Fatal(err)
	}
	if got := acc.Permissions(); !slices.Equal(got, []authz.Permission{authz.PermPOSSell}) {
		t.Fatalf("effektiv ruxsatlar %v, kutilgan faqat pos.sell", got)
	}
}

// Kesh: a'zolik o'chirilgach `Invalidate` DARHOL kuchga kiradi.
func TestAuthorizerInvalidate(t *testing.T) {
	ctx := context.Background()
	s := authz.NewMemStore()
	_ = authz.EnsureTemplates(ctx, s)
	_ = s.CreateOrganization(ctx, &authz.Organization{ID: "o", Name: "O", OwnerUserID: "w"})
	_ = s.CreateBusiness(ctx, &authz.Business{ID: "b", OrgID: "o", Type: authz.TypeRestaurant, Name: "B"})
	_ = s.CreateMembership(ctx, &authz.Membership{ID: "m", UserID: "x", OrgID: "o", BusinessID: "b",
		RoleID: authz.Template{Type: authz.TypeRestaurant, Key: "waiter"}.ID(), Status: authz.MemberActive})
	a := authz.New(s)
	if _, err := a.Check(ctx, "x", "b"); err != nil {
		t.Fatal(err)
	}
	_ = s.UpdateMembership(ctx, "m", authz.MembershipPatch{Status: authz.MemberDismissed})
	if _, err := a.Check(ctx, "x", "b"); err != nil {
		t.Fatalf("kesh ichida eski natija kutilgan edi: %v", err)
	}
	a.Invalidate()
	if _, err := a.Check(ctx, "x", "b"); !errors.Is(err, authz.ErrNotFound) {
		t.Fatalf("bo'shatilgan xodim Invalidate'dan keyin: %v", err)
	}
	if _, err := a.Check(ctx, "x|y", "b"); !errors.Is(err, authz.ErrNotFound) {
		t.Fatal("kesh kalitini buzuvchi ID qabul qilindi")
	}
}

func TestAppForClient(t *testing.T) {
	for c, want := range map[string]string{"merchant-web": "panel", "merchant-desktop": "panel",
		"ondex-pro": "pro", "ondex-go": "go", "admin-web": "", "": ""} {
		if got := authz.AppForClient(c); got != want {
			t.Errorf("%q -> %q, kutilgan %q", c, got, want)
		}
	}
}
