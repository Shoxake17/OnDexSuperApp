package payments

import (
	"context"
	"log/slog"
)

// CloseAbandoned — to'lovi hech qachon boshlanmagan (yoki barcha urinishlari
// yopilgan) karta buyurtmalarini bekor qiladi.
//
// ┌─ NEGA KERAK (audit 480, 2026-11-19) ──────────────────────────────┐
// `ExpireStale` faqat TO'LOV YOZUVI bor urinishlarni yopadi. Mijoz
// buyurtma yaratib, to'lov sahifasini ochmasdan chiqib ketsa (ilova
// yopildi, tarmoq uzildi) — to'lov yozuvi umuman yo'q va buyurtma
// `created / awaiting` holatida ABADIY qolardi (prod'da 60 kunlik 5 ta
// shunday buyurtma topildi).
//
// Xavfsizlik: buyurtmaning BIRORTA urinishi pending/held/paid bo'lsa —
// tegilmaydi: pul yechilgan bo'lishi mumkin, uni `ExpireStale`
// (provayderdan so'rab) yoki callback hal qiladi.
// └───────────────────────────────────────────────────────────────────┘
func (s *Service) CloseAbandoned(ctx context.Context, orderIDs []string) (int, error) {
	n := 0
	for _, id := range orderIDs {
		if err := ctx.Err(); err != nil {
			return n, err
		}
		attempts, err := s.repo.ListByOrder(ctx, id)
		if err != nil {
			return n, err
		}
		live := false
		for _, p := range attempts {
			switch p.Status {
			case StatusFailed, StatusCanceled:
			default:
				live = true
			}
		}
		if live {
			continue
		}
		if err := s.orders.OnPaymentFailed(ctx, id); err != nil {
			slog.Error("tashlab ketilgan to'lov: buyurtmani yopib bo'lmadi", "order", id, "err", err)
			continue
		}
		slog.Info("to'lov boshlanmagan buyurtma bekor qilindi", "order", id, "urinishlar", len(attempts))
		n++
	}
	return n, nil
}
