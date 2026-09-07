package httpapi

import (
	"context"
	"testing"
	"time"

	"chustapp/internal/orders"
	"chustapp/internal/storage"
	"chustapp/internal/users"
)

// Mijozni o'chirishdan oldingi "band emasmi" tekshiruvi
// (bug.md 27-band).
//
// ┌─ NEGA BU MUHIM ────────────────────────────────────────────────────┐
// Avval tekshiruv `ListByCustomer(ctx, id, 20)` bilan bajarilardi va
// izohda "faol buyurtma har doim shular orasida bo'ladi" deb
// yozilgandi. Bu NOTO'G'RI: faol mijozda yakunlanmagan buyurtma eng
// yangi 20 tadan pastda qolishi mumkin — o'shanda akkaunt o'chirilardi
// va buyurtma EGASIZ qolardi.
//
// Test aynan shu holatni quradi: 25 ta yakunlangan buyurtma (yangi) +
// 1 ta faol (eski). Eski kod uni KO'RMAYDI.
// └────────────────────────────────────────────────────────────────────┘

func TestBlockIfBusy_FindsActiveOrderBeyondRecentWindow(t *testing.T) {
	ctx := context.Background()
	repo := storage.NewMemoryOrderRepo()

	base := time.Now().Add(-90 * 24 * time.Hour)
	// ESKI, lekin HALI FAOL buyurtma.
	if err := repo.Save(ctx, &orders.Order{
		ID: "eski-faol", CustomerID: "u-cust",
		Status: orders.StatusPreparing, CreatedAt: base,
	}); err != nil {
		t.Fatal(err)
	}
	// Undan keyin 25 ta YAKUNLANGAN buyurtma — "oxirgi 20" oynasini
	// to'ldirish uchun.
	for i := range 25 {
		if err := repo.Save(ctx, &orders.Order{
			ID:         "yangi-" + string(rune('a'+i)),
			CustomerID: "u-cust",
			Status:     orders.StatusDelivered,
			CreatedAt:  base.Add(time.Duration(i+1) * time.Hour),
		}); err != nil {
			t.Fatal(err)
		}
	}

	s := New(Deps{OrderRepo: repo, UserRepo: storage.NewMemoryUserRepo(), DevMode: true})
	u := &users.User{ID: "u-cust", Role: users.RoleCustomer}

	if err := s.blockIfBusy(ctx, u); err == nil {
		t.Fatal("faol buyurtma TOPILMADI — akkaunt o'chirilardi va buyurtma egasiz qolardi")
	}
}

// Barcha buyurtmalar yakunlangan bo'lsa o'chirishga to'siq bo'lmasin.
func TestBlockIfBusy_AllowsWhenEverythingTerminal(t *testing.T) {
	ctx := context.Background()
	repo := storage.NewMemoryOrderRepo()

	// Har bir terminal holat bo'yicha — `served` ham (45-band).
	for i, st := range []orders.Status{
		orders.StatusDelivered, orders.StatusServed,
		orders.StatusRejected, orders.StatusCancelled,
	} {
		if err := repo.Save(ctx, &orders.Order{
			ID: "o" + string(rune('a'+i)), CustomerID: "u-cust",
			Status: st, CreatedAt: time.Now(),
		}); err != nil {
			t.Fatal(err)
		}
	}

	s := New(Deps{OrderRepo: repo, UserRepo: storage.NewMemoryUserRepo(), DevMode: true})
	u := &users.User{ID: "u-cust", Role: users.RoleCustomer}

	if err := s.blockIfBusy(ctx, u); err != nil {
		t.Fatalf("hamma buyurtma yakunlangan, lekin o'chirish bloklandi: %v", err)
	}
}

// Boshqa mijozning faol buyurtmasi bu foydalanuvchini bloklamasligi
// kerak.
func TestBlockIfBusy_IgnoresOtherCustomers(t *testing.T) {
	ctx := context.Background()
	repo := storage.NewMemoryOrderRepo()
	if err := repo.Save(ctx, &orders.Order{
		ID: "begona", CustomerID: "u-boshqa",
		Status: orders.StatusPreparing, CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	s := New(Deps{OrderRepo: repo, UserRepo: storage.NewMemoryUserRepo(), DevMode: true})
	u := &users.User{ID: "u-cust", Role: users.RoleCustomer}

	if err := s.blockIfBusy(ctx, u); err != nil {
		t.Fatalf("begona buyurtma o'chirishni bloklab qo'ydi: %v", err)
	}
}
