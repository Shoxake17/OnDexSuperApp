package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"chustapp/internal/couriers"
	"chustapp/internal/notify"
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
	if err != nil || !qurtorders.IsQurtPool(c.RestaurantID) || !c.Approved {
		httpError(w, http.StatusForbidden, errors.New("bu bo'lim faqat Qurt kuryerlari uchun"))
		return nil, false
	}
	return c, true
}

func (s *Server) registerQurtCourierRoutes(mux *http.ServeMux) {
	courier := []users.Role{users.RoleCourier}

	// Biriktirish HECH QACHON avtomatik emas: do'kon buyurtmani Logistika sahifasida kuryerga QO'LDA
	// biriktiradi. Shundan keyingina kuryerga xabar boradi (jonli kanal + push) va buyurtma
	// OnDexGO "Reys" ekranida paydo bo'ladi.
	if s.QurtOrderSvc != nil && s.QurtOrderSvc.OnAssigned == nil {
		s.QurtOrderSvc.OnAssigned = func(ctx context.Context, courierID string, count int) {
			if s.Notifier == nil || !strings.HasPrefix(courierID, "qurt-") {
				return
			}
			body := "Sizga yangi buyurtma biriktirildi"
			if count > 1 {
				body = strconv.Itoa(count) + " ta yangi buyurtma biriktirildi"
			}
			s.Notifier.Notify(ctx, strings.TrimPrefix(courierID, "qurt-"), notify.Event{
				Module: "shop", Kind: "trip_assigned", Title: "Reys", Body: body,
			})
		}
	}
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

	// ── Do'kon (POS serveri): o'z kuryerlari, biriktirish, naqd pul ────
	//
	// Logistikani SOTUVCHI o'zi hal qiladi: kuryer qo'shadi, o'z buyurtmalarini biriktiradi,
	// kuryer topshirgan pulni qabul qiladi. Har bir amal faqat O'Z do'koni doirasida: boshqa
	// do'kon kuryeri yoki buyurtmasi "topilmadi".

	ownCourier := func(w http.ResponseWriter, r *http.Request, storeID, id string) (*couriers.Courier, bool) {
		c, err := s.CourierRepo.GetByID(r.Context(), id)
		if err != nil || c.RestaurantID != qurtorders.PoolFor(storeID) {
			httpError(w, http.StatusNotFound, errors.New("kuryer topilmadi"))
			return nil, false
		}
		return c, true
	}

	// GET /merchant/qurt/couriers — do'konning kuryerlari: telefon, faol buyurtmalar, kuryerdagi pul.
	mux.HandleFunc("GET /merchant/qurt/couriers", func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.merchantQurtStore(w, r)
		if !ok || !s.qurtOrdersReady(w) || s.CourierRepo == nil {
			return
		}
		list, err := s.storeCouriers(r.Context(), id.EntityID)
		if err != nil {
			httpError(w, http.StatusInternalServerError, errors.New("server xatosi"))
			return
		}
		out := make([]map[string]any, 0, len(list))
		for _, c := range list {
			sum, err := s.QurtOrderSvc.CourierSummary(r.Context(), c.ID)
			if err != nil {
				qurtOrderError(w, err)
				return
			}
			out = append(out, map[string]any{"id": c.ID, "name": c.Name, "phone": c.phone, "summary": sum})
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, map[string]any{"couriers": out})
	})

	// POST /merchant/qurt/couriers {phone, name} — o'z kuryerini qo'shish.
	mux.HandleFunc("POST /merchant/qurt/couriers", func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.merchantQurtStore(w, r)
		if !ok || !s.qurtOrdersReady(w) || s.CourierRepo == nil {
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
		if list, err := s.storeCouriers(r.Context(), id.EntityID); err == nil && len(list) >= maxQurtCouriersPerStore {
			httpError(w, http.StatusConflict, errors.New("kuryerlar soni chegaraga yetdi"))
			return
		}
		c, err := s.addQurtCourier(r.Context(), id.EntityID, phone, name)
		switch {
		case errors.Is(err, errQurtCourierConflict):
			httpError(w, http.StatusConflict, errors.New("bu raqamda boshqa akkaunt bor — boshqa raqam kiriting"))
		case err != nil:
			slog.Error("qurt kuryeri qo'shilmadi", "store", id.EntityID, "err", err)
			httpError(w, http.StatusInternalServerError, errors.New("server xatosi"))
		default:
			writeJSON(w, http.StatusCreated, map[string]any{"id": c.ID, "name": c.Name, "phone": phone})
		}
	})

	// DELETE /merchant/qurt/couriers/{id} — faol buyurtma yoki topshirilmagan pul bo'lmasa.
	mux.HandleFunc("DELETE /merchant/qurt/couriers/{id}", func(w http.ResponseWriter, r *http.Request) {
		sid, ok := s.merchantQurtStore(w, r)
		if !ok || !s.qurtOrdersReady(w) || s.CourierRepo == nil {
			return
		}
		c, ok := ownCourier(w, r, sid.EntityID, r.PathValue("id"))
		if !ok {
			return
		}
		sum, err := s.QurtOrderSvc.CourierSummary(r.Context(), c.ID)
		if err != nil {
			qurtOrderError(w, err)
			return
		}
		if sum.ActiveCount > 0 || sum.HeldOrders > 0 {
			httpError(w, http.StatusConflict, errors.New("kuryerda faol buyurtma yoki topshirilmagan pul bor"))
			return
		}
		if err := s.CourierRepo.SetApproved(r.Context(), c.ID, false); err != nil {
			httpError(w, http.StatusInternalServerError, errors.New("server xatosi"))
			return
		}
		if accs, err := s.UserRepo.ListByRole(r.Context(), users.RoleCourier); err == nil {
			for _, u := range accs {
				if u != nil && u.EntityID == c.ID {
					if err := s.UserRepo.UpdateRole(r.Context(), u.ID, users.RoleCustomer, ""); err == nil && s.Revoked != nil {
						s.Revoked.Revoke(r.Context(), u.ID) // eski token roli ishlamasin
					}
				}
			}
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// POST /merchant/qurt/assign {courier_id, date, order_ids} — o'z buyurtmalarini (yoki butun reysni)
	// o'z kuryeriga biriktiradi.
	mux.HandleFunc("POST /merchant/qurt/assign", func(w http.ResponseWriter, r *http.Request) {
		sid, ok := s.merchantQurtStore(w, r)
		if !ok || !s.qurtOrdersReady(w) || s.CourierRepo == nil {
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
		if err != nil || c.RestaurantID != qurtorders.PoolFor(sid.EntityID) || !c.Approved {
			httpError(w, http.StatusBadRequest, errors.New("kuryer topilmadi"))
			return
		}
		if req.Date != "" {
			if _, err := time.Parse("2006-01-02", req.Date); err != nil {
				httpError(w, http.StatusBadRequest, errors.New("sana noto'g'ri (YYYY-MM-DD)"))
				return
			}
		}
		res, err := s.QurtOrderSvc.Assign(r.Context(), sid.EntityID, c.ID, req.Date, req.OrderIDs)
		if err != nil {
			qurtOrderError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, res)
	})

	// POST /merchant/qurt/orders/{id}/unassign — buyurtmani kuryerdan olish (yo'lga chiqmagan bo'lsa).
	mux.HandleFunc("POST /merchant/qurt/orders/{id}/unassign", func(w http.ResponseWriter, r *http.Request) {
		sid, ok := s.merchantQurtStore(w, r)
		if !ok || !s.qurtOrdersReady(w) {
			return
		}
		err := s.QurtOrderSvc.Unassign(r.Context(), r.PathValue("id"), func(o *qurtorders.Order) error {
			if o.StoreID != sid.EntityID {
				return qurtorders.ErrNotFound
			}
			return nil
		})
		if err != nil {
			qurtOrderError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// POST /merchant/qurt/couriers/{id}/settle {order_ids} — kuryer topshirgan naqd pulni qabul qilish.
	mux.HandleFunc("POST /merchant/qurt/couriers/{id}/settle", func(w http.ResponseWriter, r *http.Request) {
		sid, ok := s.merchantQurtStore(w, r)
		if !ok || !s.qurtOrdersReady(w) || s.CourierRepo == nil {
			return
		}
		c, ok := ownCourier(w, r, sid.EntityID, r.PathValue("id"))
		if !ok {
			return
		}
		var req struct {
			OrderIDs []string `json:"order_ids"`
		}
		_ = decodeOptionalJSON(r, &req)
		res, err := s.QurtOrderSvc.Settle(r.Context(), c.ID, sid.UserID, req.OrderIDs)
		if err != nil {
			qurtOrderError(w, err)
			return
		}
		slog.Info("qurt: kuryer puli qabul qilindi", "store", sid.EntityID, "courier", c.ID,
			"orders", res.Orders, "tiyin", res.TotalTiyin)
		writeJSON(w, http.StatusOK, res)
	})

	// ── Admin: FAQAT statistika (o'qish) ───────────────────────────────

	// GET /admin/qurt/stats — Qurt bo'yicha umumiy ko'rsatkichlar va do'konlar kesimi.
	mux.HandleFunc("GET /admin/qurt/stats", s.auth([]users.Role{users.RoleAdmin}, func(w http.ResponseWriter, r *http.Request) {
		if !s.qurtOrdersReady(w) {
			return
		}
		list, err := s.QurtOrderSvc.Repo.List(r.Context(), qurtorders.Filter{Limit: 20000})
		if err != nil {
			qurtOrderError(w, err)
			return
		}
		type storeStat struct {
			StoreID    string `json:"store_id"`
			StoreName  string `json:"store_name"`
			Orders     int    `json:"orders"`
			Delivered  int    `json:"delivered"`
			SoldTiyin  int64  `json:"sold_tiyin"`
			HeldTiyin  int64  `json:"cash_held_tiyin"`
			Couriers   int    `json:"couriers"`
			ActiveOnes int    `json:"active"`
		}
		byStatus := map[string]int{}
		stores := map[string]*storeStat{}
		var soldTiyin, heldTiyin, settledTiyin int64
		for _, o := range list {
			byStatus[string(o.Status)]++
			st := stores[o.StoreID]
			if st == nil {
				st = &storeStat{StoreID: o.StoreID, StoreName: o.StoreName}
				stores[o.StoreID] = st
			}
			st.Orders++
			switch {
			case o.Status == qurtorders.StatusDelivered:
				st.Delivered++
				st.SoldTiyin += o.TotalTiyin
				soldTiyin += o.TotalTiyin
			case !qurtorders.IsTerminal(o.Status):
				st.ActiveOnes++
			}
			switch o.CashStatus {
			case qurtorders.CashHeld:
				st.HeldTiyin += o.CashTiyin
				heldTiyin += o.CashTiyin
			case qurtorders.CashSettled:
				settledTiyin += o.CashTiyin
			}
		}
		couriersTotal := 0
		if s.CourierRepo != nil {
			if all, err := s.CourierRepo.ListAll(r.Context()); err == nil {
				for _, c := range all {
					if c != nil && c.Approved && qurtorders.IsQurtPool(c.RestaurantID) {
						couriersTotal++
						if st := stores[strings.TrimPrefix(c.RestaurantID, qurtorders.QurtPoolPrefix)]; st != nil {
							st.Couriers++
						}
					}
				}
			}
		}
		rows := make([]*storeStat, 0, len(stores))
		for _, st := range stores {
			rows = append(rows, st)
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].SoldTiyin > rows[j].SoldTiyin })
		writeJSON(w, http.StatusOK, map[string]any{
			"orders_total": len(list), "by_status": byStatus, "sold_tiyin": soldTiyin,
			"cash_held_tiyin": heldTiyin, "cash_settled_tiyin": settledTiyin,
			"couriers": couriersTotal, "stores": rows,
		})
	}))
}

// maxQurtCouriersPerStore — bitta do'kon qo'sha oladigan kuryerlar chegarasi (suiiste'moldan himoya).
const maxQurtCouriersPerStore = 30

type qurtCourierRow struct {
	*couriers.Courier
	phone string
}

// storeCouriers — do'konning tasdiqlangan kuryerlari (telefon bilan), nom bo'yicha tartiblangan.
func (s *Server) storeCouriers(ctx context.Context, storeID string) ([]qurtCourierRow, error) {
	all, err := s.CourierRepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	phones := map[string]string{}
	if accs, err := s.UserRepo.ListByRole(ctx, users.RoleCourier); err == nil {
		for _, u := range accs {
			if u != nil {
				phones[u.EntityID] = u.Phone
			}
		}
	}
	pool := qurtorders.PoolFor(storeID)
	var out []qurtCourierRow
	for _, c := range all {
		if c != nil && c.Approved && c.RestaurantID == pool {
			out = append(out, qurtCourierRow{Courier: c, phone: phones[c.ID]})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

var errQurtCourierConflict = errors.New("telefon raqamida boshqa rolli akkaunt bor")

// addQurtCourier — Qurt kuryeri akkauntini yaratadi yoki qayta yoqadi. Begona akkauntga
// (boshqa rol/restoran xodimi) TEGILMAYDI: faqat yangi raqam, mijoz akkaunti yoki shu
// kuryerning o'zi ulanadi. Tasdiq (`approved`) eng oxirida yoqiladi (fail closed).
func (s *Server) addQurtCourier(ctx context.Context, storeID, phone, name string) (*couriers.Courier, error) {
	prepare := func(userID string) (*couriers.Courier, error) {
		id := qurtCourierID(userID)
		c, err := s.CourierRepo.GetByID(ctx, id)
		switch {
		case err == nil:
			if c.RestaurantID != qurtorders.PoolFor(storeID) {
				return nil, errQurtCourierConflict // boshqa do'kon kuryeri
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
			ID: id, Name: name, RestaurantID: qurtorders.PoolFor(storeID),
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
