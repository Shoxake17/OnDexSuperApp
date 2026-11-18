package httpapi

import (
	"context"
	"log/slog"
	"time"

	"chustapp/internal/notify"
	"chustapp/internal/orders"
)

// ReadyPickupLister — tayyor, olinmagan olib ketish buyurtmalari manbai
// (`storage.PgOrderRepo`, `storage.MemoryOrderRepo`).
type ReadyPickupLister interface {
	ListReadyPickups(ctx context.Context, limit int) ([]*orders.Order, error)
}

const (
	pickupWatchdogEvery = time.Minute
	pickupWatchdogBatch = 500
)

// RunPickupWatchdog — olib ketish nazoratchisi (Plane 473/477), kuryer
// nazoratchisi bilan bir xil naqsh: holat BAZADAN o'qiladi, server qayta
// yonsa hech narsa yo'qolmaydi.
//
//   - tayyor + 30 daq, eslatma yo'q -> mijozga "Buyurtmangiz kutmoqda" (bir marta)
//   - tayyor + 2 soat               -> "olib ketilmadi" (pul qaytmaydi)
func (s *Server) RunPickupWatchdog(ctx context.Context, source ReadyPickupLister) {
	safeGo("pickup-watchdog", func() {
		s.pickupTick(ctx, source)
		t := time.NewTicker(pickupWatchdogEvery)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.pickupTick(ctx, source)
			}
		}
	})
}

func (s *Server) pickupTick(ctx context.Context, source ReadyPickupLister) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("olib ketish nazoratchisi: panika", "panic", r)
		}
	}()
	runCtx, cancel := context.WithTimeout(ctx, 2*pickupWatchdogEvery)
	defer cancel()
	list, err := source.ListReadyPickups(runCtx, pickupWatchdogBatch)
	if err != nil {
		if ctx.Err() == nil {
			slog.Warn("olib ketish nazoratchisi: ro'yxat o'qilmadi", "err", err)
		}
		return
	}
	now := s.OrderSvc.Now()
	for _, o := range list {
		if runCtx.Err() != nil {
			return
		}
		switch {
		case o.PickupNoShow(now):
			done, changed, err := s.OrderSvc.ExpirePickupNoShow(runCtx, o.ID)
			if err != nil {
				slog.Error("olib ketilmadi: yopib bo'lmadi", "order", o.ID, "err", err)
				continue
			}
			if changed {
				slog.Warn("olib ketish buyurtmasi 2 soatda olinmadi — yopildi",
					"order", done.ID, "order_number", done.OrderNumber)
			}
		case o.PickupNeedsReminder(now):
			updated, changed, err := s.OrderSvc.MarkPickupReminded(runCtx, o.ID)
			if err != nil || !changed {
				if err != nil {
					slog.Error("olib ketish eslatmasi: belgilab bo'lmadi", "order", o.ID, "err", err)
				}
				continue
			}
			s.remindPickup(runCtx, updated)
		}
	}
}

// remindPickup — mijozga shaxsiy bildirishnoma (saqlanadi, ilova yopiq
// bo'lsa push ketadi).
func (s *Server) remindPickup(ctx context.Context, o *orders.Order) {
	if s.Notifier == nil || o.CustomerID == "" {
		return
	}
	s.Notifier.Notify(ctx, o.CustomerID, notify.Event{
		Module: notify.ModuleFood,
		Kind:   "pickup_reminder",
		Title:  "Buyurtmangiz sizni kutmoqda",
		Body:   "Taom tayyor — restoranga kelib olish kodini ayting. 2 soat ichida olinmasa buyurtma yopiladi.",
		Data:   map[string]string{"order_id": o.ID, "status": string(o.Status)},
	})
}
