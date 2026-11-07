// Package authztest — `authz.Store` implementatsiyalari uchun UMUMIY
// shartnoma: xotira va Postgres ombori AYNAN bir xil qoidalarda ishlashi
// shart (bazadagi CHECK/UNIQUE cheklovlari xotirada ham takrorlanadi).
package authztest

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"chustapp/internal/identity/authz"
)

// Env — ombor va uning tashqi qismlari.
type Env struct {
	Store authz.Store
	// Users — users jadvalida MAVJUD 3 ta akkaunt: egasi, xodim, begona.
	Owner, Staff, Other string
	// SetOrgStatus / SetBusinessStatus — platform amallari (bu interfeysda yo'q).
	SetOrgStatus      func(id, status string)
	SetBusinessStatus func(id, status string)
}

type contract struct {
	ctx    context.Context
	newEnv func(t *testing.T) *Env
}

// RunStoreContract — `newEnv` har chaqirilganda TOZA ombor qaytarsin.
func RunStoreContract(t *testing.T, newEnv func(t *testing.T) *Env) {
	c := &contract{ctx: context.Background(), newEnv: newEnv}
	t.Run("egasi tashkilotning hamma biznesiga kiradi", c.ownerAllBusinesses)
	t.Run("xodim faqat o'z biznesiga, faqat rol ruxsatlari bilan", c.staffOwnBusinessOnly)
	t.Run("faol bo'lmagan a'zolik huquq bermaydi", c.inactiveMembership)
	t.Run("to'xtatilgan tashkilot va biznes", c.suspended)
	t.Run("shablonlar idempotent yoziladi", c.templatesIdempotent)
	t.Run("cheklovlar: egasi, rol turi, takror", c.constraints)
	t.Run("a'zolikni yangilash", c.update)
	t.Run("Grants: egalik xodimlikdan ustun, takrorsiz", c.grants)
	t.Run("taklif: faqat muddati o'tmagani faollashadi, qayta taklif yangilaydi", c.invites)
}

func (c *contract) invites(t *testing.T) {
	e := c.seed(t)
	s := e.Store
	now := time.Now().UTC().Truncate(time.Second)
	fresh, stale := now.Add(-time.Hour), now.Add(-authz.InviteTTL-time.Hour)
	must(t, s.CreateMembership(c.ctx, &authz.Membership{ID: "ctz_inv_new", UserID: e.Staff, OrgID: "ctz_org",
		BusinessID: "ctz_r1", RoleID: waiterTpl(), InvitedAt: fresh}))
	must(t, s.CreateMembership(c.ctx, &authz.Membership{ID: "ctz_inv_old", UserID: e.Staff, OrgID: "ctz_org",
		BusinessID: "ctz_r2", RoleID: waiterTpl(), InvitedAt: stale}))
	if m, _ := s.GetMembership(c.ctx, "ctz_inv_new"); m.Status != authz.MemberInvited || !m.InvitedAt.Equal(fresh) {
		t.Fatalf("taklif yozuvi: %+v", m)
	}
	got, err := s.ActivateInvites(c.ctx, e.Staff, now.Add(-authz.InviteTTL))
	must(t, err)
	if len(got) != 1 || got[0].ID != "ctz_inv_new" || got[0].Status != authz.MemberActive {
		t.Fatalf("faollashganlar: %+v", got)
	}
	if m, _ := s.GetMembership(c.ctx, "ctz_inv_old"); m.Status != authz.MemberInvited {
		t.Fatalf("eskirgan taklif faollashdi: %+v", m)
	}
	// qayta taklif: muddati yangilanadi
	must(t, s.UpdateMembership(c.ctx, "ctz_inv_old", authz.MembershipPatch{Status: authz.MemberInvited, InvitedAt: &now}))
	got, _ = s.ActivateInvites(c.ctx, e.Staff, now.Add(-authz.InviteTTL))
	if len(got) != 1 || got[0].ID != "ctz_inv_old" {
		t.Fatalf("qayta taklif faollashmadi: %+v", got)
	}
	if got, _ := s.ActivateInvites(c.ctx, e.Staff, now.Add(-authz.InviteTTL)); len(got) != 0 {
		t.Fatalf("ikkinchi marta faollashtirish: %+v", got)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// seed — tashkilot (2 restoran + 1 do'kon), egasi, shablonlar.
func (c *contract) seed(t *testing.T) *Env {
	t.Helper()
	e := c.newEnv(t)
	s := e.Store
	must(t, authz.EnsureTemplates(c.ctx, s))
	must(t, s.CreateOrganization(c.ctx, &authz.Organization{ID: "ctz_org", Name: "Test MCHJ", OwnerUserID: e.Owner}))
	must(t, s.CreateOrganization(c.ctx, &authz.Organization{ID: "ctz_org2", Name: "Begona MCHJ", OwnerUserID: e.Other}))
	for _, b := range []*authz.Business{
		{ID: "ctz_r1", OrgID: "ctz_org", Type: authz.TypeRestaurant, Name: "Book Cafe"},
		{ID: "ctz_r2", OrgID: "ctz_org", Type: authz.TypeRestaurant, Name: "Book Cafe 2"},
		{ID: "ctz_s1", OrgID: "ctz_org", Type: authz.TypeStore, Name: "Qurt do'koni"},
		{ID: "ctz_x1", OrgID: "ctz_org2", Type: authz.TypeRestaurant, Name: "Begona"},
	} {
		must(t, s.CreateBusiness(c.ctx, b))
	}
	must(t, s.CreateMembership(c.ctx, &authz.Membership{ID: "ctz_m_owner", UserID: e.Owner, OrgID: "ctz_org",
		IsOwner: true, Status: authz.MemberActive}))
	must(t, s.CreateMembership(c.ctx, &authz.Membership{ID: "ctz_m_owner2", UserID: e.Other, OrgID: "ctz_org2",
		IsOwner: true, Status: authz.MemberActive}))
	return e
}

func waiterTpl() string {
	return authz.Template{Type: authz.TypeRestaurant, Key: "waiter"}.ID()
}

func (c *contract) ownerAllBusinesses(t *testing.T) {
	e := c.seed(t)
	a := authz.New(e.Store)
	for _, biz := range []string{"ctz_r1", "ctz_r2", "ctz_s1"} {
		acc, err := a.Require(c.ctx, e.Owner, biz, authz.PermStaffManage)
		if err != nil || !acc.IsOwner {
			t.Fatalf("%s: egasi kira olmadi: %v", biz, err)
		}
	}
	acc, _ := a.Check(c.ctx, e.Owner, "ctz_s1")
	if !acc.Has(authz.PermPOSSell) || acc.Has(authz.PermMenuManage) {
		t.Fatalf("do'kon egasi ruxsatlari tur katalogidan emas: %v", acc.Permissions())
	}
	if _, err := a.Check(c.ctx, e.Owner, "ctz_x1"); !errors.Is(err, authz.ErrNotFound) {
		t.Fatalf("begona tashkilot biznesi: %v, kutilgan ErrNotFound", err)
	}
	if _, err := a.Check(c.ctx, e.Owner, "ctz_yoq"); !errors.Is(err, authz.ErrNotFound) {
		t.Fatalf("mavjud bo'lmagan biznes: %v", err)
	}
}

func (c *contract) staffOwnBusinessOnly(t *testing.T) {
	e := c.seed(t)
	must(t, e.Store.CreateMembership(c.ctx, &authz.Membership{ID: "ctz_m_w", UserID: e.Staff, OrgID: "ctz_org",
		BusinessID: "ctz_r1", RoleID: waiterTpl(), Apps: []string{authz.AppPro}, Status: authz.MemberActive}))
	a := authz.New(e.Store)
	acc, err := a.Require(c.ctx, e.Staff, "ctz_r1", authz.PermDineInServe)
	if err != nil {
		t.Fatalf("ofitsiant o'z ruxsati: %v", err)
	}
	if acc.IsOwner || acc.RoleKey != "waiter" || !acc.AllowsApp(authz.AppPro) || acc.AllowsApp(authz.AppPanel) {
		t.Fatalf("noto'g'ri huquq: %+v apps=%v", acc, acc.Apps())
	}
	if _, err := a.Require(c.ctx, e.Staff, "ctz_r1", authz.PermStaffManage); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("ofitsiant staff.manage: %v, kutilgan ErrForbidden", err)
	}
	if _, err := a.Check(c.ctx, e.Staff, "ctz_r2"); !errors.Is(err, authz.ErrNotFound) {
		t.Fatalf("o'sha tashkilotning boshqa biznesi: %v, kutilgan ErrNotFound", err)
	}
}

func (c *contract) inactiveMembership(t *testing.T) {
	e := c.seed(t)
	for i, st := range []string{authz.MemberInvited, authz.MemberSuspended, authz.MemberDismissed} {
		biz := []string{"ctz_r1", "ctz_r2", "ctz_s1"}[i]
		role := waiterTpl()
		if biz == "ctz_s1" {
			role = authz.Template{Type: authz.TypeStore, Key: "cashier"}.ID()
		}
		must(t, e.Store.CreateMembership(c.ctx, &authz.Membership{ID: "ctz_m_" + st, UserID: e.Staff, OrgID: "ctz_org",
			BusinessID: biz, RoleID: role, Status: st}))
		if _, err := authz.New(e.Store).Check(c.ctx, e.Staff, biz); !errors.Is(err, authz.ErrNotFound) {
			t.Errorf("holat %q huquq berdi: %v", st, err)
		}
	}
}

func (c *contract) suspended(t *testing.T) {
	e := c.seed(t)
	e.SetBusinessStatus("ctz_r1", authz.BusinessSuspended)
	if _, err := authz.New(e.Store).Check(c.ctx, e.Owner, "ctz_r1"); !errors.Is(err, authz.ErrSuspended) {
		t.Fatalf("to'xtatilgan biznes: %v", err)
	}
	e.SetBusinessStatus("ctz_r1", authz.BusinessClosed)
	if _, err := authz.New(e.Store).Check(c.ctx, e.Owner, "ctz_r1"); !errors.Is(err, authz.ErrNotFound) {
		t.Fatalf("yopilgan biznes: %v", err)
	}
	e.SetOrgStatus("ctz_org", authz.OrgSuspended)
	if _, err := authz.New(e.Store).Check(c.ctx, e.Owner, "ctz_r2"); !errors.Is(err, authz.ErrSuspended) {
		t.Fatalf("to'xtatilgan tashkilot: %v", err)
	}
	list, err := authz.New(e.Store).List(c.ctx, e.Owner)
	if err != nil || len(list) != 0 {
		t.Fatalf("to'xtatilgan tashkilot bizneslari ro'yxatda: %d %v", len(list), err)
	}
}

func (c *contract) templatesIdempotent(t *testing.T) {
	e := c.newEnv(t)
	must(t, authz.EnsureTemplates(c.ctx, e.Store))
	must(t, authz.EnsureTemplates(c.ctx, e.Store))
	rs, err := e.Store.Roles(c.ctx, "", authz.TypeRestaurant)
	must(t, err)
	want := 0
	for _, x := range authz.Templates() {
		if x.Type == authz.TypeRestaurant {
			want++
		}
	}
	if len(rs) != want {
		t.Fatalf("restoran shablonlari %d, kutilgan %d", len(rs), want)
	}
	r, err := e.Store.GetRole(c.ctx, waiterTpl())
	must(t, err)
	if r.OrgID != "" || !slices.Contains(r.Permissions, authz.PermDineInServe) {
		t.Fatalf("ofitsiant shabloni: %+v", r)
	}
	// Katalogda o'zgargan shablon qayta yozilganda yangilanadi.
	tr := authz.TemplateRoles()
	for _, x := range tr {
		if x.ID == waiterTpl() {
			x.Permissions = []authz.Permission{authz.PermMenuView}
		}
	}
	must(t, e.Store.UpsertTemplates(c.ctx, tr))
	r, _ = e.Store.GetRole(c.ctx, waiterTpl())
	if len(r.Permissions) != 1 {
		t.Fatalf("shablon yangilanmadi: %v", r.Permissions)
	}
}

func (c *contract) constraints(t *testing.T) {
	e := c.seed(t)
	s := e.Store
	bad := map[string]*authz.Membership{
		"ikkinchi faol egasi":      {ID: "ctz_b1", UserID: e.Staff, OrgID: "ctz_org", IsOwner: true, Status: authz.MemberActive},
		"xodim rolsiz":             {ID: "ctz_b2", UserID: e.Staff, OrgID: "ctz_org", BusinessID: "ctz_r1"},
		"egasida biznes":           {ID: "ctz_b3", UserID: e.Staff, OrgID: "ctz_org", BusinessID: "ctz_r1", IsOwner: true},
		"noma'lum ilova":           {ID: "ctz_b4", UserID: e.Staff, OrgID: "ctz_org", BusinessID: "ctz_r1", RoleID: waiterTpl(), Apps: []string{"admin"}},
		"do'kon roli restoranda":   {ID: "ctz_b5", UserID: e.Staff, OrgID: "ctz_org", BusinessID: "ctz_r1", RoleID: authz.Template{Type: authz.TypeStore, Key: "cashier"}.ID()},
		"begona tashkilot biznesi": {ID: "ctz_b6", UserID: e.Staff, OrgID: "ctz_org", BusinessID: "ctz_x1", RoleID: waiterTpl()},
	}
	for why, m := range bad {
		if err := s.CreateMembership(c.ctx, m); err == nil {
			t.Errorf("o'tib ketmasligi kerak edi: %s", why)
		}
	}
	// Boshqa tashkilotning maxsus roli ishlatilmaydi.
	must(t, s.CreateRole(c.ctx, &authz.Role{ID: "ctz_role_x", OrgID: "ctz_org2", BusinessType: authz.TypeRestaurant,
		Key: "barmen", Name: "Barmen", Permissions: []authz.Permission{authz.PermMenuView}}))
	if err := s.CreateMembership(c.ctx, &authz.Membership{ID: "ctz_b7", UserID: e.Staff, OrgID: "ctz_org",
		BusinessID: "ctz_r1", RoleID: "ctz_role_x"}); err == nil {
		t.Error("begona tashkilot roli bilan a'zolik yaratildi")
	}
	if err := s.CreateRole(c.ctx, &authz.Role{ID: "ctz_role_dup", OrgID: "ctz_org2", BusinessType: authz.TypeRestaurant,
		Key: "barmen", Name: "Barmen 2"}); !errors.Is(err, authz.ErrConflict) {
		t.Errorf("takroriy rol kaliti: %v", err)
	}
	must(t, s.CreateMembership(c.ctx, &authz.Membership{ID: "ctz_ok", UserID: e.Staff, OrgID: "ctz_org",
		BusinessID: "ctz_r1", RoleID: waiterTpl()}))
	if err := s.CreateMembership(c.ctx, &authz.Membership{ID: "ctz_dup", UserID: e.Staff, OrgID: "ctz_org",
		BusinessID: "ctz_r1", RoleID: waiterTpl()}); !errors.Is(err, authz.ErrConflict) {
		t.Errorf("bir biznesda ikki a'zolik: %v", err)
	}
	if err := s.CreateOrganization(c.ctx, &authz.Organization{ID: "ctz_badtin", Name: "X", OwnerUserID: e.Owner,
		TIN: "12ab"}); !errors.Is(err, authz.ErrInvalid) {
		t.Errorf("noto'g'ri STIR: %v", err)
	}
	// Roles: shablonlar + O'Z tashkiloti rollari, begonaniki emas.
	rs, err := s.Roles(c.ctx, "ctz_org", authz.TypeRestaurant)
	must(t, err)
	for _, r := range rs {
		if r.ID == "ctz_role_x" {
			t.Error("begona tashkilot roli ro'yxatda")
		}
	}
}

func (c *contract) update(t *testing.T) {
	e := c.seed(t)
	s := e.Store
	must(t, s.CreateMembership(c.ctx, &authz.Membership{ID: "ctz_m_u", UserID: e.Staff, OrgID: "ctz_org",
		BusinessID: "ctz_r1", RoleID: waiterTpl(), Apps: []string{authz.AppPro}}))
	cook := authz.Template{Type: authz.TypeRestaurant, Key: "cook"}.ID()
	must(t, s.UpdateMembership(c.ctx, "ctz_m_u", authz.MembershipPatch{RoleID: cook, Status: authz.MemberActive}))
	m, err := s.GetMembership(c.ctx, "ctz_m_u")
	must(t, err)
	if m.RoleID != cook || m.Status != authz.MemberActive || !slices.Equal(m.Apps, []string{authz.AppPro}) {
		t.Fatalf("yangilanish: %+v", m)
	}
	must(t, s.UpdateMembership(c.ctx, "ctz_m_u", authz.MembershipPatch{Apps: []string{}}))
	m, _ = s.GetMembership(c.ctx, "ctz_m_u")
	if len(m.Apps) != 0 || m.RoleID != cook {
		t.Fatalf("ilovalar tozalanmadi yoki rol o'zgardi: %+v", m)
	}
	if err := s.UpdateMembership(c.ctx, "ctz_m_u", authz.MembershipPatch{
		RoleID: authz.Template{Type: authz.TypeStore, Key: "cashier"}.ID()}); err == nil {
		t.Error("boshqa tur roli berildi")
	}
	if err := s.UpdateMembership(c.ctx, "ctz_yoq", authz.MembershipPatch{Status: authz.MemberActive}); !errors.Is(err, authz.ErrNotFound) {
		t.Errorf("mavjud bo'lmagan a'zolik: %v", err)
	}
	ms, err := s.Members(c.ctx, "ctz_r1")
	must(t, err)
	if len(ms) != 1 || ms[0].ID != "ctz_m_u" {
		t.Fatalf("Members: %d (egasi kirmasligi kerak)", len(ms))
	}
}

func (c *contract) grants(t *testing.T) {
	e := c.seed(t)
	// Egasi o'z biznesida xodim sifatida ham yozilgan bo'lsa — egalik ustun.
	must(t, e.Store.CreateMembership(c.ctx, &authz.Membership{ID: "ctz_m_ow_staff", UserID: e.Owner, OrgID: "ctz_org",
		BusinessID: "ctz_r1", RoleID: waiterTpl(), Status: authz.MemberActive}))
	must(t, e.Store.CreateMembership(c.ctx, &authz.Membership{ID: "ctz_m_s", UserID: e.Staff, OrgID: "ctz_org",
		BusinessID: "ctz_r2", RoleID: waiterTpl(), Status: authz.MemberActive}))
	gs, err := e.Store.Grants(c.ctx, e.Owner)
	must(t, err)
	if len(gs) != 3 {
		t.Fatalf("egasi bizneslari %d, kutilgan 3", len(gs))
	}
	for _, g := range gs {
		if !g.IsOwner {
			t.Fatalf("%s: egalik ustun bo'lishi kerak", g.Business.ID)
		}
	}
	g, err := e.Store.Grant(c.ctx, e.Owner, "ctz_r1")
	must(t, err)
	if !g.IsOwner {
		t.Fatal("Grant: egalik ustun emas")
	}
	gs, err = e.Store.Grants(c.ctx, e.Staff)
	must(t, err)
	if len(gs) != 1 || gs[0].Business.ID != "ctz_r2" || gs[0].Role == nil || gs[0].OrgStatus != authz.OrgPending {
		t.Fatalf("xodim huquqlari: %+v", gs)
	}
}
