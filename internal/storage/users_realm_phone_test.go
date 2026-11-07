package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"chustapp/internal/users"
)

// 0069: telefon REALM ichida unikal. Xotira va Postgres ombori bir xil.
func TestUsers_PhoneUniquePerRealm(t *testing.T) {
	run := func(t *testing.T, repo users.Repository, cleanup func()) {
		ctx := context.Background()
		const phone = "+998987650401"
		cleanup()
		t.Cleanup(cleanup)
		t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		mk := func(id string, role users.Role, at time.Time) error {
			return repo.Create(ctx, &users.User{ID: id, Phone: phone, Role: role, CreatedAt: at, PhoneVerified: true})
		}
		if err := mk("rp_cust", users.RoleCustomer, t0); err != nil {
			t.Fatal(err)
		}
		if err := mk("rp_cust2", users.RoleCustomer, t0.Add(time.Hour)); !errors.Is(err, users.ErrPhoneTaken) {
			t.Fatalf("bir realmda ikki akkaunt: %v", err)
		}
		if err := mk("rp_merch", users.RoleMerchant, t0.Add(time.Hour)); err != nil {
			t.Fatalf("mijoz raqamiga merchant akkaunt ochilmadi: %v", err)
		}
		if err := mk("rp_waiter", users.RoleWaiter, t0.Add(2*time.Hour)); !errors.Is(err, users.ErrPhoneTaken) {
			t.Fatalf("merchant realmida ikkinchi akkaunt: %v", err)
		}
		// eski qidiruv — ENG ESKISI (0069 dan oldingi natija o'zgarmaydi)
		if u, err := repo.GetByPhone(ctx, phone); err != nil || u.ID != "rp_cust" {
			t.Fatalf("GetByPhone: %v %v, kutilgan rp_cust", u, err)
		}
		for realm, want := range map[users.Realm]string{users.RealmCustomer: "rp_cust", users.RealmMerchant: "rp_merch"} {
			if u, err := repo.GetByPhoneRealm(ctx, realm, phone); err != nil || u.ID != want {
				t.Fatalf("GetByPhoneRealm(%s): %v %v", realm, u, err)
			}
		}
		if _, err := repo.GetByPhoneRealm(ctx, users.RealmPlatform, phone); !errors.Is(err, users.ErrUserNotFound) {
			t.Fatalf("platform realmi: %v", err)
		}
		// eski K4 yo'li: mijozni xodimga aylantirish — merchant akkaunt bor, rad
		if err := repo.UpdateRole(ctx, "rp_cust", users.RoleWaiter, "r1"); !errors.Is(err, users.ErrPhoneTaken) {
			t.Fatalf("UpdateRole to'qnashuvi: %v", err)
		}
	}

	t.Run("xotira", func(t *testing.T) {
		run(t, NewMemoryUserRepo(), func() {})
	})
	t.Run("postgres", func(t *testing.T) {
		pool := realmTestPool(t)
		run(t, NewPgUserRepo(pool), func() {
			_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id LIKE 'rp_%'`)
		})
	})
}
