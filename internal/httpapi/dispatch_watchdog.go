// Kuryer qidiruvi nazoratchisi.
package httpapi

import (
	"context"
	"log/slog"
	"time"

	"chustapp/internal/orders"
)

// AwaitingCourierLister — kuryer kutayotgan buyurtmalar manbai
// (`storage.PgOrderRepo`, `storage.MemoryOrderRepo`).
type AwaitingCourierLister interface {
	ListAwaitingCourier(ctx context.Context, limit int) ([]*orders.Order, error)
}

const (
	// dispatchWatchdogEvery — nazorat tsikli oralig'i. "Kuryer topilmadi"
	// tugmalari va avtomatik bekor qilish muddatdan ko'pi bilan shuncha
	// kechikadi.
	dispatchWatchdogEvery = 15 * time.Second
	// dispatchWatchdogBatch — bitta tsiklda o'qiladigan eng ko'p buyurtma.
	dispatchWatchdogBatch = 500
	// dispatchRelaunchGap — to'xtab qolgan qidiruvni qayta ishga
	// tushirishlar orasidagi eng qisqa vaqt. Doimiy xato bo'lsa (masalan
	// restoran katalogdan o'chirilgan) log va bildirishnoma bo'roni
	// bo'lmaydi.
	dispatchRelaunchGap = time.Minute
)

// RunDispatchWatchdog — kuryer qidiruvi NAZORATCHISI (ctx tugaguncha).
//
// ┌─ NIMA QILADI ─────────────────────────────────────────────────────┐
// Har tsiklda kuryer kutayotgan buyurtmalarni BAZADAN o'qiydi va:
//
//  1. taom tayyor bo'lganiga `orders.CourierSearchWindow` bo'lib kuryer
//     topilmagan buyurtmani "kuryer topilmadi" qiladi — qidiruv
//     to'xtaydi, restoran paneliga "Bekor qilish" va "Kuryer qidirish"
//     tugmalari chiqadi;
//  2. shu holatda `orders.CourierNotFoundCancelAfter` davomida javob
//     bo'lmasa buyurtmani TIZIM nomidan bekor qiladi;
//  3. qidiruvi ishlamay qolgan buyurtmaga (server qayta ishga tushdi,
//     goroutine xato bilan chiqdi) qidiruvni qayta boshlaydi.
//
// NEGA BUYURTMA BOSHIGA TAYMER EMAS: xotiradagi taymer server qayta
// ishga tushganda yo'qoladi — aynan shu turdagi xato buyurtmalarni
// abadiy "Kuryer qidirilmoqda" holatida qoldirgan edi. Nazoratchi faqat
// bazadagi holat va muddatga qaraydi, shuning uchun hech narsa
// yo'qolmaydi. Birinchi tsikl DARHOL ishlaydi — startdagi tiklash ham
// shu.
// └───────────────────────────────────────────────────────────────────┘
func (s *Server) RunDispatchWatchdog(ctx context.Context, source AwaitingCourierLister) {
	w := newDispatchWatchdog(s, source)
	safeGo("dispatch-watchdog", func() {
		w.runOnce(ctx)
		t := time.NewTicker(dispatchWatchdogEvery)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				w.runOnce(ctx)
			}
		}
	})
}

type dispatchWatchdog struct {
	s      *Server
	source AwaitingCourierLister
	// relaunched — qidiruv oxirgi marta qachon qayta boshlangani.
	// Faqat nazoratchi goroutine'i ishlatadi — qulf kerak emas.
	relaunched map[string]time.Time
}

func newDispatchWatchdog(s *Server, source AwaitingCourierLister) *dispatchWatchdog {
	return &dispatchWatchdog{s: s, source: source, relaunched: map[string]time.Time{}}
}

// runOnce — bitta tsikl. Panika nazoratchini TO'XTATMAYDI: keyingi tsikl
// baribir ishlaydi (aks holda bitta buzuq yozuv butun mexanizmni
// jimgina o'chirib qo'yardi).
func (w *dispatchWatchdog) runOnce(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("dispatch nazoratchisi: panika", "panic", r)
		}
	}()
	runCtx, cancel := context.WithTimeout(ctx, 2*dispatchWatchdogEvery)
	defer cancel()
	if err := w.tick(runCtx); err != nil && ctx.Err() == nil {
		slog.Warn("dispatch nazoratchisi: tsikl xatosi", "err", err)
	}
}

func (w *dispatchWatchdog) tick(ctx context.Context) error {
	list, err := w.source.ListAwaitingCourier(ctx, dispatchWatchdogBatch)
	if err != nil {
		return err
	}
	if len(list) >= dispatchWatchdogBatch {
		slog.Warn("dispatch nazoratchisi: kuryer kutayotgan buyurtmalar juda ko'p — qolgani keyingi tsiklda",
			"limit", dispatchWatchdogBatch)
	}
	seen := make(map[string]struct{}, len(list))
	for _, o := range list {
		if err := ctx.Err(); err != nil {
			return err
		}
		seen[o.ID] = struct{}{}
		// Bitta buyurtmaning xatosi qolganlarini to'xtatmaydi.
		if err := w.check(ctx, o); err != nil {
			slog.Error("dispatch nazoratchisi: buyurtmani tekshirib bo'lmadi", "order", o.ID, "err", err)
		}
	}
	for id := range w.relaunched {
		if _, ok := seen[id]; !ok {
			delete(w.relaunched, id)
		}
	}
	return nil
}

// check — bitta buyurtma bo'yicha qaror. Har bir o'zgartirish
// `orders.Service` orqali optimistik qulf bilan, shartni QAYTA tekshirib
// yoziladi: ro'yxat o'qilgandan beri restoran tugma bosgan yoki kuryer
// topilgan bo'lsa, eskirgan qaror qo'llanmaydi.
func (w *dispatchWatchdog) check(ctx context.Context, o *orders.Order) error {
	svc := w.s.OrderSvc
	if o.Status == orders.StatusReady && o.DispatchState != orders.DispatchNotFound && o.DispatchDeadline == nil {
		updated, _, err := svc.EnsureCourierSearchDeadline(ctx, o.ID)
		if err != nil {
			return err
		}
		o = updated
	}
	now := svc.Now()
	switch {
	case o.CourierSearchExpired(now):
		updated, changed, err := svc.MarkCourierNotFound(ctx, o.ID)
		if err != nil || !changed {
			return err
		}
		w.s.stopDispatch(updated.ID)
		slog.Warn("kuryer topilmadi — restoran qarori kutilmoqda",
			"order", updated.ID, "order_number", updated.OrderNumber, "avto_bekor", updated.DispatchDeadline)
		w.s.publishDispatchState("courier_not_found", updated)
		if w.s.AlertsSvc != nil && updated.DispatchDeadline != nil {
			w.s.AlertsSvc.CourierNotFound(updated.RestaurantID, updated.ID, updated.OrderNumber, *updated.DispatchDeadline)
		}
	case o.CourierNotFoundExpired(now):
		cancelled, changed, err := svc.AutoCancelCourierNotFound(ctx, o.ID)
		if err != nil || !changed {
			return err
		}
		// Bekor qilishning o'zi (to'lov qaytarish, mijoz/restoran/admin
		// xabarlari) `ChangeStatus` ichida bajarildi.
		w.s.stopDispatch(cancelled.ID)
		slog.Warn("kuryer topilmagani uchun buyurtma avtomatik bekor qilindi",
			"order", cancelled.ID, "order_number", cancelled.OrderNumber)
	case o.NeedsDispatchRecovery():
		if w.s.Dispatcher == nil || w.s.Dispatcher.IsRunning(o.ID) {
			return nil
		}
		if last, ok := w.relaunched[o.ID]; ok && now.Sub(last) < dispatchRelaunchGap {
			return nil
		}
		w.relaunched[o.ID] = now
		slog.Warn("dispatch nazoratchisi: kuryer qidiruvi ishlamayapti — qayta boshlandi",
			"order", o.ID, "status", o.Status)
		w.s.launchDispatch(o)
	}
	return nil
}
