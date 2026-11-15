package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"chustapp/internal/alerts"
	"chustapp/internal/catalog"
	"chustapp/internal/modules/reservation"
	"chustapp/internal/notify"
	"chustapp/internal/payments"
)

// ┌─ BRON MODULI PORTLARI (RS-3) ─────────────────────────────────────────┐
// `internal/modules/reservation` stol, ish vaqti, to'lov, hamyon va
// bildirishnomani o'zi bilmaydi — shu adapterlar orqali oladi. Modul bu
// paketlarni import qilmaydi (ADR-003 chegarasi), httpapi esa ularning
// hammasini allaqachon biladi.
// └──────────────────────────────────────────────────────────────────────┘

// ReservationDeps — main.go `reservation.New` ga beradigan portlar.
func (s *Server) ReservationDeps() (reservation.Tables, reservation.Hours, reservation.Payments, reservation.Wallet, reservation.Notifier) {
	var pay reservation.Payments
	if s.Payments != nil {
		pay = rsvPayments{s.Payments}
	}
	var wal reservation.Wallet
	if s.WalletSvc != nil {
		wal = rsvWallet{s}
	}
	var hours reservation.Hours
	if s.CatalogRepo != nil {
		hours = rsvHours{s.CatalogRepo}
	}
	return rsvTables{s}, hours, pay, wal, rsvNotifier{s}
}

type rsvTables struct{ s *Server }

func (a rsvTables) ListTables(ctx context.Context, restaurantID string) ([]reservation.Table, error) {
	if a.s.TableSvc == nil {
		return nil, nil
	}
	list, err := a.s.TableSvc.List(ctx, restaurantID)
	if err != nil {
		return nil, err
	}
	out := make([]reservation.Table, 0, len(list))
	for _, t := range list {
		c := 0
		if t.Capacity != nil {
			c = *t.Capacity
		}
		out = append(out, reservation.Table{ID: t.ID, Label: t.DisplayLabel(), Zone: t.Zone, Capacity: c, Active: t.Active})
	}
	return out, nil
}

type rsvHours struct{ repo catalog.Repository }

// OpenBetween — kelish vaqtida ochiq va kamida 1 soat (yoki butun bron
// qisqaroq bo'lsa — oxirigacha) ochiq qoladi.
func (a rsvHours) OpenBetween(ctx context.Context, restaurantID string, start, end time.Time) (bool, error) {
	r, err := a.repo.GetRestaurant(ctx, restaurantID)
	if err != nil {
		return false, err
	}
	if r.WorkingHours == nil {
		return true, nil
	}
	last := start.Add(time.Hour)
	if end.Before(last) {
		last = end
	}
	last = last.Add(-time.Minute)
	if !r.WorkingHours.IsOpenAt(start) || !r.WorkingHours.IsOpenAt(last) {
		return false, nil
	}
	if next, ok := r.WorkingHours.NextChangeAfter(start); ok && next.Before(last) {
		return false, nil
	}
	return true, nil
}

type rsvPayments struct{ svc *payments.Service }

func (a rsvPayments) StartDeposit(ctx context.Context, reservationID, customerID, restaurantID, description, channel string) (*reservation.PayLink, error) {
	p, err := a.svc.StartForReservation(ctx, reservationID, customerID, restaurantID, description, channel)
	if err != nil {
		return nil, err
	}
	return &reservation.PayLink{PaymentID: p.ID, PayURL: p.PayURL, ExpiresAt: p.ExpiresAt}, nil
}

func (a rsvPayments) RefundDeposit(ctx context.Context, reservationID string) error {
	return a.svc.RefundReservation(ctx, reservationID)
}

type rsvWallet struct{ s *Server }

func (a rsvWallet) Debit(ctx context.Context, ref, userID string, amount int64) error {
	return a.s.WalletSvc.DebitForOrder(ctx, ref, userID, amount)
}

func (a rsvWallet) Refund(ctx context.Context, ref, userID string, amount int64) error {
	return a.s.WalletSvc.RefundForOrder(ctx, ref, userID, amount)
}

// rsvNotifier — restoran paneli (jonli kanal + qo'ng'iroqcha) va mijoz (push).
type rsvNotifier struct{ s *Server }

func (a rsvNotifier) Notify(ctx context.Context, e reservation.Event) {
	r := e.Reservation
	at := r.StartsAt.In(reservation.Tashkent).Format("02.01 15:04")
	if a.s.Hub != nil {
		a.s.Hub.Send(restaurantTopic(r.RestaurantID), map[string]any{
			"type": e.Type, "reservation_id": r.ID, "starts_at": r.StartsAt, "status": r.Status,
		})
	}

	// Restoran qo'ng'iroqchasi — faqat e'tibor talab qiladiganlar.
	if a.s.AlertsSvc != nil {
		var in *alerts.Input
		switch e.Type {
		case "reservation_new":
			in = &alerts.Input{Kind: "reservation_new", Category: alerts.CategoryNew,
				Title: "Yangi stol broni", Body: fmt.Sprintf("%s, %d kishi, %s", at, r.PartySize, r.TableLabel)}
		case "reservation_cancelled":
			if r.CancelledBy == reservation.ByCustomer {
				in = &alerts.Input{Kind: "reservation_cancelled", Category: alerts.CategoryImportant,
					Title: "Bron bekor qilindi", Body: fmt.Sprintf("%s, %d kishi — mijoz bekor qildi", at, r.PartySize)}
			}
		}
		if in != nil {
			in.Data = map[string]string{"reservation_id": r.ID}
			in.DedupeKey = e.Type + ":" + r.ID
			if _, err := a.s.AlertsSvc.Publish(ctx, r.RestaurantID, *in); err != nil {
				slog.Warn("bron: restoran bildirishnomasi", "reservation", r.ID, "err", err)
			}
		}
	}

	// Mijozga.
	if a.s.Notifier == nil {
		return
	}
	var title, body string
	switch e.Type {
	case "reservation_new":
		title, body = "Stol band qilindi", fmt.Sprintf("%s, %d kishi. Sizni kutamiz!", at, r.PartySize)
	case "reservation_reminder":
		title, body = "Bron eslatmasi", fmt.Sprintf("Bugun %s da stolingiz tayyor (%d kishi).", r.StartsAt.In(reservation.Tashkent).Format("15:04"), r.PartySize)
	case "reservation_cancelled":
		if r.CancelledBy != reservation.ByRestaurant {
			return
		}
		body = fmt.Sprintf("%s dagi bron restoran tomonidan bekor qilindi: %s.", at, r.CancelReason)
		if r.DepositState == reservation.DepositRefunded {
			body += " Zaklad to'liq qaytarildi."
		}
		title = "Bron bekor qilindi"
	case "reservation_expired":
		title, body = "Bron tasdiqlanmadi", "Zaklad muddatida to'lanmadi — stol bo'shatildi."
	default:
		return
	}
	a.s.Notifier.Notify(ctx, r.CustomerID, notify.Event{Module: notify.ModuleFood, Kind: e.Type,
		Title: title, Body: body, Data: map[string]string{"type": e.Type, "reservation_id": r.ID}})
}
