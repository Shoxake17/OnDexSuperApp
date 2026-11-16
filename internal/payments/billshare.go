package payments

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// ┌─ STOL HISOBI ULUSHI (RS-5) ───────────────────────────────────────────┐
// Mehmon stol hisobining o'z ulushini (teng yoki o'z taomlari + ixtiyoriy
// choy puli) o'z telefonidan to'laydi. Pul restoran hisobiga darhol
// yechiladi (Hold=false) — mehmon allaqachon ovqatlangan, bloklab turish
// ma'nosiz. Summa `BillSink` dan (klientdan emas).
// └──────────────────────────────────────────────────────────────────────┘

// BillSink — `internal/modules/tablebill` tomoni.
type BillSink interface {
	// ShareAmountTiyin — ulushning HAQIQIY summasi (choy puli bilan).
	ShareAmountTiyin(ctx context.Context, shareID string) (int64, error)
	OnSharePaid(ctx context.Context, shareID string) error
	OnShareFailed(ctx context.Context, shareID string) error
}

// ErrBillsDisabled — hisob ulushi to'lovi ulanmagan.
var ErrBillsDisabled = errors.New("stol hisobi to'lovi sozlanmagan")

// SetBillSink — tablebill modulini ulaydi (main.go).
func (s *Service) SetBillSink(b BillSink) { s.bills = b }

// StartForBillShare — ulush to'lovini boshlaydi. Takroriy chaqiriq amaldagi
// havolani qaytaradi.
func (s *Service) StartForBillShare(ctx context.Context, shareID, customerID, restaurantID, channel string) (*Payment, error) {
	if s.bills == nil {
		return nil, ErrBillsDisabled
	}
	existing, err := s.repo.ListByBillShare(ctx, shareID)
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
	amount, err := s.bills.ShareAmountTiyin(ctx, shareID)
	if err != nil {
		return nil, err
	}
	if amount <= 0 {
		return nil, errors.New("ulush summasi noto'g'ri")
	}
	p := &Payment{
		ID: s.idgen(), BillShareID: shareID, CustomerID: customerID, RestaurantID: restaurantID,
		Provider: s.provider.Name(), AmountTiyin: amount, Status: StatusPending,
		CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(15 * time.Minute),
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	res, err := s.provider.Create(ctx, CreateRequest{
		PaymentID: p.ID, AmountTiyin: amount, Description: "Stol hisobi",
		Hold: false, RestaurantID: restaurantID,
		Items:   []Item{{Name: "Stol hisobi (ulush)", Qty: 1, UnitPriceTiyin: amount, TotalTiyin: amount}},
		Channel: channel, ReturnURL: s.returnURL, NotifyURL: s.notifyURL, TTLMinutes: 15,
		Language: "uz", CustomerID: customerID,
	})
	if err != nil {
		p.Status, p.ReviewReason, p.UpdatedAt = StatusFailed, err.Error(), s.now()
		if uerr := s.repo.Update(ctx, p); uerr != nil {
			slog.Error("ulush to'lovi holatini saqlab bo'lmadi", "payment", p.ID, "error", uerr)
		}
		return nil, err
	}
	p.ProviderPaymentID, p.PayURL, p.UpdatedAt = res.ProviderPaymentID, res.PayURL, s.now()
	if err := s.repo.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}
