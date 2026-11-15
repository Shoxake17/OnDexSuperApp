package payments

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// ┌─ BRON ZAKLADI (RS-3) ─────────────────────────────────────────────────┐
// Zaklad — bron uchun oldindan to'lov, pul RESTORAN hisobiga tushadi
// (ko'p ijarachili provayder, `CreateRequest.RestaurantID`).
//
// Buyurtmadan farqi: pul BLOKLANMAYDI, darhol yechiladi (Hold=false).
// Bron kunlar oldin qilinadi, karta bloki esa bank tomonida bir necha
// kunda o'z-o'zidan bo'shab ketadi — bron kuni zaklad yo'q bo'lib qolardi.
// Shuning uchun: to'lov = darhol yechish, erta bekor qilish = Refund.
//
// Buyurtma oqimi (OrderSink, Capture/Release) bu yerga TEGMAYDI.
// └──────────────────────────────────────────────────────────────────────┘

// DepositSink — bron moduli (`internal/modules/reservation`) tomoni.
// `payments` bron paketini import qilmaydi (modul chegarasi).
type DepositSink interface {
	// DepositAmountTiyin — bronning HAQIQIY zakladi (klientdan olinmaydi).
	DepositAmountTiyin(ctx context.Context, reservationID string) (int64, error)
	// OnDepositPaid — zaklad to'landi: bron tasdiqlanadi.
	OnDepositPaid(ctx context.Context, reservationID string) error
	// OnDepositFailed — to'lov amalga oshmadi/muddati tugadi.
	OnDepositFailed(ctx context.Context, reservationID string) error
}

// SetDepositSink — bron modulini ulaydi (main.go). nil — zaklad o'chiq.
func (s *Service) SetDepositSink(d DepositSink) { s.deposits = d }

// ErrDepositsDisabled — bron zakladi ulanmagan.
var ErrDepositsDisabled = errors.New("bron zakladi to'lovi sozlanmagan")

func (s *Service) notifyPaid(ctx context.Context, p *Payment) error {
	if p.ReservationID != "" {
		if s.deposits == nil {
			return ErrDepositsDisabled
		}
		return s.deposits.OnDepositPaid(ctx, p.ReservationID)
	}
	return s.orders.OnPaymentHeld(ctx, p.OrderID)
}

func (s *Service) notifyFailed(ctx context.Context, p *Payment) error {
	if p.ReservationID != "" {
		if s.deposits == nil {
			return ErrDepositsDisabled
		}
		return s.deposits.OnDepositFailed(ctx, p.ReservationID)
	}
	return s.orders.OnPaymentFailed(ctx, p.OrderID)
}

// StartForReservation — zaklad to'lovini boshlaydi va to'lov sahifasi
// havolasini qaytaradi. Takroriy chaqiriq amaldagi havolani qaytaradi.
func (s *Service) StartForReservation(ctx context.Context, reservationID, customerID, restaurantID, description, channel string) (*Payment, error) {
	if s.deposits == nil {
		return nil, ErrDepositsDisabled
	}
	existing, err := s.repo.ListByReservation(ctx, reservationID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	for _, p := range existing {
		switch {
		case p.Status == StatusPaid || p.Status == StatusHeld:
			return nil, ErrAlreadyPaid
		case p.Active(now) && p.PayURL != "":
			return p, nil
		}
	}
	amount, err := s.deposits.DepositAmountTiyin(ctx, reservationID)
	if err != nil {
		return nil, err
	}
	if amount <= 0 {
		return nil, errors.New("zaklad summasi noto'g'ri")
	}
	p := &Payment{
		ID:            s.idgen(),
		ReservationID: reservationID,
		CustomerID:    customerID,
		RestaurantID:  restaurantID,
		Provider:      s.provider.Name(),
		AmountTiyin:   amount,
		Status:        StatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
		ExpiresAt:     now.Add(time.Duration(s.depositTTL()) * time.Minute),
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	res, err := s.provider.Create(ctx, CreateRequest{
		PaymentID:    p.ID,
		AmountTiyin:  amount,
		Description:  description,
		Hold:         false, // darhol yechiladi (yuqoridagi izoh)
		RestaurantID: restaurantID,
		Items:        []Item{{Name: "Stol broni zakladi", Qty: 1, UnitPriceTiyin: amount, TotalTiyin: amount}},
		Channel:      channel,
		ReturnURL:    s.returnURL,
		NotifyURL:    s.notifyURL,
		TTLMinutes:   s.depositTTL(),
		Language:     "uz",
		CustomerID:   customerID,
	})
	if err != nil {
		p.Status = StatusFailed
		p.ReviewReason = err.Error()
		p.UpdatedAt = s.now()
		if uerr := s.repo.Update(ctx, p); uerr != nil {
			slog.Error("zaklad to'lovi holatini saqlab bo'lmadi", "payment", p.ID, "error", uerr)
		}
		return nil, err
	}
	p.ProviderPaymentID = res.ProviderPaymentID
	p.PayURL = res.PayURL
	p.UpdatedAt = s.now()
	if err := s.repo.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// depositTTL — zaklad havolasi bron "hold" muddatidan oshmaydi (15 daq):
// to'lanmagan bron stolni uzoq band qilib turmasin.
func (s *Service) depositTTL() int {
	if s.ttlMinutes > 0 && s.ttlMinutes < 15 {
		return s.ttlMinutes
	}
	return 15
}

// RefundReservation — zakladni to'liq qaytaradi (erta bekor qilish yoki
// restoran bekor qildi). To'lanmagan urinishlar yopiladi. Idempotent.
func (s *Service) RefundReservation(ctx context.Context, reservationID string) error {
	list, err := s.repo.ListByReservation(ctx, reservationID)
	if err != nil {
		return err
	}
	for _, p := range list {
		switch p.Status {
		case StatusPaid:
			if err := s.provider.Refund(ctx, p.RestaurantID, p.ProviderPaymentID, "rf-"+p.ID, p.AmountTiyin); err != nil {
				return fmt.Errorf("zakladni qaytarib bo'lmadi: %w", err)
			}
			p.Status = StatusRefunded
		case StatusHeld:
			if err := s.provider.Cancel(ctx, p.RestaurantID, p.ProviderPaymentID); err != nil {
				return fmt.Errorf("zaklad blokini bo'shatib bo'lmadi: %w", err)
			}
			p.Status = StatusCanceled
		case StatusPending:
			if s.cancelsUnpaid() && p.ProviderPaymentID != "" {
				if err := s.provider.Cancel(ctx, p.RestaurantID, p.ProviderPaymentID); err != nil {
					slog.Warn("zaklad: to'lanmagan invoysni yopib bo'lmadi", "payment", p.ID, "err", err)
					continue
				}
			}
			p.Status = StatusCanceled
			p.ReviewReason = "bron bekor qilindi"
		default:
			continue
		}
		p.UpdatedAt = s.now()
		if err := s.repo.Update(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

// ListByReservation — bron zakladi urinishlari (HTTP qatlami uchun).
func (s *Service) ListByReservation(ctx context.Context, reservationID string) ([]*Payment, error) {
	return s.repo.ListByReservation(ctx, reservationID)
}
