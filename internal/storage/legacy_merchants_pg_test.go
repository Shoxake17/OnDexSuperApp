package storage

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"chustapp/internal/identity/authz"
	"chustapp/internal/identity/legacymigrate"
)

// A1b: haqiqiy bazada o'qish -> reja -> qo'llash; ikkinchi marta hech narsa
// qo'shilmaydi; natija authz orqali ishlaydi (egasi va xodim huquqi).
func TestLegacyMerchantsMigration(t *testing.T) {
	pool := realmTestPool(t)
	ctx := context.Background()
	clean := func() {
		for _, q := range []string{
			`DELETE FROM memberships WHERE user_id LIKE 'lm_%'`,
			`DELETE FROM businesses WHERE legacy_entity_id LIKE 'lm_%'`,
			`DELETE FROM organizations WHERE owner_user_id LIKE 'lm_%'`,
			`DELETE FROM orders WHERE id LIKE 'lm_%'`,
			`DELETE FROM staff_members WHERE id LIKE 'lm_%'`,
			`DELETE FROM couriers WHERE id LIKE 'lm_%'`,
			`DELETE FROM users WHERE id LIKE 'lm_%'`,
		} {
			if _, err := pool.Exec(ctx, q); err != nil {
				t.Fatalf("%s: %v", q, err)
			}
		}
	}
	// Bazada BOSHQA eski akkauntlar ham bo'lishi mumkin — test faqat o'z
	// yozuvlarini tekshiradi (reja hammasini o'z ichiga oladi).
	clean()
	t.Cleanup(clean)
	must := func(q string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	now := time.Now().UTC()
	must(`INSERT INTO users (id, phone, name, role, entity_id, created_at) VALUES
		('lm_rest', '+998987650501', 'Ali (xodim)', 'restaurant', 'lm_r1', $1),
		('lm_store', '+998987650502', 'Qurt do''koni', 'store', 'lm_s1', $1),
		('lm_waiter', '+998987650503', 'Vali', 'waiter', 'lm_r1', $1),
		('lm_cour', '+998987650504', 'Kuryer', 'courier', 'lm_c1', $1),
		('lm_qcour', '+998987650505', 'Qurt kuryer', 'courier', 'lm_c2', $1)`, now)
	must(`INSERT INTO couriers (id, name, restaurant_id) VALUES ('lm_c1', 'K', 'lm_r1'), ('lm_c2', 'Q', 'qurt:lm_s1')`)
	must(`INSERT INTO staff_members (id, restaurant_id, number, first_name, phone, position, user_id)
		VALUES ('lm_sm1', 'lm_r1', 1, 'Vali', '+998987650503', 'waiter', 'lm_waiter')`)
	must(`INSERT INTO orders (id, customer_id, restaurant_id, status, created_at, updated_at)
		VALUES ('lm_o1', 'lm_waiter', 'x', 'delivered', $1, $1)`, now)

	n := 0
	plan := func() *legacymigrate.Plan {
		in, err := LoadLegacyMerchants(ctx, pool)
		if err != nil {
			t.Fatal(err)
		}
		// Faqat shu testning akkauntlari: bazadagi boshqa testlarning eski
		// akkauntlariga a'zolik yozilsa, ularning tozalashi (FK) buziladi.
		mine := in.Users[:0]
		for _, u := range in.Users {
			if strings.HasPrefix(u.ID, "lm_") {
				mine = append(mine, u)
			}
		}
		in.Users = mine
		in.RestaurantNames["lm_r1"] = "Book Cafe"
		in.Now = now
		in.NewID = func() string { n++; return fmt.Sprintf("lm_id_%d_%d", now.UnixNano(), n) }
		return legacymigrate.Build(*in)
	}
	if err := LegacySchemaReady(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := authz.EnsureTemplates(ctx, NewPgAuthzStore(pool)); err != nil {
		t.Fatal(err)
	}
	p := plan()
	former := false
	for _, f := range p.FormerCustomers {
		former = former || f.UserID == "lm_waiter"
	}
	if !former {
		t.Fatalf("avval mijoz bo'lgan ofitsiant hisobotda yo'q:\n%s", p.Summary())
	}
	if _, err := ApplyLegacyPlan(ctx, pool, p, now); err != nil {
		t.Fatal(err)
	}

	// Ikkinchi marta: bizning yozuvlar uchun hech narsa rejalashtirilmaydi.
	for _, m := range plan().Memberships {
		if strings.HasPrefix(m.UserID, "lm_") {
			t.Fatalf("takroriy ishga tushirishda yana rejalashtirildi: %+v", m)
		}
	}

	az := authz.New(NewPgAuthzStore(pool))
	var restBiz, storeBiz string
	if err := pool.QueryRow(ctx, `SELECT id FROM businesses WHERE type = 'restaurant' AND legacy_entity_id = 'lm_r1'`).Scan(&restBiz); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM businesses WHERE type = 'store' AND legacy_entity_id = 'lm_s1'`).Scan(&storeBiz); err != nil {
		t.Fatal(err)
	}
	if acc, err := az.Require(ctx, "lm_rest", restBiz, authz.PermStaffManage); err != nil || !acc.IsOwner || acc.OrgStatus != authz.OrgActive || acc.Business.Name != "Book Cafe" {
		t.Fatalf("egasi: %+v %v", acc, err)
	}
	if acc, err := az.Require(ctx, "lm_waiter", restBiz, authz.PermDineInServe); err != nil || !acc.AllowsApp(authz.AppPro) {
		t.Fatalf("ofitsiant: %+v %v", acc, err)
	}
	if acc, err := az.Require(ctx, "lm_qcour", storeBiz, authz.PermDeliveryRun); err != nil || !acc.AllowsApp(authz.AppGo) {
		t.Fatalf("qurt kuryeri: %+v %v", acc, err)
	}
	if _, err := az.Check(ctx, "lm_cour", storeBiz); err == nil {
		t.Fatal("restoran kuryeri qurt do'koniga kirdi")
	}
	var role string
	if err := pool.QueryRow(ctx, `SELECT role FROM users WHERE id = 'lm_waiter'`).Scan(&role); err != nil || role != "waiter" {
		t.Fatalf("eski rol o'zgardi: %q %v", role, err)
	}
}
