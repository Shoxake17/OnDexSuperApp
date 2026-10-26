package orders

import (
	"testing"
	"time"
)

func deliveredOrder(createdAt time.Time, durationMinutes int) *Order {
	deliveredAt := createdAt.Add(time.Duration(durationMinutes) * time.Minute)
	return &Order{
		CreatedAt: createdAt,
		History:   []StatusChange{{To: StatusDelivered, At: deliveredAt}},
	}
}

func TestComputeETARange_NoData(t *testing.T) {
	lo, hi := ComputeETARange(nil)
	if lo != 0 || hi != 0 {
		t.Fatalf("ma'lumot yo'qda (0,0) kutilgan, keldi: %d,%d", lo, hi)
	}
}

func TestComputeETARange_RoundsToNearestFive(t *testing.T) {
	now := time.Now()
	list := []*Order{
		deliveredOrder(now, 23), // -> 25
		deliveredOrder(now, 27), // -> 25
	}
	lo, hi := ComputeETARange(list)
	if lo != 25 || hi != 25+etaRoundStepMinutes {
		t.Fatalf("25-30 kutilgan (bitta qiymatga tushsa kengaytiriladi), keldi: %d,%d", lo, hi)
	}
}

func TestComputeETARange_ClampsToBounds(t *testing.T) {
	now := time.Now()
	list := []*Order{
		deliveredOrder(now, 5),   // juda tez -> 20 ga qisiladi
		deliveredOrder(now, 120), // juda sekin -> 40 ga qisiladi
	}
	lo, hi := ComputeETARange(list)
	if lo != etaFloorMinutes || hi != etaCeilMinutes {
		t.Fatalf("20-40 kutilgan, keldi: %d,%d", lo, hi)
	}
}

func TestComputeETARange_IgnoresUnreasonableDurations(t *testing.T) {
	now := time.Now()
	list := []*Order{
		deliveredOrder(now, 28),
		deliveredOrder(now, -5),  // mantiqsiz — chetga chiqariladi
		deliveredOrder(now, 400), // mantiqsiz uzun — chetga chiqariladi
	}
	lo, hi := ComputeETARange(list)
	if lo != 30 || hi != 30+etaRoundStepMinutes {
		t.Fatalf("faqat 28 daqiqalik yozuv hisobga olinishi kerak edi, keldi: %d,%d", lo, hi)
	}
}

func TestComputeETARange_ServedCountsForDineIn(t *testing.T) {
	now := time.Now()
	o := &Order{
		CreatedAt: now,
		History:   []StatusChange{{To: StatusServed, At: now.Add(32 * time.Minute)}},
	}
	lo, hi := ComputeETARange([]*Order{o})
	if lo != 30 || hi != 35 {
		t.Fatalf("30-35 kutilgan, keldi: %d,%d", lo, hi)
	}
}
