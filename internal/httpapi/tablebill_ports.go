package httpapi

import (
	"context"
	"time"

	"chustapp/internal/modules/tablebill"
	"chustapp/internal/orders"
)

// ┌─ STOL HISOBI MODULI PORTLARI (RS-5) ──────────────────────────────────┐
// tablebill buyurtmalar, bron, to'lov va jonli kanalni portlar orqali oladi.
// └──────────────────────────────────────────────────────────────────────┘

// TableBillDeps — main.go `tablebill.New` ga beradigan portlar.
func (s *Server) TableBillDeps() (tablebill.Orders, tablebill.Deposits, tablebill.Payments, tablebill.Notifier) {
	var dep tablebill.Deposits
	if s.ReservationSvc != nil {
		dep = s.ReservationSvc
	}
	var pay tablebill.Payments
	if s.Payments != nil {
		pay = tbPayments{s}
	}
	return tbOrders{s}, dep, pay, tbNotifier{s}
}

type tbOrders struct{ s *Server }

// TableOrders — stolning [since, ...) dagi bekor qilinmagan stol buyurtmalari.
// Qator summalari buyurtma JAMI'siga moslab taqsimlanadi (aksiya/chegirma
// bo'lsa ham qatorlar yig'indisi = buyurtma jami; qoldiq oxirgi qatorga).
func (a tbOrders) TableOrders(ctx context.Context, restaurantID, tableID string, since time.Time) ([]tablebill.Order, error) {
	list, err := a.s.OrderRepo.ListByRestaurant(ctx, restaurantID, 300)
	if err != nil {
		return nil, err
	}
	var out []tablebill.Order
	for _, o := range list {
		if !o.IsDineIn() || o.TableID != tableID || o.CreatedAt.Before(since) ||
			o.Status == orders.StatusCancelled || o.Status == orders.StatusRejected {
			continue
		}
		out = append(out, tablebill.Order{ID: o.ID, Number: o.OrderNumber, Status: string(o.Status),
			TotalTiyin: o.TotalTiyin, Lines: billLines(o), WaiterID: o.WaiterID, WaiterName: o.WaiterName,
			CreatedAt: o.CreatedAt})
	}
	return out, nil
}

func billLines(o *orders.Order) []tablebill.Line {
	raw := make([]int64, len(o.Items))
	var sum int64
	for i, it := range o.Items {
		unit := it.PriceTiyin
		if it.DiscountPriceTiyin > 0 && it.DiscountPriceTiyin < unit {
			unit = it.DiscountPriceTiyin
		}
		raw[i] = unit * int64(it.Qty)
		sum += raw[i]
	}
	lines := make([]tablebill.Line, len(o.Items))
	var acc int64
	for i, it := range o.Items {
		amt := raw[i]
		if sum > 0 && sum != o.TotalTiyin {
			amt = raw[i] * o.TotalTiyin / sum
		}
		if i == len(o.Items)-1 {
			amt = o.TotalTiyin - acc
		}
		acc += amt
		unit := int64(0)
		if it.Qty > 0 {
			unit = amt / int64(it.Qty)
		}
		lines[i] = tablebill.Line{Name: it.Name, Qty: it.Qty, UnitTiyin: unit, AmountTiyin: amt}
	}
	return lines
}

type tbPayments struct{ s *Server }

func (a tbPayments) StartShare(ctx context.Context, shareID, customerID, restaurantID, channel string) (*tablebill.PayLink, error) {
	p, err := a.s.Payments.StartForBillShare(ctx, shareID, customerID, restaurantID, channel)
	if err != nil {
		return nil, err
	}
	return &tablebill.PayLink{PaymentID: p.ID, PayURL: p.PayURL, ExpiresAt: p.ExpiresAt}, nil
}

type tbNotifier struct{ s *Server }

func (a tbNotifier) BillChanged(_ context.Context, b *tablebill.Bill, event string) {
	if a.s.Hub == nil || b == nil {
		return
	}
	a.s.Hub.Send(restaurantTopic(b.RestaurantID), map[string]any{
		"type": event, "bill_id": b.ID, "table_id": b.TableID, "table_label": b.TableLabel, "status": b.Status,
	})
}
