package httpapi

import (
	"net/http"
	"sort"
	"time"

	"chustapp/internal/modules/reservation"
	"chustapp/internal/orders"
	"chustapp/internal/users"
)

// ┌─ RS-6: ADMIN — RESTORANLAR JONLI HOLATI ──────────────────────────────┐
// GET /admin/restaurants/live — butun platforma bo'yicha "hozir nima
// bo'lyapti": har restoran uchun faol buyurtmalar holati, KECHIKAYOTGAN
// buyurtmalar (sabab bilan), bugungi bronlar va ochiq stol hisoblari.
// Admin paneli 10 soniyada so'raydi. Mijoz shaxsiy ma'lumoti (telefon,
// manzil) BERILMAYDI — faqat operatsion raqamlar.
//
// Kechikish qoidalari (operatsion, sozlanmaydi):
//   * created  — 5 daqiqadan ko'p qabul qilinmagan;
//   * accepted/preparing — va'da qilingan tayyorlash vaqtidan 10 daq o'tgan;
//   * ready    — yetkazishda 10 daq kuryer olmagan, stolda 15 daq berilmagan;
//   * picked_up — 60 daqiqadan ko'p yo'lda.
// └──────────────────────────────────────────────────────────────────────┘

type liveDelay struct {
	OrderID     string `json:"order_id"`
	OrderNumber string `json:"order_number"`
	Status      string `json:"status"`
	Type        string `json:"type"`
	Minutes     int    `json:"minutes"`
	Reason      string `json:"reason"`
}

type liveRestaurant struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	OpenNow      bool           `json:"open_now"`
	Active       map[string]int `json:"active"`
	ActiveTotal  int            `json:"active_total"`
	Delayed      []liveDelay    `json:"delayed"`
	Reservations map[string]int `json:"reservations"`
	OpenBills    int            `json:"open_bills"`
	BillsDue     int64          `json:"bills_due_tiyin"`
}

// statusSince — buyurtma shu holatga qachon o'tgani (tarixdan).
func statusSince(o *orders.Order) time.Time {
	for i := len(o.History) - 1; i >= 0; i-- {
		if o.History[i].To == o.Status {
			return o.History[i].At
		}
	}
	return o.CreatedAt
}

// orderDelay — kechikish (yo'q bo'lsa ok=false).
func orderDelay(o *orders.Order, now time.Time) (liveDelay, bool) {
	since := statusSince(o)
	d := liveDelay{OrderID: o.ID, OrderNumber: o.OrderNumber, Status: string(o.Status), Type: string(o.Type.Normalized())}
	over := func(limit time.Duration, reason string) (liveDelay, bool) {
		if late := now.Sub(since) - limit; late > 0 {
			d.Minutes, d.Reason = int(now.Sub(since).Minutes()), reason
			return d, true
		}
		return d, false
	}
	switch o.Status {
	case orders.StatusCreated:
		return over(5*time.Minute, "restoran qabul qilmayapti")
	case orders.StatusAccepted, orders.StatusPreparing:
		prep := time.Duration(o.PreparationMinutes) * time.Minute
		if prep <= 0 {
			prep = 30 * time.Minute
		}
		// Tayyorlash vaqti "qabul qilindi" dan hisoblanadi.
		for _, h := range o.History {
			if h.To == orders.StatusAccepted {
				since = h.At
			}
		}
		return over(prep+10*time.Minute, "tayyorlash va'da qilinganidan kechikdi")
	case orders.StatusReady:
		if o.IsDineIn() {
			return over(15*time.Minute, "tayyor taom stolga berilmadi")
		}
		return over(10*time.Minute, "tayyor taomni kuryer olmadi")
	case orders.StatusPickedUp:
		return over(60*time.Minute, "yetkazish uzoq davom etmoqda")
	}
	return d, false
}

func (s *Server) registerAdminLiveRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/restaurants/live", s.auth([]users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			now := time.Now()
			rests, err := s.CatalogRepo.ListRestaurants(ctx)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			out := make([]*liveRestaurant, 0, len(rests))
			byID := map[string]*liveRestaurant{}
			for _, rest := range rests {
				lr := &liveRestaurant{ID: rest.ID, Name: rest.Name, OpenNow: rest.AcceptingOrdersAt(now),
					Active: map[string]int{}, Delayed: []liveDelay{}, Reservations: map[string]int{}}
				out = append(out, lr)
				byID[rest.ID] = lr
			}
			recent, err := s.OrderRepo.ListRecent(ctx, 2000)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			for _, o := range recent {
				lr := byID[o.RestaurantID]
				if lr == nil || o.IsTerminal() || now.Sub(o.CreatedAt) > 24*time.Hour {
					continue
				}
				// To'lanmagan karta buyurtmasi restoranga hali ko'rinmaydi.
				if o.PaymentMethod == orders.PaymentCard && o.PaymentState != orders.PaymentHeld && o.PaymentState != orders.PaymentPaid {
					continue
				}
				lr.Active[string(o.Status)]++
				lr.ActiveTotal++
				if d, ok := orderDelay(o, now); ok {
					lr.Delayed = append(lr.Delayed, d)
				}
			}
			dayStart := time.Date(now.In(reservation.Tashkent).Year(), now.In(reservation.Tashkent).Month(),
				now.In(reservation.Tashkent).Day(), 0, 0, 0, 0, reservation.Tashkent)
			for _, lr := range out {
				if s.ReservationSvc != nil {
					if list, err := s.ReservationSvc.ForRestaurant(ctx, lr.ID, dayStart, dayStart.Add(24*time.Hour)); err == nil {
						for _, x := range list {
							lr.Reservations[string(x.Status)]++
						}
					}
				}
				if s.TableBillSvc != nil {
					if bills, err := s.TableBillSvc.OpenBills(ctx, lr.ID); err == nil {
						for _, b := range bills {
							lr.OpenBills++
							lr.BillsDue += b.RemainingTiyin
						}
					}
				}
				sort.Slice(lr.Delayed, func(i, j int) bool { return lr.Delayed[i].Minutes > lr.Delayed[j].Minutes })
			}
			// Muammolilar tepada: kechikish soni, keyin faol buyurtmalar.
			sort.SliceStable(out, func(i, j int) bool {
				if len(out[i].Delayed) != len(out[j].Delayed) {
					return len(out[i].Delayed) > len(out[j].Delayed)
				}
				return out[i].ActiveTotal > out[j].ActiveTotal
			})
			writeJSON(w, http.StatusOK, map[string]any{"generated_at": now.UTC(), "restaurants": out})
		}))
}
