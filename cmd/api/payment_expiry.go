package main

import (
	"context"
	"log/slog"
	"time"

	"chustapp/internal/payments"
)

// abandonedUnpaidLister — to'lovi kutilayotgan eski karta buyurtmalari
// (`storage.PgOrderRepo`, `storage.MemoryOrderRepo`).
type abandonedUnpaidLister interface {
	ListAbandonedUnpaid(ctx context.Context, before time.Time, limit int) ([]string, error)
}

const (
	paymentExpiryEvery = time.Minute
	paymentExpiryBatch = 200
	// abandonedUnpaidAfter — buyurtma yaratilgandan shuncha vaqt o'tib ham
	// jonli to'lov urinishi bo'lmasa, u bekor qilinadi. To'lov havolasining
	// muddatidan (PAYMENT_TTL_MINUTES, odatda 15–30 daq) ataylab uzunroq:
	// sekin bank OTP'si yoki qayta urinish bilan to'layotgan mijozga tegmaydi.
	abandonedUnpaidAfter = time.Hour
)

// runPaymentExpiry — to'lov muddati nazoratchisi (audit 480, 2026-11-19).
//
// `payments.Service.ExpireStale` yozilgan va testlangan edi, lekin prod'da
// HECH QAYERDA chaqirilmagan: muddati o'tgan urinishlar `pending`, karta
// buyurtmalari esa `created / awaiting` holatida abadiy qolardi (bron
// zakladi va stol hisobi ulushlari ham). Endi har daqiqada:
//  1. muddati o'tgan urinishlar provayderdan so'ralib yopiladi;
//  2. to'lovi umuman boshlanmagan eski karta buyurtmalari bekor qilinadi.
//
// Bir nechta server nusxasida ham xavfsiz: holat o'tishlari optimistik
// qulf bilan yoziladi, takroriy OnPaymentFailed — no-op.
func runPaymentExpiry(ctx context.Context, svc *payments.Service, orders abandonedUnpaidLister) {
	tick := time.NewTicker(paymentExpiryEvery)
	defer tick.Stop()
	for {
		paymentExpiryOnce(ctx, svc, orders)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func paymentExpiryOnce(ctx context.Context, svc *payments.Service, orders abandonedUnpaidLister) {
	c, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if n, err := svc.ExpireStale(c, paymentExpiryBatch); err != nil {
		slog.Error("to'lov muddati: urinishlarni yopib bo'lmadi", "err", err)
	} else if n > 0 {
		slog.Info("to'lov muddati: muddati o'tgan urinishlar yopildi", "count", n)
	}
	if orders == nil {
		return
	}
	ids, err := orders.ListAbandonedUnpaid(c, time.Now().Add(-abandonedUnpaidAfter), paymentExpiryBatch)
	if err != nil {
		slog.Error("to'lov muddati: tashlab ketilgan buyurtmalarni o'qib bo'lmadi", "err", err)
		return
	}
	if len(ids) == 0 {
		return
	}
	if _, err := svc.CloseAbandoned(c, ids); err != nil {
		slog.Error("to'lov muddati: tashlab ketilgan buyurtmalarni yopib bo'lmadi", "err", err)
	}
}
