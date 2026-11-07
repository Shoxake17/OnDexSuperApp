package legacymigrate_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"chustapp/internal/identity/authz"
	"chustapp/internal/identity/legacymigrate"
	"chustapp/internal/qurtorders"
	"chustapp/internal/users"
)

func TestQurtPoolPrefixMatches(t *testing.T) {
	if legacymigrate.QurtPoolPrefix != qurtorders.QurtPoolPrefix {
		t.Fatalf("qurt havza prefiksi farq qiladi: %q vs %q", legacymigrate.QurtPoolPrefix, qurtorders.QurtPoolPrefix)
	}
}

func ids() func() string {
	n := 0
	return func() string { n++; return fmt.Sprintf("id%03d", n) }
}

var t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func u(id string, role users.Role, entity string, age time.Duration) *users.User {
	return &users.User{ID: id, Phone: "+99890" + id, Role: role, EntityID: entity, Name: "acc " + id, CreatedAt: t0.Add(age)}
}

func reasons(ns []legacymigrate.Note) string {
	var out []string
	for _, n := range ns {
		out = append(out, n.UserID+": "+n.Reason)
	}
	return strings.Join(out, "\n")
}

func TestBuild(t *testing.T) {
	in := legacymigrate.Input{
		Users: []*users.User{
			u("w1", users.RoleWaiter, "r1", 5*time.Hour), // egasidan oldin ro'yxatda — tartib baribir ishlaydi
			u("rest1", users.RoleRestaurant, "r1", 0),
			u("rest1b", users.RoleRestaurant, "r1", time.Hour), // ikkinchi egasi
			u("store1", users.RoleStore, "s1", 0),
			u("c_rest", users.RoleCourier, "cr1", 0),
			u("c_qurt", users.RoleCourier, "cq1", 0),
			u("c_plat", users.RoleCourier, "cp1", 0),
			u("c_none", users.RoleCourier, "cx", 0),
			u("w_orphan", users.RoleWaiter, "r_yoq", 0),
			u("noent", users.RoleStore, "", 0),
		},
		RestaurantNames: map[string]string{"r1": "Book Cafe"},
		CourierPools:    map[string]string{"cr1": "r1", "cq1": "qurt:s1", "cp1": ""},
		StaffByUser:     map[string]legacymigrate.StaffLink{"w1": {MemberID: "sm1", RestaurantID: "r1", Status: "on_leave"}},
		CustomerOrders:  map[string]int{"w1": 3},
		NewID:           ids(), Now: t0,
	}
	p := legacymigrate.Build(in)
	if len(p.Orgs) != 2 || len(p.Businesses) != 2 {
		t.Fatalf("tashkilot %d, biznes %d (kutilgan 2/2)\n%s", len(p.Orgs), len(p.Businesses), p.Summary())
	}
	byLegacy := map[string]*authz.Business{}
	for _, b := range p.Businesses {
		byLegacy[b.LegacyEntityID] = b
	}
	if b := byLegacy["r1"]; b == nil || b.Name != "Book Cafe" || b.Type != authz.TypeRestaurant {
		t.Fatalf("restoran biznesi: %+v", b)
	}
	if b := byLegacy["s1"]; b == nil || b.Type != authz.TypeStore || b.Name != "acc store1" {
		t.Fatalf("do'kon biznesi: %+v", b)
	}
	for _, o := range p.Orgs {
		if o.Status != authz.OrgActive {
			t.Errorf("tashkilot holati %q", o.Status)
		}
	}
	got := map[string]*authz.Membership{}
	for _, m := range p.Memberships {
		got[m.UserID] = m
	}
	if m := got["rest1"]; m == nil || !m.IsOwner || m.BusinessID != "" {
		t.Fatalf("egasi: %+v", m)
	}
	if got["rest1b"] != nil || !strings.Contains(reasons(p.Warnings), "rest1b: shu biznesning IKKINCHI egasi") {
		t.Fatalf("ikkinchi egasi ogohlantirishsiz:\n%s", reasons(p.Warnings))
	}
	w := got["w1"]
	if w == nil || w.BusinessID != byLegacy["r1"].ID || w.RoleID != "tpl_restaurant_waiter" ||
		w.Apps[0] != authz.AppPro || w.Status != authz.MemberSuspended || w.StaffMemberID != "sm1" || w.OrgID != byLegacy["r1"].OrgID {
		t.Fatalf("ofitsiant (ta'tilda): %+v", w)
	}
	if c := got["c_rest"]; c == nil || c.RoleID != "tpl_restaurant_courier" || c.Apps[0] != authz.AppGo || c.Status != authz.MemberActive {
		t.Fatalf("restoran kuryeri: %+v", c)
	}
	if c := got["c_qurt"]; c == nil || c.BusinessID != byLegacy["s1"].ID || c.RoleID != "tpl_store_courier" {
		t.Fatalf("qurt kuryeri: %+v", c)
	}
	for _, id := range []string{"c_plat", "c_none", "w_orphan", "noent"} {
		if got[id] != nil {
			t.Errorf("%s ko'chirilmasligi kerak edi", id)
		}
	}
	if !strings.Contains(reasons(p.Skipped), "c_plat: platforma kuryeri") {
		t.Errorf("platforma kuryeri o'tkazib yuborilmadi:\n%s", reasons(p.Skipped))
	}
	for _, want := range []string{"c_none: kuryer yozuvi topilmadi", "w_orphan:", "noent: entity_id bo'sh"} {
		if !strings.Contains(reasons(p.Warnings), want) {
			t.Errorf("ogohlantirish yo'q: %q\n%s", want, reasons(p.Warnings))
		}
	}
	if len(p.FormerCustomers) != 1 || p.FormerCustomers[0].UserID != "w1" {
		t.Fatalf("avval mijoz bo'lganlar: %+v", p.FormerCustomers)
	}
	if !strings.Contains(p.Summary(), "Xodim a'zoliklari:  3") {
		t.Errorf("hisobot:\n%s", p.Summary())
	}
}

// Takroriy ishga tushirish: mavjud biznes/a'zolik qayta yaratilmaydi, mavjud
// biznesga yangi xodim esa to'g'ri tashkilot bilan qo'shiladi.
func TestBuild_Idempotent(t *testing.T) {
	in := legacymigrate.Input{
		Users:               []*users.User{u("rest1", users.RoleRestaurant, "r1", 0), u("w1", users.RoleWaiter, "r1", 0), u("w2", users.RoleWaiter, "r1", 0)},
		ExistingBusinesses:  map[string]string{"restaurant|r1": "B1"},
		ExistingBusinessOrg: map[string]string{"B1": "O1"},
		ExistingMemberships: map[string]bool{"w1|B1": true},
		NewID:               ids(), Now: t0,
	}
	p := legacymigrate.Build(in)
	if len(p.Orgs)+len(p.Businesses) != 0 || len(p.Memberships) != 1 {
		t.Fatalf("takroriy reja: %s", p.Summary())
	}
	if m := p.Memberships[0]; m.UserID != "w2" || m.BusinessID != "B1" || m.OrgID != "O1" {
		t.Fatalf("yangi xodim: %+v", m)
	}
}
