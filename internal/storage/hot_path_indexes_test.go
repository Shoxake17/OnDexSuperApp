package storage

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"chustapp/internal/orders"
	"chustapp/internal/users"
)

// Qisman indeks faqat so'rov sharti indeks shartiga HARFMA-HARF mos kelsa
// ishlatiladi — migratsiya va kod ajralib ketmasin.
func TestActiveCourierIndexMatchesQuery(t *testing.T) {
	sqlBytes, err := migrationFS.ReadFile("migrations/0054_hot_path_indexes.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sqlBytes), "WHERE "+activeOrderStatusFilter+";") {
		t.Fatalf("migratsiya sharti so'rovdagidan farq qiladi: %q", activeOrderStatusFilter)
	}
	for _, s := range orders.TerminalStatusStrings() {
		if !strings.Contains(activeOrderStatusFilter, "'"+s+"'") {
			t.Fatalf("yakunlangan holat %q shartda yo'q", s)
		}
	}
}

func TestSQLStatusListRejectsUnsafe(t *testing.T) {
	if got := sqlStatusList([]string{"delivered", "picked_up"}); got != "'delivered', 'picked_up'" {
		t.Fatalf("ro'yxat: %q", got)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("xavfli nom qabul qilindi")
		}
	}()
	sqlStatusList([]string{"x'); DROP TABLE orders; --"})
}

func TestMemoryUserGetByRoleEntity(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	repo := NewMemoryUserRepo()
	for _, u := range []users.User{
		{ID: "u-new", Phone: "+998900000002", Role: users.RoleRestaurant, EntityID: "r1", CreatedAt: now},
		{ID: "u-old", Phone: "+998900000001", Role: users.RoleRestaurant, EntityID: "r1", CreatedAt: now.Add(-time.Hour)},
		{ID: "u-waiter", Phone: "+998900000003", Role: users.RoleWaiter, EntityID: "r1", CreatedAt: now.Add(-2 * time.Hour)},
	} {
		if err := repo.Create(ctx, &u); err != nil {
			t.Fatal(err)
		}
	}
	// Eng eski akkaunt — avvalgi `ListByRole(... ORDER BY created_at)` bilan bir xil.
	got, err := repo.GetByRoleEntity(ctx, users.RoleRestaurant, "r1")
	if err != nil || got.ID != "u-old" {
		t.Fatalf("kutilgan u-old, keldi %+v %v", got, err)
	}
	if _, err := repo.GetByRoleEntity(ctx, users.RoleRestaurant, "r2"); !errors.Is(err, users.ErrUserNotFound) {
		t.Fatalf("topilmasa ErrUserNotFound: %v", err)
	}
	if _, err := repo.GetByRoleEntity(ctx, users.RoleCourier, "r1"); !errors.Is(err, users.ErrUserNotFound) {
		t.Fatalf("boshqa rol: %v", err)
	}
}
