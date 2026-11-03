package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"chustapp/internal/couriers"
	"chustapp/internal/qurtorders"
	"chustapp/internal/ratelimit"
	"chustapp/internal/users"
)

// Qurt kuryeri amallari: kuryer bo'yicha cheklov (bosib yuborish, suiiste'mol).
var qurtCourierLimiter = ratelimit.New(2, 30)

// qurtCourierID — Qurt kuryeri yozuvining barqaror ID'si (akkaunt ID'sidan): qayta
// qo'shilganda xuddi o'sha yozuv qaytadi va yetkazmalar/pul tarixi uzilmaydi.
func qurtCourierID(userID string) string { return "qurt-" + userID }

// courierOrderView — kuryer ko'radigan buyurtma. Mijozning ismi, telefoni va manzili
// FAQAT do'kondan olingandan keyin (collected va keyin) ochiladi: olishdan oldin kuryer
// faqat do'kon va tovarni biladi.
type courierOrderView struct {
	ID           string            `json:"id"`
	Number       string            `json:"number"`
	Status       qurtorders.Status `json:"status"`
	DeliveryDate string            `json:"delivery_date"`
	StoreName    string            `json:"store_name"`
	Items        []courierItemView `json:"items"`
	TotalTiyin   int64             `json:"total_tiyin"`
	CashStatus   string            `json:"cash_status,omitempty"`
	RejectReason string            `json:"reject_reason,omitempty"`
	Customer     *courierCustomer  `json:"customer,omitempty"`
}

type courierItemView struct {
	Name        string `json:"name"`
	Qty         int    `json:"qty"`
	WeightGrams int    `json:"weight_grams,omitempty"`
}

type courierCustomer struct {
	Name    string             `json:"name"`
	Phone   string             `json:"phone"`
	Address qurtorders.Address `json:"address"`
}

func toCourierView(o *qurtorders.Order) courierOrderView {
	v := courierOrderView{
		ID: o.ID, Number: o.Number, Status: o.Status, DeliveryDate: o.DeliveryDate,
		StoreName: o.StoreName, TotalTiyin: o.TotalTiyin, CashStatus: string(o.CashStatus),
		RejectReason: o.RejectReason, Items: make([]courierItemView, 0, len(o.Items)),
	}
	for _, it := range o.Items {
		v.Items = append(v.Items, courierItemView{Name: it.Name, Qty: it.Qty, WeightGrams: it.WeightGrams})
	}
	switch o.Status {
	case qurtorders.StatusCollected, qurtorders.StatusOutForDelivery, qurtorders.StatusFailed, qurtorders.StatusDelivered:
		v.Customer = &courierCustomer{Name: o.CustomerName, Phone: o.CustomerPhone, Address: o.Address}
	}
	return v
}

// qurtCourier — so'rov egasi tasdiqlangan QURT kuryerimi. Boshqa kuryerlarga (restoran/platforma)
// bu bo'lim yopiq: 403.
func (s *Server) qurtCourier(w http.ResponseWriter, r *http.Request) (*couriers.Courier, bool) {
	if s.QurtOrderSvc == nil || s.CourierRepo == nil {
		httpError(w, http.StatusServiceUnavailable, errors.New("qurt logistikasi sozlanmagan"))
		return nil, false
	}
	c, err := s.CourierRepo.GetByID(r.Context(), claimsFrom(r).EntityID)
	if err != nil || c.RestaurantID != qurtorders.QurtPool || !c.Approved {
		httpError(w, http.StatusForbidden, errors.New("bu bo'lim faqat Qurt kuryerlari uchun"))
		return nil, false
	}
	return c, true
}

func (s *Server) registerQurtCourierRoutes(mux *http.ServeMux) {
	courier := []users.Role{users.RoleCourier}

	// GET /courier/qurt/me — Qurt kuryerimi? + kuryerdagi pul. Ilova shu orqali Qurt bo'limini ochadi
	// (403 = oddiy kuryer).
	mux.HandleFunc("GET /courier/qurt/me", s.auth(courier, func(w http.ResponseWriter, r *http.Request) {
		c, ok := s.qurtCourier(w, r)
		if !ok {
			return
		}
		sum, err := s.QurtOrderSvc.CourierSummary(r.Context(), c.ID)
		if err != nil {
			qurtOrderError(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, map[string]any{"courier_id": c.ID, "name": c.Name, "summary": sum})
	}))

	// GET /courier/qurt/orders — menga biriktirilgan faol buyurtmalar (reys sanasi bo'yicha).
	mux.HandleFunc("GET /courier/qurt/orders", s.auth(courier, func(w http.ResponseWriter, r *http.Request) {
		c, ok := s.qurtCourier(w, r)
		if !ok {
			return
		}
		list, err := s.QurtOrderSvc.Repo.List(r.Context(), qurtorders.Filter{
			CourierID: c.ID, Limit: 500, Statuses: []qurtorders.Status{
				qurtorders.StatusConfirmed, qurtorders.StatusPacked, qurtorders.StatusCollected,
				qurtorders.StatusOutForDelivery, qurtorders.StatusFailed,
			},
		})
		if err != nil {
			qurtOrderError(w, err)
			return
		}
		sort.SliceStable(list, func(i, j int) bool { return list[i].DeliveryDate < list[j].DeliveryDate })
		views := make([]courierOrderView, 0, len(list))
		for _, o := range list {
			views = append(views, toCourierView(o))
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, map[string]any{"orders": views})
	}))

	// POST /courier/qurt/orders/{id}/transition {to, reason} — kuryer faqat O'Z buyurtmasini va
	// faqat logistika qadamlarini o'zgartiradi: collected | out_for_delivery | delivered | failed.
	mux.HandleFunc("POST /courier/qurt/orders/{id}/transition", s.auth(courier, func(w http.ResponseWriter, r *http.Request) {
		c, ok := s.qurtCourier(w, r)
		if !ok {
			return
		}
		if rateLimited(w, qurtCourierLimiter, c.ID) {
			return
		}
		var req struct {
			To     string `json:"to"`
			Reason string `json:"reason"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		to := qurtorders.Status(req.To)
		switch to {
		case qurtorders.StatusCollected, qurtorders.StatusOutForDelivery, qurtorders.StatusDelivered, qurtorders.StatusFailed:
		default:
			httpError(w, http.StatusBadRequest, errors.New("kuryer bu holatni o'zgartira olmaydi"))
			return
		}
		o, err := s.QurtOrderSvc.Transition(r.Context(), r.PathValue("id"), to, qurtorders.ActorLogistics, c.ID, req.Reason,
			func(o *qurtorders.Order) error {
				if o.CourierID != c.ID {
					return qurtorders.ErrNotFound // boshqa kuryerning buyurtmasi mavjud emasdek
				}
				return nil
			})
		if err != nil {
			qurtOrderError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toCourierView(o))
	}))

	// ── Admin: Qurt kuryerlari, biriktirish, naqd pul ──────────────────
	admin := []users.Role{users.RoleAdmin}

	// GET /admin/qurt/couriers — Qurt kuryerlari: telefon, faol buyurtmalar, kuryerdagi pul.
	mux.HandleFunc("GET /admin/qurt/couriers", s.auth(admin, func(w http.ResponseWriter, r *http.Request) {
		if !s.qurtOrdersReady(w) || s.CourierRepo == nil {
			return
		}
		all, err := s.CourierRepo.ListAll(r.Context())
		if err != nil {
			httpError(w, http.StatusInternalServerError, errors.New("server xatosi"))
			return
		}
		phones := map[string]string{}
		if accs, err := s.UserRepo.ListByRole(r.Context(), users.RoleCourier); err == nil {
			for _, u := range accs {
				if u != nil {
					phones[u.EntityID] = u.Phone
				}
			}
		}
		out := []map[string]any{}
		for _, c := range all {
			if c == nil || c.RestaurantID != qurtorders.QurtPool || !c.Approved {
				continue
			}
			sum, err := s.QurtOrderSvc.CourierSummary(r.Context(), c.ID)
			if err != nil {
				qurtOrderError(w, err)
				return
			}
			out = append(out, map[string]any{
				"id": c.ID, "name": c.Name, "phone": phones[c.ID], "summary": sum,
			})
		}
		sort.Slice(out, func(i, j int) bool { return out[i]["name"].(string) < out[j]["name"].(string) })
		writeJSON(w, http.StatusOK, map[string]any{"couriers": out})
	}))

	// POST /admin/qurt/couriers {phone, name} — Qurt kuryeri qo'shish (OnDex kuryerlaridan ALOHIDA).
	mux.HandleFunc("POST /admin/qurt/couriers", s.auth(admin, func(w http.ResponseWriter, r *http.Request) {
		if !s.qurtOrdersReady(w) || s.CourierRepo == nil {
			return
		}
		var req struct {
			Phone string `json:"phone"`
			Name  string `json:"name"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		name := strings.TrimSpace(req.Name)
		if n := utf8.RuneCountInString(name); n < 2 || n > 80 {
			httpError(w, http.StatusBadRequest, errors.New("ism 2..80 belgi bo'lsin"))
			return
		}
		phone, err := users.NormalizePhone(req.Phone)
		if err != nil {
			httpError(w, http.StatusBadRequest, errors.New("telefon raqami noto'g'ri"))
			return
		}
		c, err := s.addQurtCourier(r.Context(), phone, name)
		switch {
		case errors.Is(err, errQurtCourierConflict):
			httpError(w, http.StatusConflict, errors.New("bu raqamda boshqa rolli akkaunt bor — boshqa raqam kiriting"))
		case err != nil:
			slog.Error("qurt kuryeri qo'shilmadi", "err", err)
			httpError(w, http.StatusInternalServerError, errors.New("server xatosi"))
		default:
			writeJSON(w, http.StatusCreated, map[string]any{"id": c.ID, "name": c.Name, "phone": phone})
		}
	}))

	// DELETE /admin/qurt/couriers/{id} — kuryerni olib tashlash: faol buyurtma yoki topshirilmagan
	// pul bo'lmasa.
	mux.HandleFunc("DELETE /admin/qurt/couriers/{id}", s.auth(admin, func(w http.ResponseWriter, r *http.Request) {
		if !s.qurtOrdersReady(w) || s.CourierRepo == nil {
			return
		}
		id := r.PathValue("id")
		c, err := s.CourierRepo.GetByID(r.Context(), id)
		if err != nil || c.RestaurantID != qurtorders.QurtPool {
			httpError(w, http.StatusNotFound, errors.New("kuryer topilmadi"))
			return
		}
		sum, err := s.QurtOrderSvc.CourierSummary(r.Context(), id)
		if err != nil {
			qurtOrderError(w, err)
			return
		}
		if sum.ActiveCount > 0 || sum.HeldOrders > 0 {
			httpError(w, http.StatusConflict, errors.New("kuryerda faol buyurtma yoki topshirilmagan pul bor"))
			return
		}
		if err := s.CourierRepo.SetApproved(r.Context(), id, false); err != nil {
			httpError(w, http.StatusInternalServerError, errors.New("server xatosi"))
			return
		}
		if accs, err := s.UserRepo.ListByRole(r.Context(), users.RoleCourier); err == nil {
			for _, u := range accs {
				if u != nil && u.EntityID == id {
					if err := s.UserRepo.UpdateRole(r.Context(), u.ID, users.RoleCustomer, ""); err == nil && s.Revoked != nil {
						s.Revoked.Revoke(r.Context(), u.ID) // eski token roli ishlamasin
					}
				}
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	// POST /admin/qurt/assign {courier_id, date, order_ids} — reys (yoki tanlangan buyurtmalar)
	// kuryerga biriktiriladi.
	mux.HandleFunc("POST /admin/qurt/assign", s.auth(admin, func(w http.ResponseWriter, r *http.Request) {
		if !s.qurtOrdersReady(w) || s.CourierRepo == nil {
			return
		}
		var req struct {
			CourierID string   `json:"courier_id"`
			Date      string   `json:"date"`
			OrderIDs  []string `json:"order_ids"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		c, err := s.CourierRepo.GetByID(r.Context(), req.CourierID)
		if err != nil || c.RestaurantID != qurtorders.QurtPool || !c.Approved {
			httpError(w, http.StatusBadRequest, errors.New("Qurt kuryeri topilmadi"))
			return
		}
		if req.Date != "" {
			if _, err := time.Parse("2006-01-02", req.Date); err != nil {
				httpError(w, http.StatusBadRequest, errors.New("sana noto'g'ri (YYYY-MM-DD)"))
				return
			}
		}
		res, err := s.QurtOrderSvc.Assign(r.Context(), c.ID, req.Date, req.OrderIDs)
		if err != nil {
			qurtOrderError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, res)
	}))

	// POST /admin/qurt/orders/{id}/unassign — buyurtmani kuryerdan olish (yo'lga chiqmagan bo'lsa).
	mux.HandleFunc("POST /admin/qurt/orders/{id}/unassign", s.auth(admin, func(w http.ResponseWriter, r *http.Request) {
		if !s.qurtOrdersReady(w) {
			return
		}
		if err := s.QurtOrderSvc.Unassign(r.Context(), r.PathValue("id")); err != nil {
			qurtOrderError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	// POST /admin/qurt/couriers/{id}/settle {order_ids} — kuryer topshirgan naqd pulni qabul qilish.
	mux.HandleFunc("POST /admin/qurt/couriers/{id}/settle", s.auth(admin, func(w http.ResponseWriter, r *http.Request) {
		if !s.qurtOrdersReady(w) {
			return
		}
		var req struct {
			OrderIDs []string `json:"order_ids"`
		}
		_ = decodeOptionalJSON(r, &req)
		res, err := s.QurtOrderSvc.Settle(r.Context(), r.PathValue("id"), claimsFrom(r).Subject, req.OrderIDs)
		if err != nil {
			qurtOrderError(w, err)
			return
		}
		slog.Info("qurt: kuryer puli qabul qilindi", "courier", r.PathValue("id"), "admin", claimsFrom(r).Subject,
			"orders", res.Orders, "tiyin", res.TotalTiyin)
		writeJSON(w, http.StatusOK, res)
	}))
}

var errQurtCourierConflict = errors.New("telefon raqamida boshqa rolli akkaunt bor")

// addQurtCourier — Qurt kuryeri akkauntini yaratadi yoki qayta yoqadi. Begona akkauntga
// (boshqa rol/restoran xodimi) TEGILMAYDI: faqat yangi raqam, mijoz akkaunti yoki shu
// kuryerning o'zi ulanadi. Tasdiq (`approved`) eng oxirida yoqiladi (fail closed).
func (s *Server) addQurtCourier(ctx context.Context, phone, name string) (*couriers.Courier, error) {
	prepare := func(userID string) (*couriers.Courier, error) {
		id := qurtCourierID(userID)
		c, err := s.CourierRepo.GetByID(ctx, id)
		switch {
		case err == nil:
			if c.RestaurantID != qurtorders.QurtPool {
				return nil, errQurtCourierConflict
			}
			if c.Name != name {
				if err := s.CourierRepo.SetName(ctx, id, name); err != nil {
					return nil, err
				}
				c.Name = name
			}
			return c, nil
		case !errors.Is(err, couriers.ErrNoCourier):
			return nil, err
		}
		nc := &couriers.Courier{
			ID: id, Name: name, RestaurantID: qurtorders.QurtPool,
			VehicleType: couriers.VehicleMoped, Rating: 5.0,
		}
		return nc, s.CourierRepo.Create(ctx, nc)
	}

	u, err := s.UserRepo.GetByPhone(ctx, phone)
	switch {
	case err == nil:
		switch {
		case u.Role == users.RoleCourier && u.EntityID == qurtCourierID(u.ID):
			// shu kuryerning o'zi — qayta yoqish
		case u.Role == users.RoleCustomer && u.EntityID == "":
			// oddiy mijoz akkaunti — kuryerga aylantiriladi
		default:
			return nil, errQurtCourierConflict
		}
		c, err := prepare(u.ID)
		if err != nil {
			return nil, err
		}
		if u.Role != users.RoleCourier {
			if err := s.UserRepo.UpdateRole(ctx, u.ID, users.RoleCourier, c.ID); err != nil {
				return nil, err
			}
			if s.Revoked != nil {
				s.Revoked.Revoke(ctx, u.ID) // eski (mijoz) token yangi rol bilan almashsin
			}
		}
		return c, s.CourierRepo.SetApproved(ctx, c.ID, true)
	case !errors.Is(err, users.ErrUserNotFound):
		return nil, err
	}

	uid := NewID()
	c, err := prepare(uid)
	if err != nil {
		return nil, err
	}
	acc := users.User{ID: uid, Phone: phone, Name: name, Role: users.RoleCourier, EntityID: c.ID, CreatedAt: time.Now()}
	if err := s.UserRepo.Create(ctx, &acc); err != nil {
		if errors.Is(err, users.ErrPhoneTaken) {
			return nil, errQurtCourierConflict
		}
		return nil, err
	}
	return c, s.CourierRepo.SetApproved(ctx, c.ID, true)
}
