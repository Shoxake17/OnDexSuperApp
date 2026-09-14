package alerts

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"chustapp/internal/orders"
	"chustapp/internal/staff"
)

// ─── Buyurtmalar ───────────────────────────────────────────────────────

// WrapOrders — mavjud `orders.Notifier` (jonli kanal, mijoz push) ustiga
// restoran bildirishnomalarini qo'shadi. Ichki notifier O'ZGARISHSIZ
// birinchi chaqiriladi.
func (s *Service) WrapOrders(inner orders.Notifier) orders.Notifier {
	return &orderNotifier{inner: inner, svc: s}
}

type orderNotifier struct {
	inner orders.Notifier
	svc   *Service
}

func orderLabel(number, id string) string {
	if strings.TrimSpace(number) != "" {
		return "#" + number
	}
	if len(id) > 8 {
		id = id[:8]
	}
	return "#" + id
}

func (n *orderNotifier) OrderCreated(o *orders.Order) {
	if n.inner != nil {
		n.inner.OrderCreated(o)
	}
	if o == nil || n.svc == nil {
		return
	}
	// Buyurtma ko'rsatkichidan NUSXA — goroutine ichida o'zgarib qolmasin.
	id, number, rid, total := o.ID, o.OrderNumber, o.RestaurantID, o.TotalTiyin
	where := "Yetkazib berish"
	if o.IsDineIn() {
		where = "Stol"
		if label := strings.TrimSpace(o.TableLabel); label != "" {
			where = "Stol: " + label
		}
	}
	n.svc.publishAsync(rid, func(context.Context) (Input, bool) {
		return Input{
			Kind: KindNewOrder, Category: CategoryNew,
			Title:     "Yangi buyurtma",
			Body:      fmt.Sprintf("%s buyurtma tushdi · %s. Jami: %s so'm", orderLabel(number, id), where, formatSum(total)),
			Data:      map[string]string{"order_id": id, "order_number": number},
			DedupeKey: "new_order:" + id,
		}, true
	})
}

func (n *orderNotifier) OrderStatusChanged(o *orders.Order, from orders.Status) {
	if n.inner != nil {
		n.inner.OrderStatusChanged(o, from)
	}
	if o == nil || n.svc == nil || o.Status != orders.StatusCancelled {
		return
	}
	var by orders.Actor
	if len(o.History) > 0 {
		by = o.History[len(o.History)-1].By
	}
	// Restoranning O'Z amali haqida xabar berish ma'nosiz.
	if by == orders.ActorRestaurant {
		return
	}
	who := "Mijoz"
	if by == orders.ActorAdmin {
		who = "OnDex administratori"
	}
	id, number, rid, total := o.ID, o.OrderNumber, o.RestaurantID, o.TotalTiyin
	body := fmt.Sprintf("%s %s buyurtmani bekor qildi. Summa: %s so'm", who, orderLabel(number, id), formatSum(total))
	if by == orders.ActorSystem {
		// Tizim tayyor buyurtmani bitta sababga ko'ra bekor qiladi: kuryer
		// topilmadi va restoran belgilangan muddatda qaror qilmadi
		// (`orders.Service.AutoCancelCourierNotFound`).
		body = fmt.Sprintf("%s buyurtmaga kuryer topilmadi va javob berilmagani uchun u avtomatik bekor qilindi. Summa: %s so'm",
			orderLabel(number, id), formatSum(total))
	}
	n.svc.publishAsync(rid, func(context.Context) (Input, bool) {
		return Input{
			Kind: KindOrderCancelled, Category: CategoryImportant,
			Title:     "Buyurtma bekor qilindi",
			Body:      body,
			Data:      map[string]string{"order_id": id, "order_number": number},
			DedupeKey: "order_cancelled:" + id,
		}, true
	})
}

// lookupNumber — buyurtma raqami (topilmasa bo'sh — ID qisqartmasi ishlatiladi).
func (s *Service) lookupNumber(ctx context.Context, orderID string) string {
	if s.orderLookup == nil || orderID == "" {
		return ""
	}
	o, err := s.orderLookup(ctx, orderID)
	if err != nil || o == nil {
		return ""
	}
	return o.OrderNumber
}

// PaymentReceived — karta to'lovi haqiqatan yechildi (Octo).
func (s *Service) PaymentReceived(restaurantID, paymentID, orderID string, amountTiyin int64) {
	s.publishAsync(restaurantID, func(ctx context.Context) (Input, bool) {
		number := s.lookupNumber(ctx, orderID)
		return Input{
			Kind: KindPaymentReceived, Category: CategorySuccess,
			Title:     "To'lov qabul qilindi",
			Body:      fmt.Sprintf("Karta orqali %s so'm to'lov amalga oshirildi. Buyurtma %s", formatSum(amountTiyin), orderLabel(number, orderID)),
			Data:      map[string]string{"order_id": orderID, "order_number": number},
			DedupeKey: "payment_paid:" + paymentID,
		}, true
	})
}

// DispatchFailed — kuryer qidiruvida INFRATUZILMA xatosi (oddiy "kuryer
// yo'q" holati emas — u ichkarida cheksiz qayta uriniladi).
func (s *Service) DispatchFailed(restaurantID, orderID string) {
	s.publishAsync(restaurantID, func(ctx context.Context) (Input, bool) {
		number := s.lookupNumber(ctx, orderID)
		return Input{
			Kind: KindDispatchFailed, Category: CategoryImportant,
			Title: "Kuryer topishda xatolik",
			Body: fmt.Sprintf("%s buyurtma uchun kuryer qidirishda tizim xatosi yuz berdi. "+
				"Buyurtma holatini tekshiring yoki OnDex qo'llab-quvvatlash xizmatiga murojaat qiling.", orderLabel(number, orderID)),
			Data:      map[string]string{"order_id": orderID, "order_number": number},
			DedupeKey: "dispatch_failed:" + orderID,
		}, true
	})
}

// CourierNotFound — tayyor buyurtmaga belgilangan muddatda kuryer
// topilmadi. Restoran qaror qilishi kerak: bekor qilish yoki qayta
// qidirish — aks holda `autoCancelAt` da tizim o'zi bekor qiladi.
func (s *Service) CourierNotFound(restaurantID, orderID, orderNumber string, autoCancelAt time.Time) {
	s.publishAsync(restaurantID, func(context.Context) (Input, bool) {
		return Input{
			Kind: KindCourierNotFound, Category: CategoryImportant,
			Title: "Kuryer topilmadi",
			Body: fmt.Sprintf("%s buyurtmaga %d daqiqa ichida kuryer topilmadi. Buyurtmani bekor qiling yoki kuryerni qayta qidiring — "+
				"aks holda soat %s da avtomatik bekor qilinadi.",
				orderLabel(orderNumber, orderID), int(orders.CourierSearchWindow.Minutes()),
				autoCancelAt.In(Location).Format("15:04")),
			Data: map[string]string{"order_id": orderID, "order_number": orderNumber},
			// Har qidiruv tsikli alohida: restoran qayta qidirib, yana
			// topilmasa YANGI bildirishnoma kerak.
			DedupeKey: fmt.Sprintf("courier_not_found:%s:%d", orderID, autoCancelAt.Unix()),
		}, true
	})
}

// ─── Xodimlar ──────────────────────────────────────────────────────────

// StaffEvents — `staff.Service.WithObserver` uchun. Faqat restoran
// egasi uchun ahamiyatli hodisalar: yangi xodim, holat va lavozim.
// Maosh va telefon HECH QACHON bildirishnomaga yozilmaydi.
func (s *Service) StaffEvents(m staff.Member, events []staff.Event) {
	name := m.FullName()
	if name == "" {
		name = "Xodim"
	}
	position := m.Position.Title()
	for _, ev := range events {
		var in Input
		switch ev.Kind {
		case staff.EventCreated:
			in = Input{Kind: KindStaffAdded, Category: CategoryInfo, Title: "Yangi xodim qo'shildi",
				Body: fmt.Sprintf("%s (%s) xodimlar ro'yxatiga qo'shildi.", name, position)}
		case staff.EventStatusChanged:
			title := map[string]string{
				string(staff.StatusOnLeave):   "Xodim ta'tilga chiqdi",
				string(staff.StatusDismissed): "Xodim ishdan bo'shatildi",
			}[ev.To]
			if title == "" {
				title = "Xodim ishga qaytdi"
				if ev.From == string(staff.StatusDismissed) {
					title = "Xodim qayta ishga olindi"
				}
			}
			in = Input{Kind: KindStaffStatus, Category: CategoryActivity, Title: title,
				Body: fmt.Sprintf("%s (%s) — holati: %s.", name, position, staff.Status(ev.To).Title())}
		case staff.EventPositionChanged:
			in = Input{Kind: KindStaffPosition, Category: CategoryInfo, Title: "Xodim lavozimi o'zgartirildi",
				Body: fmt.Sprintf("%s: %s → %s.", name, staff.Position(ev.From).Title(), staff.Position(ev.To).Title())}
		default:
			continue
		}
		in.Data = map[string]string{"staff_id": m.ID}
		in.DedupeKey = "staff:" + ev.ID
		captured := in
		s.publishAsync(m.RestaurantID, func(context.Context) (Input, bool) { return captured, true })
	}
}

// ─── Fon vazifalari ────────────────────────────────────────────────────

// JobDeps — fon vazifalari uchun manbalar (`main` ulaydi).
type JobDeps struct {
	// RecentOrders — oxirgi buyurtmalar (qabul qilinmagan buyurtma eslatmasi).
	RecentOrders func(ctx context.Context) ([]*orders.Order, error)
	// Restaurants — barcha restoran ID'lari (kunlik hisobot).
	Restaurants func(ctx context.Context) ([]string, error)
	// DaySummary — [from, to) oralig'idagi buyurtmalar xulosasi. nil — hisobot yo'q.
	DaySummary func(ctx context.Context, restaurantID string, from, to time.Time) (DaySummary, error)
}

type DaySummary struct {
	Orders       int
	Completed    int
	Cancelled    int
	RevenueTiyin int64
}

const (
	// WaitingAfter — shuncha vaqt qabul qilinmagan buyurtma uchun eslatma.
	WaitingAfter = 5 * time.Minute
	// waitingMaxAge — juda eski (masalan server uzoq o'chiq bo'lgan)
	// buyurtmalar uchun eslatma yuborilmaydi — ular endi eslatma emas.
	waitingMaxAge = 3 * time.Hour
)

// CheckWaitingOrders — qabul qilinmagan buyurtmalar eslatmasi (har
// buyurtma uchun BIR marta — `DedupeKey`). Yuborilganlar sonini qaytaradi.
func (s *Service) CheckWaitingOrders(ctx context.Context, d JobDeps) (int, error) {
	if d.RecentOrders == nil {
		return 0, nil
	}
	list, err := d.RecentOrders(ctx)
	if err != nil {
		return 0, err
	}
	now := s.now()
	sent := 0
	for _, o := range list {
		if o == nil || o.Status != orders.StatusCreated || o.AwaitingPayment() {
			continue
		}
		age := now.Sub(o.CreatedAt)
		if age < WaitingAfter || age > waitingMaxAge {
			continue
		}
		n, err := s.Publish(ctx, o.RestaurantID, Input{
			Kind: KindOrderWaiting, Category: CategoryReminder,
			Title: "Buyurtma qabul qilinmagan",
			Body: fmt.Sprintf("%s buyurtma %d daqiqadan beri javob kutmoqda. Mijozni kuttirmang.",
				orderLabel(o.OrderNumber, o.ID), int(age.Minutes())),
			Data:      map[string]string{"order_id": o.ID, "order_number": o.OrderNumber},
			DedupeKey: "order_waiting:" + o.ID,
		})
		if err != nil {
			return sent, err
		}
		if n != nil {
			sent++
		}
	}
	return sent, nil
}

// CheckDailyReports — kechagi kun hisoboti (Toshkent vaqti, kun
// tugagach). Buyurtma bo'lmagan restoranga yuborilmaydi.
func (s *Service) CheckDailyReports(ctx context.Context, d JobDeps) (int, error) {
	if d.Restaurants == nil || d.DaySummary == nil {
		return 0, nil
	}
	local := s.now().In(Location)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, Location)
	// Yarim tundan keyin 5 daqiqa — kechikkan yozuvlar ham kirsin.
	if local.Before(today.Add(5 * time.Minute)) {
		return 0, nil
	}
	from := today.AddDate(0, 0, -1)
	ids, err := d.Restaurants(ctx)
	if err != nil {
		return 0, err
	}
	dateKey := from.Format("2006-01-02")
	sent := 0
	for _, rid := range ids {
		sum, err := d.DaySummary(ctx, rid, from, today)
		if err != nil {
			slog.Warn("kunlik hisobot: xulosa olinmadi", "restaurant", rid, "err", err)
			continue
		}
		if sum.Orders == 0 {
			continue
		}
		n, err := s.Publish(ctx, rid, Input{
			Kind: KindDailyReport, Category: CategoryReport,
			Title: "Kunlik hisobot tayyor",
			Body: fmt.Sprintf("%d-%s: %d ta buyurtma, %d tasi bajarildi, %d tasi bekor qilindi. Savdo: %s so'm. Ko'rish uchun bosing.",
				from.Day(), uzMonths[from.Month()-1], sum.Orders, sum.Completed, sum.Cancelled, formatSum(sum.RevenueTiyin)),
			Data:      map[string]string{"date": dateKey},
			DedupeKey: "daily_report:" + dateKey,
		})
		if err != nil {
			return sent, err
		}
		if n != nil {
			sent++
		}
	}
	return sent, nil
}

// RunJobs — fon vazifalarini ishga tushiradi (ctx tugaguncha).
func (s *Service) RunJobs(ctx context.Context, d JobDeps) {
	run := func(name string, every time.Duration, fn func(ctx context.Context) error) {
		safegoLoop(ctx, name, every, fn)
	}
	run("alerts.waiting", 30*time.Second, func(ctx context.Context) error {
		_, err := s.CheckWaitingOrders(ctx, d)
		return err
	})
	run("alerts.daily", 5*time.Minute, func(ctx context.Context) error {
		_, err := s.CheckDailyReports(ctx, d)
		return err
	})
	run("alerts.retention", 6*time.Hour, func(ctx context.Context) error {
		_, err := s.store.DeleteOlderThan(ctx, s.now().Add(-Retention))
		return err
	})
}
