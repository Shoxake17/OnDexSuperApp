package tablebill

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"
)

// Deps — Service bog'liqliklari (Deposits/Payments/Notifier ixtiyoriy).
type Deps struct {
	Store    Store
	Orders   Orders
	Deposits Deposits
	Payments Payments
	Notifier Notifier
	NewID    func() string
	Now      func() time.Time
}

type Service struct{ d Deps }

func New(d Deps) (*Service, error) {
	if d.Store == nil || d.Orders == nil || d.NewID == nil {
		return nil, errors.New("tablebill: Store, Orders, NewID majburiy")
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Service{d: d}, nil
}

func (s *Service) now() time.Time { return s.d.Now().UTC() }

// LineState — mijoz ko'radigan qator: nechtasi to'langan/band.
type LineState struct {
	Line
	Index   int `json:"line"`
	PaidQty int `json:"paid_qty"` // to'langan + to'lanayotgan
}

// OrderView — hisobdagi buyurtma.
type OrderView struct {
	ID         string      `json:"id"`
	Number     string      `json:"order_number"`
	Status     string      `json:"status"`
	TotalTiyin int64       `json:"total_tiyin"`
	Lines      []LineState `json:"lines"`
}

// View — hisobning to'liq holati (summalar serverda hisoblanadi).
type View struct {
	Bill           Bill        `json:"bill"`
	Orders         []OrderView `json:"orders"`
	TotalTiyin     int64       `json:"total_tiyin"`     // buyurtmalar jami
	DueTiyin       int64       `json:"due_tiyin"`       // jami - zaklad
	PaidTiyin      int64       `json:"paid_tiyin"`      // to'langan ulushlar (choy pulisiz)
	PendingTiyin   int64       `json:"pending_tiyin"`   // to'lanayotgan (15 daq ichida)
	RemainingTiyin int64       `json:"remaining_tiyin"` // due - paid - pending
	TipsTiyin      int64       `json:"tips_tiyin"`
	Shares         []*Share    `json:"shares"`
	WaiterName     string      `json:"waiter_name,omitempty"`
}

// activePending — to'lanmagan, lekin hali amal qilayotgan karta ulushi.
func (s *Service) activePending(sh *Share) bool {
	return sh.Status == SharePending && s.now().Sub(sh.CreatedAt) < ShareHold
}

// Current — stolning ochiq hisobi (yo'q bo'lsa ochiladi): yangi buyurtmalar
// qo'shiladi, o'tirgan bron zakladi bir marta ayriladi.
func (s *Service) Current(ctx context.Context, restaurantID, tableID, tableLabel string) (*View, error) {
	b, err := s.d.Store.GetOpen(ctx, tableID)
	if errors.Is(err, ErrNotFound) {
		now := s.now()
		nb := &Bill{ID: s.d.NewID(), RestaurantID: restaurantID, TableID: tableID, TableLabel: tableLabel,
			Status: StatusOpen, CreatedAt: now, UpdatedAt: now, Version: 1}
		if cerr := s.d.Store.CreateOpen(ctx, nb); cerr != nil && !errors.Is(cerr, ErrConflict) {
			return nil, cerr
		}
		b, err = s.d.Store.GetOpen(ctx, tableID)
	}
	if err != nil {
		return nil, err
	}
	if b.RestaurantID != restaurantID {
		return nil, ErrNotFound
	}
	orders, err := s.d.Orders.TableOrders(ctx, restaurantID, tableID, s.now().Add(-OrderWindow))
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(orders))
	for _, o := range orders {
		ids = append(ids, o.ID)
	}
	attached, err := s.d.Store.Attached(ctx, ids)
	if err != nil {
		return nil, err
	}
	var fresh []string
	for _, id := range ids {
		if !attached[id] {
			fresh = append(fresh, id)
		}
	}
	if len(fresh) > 0 {
		if err := s.d.Store.Attach(ctx, b.ID, fresh); err != nil {
			return nil, err
		}
	}
	if b.DepositReservationID == "" && s.d.Deposits != nil {
		if rid, amt, ok := s.d.Deposits.SeatedDeposit(ctx, restaurantID, tableID); ok && amt > 0 {
			if err := s.d.Store.SetDeposit(ctx, b.ID, rid, amt); err == nil {
				b.DepositReservationID, b.DepositTiyin = rid, amt
			}
		}
	}
	return s.view(ctx, b, orders)
}

// ViewOf — hisob holati (ID bo'yicha).
func (s *Service) ViewOf(ctx context.Context, billID string) (*View, error) {
	b, err := s.d.Store.Get(ctx, billID)
	if err != nil {
		return nil, err
	}
	orders, err := s.d.Orders.TableOrders(ctx, b.RestaurantID, b.TableID, b.CreatedAt.Add(-OrderWindow))
	if err != nil {
		return nil, err
	}
	return s.view(ctx, b, orders)
}

func (s *Service) view(ctx context.Context, b *Bill, all []Order) (*View, error) {
	ids, err := s.d.Store.OrderIDs(ctx, b.ID)
	if err != nil {
		return nil, err
	}
	in := map[string]bool{}
	for _, id := range ids {
		in[id] = true
	}
	shares, err := s.d.Store.Shares(ctx, b.ID)
	if err != nil {
		return nil, err
	}
	// Qator bo'yicha to'langan/band miqdor.
	taken := map[string]int{}
	v := &View{Bill: *b, Shares: shares}
	for _, sh := range shares {
		active := sh.Status == SharePaid || s.activePending(sh)
		switch {
		case sh.Status == SharePaid:
			v.PaidTiyin += sh.AmountTiyin
			v.TipsTiyin += sh.TipTiyin
		case s.activePending(sh):
			v.PendingTiyin += sh.AmountTiyin
		}
		if active {
			for _, it := range sh.Items {
				taken[fmt.Sprintf("%s#%d", it.OrderID, it.Line)] += it.Qty
			}
		}
	}
	for _, o := range all {
		if !in[o.ID] {
			continue
		}
		ov := OrderView{ID: o.ID, Number: o.Number, Status: o.Status, TotalTiyin: o.TotalTiyin}
		for i, l := range o.Lines {
			ov.Lines = append(ov.Lines, LineState{Line: l, Index: i, PaidQty: min(taken[fmt.Sprintf("%s#%d", o.ID, i)], l.Qty)})
		}
		v.Orders = append(v.Orders, ov)
		v.TotalTiyin += o.TotalTiyin
		if o.WaiterName != "" {
			v.WaiterName = o.WaiterName
		}
	}
	sort.Slice(v.Orders, func(i, j int) bool { return v.Orders[i].Number < v.Orders[j].Number })
	v.DueTiyin = max(v.TotalTiyin-b.DepositTiyin, 0)
	v.RemainingTiyin = max(v.DueTiyin-v.PaidTiyin-v.PendingTiyin, 0)
	return v, nil
}

// ShareInput — mehmon ulushi.
type ShareInput struct {
	BillID     string
	PayerID    string
	Kind       string // equal | items
	Parts      int    // equal
	Items      []ItemRef
	TipPercent int
	Channel    string
}

// ShareResult — yaratilgan ulush va to'lov havolasi.
type ShareResult struct {
	Share *Share   `json:"share"`
	Pay   *PayLink `json:"pay"`
}

// CreateShare — ulush summasini hisoblaydi, band qiladi va karta to'lovini
// boshlaydi. Ikki mehmon bir qatorni bir vaqtda tanlasa — versiya qulfi
// bittasini o'tkazadi, ikkinchisi ErrConflict (qayta ko'radi).
func (s *Service) CreateShare(ctx context.Context, restaurantID string, in ShareInput) (*ShareResult, error) {
	if s.d.Payments == nil {
		return nil, ErrPaymentDisabled
	}
	if !AllowedTips[in.TipPercent] {
		return nil, fmt.Errorf("%w: choy puli 0, 5 yoki 10%%", ErrInvalid)
	}
	b, err := s.d.Store.Get(ctx, in.BillID)
	if err != nil {
		return nil, err
	}
	if b.RestaurantID != restaurantID {
		return nil, ErrNotFound
	}
	if b.Status != StatusOpen {
		return nil, ErrNothingDue
	}
	v, err := s.ViewOf(ctx, b.ID)
	if err != nil {
		return nil, err
	}
	if v.RemainingTiyin <= 0 {
		if v.PendingTiyin > 0 {
			return nil, ErrPendingPayments
		}
		return nil, ErrNothingDue
	}
	var amount int64
	var items []ItemRef
	switch in.Kind {
	case KindEqual:
		if in.Parts < 1 || in.Parts > MaxParts {
			return nil, fmt.Errorf("%w: kishi soni 1-%d", ErrInvalid, MaxParts)
		}
		amount = (v.DueTiyin + int64(in.Parts) - 1) / int64(in.Parts)
	case KindItems:
		lines := map[string]LineState{}
		for _, o := range v.Orders {
			for _, l := range o.Lines {
				lines[fmt.Sprintf("%s#%d", o.ID, l.Index)] = l
			}
		}
		seen := map[string]bool{}
		for _, it := range in.Items {
			k := fmt.Sprintf("%s#%d", it.OrderID, it.Line)
			l, ok := lines[k]
			if !ok || it.Qty < 1 || seen[k] || it.Qty > l.Qty-l.PaidQty {
				return nil, fmt.Errorf("%w: taom tanlovi (allaqachon to'langan bo'lishi mumkin)", ErrInvalid)
			}
			seen[k] = true
			// Qator summasi qty bo'yicha; oxirgi qism yaxlitlash qoldig'i bilan.
			part := l.UnitTiyin * int64(it.Qty)
			if it.Qty == l.Qty-l.PaidQty {
				part = l.AmountTiyin - l.UnitTiyin*int64(l.PaidQty)
			}
			amount += part
			items = append(items, it)
		}
		if len(items) == 0 {
			return nil, fmt.Errorf("%w: taom tanlanmagan", ErrInvalid)
		}
	default:
		return nil, fmt.Errorf("%w: rejim", ErrInvalid)
	}
	amount = min(amount, v.RemainingTiyin)
	if amount <= 0 {
		return nil, ErrNothingDue
	}
	sh := &Share{
		ID: s.d.NewID(), BillID: b.ID, PayerID: in.PayerID, Kind: in.Kind, Method: MethodCard,
		AmountTiyin: amount, TipTiyin: amount * int64(in.TipPercent) / 100, Items: items,
		WaiterName: v.WaiterName, Status: SharePending, CreatedAt: s.now(),
	}
	sh.WaiterID = s.waiterOf(ctx, b)
	if err := s.d.Store.AddShare(ctx, sh, b.Version); err != nil {
		return nil, err
	}
	link, err := s.d.Payments.StartShare(ctx, sh.ID, in.PayerID, restaurantID, in.Channel)
	if err != nil {
		_ = s.d.Store.SetShareStatus(ctx, sh.ID, SharePending, ShareFailed, nil)
		return nil, err
	}
	return &ShareResult{Share: sh, Pay: link}, nil
}

// waiterOf — choy puli egasi: hisobdagi oxirgi buyurtmani olgan ofitsiant.
func (s *Service) waiterOf(ctx context.Context, b *Bill) string {
	orders, err := s.d.Orders.TableOrders(ctx, b.RestaurantID, b.TableID, b.CreatedAt.Add(-OrderWindow))
	if err != nil {
		return ""
	}
	w := ""
	for _, o := range orders {
		if o.WaiterID != "" {
			w = o.WaiterID
		}
	}
	return w
}

// ShareAmountTiyin — payments.BillSink: to'lov summasi (ulush + choy puli).
func (s *Service) ShareAmountTiyin(ctx context.Context, shareID string) (int64, error) {
	sh, err := s.d.Store.GetShare(ctx, shareID)
	if err != nil {
		return 0, err
	}
	if sh.Status != SharePending {
		return 0, ErrConflict
	}
	return sh.AmountTiyin + sh.TipTiyin, nil
}

// OnSharePaid — payments.BillSink: ulush to'landi; hisob to'liq yopilsa — yopiladi.
func (s *Service) OnSharePaid(ctx context.Context, shareID string) error {
	sh, err := s.d.Store.GetShare(ctx, shareID)
	if err != nil {
		return err
	}
	if sh.Status == SharePaid {
		return nil
	}
	t := s.now()
	// Muddati o'tgan (failed) ulush ham to'lansa — pul keldi, qabul qilinadi.
	if err := s.d.Store.SetShareStatus(ctx, sh.ID, sh.Status, SharePaid, &t); err != nil && !errors.Is(err, ErrConflict) {
		return err
	}
	return s.maybeClose(ctx, sh.BillID, "share_paid")
}

// OnShareFailed — payments.BillSink.
func (s *Service) OnShareFailed(ctx context.Context, shareID string) error {
	err := s.d.Store.SetShareStatus(ctx, shareID, SharePending, ShareFailed, nil)
	if errors.Is(err, ErrConflict) {
		return nil
	}
	return err
}

func (s *Service) maybeClose(ctx context.Context, billID, event string) error {
	v, err := s.ViewOf(ctx, billID)
	if err != nil {
		return err
	}
	b := v.Bill
	if b.Status == StatusOpen && v.DueTiyin > 0 && v.PaidTiyin >= v.DueTiyin {
		if err := s.d.Store.Close(ctx, b.ID, s.now()); err != nil {
			return err
		}
		b.Status = StatusPaid
		event = "bill_paid"
	}
	if s.d.Notifier != nil {
		s.d.Notifier.BillChanged(context.WithoutCancel(ctx), &b, event)
	}
	return nil
}

// CloseByStaff — ofitsiant/restoran qolgan summani naqd yoki terminal
// orqali qabul qildi. Ilovada to'lov ketayotgan bo'lsa — kutiladi.
func (s *Service) CloseByStaff(ctx context.Context, restaurantID, billID, method, staffID string) (*View, error) {
	if method != MethodCash && method != MethodTerminal {
		return nil, fmt.Errorf("%w: usul naqd yoki terminal", ErrInvalid)
	}
	b, err := s.d.Store.Get(ctx, billID)
	if err != nil {
		return nil, err
	}
	if b.RestaurantID != restaurantID {
		return nil, ErrNotFound
	}
	if b.Status != StatusOpen {
		return nil, ErrNothingDue
	}
	v, err := s.ViewOf(ctx, billID)
	if err != nil {
		return nil, err
	}
	if v.PendingTiyin > 0 {
		return nil, ErrPendingPayments
	}
	if v.RemainingTiyin > 0 {
		t := s.now()
		sh := &Share{ID: s.d.NewID(), BillID: b.ID, PayerID: staffID, Kind: KindStaff, Method: method,
			AmountTiyin: v.RemainingTiyin, WaiterName: v.WaiterName, Status: SharePaid, CreatedAt: t, PaidAt: &t}
		if err := s.d.Store.AddShare(ctx, sh, b.Version); err != nil {
			return nil, err
		}
	}
	if err := s.maybeClose(ctx, billID, "bill_staff_closed"); err != nil {
		return nil, err
	}
	if v.DueTiyin == 0 && b.Status == StatusOpen {
		// Hisob bo'sh (masalan zaklad hammasini qopladi) — baribir yopiladi.
		if err := s.d.Store.Close(ctx, b.ID, s.now()); err != nil {
			return nil, err
		}
	}
	return s.ViewOf(ctx, billID)
}

// OpenBills — restoranning ochiq hisoblari (panel/ofitsiant).
func (s *Service) OpenBills(ctx context.Context, restaurantID string) ([]*View, error) {
	list, err := s.d.Store.ListOpen(ctx, restaurantID)
	if err != nil {
		return nil, err
	}
	out := make([]*View, 0, len(list))
	for _, b := range list {
		v, err := s.ViewOf(ctx, b.ID)
		if err != nil {
			slog.Warn("tablebill: hisob ko'rinishi", "bill", b.ID, "err", err)
			continue
		}
		if len(v.Orders) == 0 {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}

// Tips — choy pullari ofitsiant bo'yicha (panel). Davr 93 kundan oshmaydi.
func (s *Service) Tips(ctx context.Context, restaurantID string, from, to time.Time) ([]WaiterTips, error) {
	if !to.After(from) || to.Sub(from) > 93*24*time.Hour {
		return nil, fmt.Errorf("%w: davr", ErrInvalid)
	}
	out, err := s.d.Store.Tips(ctx, restaurantID, from, to)
	if out == nil {
		out = []WaiterTips{}
	}
	return out, err
}