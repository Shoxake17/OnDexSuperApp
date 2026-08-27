package storage

import (
	"context"
	"testing"

	"chustapp/internal/orders"
)

// To'lanmagan KARTA buyurtmasi restoran ro'yxatiga TUSHMASLIGI kerak.
//
// Jonli sinovda aynan shu bo'shliq topilgan edi: buyurtmani qabul
// qilib bo'lmasdi (`ChangeStatus` to'sardi), lekin u panelda ko'rinib
// turardi va xodim "Qabul qilish" bosib xato olardi.
func TestListByRestaurantHidesUnpaidCardOrders(t *testing.T) {
	repo := NewMemoryOrderRepo()
	ctx := context.Background()

	save := func(o *orders.Order) {
		t.Helper()
		if err := repo.Save(ctx, o); err != nil {
			t.Fatal(err)
		}
	}

	// 1. To'lov kutayotgan karta buyurtmasi — KO'RINMASLIGI kerak.
	save(&orders.Order{
		ID: "awaiting", RestaurantID: "r1", Status: orders.StatusCreated,
		PaymentMethod: orders.PaymentCard, PaymentState: orders.PaymentAwaiting,
	})
	// 2. Puli bloklangan karta buyurtmasi — KO'RINADI.
	save(&orders.Order{
		ID: "held", RestaurantID: "r1", Status: orders.StatusCreated,
		PaymentMethod: orders.PaymentCard, PaymentState: orders.PaymentHeld,
	})
	// 3. Naqd buyurtma — har doim ko'rinadi.
	save(&orders.Order{
		ID: "cash", RestaurantID: "r1", Status: orders.StatusCreated,
		PaymentMethod: orders.PaymentCash,
	})
	// 4. To'lovi amalga oshmagan karta buyurtmasi — ko'rinmaydi.
	save(&orders.Order{
		ID: "failed", RestaurantID: "r1", Status: orders.StatusCreated,
		PaymentMethod: orders.PaymentCard, PaymentState: orders.PaymentFailed,
	})

	list, err := repo.ListByRestaurant(ctx, "r1", 50)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, o := range list {
		seen[o.ID] = true
	}
	if seen["awaiting"] {
		t.Error("TO'LANMAGAN karta buyurtmasi restoran ro'yxatida ko'rindi")
	}
	if seen["failed"] {
		t.Error("to'lovi amalga oshmagan buyurtma ro'yxatda ko'rindi")
	}
	if !seen["held"] {
		t.Error("puli bloklangan buyurtma ro'yxatda BO'LISHI kerak")
	}
	if !seen["cash"] {
		t.Error("naqd buyurtma ro'yxatda BO'LISHI kerak")
	}
}
