package qurtorders

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"chustapp/internal/qurt"
)

// Filter — ro'yxat sharti (bo'sh maydon = shart yo'q). Natija: yangisi birinchi.
type Filter struct {
	Statuses     []Status
	StoreID      string
	CustomerID   string
	DeliveryDate string
	Limit        int
	Offset       int
}

// Repository — buyurtmalar ombori.
type Repository interface {
	// CreateMany — hammasini yozadi; birortasi yozilmasa, yozilganlarini qaytarib olib tashlaydi.
	CreateMany(ctx context.Context, orders []*Order) error
	Get(ctx context.Context, id string) (*Order, error)
	List(ctx context.Context, f Filter) ([]*Order, error)
	// ByIdempotency — shu mijozning shu kalit bilan yaratgan buyurtmalari.
	ByIdempotency(ctx context.Context, customerID, key string) ([]*Order, error)
	// Update — `o.Version` YANGI versiya; bazadagi versiya `o.Version-1` bo'lsagina yozadi,
	// aks holda ErrConflict.
	Update(ctx context.Context, o *Order) error
}

// Service — buyurtma yaratish va holat o'tishlari.
type Service struct {
	Repo     Repository
	Products qurt.Repository
	Schedule func() Schedule
	NewID    func() string
	Now      func() time.Time
	// OnEvent — holat o'zgarganda (yaratish ham): "created", "confirmed", ... Xatosi
	// buyurtmaga ta'sir qilmaydi. POS xabari va push shu yerga ulanadi.
	OnEvent func(ctx context.Context, event string, o *Order)
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) schedule() Schedule {
	if s.Schedule != nil {
		return s.Schedule()
	}
	return DefaultSchedule()
}

func (s *Service) emit(ctx context.Context, event string, o *Order) {
	if s.OnEvent != nil {
		s.OnEvent(ctx, event, o)
	}
}

// Line — savatdagi pozitsiya (mijoz FAQAT mahsulot va miqdorni yuboradi).
type Line struct {
	ProductID string
	Qty       int
}

// CreateInput — buyurtma yaratish ma'lumoti. Narx, nom, do'kon — serverdan.
type CreateInput struct {
	CustomerID     string
	CustomerName   string
	CustomerPhone  string
	Address        Address
	Comment        string
	Lines          []Line
	IdempotencyKey string
	// ActiveStores — hozir buyurtma qabul qila oladigan qurt do'konlari (ID -> nomi).
	ActiveStores map[string]string
}

// StockError — qaysi mahsulotda qoldiq yetmadi.
type StockError struct{ Name string }

func (e *StockError) Error() string {
	if e.Name == "" {
		return "qoldiq yetarli emas"
	}
	return fmt.Sprintf("«%s» dan so'ralgan miqdor qolmagan", e.Name)
}
func (e *StockError) Unwrap() error { return qurt.ErrInsufficientStock }

func (s *Service) newOrderID() string { return s.NewID() }

func orderNumber(id string) string {
	id = strings.ToUpper(strings.ReplaceAll(id, "-", ""))
	for len(id) < 8 {
		id += "0"
	}
	return "Q-" + id[:8]
}

// Create — savatni do'konlar bo'yicha buyurtmalarga bo'lib yaratadi. Qoldiq
// ATOMIK band qilinadi; birortasi yetmasa hech narsa yaratilmaydi.
func (s *Service) Create(ctx context.Context, in CreateInput) ([]*Order, error) {
	if len(in.IdempotencyKey) > 128 {
		return nil, invalid("idempotency_key juda uzun")
	}
	if in.IdempotencyKey != "" {
		if prev, err := s.Repo.ByIdempotency(ctx, in.CustomerID, in.IdempotencyKey); err != nil {
			return nil, err
		} else if len(prev) > 0 {
			return prev, nil // takroriy so'rov — avvalgi natija
		}
	}
	if len(in.Lines) == 0 || len(in.Lines) > MaxLines {
		return nil, invalid("savat bo'sh yoki juda katta")
	}
	if strings.TrimSpace(in.Address.Text) == "" {
		return nil, invalid("yetkazish manzili yo'q")
	}
	if utf8.RuneCountInString(in.Comment) > MaxCommentLen {
		return nil, invalid("izoh juda uzun")
	}

	// Takror mahsulotlarni birlashtiramiz (tartib saqlanadi).
	var order []string
	qty := map[string]int{}
	for _, l := range in.Lines {
		if l.ProductID == "" || l.Qty < 1 || l.Qty > MaxQtyPerLine {
			return nil, invalid(fmt.Sprintf("miqdor 1..%d bo'lsin", MaxQtyPerLine))
		}
		if _, seen := qty[l.ProductID]; !seen {
			order = append(order, l.ProductID)
		}
		qty[l.ProductID] += l.Qty
		if qty[l.ProductID] > MaxQtyPerLine {
			return nil, invalid(fmt.Sprintf("bitta mahsulot ko'pi bilan %d dona", MaxQtyPerLine))
		}
	}

	prods, err := s.Products.GetMany(ctx, order)
	if err != nil {
		return nil, err
	}
	byID := map[string]*qurt.Product{}
	for _, p := range prods {
		byID[p.ID] = p
	}
	byStore := map[string][]Item{}
	var storeOrder []string
	var reserve []qurt.Reservation
	for _, id := range order {
		p := byID[id]
		if p == nil || !p.Available {
			return nil, &StockError{Name: nameOr(p)}
		}
		if _, ok := in.ActiveStores[p.StoreID]; !ok {
			return nil, &StockError{Name: p.Name} // do'kon endi qurt emas / o'chirilgan
		}
		if _, seen := byStore[p.StoreID]; !seen {
			storeOrder = append(storeOrder, p.StoreID)
		}
		byStore[p.StoreID] = append(byStore[p.StoreID], Item{
			ProductID: p.ID, Name: p.Name, Qty: qty[id], PriceTiyin: p.PriceTiyin,
			ImageURL: p.ImageURL, WeightGrams: p.WeightGrams,
		})
		reserve = append(reserve, qurt.Reservation{ProductID: p.ID, Qty: qty[id]})
	}

	if err := s.Products.Reserve(ctx, reserve); err != nil {
		if errors.Is(err, qurt.ErrInsufficientStock) {
			return nil, &StockError{Name: s.firstShort(ctx, reserve, byID)}
		}
		return nil, err
	}

	date, _ := s.schedule().NextDelivery(s.now())
	now := s.now().UTC()
	group := s.newOrderID()
	var out []*Order
	for _, storeID := range storeOrder {
		items := byStore[storeID]
		var sub int64
		for _, it := range items {
			sub += it.PriceTiyin * int64(it.Qty)
		}
		if sub <= 0 || sub > MaxTotalTiyin {
			_ = s.Products.Release(ctx, reserve)
			return nil, invalid("buyurtma summasi noto'g'ri")
		}
		id := s.newOrderID()
		out = append(out, &Order{
			ID: id, Number: orderNumber(id), GroupID: group,
			CustomerID: in.CustomerID, StoreID: storeID, StoreName: in.ActiveStores[storeID],
			Items: items, SubtotalTiyin: sub, TotalTiyin: sub, // yetkazish haqi hozircha 0
			PaymentMethod: "cash", Status: StatusNew, DeliveryDate: date,
			CustomerName: in.CustomerName, CustomerPhone: in.CustomerPhone,
			Address: in.Address, Comment: strings.TrimSpace(in.Comment),
			History:        []StatusChange{{From: "", To: StatusNew, By: ActorCustomer, ByID: in.CustomerID, At: now}},
			Reserved:       true,
			IdempotencyKey: in.IdempotencyKey,
			CreatedAt:      now, UpdatedAt: now, Version: 1,
		})
	}
	if err := s.Repo.CreateMany(ctx, out); err != nil {
		_ = s.Products.Release(ctx, reserve)
		return nil, err
	}
	for _, o := range out {
		s.emit(ctx, "created", o)
	}
	return out, nil
}

func nameOr(p *qurt.Product) string {
	if p == nil {
		return ""
	}
	return p.Name
}

// firstShort — xabar uchun: qaysi mahsulotda qoldiq kam (band qilish allaqachon rad etilgan).
func (s *Service) firstShort(ctx context.Context, res []qurt.Reservation, byID map[string]*qurt.Product) string {
	ids := make([]string, 0, len(res))
	for _, r := range res {
		ids = append(ids, r.ProductID)
	}
	fresh, err := s.Products.GetMany(ctx, ids)
	if err != nil {
		return ""
	}
	left := map[string]int{}
	for _, p := range fresh {
		left[p.ID] = p.Left()
	}
	for _, r := range res {
		if left[r.ProductID] < r.Qty {
			return nameOr(byID[r.ProductID])
		}
	}
	return ""
}

// Transition — holatni o'zgartiradi. `check` buyurtmani yuklagach chaqiriladi
// (egalik tekshiruvi); xato qaytarsa buyurtma o'zgarmaydi. Egalik mos kelmasa
// chaqiruvchi ErrNotFound qaytarishi kerak (mavjudligini oshkor qilmaslik uchun).
func (s *Service) Transition(ctx context.Context, id string, to Status, by Actor, byID, reason string, check func(*Order) error) (*Order, error) {
	o, err := s.Repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if check != nil {
		if err := check(o); err != nil {
			return nil, err
		}
	}
	if err := ValidateTransition(o.Status, to, by); err != nil {
		return nil, err
	}

	reason = strings.TrimSpace(reason)
	needReason := to == StatusRejected || to == StatusFailed || (to == StatusCancelled && by != ActorCustomer)
	if needReason {
		if reason, err = NormalizeReason(reason); err != nil {
			return nil, err
		}
	} else if utf8.RuneCountInString(reason) > MaxReasonLen {
		return nil, invalid("sabab juda uzun")
	}

	from := o.Status
	releasing := o.Reserved
	o.Status = to
	if reason != "" && (to == StatusRejected || to == StatusCancelled || to == StatusFailed) {
		o.RejectReason = reason
	}
	// `new` dan chiqqan har buyurtma bandni qo'yib yuboradi: tasdiqlangandan
	// keyin qoldiqni POS o'zi kamaytiradi (ikki marta sanalmasin).
	o.Reserved = false
	now := s.now().UTC()
	o.History = append(o.History, StatusChange{From: from, To: to, By: by, ByID: byID, Reason: reason, At: now})
	o.UpdatedAt = now
	o.Version++
	if err := s.Repo.Update(ctx, o); err != nil {
		return nil, err
	}
	if releasing {
		var rel []qurt.Reservation
		for pid, q := range o.ReservationItems() {
			rel = append(rel, qurt.Reservation{ProductID: pid, Qty: q})
		}
		if err := s.Products.Release(ctx, rel); err != nil {
			// Buyurtma saqlandi; band qaytmay qolsa mahsulot "qolmagandek" ko'rinadi.
			slog.Error("qurt: bandni qaytarib bo'lmadi", "order", o.ID, "err", err)
		}
	}
	s.emit(ctx, string(to), o)
	return o, nil
}

// TransitionDate — reysdagi barcha `from` holatidagi buyurtmalarni `to` ga o'tkazadi
// (masalan, reys yo'lga chiqdi: collected -> out_for_delivery). Birini o'tkazib
// bo'lmasa qolganlari davom etadi; natija: o'tganlar va o'tmaganlar soni.
func (s *Service) TransitionDate(ctx context.Context, date string, from, to Status, by Actor, byID string) (done, skipped int, err error) {
	list, err := s.Repo.List(ctx, Filter{DeliveryDate: date, Statuses: []Status{from}, Limit: 1000})
	if err != nil {
		return 0, 0, err
	}
	for _, o := range list {
		if _, err := s.Transition(ctx, o.ID, to, by, byID, "", nil); err != nil {
			skipped++
			continue
		}
		done++
	}
	return done, skipped, nil
}
