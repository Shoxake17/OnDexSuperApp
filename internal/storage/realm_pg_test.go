package storage

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/users"
)

// testPGPool — haqiqiy Postgres (TEST_DATABASE_URL). CI'da baza bo'lmasa test
// YIQILADI (jim o'tkazib yuborish taqiqlangan, bug.md 102-band).
func realmTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("CI'da TEST_DATABASE_URL BO'LISHI SHART")
		}
		t.Skip("TEST_DATABASE_URL berilmagan — Postgres testi o'tkazib yuborildi")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return pool
}

// Bazadagi `ondex_realm_for_role` (0063) va Go'dagi `users.RealmForRole` AYNAN
// bir xil bo'lishi SHART — trigger ham, kod ham shu jadvalga tayanadi.
func TestRealm_SQLMatchesGo(t *testing.T) {
	pool := realmTestPool(t)
	ctx := context.Background()
	roles := []users.Role{users.RoleCustomer, users.RoleCourier, users.RoleRestaurant,
		users.RoleWaiter, users.RoleAdmin, users.RoleStore, "", "noma_lum"}
	for _, role := range roles {
		var got string
		if err := pool.QueryRow(ctx, `SELECT ondex_realm_for_role($1)`, string(role)).Scan(&got); err != nil {
			t.Fatalf("ondex_realm_for_role(%q): %v", role, err)
		}
		if want := string(users.RealmForRole(role)); got != want {
			t.Errorf("rol %q: SQL %q, Go %q — 0063 va realm.go farq qiladi", role, got, want)
		}
	}
}

// Trigger: rol qanday yozilmasin (repo, seed, xom SQL), realm roldan
// hisoblanadi; realmni qo'lda boshqa qiymatga qo'yib bo'lmaydi (A1).
func TestRealm_TriggerKeepsRealmInSync(t *testing.T) {
	pool := realmTestPool(t)
	ctx := context.Background()
	const id = "realmtest_u1"
	const phone = "+998987650001"
	cleanup := func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id) }
	cleanup()
	t.Cleanup(cleanup)

	repo := NewPgUserRepo(pool)
	if err := repo.Create(ctx, &users.User{ID: id, Phone: phone, Role: users.RoleCustomer,
		CreatedAt: time.Now(), PhoneVerified: true}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	read := func() users.Realm {
		u, err := repo.GetByID(ctx, id)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		return u.Realm
	}
	if r := read(); r != users.RealmCustomer {
		t.Fatalf("yangi mijoz realmi %q", r)
	}

	if err := repo.UpdateRole(ctx, id, users.RoleWaiter, "r1"); err != nil {
		t.Fatalf("UpdateRole: %v", err)
	}
	if r := read(); r != users.RealmMerchant {
		t.Fatalf("ofitsiant realmi %q, kutilgan merchant", r)
	}

	// Xom SQL bilan rol o'zgarsa ham (PromoteToAdmin naqshi) — trigger ushlaydi.
	if _, err := pool.Exec(ctx, `UPDATE users SET role = 'admin', entity_id = '' WHERE id = $1`, id); err != nil {
		t.Fatalf("xom UPDATE: %v", err)
	}
	if r := read(); r != users.RealmPlatform {
		t.Fatalf("admin realmi %q, kutilgan platform", r)
	}

	// Realmni qo'lda o'zgartirishga urinish — trigger roldan qaytadan hisoblaydi.
	if _, err := pool.Exec(ctx, `UPDATE users SET realm = 'customer' WHERE id = $1`, id); err != nil {
		t.Fatalf("realm UPDATE: %v", err)
	}
	if r := read(); r != users.RealmPlatform {
		t.Fatalf("realm qo'lda o'zgarib qoldi: %q", r)
	}
}

// Audit jurnali faqat qo'shiladi: UPDATE, DELETE, TRUNCATE rad etiladi.
func TestAuditLog_AppendOnly(t *testing.T) {
	pool := realmTestPool(t)
	ctx := context.Background()
	var id int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO audit_log (actor_user_id, action, result) VALUES ('realmtest', 'test.append_only', 'ok') RETURNING id`,
	).Scan(&id); err != nil {
		t.Fatalf("INSERT: %v", err)
	}
	for name, q := range map[string]string{
		"UPDATE":   `UPDATE audit_log SET result = 'failed' WHERE id = $1`,
		"DELETE":   `DELETE FROM audit_log WHERE id = $1`,
		"TRUNCATE": `TRUNCATE audit_log`,
	} {
		var err error
		if name == "TRUNCATE" {
			_, err = pool.Exec(ctx, q)
		} else {
			_, err = pool.Exec(ctx, q, id)
		}
		if err == nil {
			t.Errorf("%s audit_log'da o'tib ketdi — faqat qo'shish bo'lishi kerak", name)
		}
	}
	var result string
	if err := pool.QueryRow(ctx, `SELECT result FROM audit_log WHERE id = $1`, id).Scan(&result); err != nil || result != "ok" {
		t.Fatalf("yozuv o'zgargan yoki yo'qolgan: %q %v", result, err)
	}
}

// A'zolik CHECK'lari: egasi biznessiz/rolsiz, xodim biznes+rol bilan;
// tashkilotda bitta faol egasi.
func TestMemberships_Constraints(t *testing.T) {
	pool := realmTestPool(t)
	ctx := context.Background()
	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM memberships WHERE id LIKE 'mtest_%'`)
		_, _ = pool.Exec(ctx, `DELETE FROM roles WHERE id LIKE 'mtest_%'`)
		_, _ = pool.Exec(ctx, `DELETE FROM businesses WHERE id LIKE 'mtest_%'`)
		_, _ = pool.Exec(ctx, `DELETE FROM organizations WHERE id LIKE 'mtest_%'`)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id LIKE 'mtest_%'`)
	}
	cleanup()
	t.Cleanup(cleanup)

	must := func(q string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	mustFail := func(why, q string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, q, args...); err == nil {
			t.Errorf("o'tib ketmasligi kerak edi: %s", why)
		}
	}

	must(`INSERT INTO users (id, phone, role) VALUES ('mtest_owner', '+998987650010', 'restaurant'),
	                                                  ('mtest_staff', '+998987650011', 'waiter'),
	                                                  ('mtest_other', '+998987650012', 'restaurant')`)
	must(`INSERT INTO organizations (id, name, owner_user_id) VALUES ('mtest_org', 'Test MCHJ', 'mtest_owner')`)
	mustFail("faol tashkilot tasdiqsiz", `UPDATE organizations SET status = 'active' WHERE id = 'mtest_org'`)
	mustFail("STIR 9 raqam emas", `UPDATE organizations SET tin = '12ab' WHERE id = 'mtest_org'`)
	must(`INSERT INTO businesses (id, org_id, type, name) VALUES ('mtest_biz', 'mtest_org', 'restaurant', 'Book Cafe')`)
	must(`INSERT INTO roles (id, business_type, key, name, permissions) VALUES ('mtest_role', 'restaurant', 'mtest_waiter', 'Ofitsiant', '{orders.read}')`)

	must(`INSERT INTO memberships (id, user_id, org_id, is_owner, status) VALUES ('mtest_m_owner', 'mtest_owner', 'mtest_org', TRUE, 'active')`)
	mustFail("ikkinchi egasi", `INSERT INTO memberships (id, user_id, org_id, is_owner, status) VALUES ('mtest_m_owner2', 'mtest_other', 'mtest_org', TRUE, 'active')`)
	mustFail("egasida biznes", `INSERT INTO memberships (id, user_id, org_id, business_id, is_owner) VALUES ('mtest_bad1', 'mtest_other', 'mtest_org', 'mtest_biz', TRUE)`)
	mustFail("xodim rolsiz", `INSERT INTO memberships (id, user_id, org_id, business_id) VALUES ('mtest_bad2', 'mtest_staff', 'mtest_org', 'mtest_biz')`)
	mustFail("noma'lum ilova", `INSERT INTO memberships (id, user_id, org_id, business_id, role_id, apps) VALUES ('mtest_bad3', 'mtest_staff', 'mtest_org', 'mtest_biz', 'mtest_role', '{admin}')`)
	must(`INSERT INTO memberships (id, user_id, org_id, business_id, role_id, apps) VALUES ('mtest_m_staff', 'mtest_staff', 'mtest_org', 'mtest_biz', 'mtest_role', '{pro}')`)
	mustFail("bir biznesda ikki a'zolik", `INSERT INTO memberships (id, user_id, org_id, business_id, role_id) VALUES ('mtest_dup', 'mtest_staff', 'mtest_org', 'mtest_biz', 'mtest_role')`)
}
