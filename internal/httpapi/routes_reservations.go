package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"chustapp/internal/modules/reservation"
	"chustapp/internal/payments"
	"chustapp/internal/users"
)

// ┌─ STOL BRONI (RS-3/RS-4) ──────────────────────────────────────────────┐
// Mijoz:
//   GET  /restaurants/{id}/reservations/availability?date=YYYY-MM-DD&party=N
//   POST /restaurants/{id}/reservations   {starts_at, party_size, zone, method, channel, comment}
//   GET  /me/reservations
//   POST /me/reservations/{id}/cancel
//   POST /me/reservations/{id}/pay        (to'lanmagan karta zakladi havolasi)
// Restoran paneli:
//   GET  /restaurants/{id}/reservations?from=RFC3339&to=RFC3339
//   GET  /restaurants/{id}/reservation-settings
//   PUT  /restaurants/{id}/reservation-settings
//   POST /reservations/{id}/seat | /complete | /no-show | /cancel {reason}
// Qoidalar `internal/modules/reservation` da; bu yerda faqat HTTP va egalik.
// └──────────────────────────────────────────────────────────────────────┘

func reservationStatus(err error) int {
	switch {
	case errors.Is(err, reservation.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, reservation.ErrInvalid), errors.Is(err, reservation.ErrClosed):
		return http.StatusBadRequest
	case errors.Is(err, reservation.ErrDisabled), errors.Is(err, reservation.ErrPaymentDisabled),
		errors.Is(err, payments.ErrDepositsDisabled), errors.Is(err, payments.ErrNotConfigured):
		return http.StatusUnprocessableEntity
	case errors.Is(err, reservation.ErrNoTable), errors.Is(err, reservation.ErrSlotTaken),
		errors.Is(err, reservation.ErrConflict), errors.Is(err, payments.ErrAlreadyPaid):
		return http.StatusConflict
	case errors.Is(err, reservation.ErrTooMany):
		return http.StatusTooManyRequests
	case errors.Is(err, reservation.ErrWalletFunds):
		return http.StatusPaymentRequired
	}
	return http.StatusInternalServerError
}

// reservationView — panel uchun: mijoz ismi va telefoni qo'shiladi
// (restoran mijozga qo'ng'iroq qila olishi kerak).
func (s *Server) reservationView(r *http.Request, x *reservation.Reservation, withCustomer bool) map[string]any {
	v := map[string]any{
		"id": x.ID, "restaurant_id": x.RestaurantID, "table_id": x.TableID, "table_label": x.TableLabel,
		"party_size": x.PartySize, "starts_at": x.StartsAt, "ends_at": x.EndsAt, "status": x.Status,
		"deposit_tiyin": x.DepositTiyin, "deposit_method": x.DepositMethod, "deposit_state": x.DepositState,
		"comment": x.Comment, "hold_expires_at": x.HoldExpiresAt, "cancel_reason": x.CancelReason,
		"cancelled_by": x.CancelledBy, "created_at": x.CreatedAt,
		// Mijoz uchun: bepul bekor qilish shu paytgacha.
		"free_cancel_until": x.StartsAt.Add(-reservation.FreeCancelBefore),
	}
	if withCustomer && s.UserRepo != nil {
		if u, err := s.UserRepo.GetByID(r.Context(), x.CustomerID); err == nil {
			v["customer_name"], v["customer_phone"] = u.Name, u.Phone
		}
	}
	return v
}

// payLinkView — to'lov havolasi + ilova WebView'i kutadigan qaytish manzili.
func (s *Server) payLinkView(l *reservation.PayLink) map[string]any {
	v := map[string]any{"payment_id": l.PaymentID, "pay_url": l.PayURL, "expires_at": l.ExpiresAt}
	if s.Payments != nil {
		v["return_url"] = s.Payments.ReturnURL()
	}
	return v
}

// bookableTables — bronga yaroqli (faol, sig'imi kiritilgan) stollar soni.
// 0 bo'lsa bron yoqilgan bo'lsa ham bo'sh vaqt chiqmaydi — panel ogohlantiradi.
func (s *Server) bookableTables(r *http.Request, restaurantID string) int {
	if s.TableSvc == nil {
		return 0
	}
	list, err := s.TableSvc.List(r.Context(), restaurantID)
	if err != nil {
		return 0
	}
	n := 0
	for _, t := range list {
		if t.Active && t.Capacity != nil && *t.Capacity > 0 {
			n++
		}
	}
	return n
}

func (s *Server) registerReservationRoutes(mux *http.ServeMux) {
	staff := []users.Role{users.RoleRestaurant, users.RoleAdmin}
	customer := []users.Role{users.RoleCustomer}

	ready := func(w http.ResponseWriter) bool {
		if s.ReservationSvc == nil {
			httpError(w, http.StatusServiceUnavailable, errors.New("stol broni sozlanmagan"))
			return false
		}
		return true
	}
	fail := func(w http.ResponseWriter, err error) { httpError(w, reservationStatus(err), err) }

	// ── Mijoz ──

	// OCHIQ (menyu kabi): bo'sh vaqtlar va zaklad qoidalari — istalgan foydalanuvchi
	// (mehmon, admin akkauntidagi egasi ham) ko'radi; bron qilishning o'zi mijoz
	// akkauntini talab qiladi. Avval faqat mijoz roli edi va admin/kirmagan
	// foydalanuvchida ilova bron tugmasini umuman ko'rsatmasdi (2026-11-17).
	mux.HandleFunc("GET /restaurants/{id}/reservations/availability", publicCache(0, 0,
		func(w http.ResponseWriter, r *http.Request) {
			if !ready(w) {
				return
			}
			rid := r.PathValue("id")
			if _, err := s.CatalogRepo.GetRestaurant(r.Context(), rid); err != nil {
				fail(w, reservation.ErrNotFound)
				return
			}
			day, err := time.ParseInLocation("2006-01-02", r.URL.Query().Get("date"), reservation.Tashkent)
			if err != nil {
				httpError(w, http.StatusBadRequest, errors.New("date: YYYY-MM-DD"))
				return
			}
			party, _ := strconv.Atoi(r.URL.Query().Get("party"))
			st, err := s.ReservationSvc.Settings(r.Context(), rid)
			if err != nil {
				fail(w, err)
				return
			}
			slots, err := s.ReservationSvc.Availability(r.Context(), rid, day, party)
			if err != nil {
				fail(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"enabled": st.Enabled, "deposit_per_person_tiyin": st.DepositPerPersonTiyin,
				"zone_deposits": st.ZoneDeposits, "slot_minutes": st.SlotMinutes,
				"free_cancel_hours": int(reservation.FreeCancelBefore.Hours()),
				"wallet": s.WalletSvc != nil, "card": s.Payments != nil,
				"slots": slots, "bookable_tables": s.bookableTables(r, rid),
			})
		}))

	mux.HandleFunc("POST /restaurants/{id}/reservations", s.auth(customer,
		func(w http.ResponseWriter, r *http.Request) {
			if !ready(w) {
				return
			}
			var req struct {
				StartsAt  time.Time `json:"starts_at"`
				PartySize int       `json:"party_size"`
				Zone      string    `json:"zone"`
				Method    string    `json:"method"`
				Channel   string    `json:"channel"`
				Comment   string    `json:"comment"`
			}
			if !decodeJSON(w, r, &req) {
				return
			}
			res, err := s.ReservationSvc.Create(r.Context(), reservation.CreateInput{
				RestaurantID: r.PathValue("id"), CustomerID: claimsFrom(r).Subject,
				StartsAt: req.StartsAt, PartySize: req.PartySize, Zone: req.Zone,
				Method: reservation.Method(req.Method), Channel: req.Channel, Comment: req.Comment,
			})
			if err != nil {
				fail(w, err)
				return
			}
			out := map[string]any{"reservation": s.reservationView(r, res.Reservation, false)}
			if res.Pay != nil {
				out["pay"] = s.payLinkView(res.Pay)
			}
			writeJSON(w, http.StatusCreated, out)
		}))

	mux.HandleFunc("GET /me/reservations", s.auth(customer,
		func(w http.ResponseWriter, r *http.Request) {
			if !ready(w) {
				return
			}
			list, err := s.ReservationSvc.ForCustomer(r.Context(), claimsFrom(r).Subject)
			if err != nil {
				fail(w, err)
				return
			}
			out := make([]map[string]any, 0, len(list))
			for _, x := range list {
				v := s.reservationView(r, x, false)
				if s.CatalogRepo != nil {
					if rest, err := s.CatalogRepo.GetRestaurant(r.Context(), x.RestaurantID); err == nil {
						v["restaurant_name"] = rest.Name
					}
				}
				out = append(out, v)
			}
			writeJSON(w, http.StatusOK, out)
		}))

	mux.HandleFunc("POST /me/reservations/{id}/cancel", s.auth(customer,
		func(w http.ResponseWriter, r *http.Request) {
			if !ready(w) {
				return
			}
			x, err := s.ReservationSvc.CancelByCustomer(r.Context(), claimsFrom(r).Subject, r.PathValue("id"))
			if err != nil {
				fail(w, err)
				return
			}
			writeJSON(w, http.StatusOK, s.reservationView(r, x, false))
		}))

	mux.HandleFunc("POST /me/reservations/{id}/pay", s.auth(customer,
		func(w http.ResponseWriter, r *http.Request) {
			if !ready(w) {
				return
			}
			if s.Payments == nil {
				fail(w, reservation.ErrPaymentDisabled)
				return
			}
			x, err := s.ReservationSvc.Get(r.Context(), r.PathValue("id"))
			if err != nil || x.CustomerID != claimsFrom(r).Subject {
				fail(w, reservation.ErrNotFound)
				return
			}
			if x.Status != reservation.StatusHeld || x.DepositMethod != reservation.MethodCard {
				fail(w, reservation.ErrConflict)
				return
			}
			p, err := s.Payments.StartForReservation(r.Context(), x.ID, x.CustomerID, x.RestaurantID,
				"Stol broni zakladi", r.URL.Query().Get("channel"))
			if err != nil {
				fail(w, err)
				return
			}
			writeJSON(w, http.StatusOK, s.payLinkView(&reservation.PayLink{PaymentID: p.ID, PayURL: p.PayURL, ExpiresAt: p.ExpiresAt}))
		}))

	// ── Restoran paneli ──

	mux.HandleFunc("GET /restaurants/{id}/reservations", s.auth(staff,
		func(w http.ResponseWriter, r *http.Request) {
			if !ready(w) {
				return
			}
			rid, ok := entityIDFor(claimsFrom(r), r.PathValue("id"))
			if !ok {
				fail(w, reservation.ErrNotFound)
				return
			}
			from, err1 := time.Parse(time.RFC3339, r.URL.Query().Get("from"))
			to, err2 := time.Parse(time.RFC3339, r.URL.Query().Get("to"))
			if err1 != nil || err2 != nil {
				httpError(w, http.StatusBadRequest, errors.New("from/to: RFC3339"))
				return
			}
			list, err := s.ReservationSvc.ForRestaurant(r.Context(), rid, from, to)
			if err != nil {
				fail(w, err)
				return
			}
			out := make([]map[string]any, 0, len(list))
			for _, x := range list {
				out = append(out, s.reservationView(r, x, true))
			}
			writeJSON(w, http.StatusOK, out)
		}))

	mux.HandleFunc("GET /restaurants/{id}/reservation-settings", s.auth(staff,
		func(w http.ResponseWriter, r *http.Request) {
			if !ready(w) {
				return
			}
			rid, ok := entityIDFor(claimsFrom(r), r.PathValue("id"))
			if !ok {
				fail(w, reservation.ErrNotFound)
				return
			}
			st, err := s.ReservationSvc.Settings(r.Context(), rid)
			if err != nil {
				fail(w, err)
				return
			}
			writeJSON(w, http.StatusOK, st)
		}))

	mux.HandleFunc("PUT /restaurants/{id}/reservation-settings", s.auth(staff,
		func(w http.ResponseWriter, r *http.Request) {
			if !ready(w) {
				return
			}
			rid, ok := entityIDFor(claimsFrom(r), r.PathValue("id"))
			if !ok {
				fail(w, reservation.ErrNotFound)
				return
			}
			var in reservation.Settings
			if !decodeJSON(w, r, &in) {
				return
			}
			in.RestaurantID = rid
			st, err := s.ReservationSvc.SaveSettings(r.Context(), in)
			if err != nil {
				fail(w, err)
				return
			}
			writeJSON(w, http.StatusOK, st)
		}))

	// Holat amallari: restoran ID'si TOKENDAN (admin — bronning o'zidan).
	act := func(do func(r *http.Request, rid, id string) (*reservation.Reservation, error)) http.HandlerFunc {
		return s.auth(staff, func(w http.ResponseWriter, r *http.Request) {
			if !ready(w) {
				return
			}
			id := r.PathValue("id")
			c := claimsFrom(r)
			rid := c.EntityID
			if c.Role == users.RoleAdmin {
				x, err := s.ReservationSvc.Get(r.Context(), id)
				if err != nil {
					fail(w, err)
					return
				}
				rid = x.RestaurantID
			}
			x, err := do(r, rid, id)
			if err != nil {
				fail(w, err)
				return
			}
			writeJSON(w, http.StatusOK, s.reservationView(r, x, true))
		})
	}
	mux.HandleFunc("POST /reservations/{id}/seat", act(func(r *http.Request, rid, id string) (*reservation.Reservation, error) {
		return s.ReservationSvc.Seat(r.Context(), rid, id)
	}))
	mux.HandleFunc("POST /reservations/{id}/complete", act(func(r *http.Request, rid, id string) (*reservation.Reservation, error) {
		return s.ReservationSvc.Complete(r.Context(), rid, id)
	}))
	mux.HandleFunc("POST /reservations/{id}/no-show", act(func(r *http.Request, rid, id string) (*reservation.Reservation, error) {
		return s.ReservationSvc.MarkNoShow(r.Context(), rid, id)
	}))
	mux.HandleFunc("POST /reservations/{id}/cancel", act(func(r *http.Request, rid, id string) (*reservation.Reservation, error) {
		var req struct {
			Reason string `json:"reason"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&req); err != nil {
			return nil, reservation.ErrInvalid
		}
		return s.ReservationSvc.CancelByRestaurant(r.Context(), rid, id, req.Reason)
	}))
}
