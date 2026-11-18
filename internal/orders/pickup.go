package orders

import (
	"context"
	"errors"
	"time"
)

// Olib ketish (Borish, Plane 473/477) — egasi qarori 2026-11-18:
// tayyor bo'lgach 30 daqiqada mijozga eslatma, 2 soatda "olib ketilmadi"
// (pul QAYTMAYDI — taom tayyorlangan).
const (
	PickupRemindAfter = 30 * time.Minute
	PickupNoShowAfter = 2 * time.Hour
)

// PickupReadySince — buyurtma qachon "tayyor" bo'lgani (history'dan;
// ReadyAt — TAXMINIY vaqt, u emas).
func (o *Order) PickupReadySince() (time.Time, bool) {
	if !o.IsPickup() || o.Status != StatusReady {
		return time.Time{}, false
	}
	return o.lastChangeTo(StatusReady)
}

// PickupNeedsReminder — eslatma vaqti keldi va hali yuborilmagan.
func (o *Order) PickupNeedsReminder(now time.Time) bool {
	at, ok := o.PickupReadySince()
	return ok && o.PickupRemindedAt == nil && now.Sub(at) >= PickupRemindAfter
}

// PickupNoShow — 2 soat o'tdi, mijoz kelmadi.
func (o *Order) PickupNoShow(now time.Time) bool {
	at, ok := o.PickupReadySince()
	return ok && now.Sub(at) >= PickupNoShowAfter
}

// MarkPickupReminded — eslatma BIR MARTA: belgi optimistik qulf bilan
// yoziladi, changed=true bo'lgandagina chaqiruvchi push yuboradi.
func (s *Service) MarkPickupReminded(ctx context.Context, orderID string) (*Order, bool, error) {
	return s.updateDispatch(ctx, orderID, func(o *Order, now time.Time) (bool, error) {
		if !o.PickupNeedsReminder(now) {
			return false, nil
		}
		t := now
		o.PickupRemindedAt = &t
		return true, nil
	})
}

// ExpirePickupNoShow — "olib ketilmadi" (tizim nomidan). Oraliqda restoran
// kod bilan bergan bo'lsa o'tish rad etiladi — changed=false.
func (s *Service) ExpirePickupNoShow(ctx context.Context, orderID string) (*Order, bool, error) {
	o, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return nil, false, err
	}
	if !o.PickupNoShow(s.now()) {
		return o, false, nil
	}
	done, err := s.ChangeStatus(ctx, orderID, StatusNotPickedUp, ActorSystem)
	if err != nil {
		var terr *TransitionError
		if errors.As(err, &terr) || errors.Is(err, ErrConflict) {
			return o, false, nil
		}
		return nil, false, err
	}
	return done, true, nil
}
