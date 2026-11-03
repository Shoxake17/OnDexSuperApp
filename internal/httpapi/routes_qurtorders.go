package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"chustapp/internal/delivery"
	"chustapp/internal/qurtorders"
	"chustapp/internal/ratelimit"
	"chustapp/internal/users"
)

// Qurt buyurtmalari limitlari: mijoz bo'yicha (token) va POS serveri bo'yicha.
var (
	qurtOrderCreateLimiter   = ratelimit.New(1, 20)
	qurtOrderMerchantLimiter = ratelimit.New(20, 60)
)

// qurtOrderError — domen xatosini HTTP javobiga aylantiradi. Ichki xato matni
// mijozga CHIQMAYDI (500 da umumiy xabar).
func qurtOrderError(w http.ResponseWriter, err error) {
	var stock *qurtorders.StockError
	var tr *qurtorders.TransitionError
	var changed *qurtorders.TotalChangedError
	switch {
	case errors.As(err, &changed):
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": "narx o'zgardi — yangi summani tasdiqlang", "code": "total_changed",
			"total_tiyin": changed.ActualTiyin,
		})
	case errors.Is(err, qurtorders.ErrNotFound):
		httpError(w, http.StatusNotFound, errors.New("buyurtma topilmadi"))
	case errors.As(err, &stock):
		writeJSON(w, http.StatusConflict, map[string]any{"error": stock.Error(), "code": "out_of_stock"})
	case errors.As(err, &tr):
		httpError(w, http.StatusConflict, err)
	case errors.Is(err, qurtorders.ErrConflict):
		httpError(w, http.StatusConflict, errors.New("buyurtma boshqa joyda o'zgardi — yangilab qayta urining"))
	case errors.Is(err, qurtorders.ErrInvalid):
		httpError(w, http.StatusBadRequest, err)
	default:
		slog.Error("qurt buyurtma xatosi", "err", err)
		httpError(w, http.StatusInternalServerError, errors.New("server xatosi"))
	}
}

// storeOrderView — DO'KON ko'radigan buyurtma: tovar, miqdor, summa, reys sanasi.
// Mijozning manzili va telefoni do'konga BERILMAYDI (ularni faqat logistika
// ko'radi) — Uzum Market sotuvchisi ham xaridorni ko'rmaydi. Ism faqat birinchi so'z.
type storeOrderView struct {
	ID            string                   `json:"id"`
	Number        string                   `json:"number"`
	Status        qurtorders.Status        `json:"status"`
	DeliveryDate  string                   `json:"delivery_date"`
	Items         []storeOrderItem         `json:"items"`
	TotalTiyin    int64                    `json:"total_tiyin"`
	CustomerName  string                   `json:"customer_name"`
	RejectReason  string                   `json:"reject_reason,omitempty"`
	History       []storeOrderHistoryEntry `json:"history"`
	CreatedAt     time.Time                `json:"created_at"`
	PaymentMethod string                   `json:"payment_method"`
}

// storeOrderItem — do'konga ko'rinadigan pozitsiya. Barcode — do'konning O'Z mahsulot kaliti
// (POS ombor qoldig'ini kamaytirish uchun); OnDex ichki ID'si berilmaydi.
type storeOrderItem struct {
	Barcode     string `json:"barcode"`
	Name        string `json:"name"`
	Qty         int    `json:"qty"`
	PriceTiyin  int64  `json:"price_tiyin"`
	ImageURL    string `json:"image_url,omitempty"`
	WeightGrams int    `json:"weight_grams,omitempty"`
}

type storeOrderHistoryEntry struct {
	To qurtorders.Status `json:"to"`
	At time.Time         `json:"at"`
}

func firstWord(s string) string {
	f := strings.Fields(s)
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

func storeItems(items []qurtorders.Item) []storeOrderItem {
	out := make([]storeOrderItem, 0, len(items))
	for _, it := range items {
		out = append(out, storeOrderItem{
			Barcode: it.ExternalID, Name: it.Name, Qty: it.Qty, PriceTiyin: it.PriceTiyin,
			ImageURL: it.ImageURL, WeightGrams: it.WeightGrams,
		})
	}
	return out
}

func toStoreView(o *qurtorders.Order) storeOrderView {
	v := storeOrderView{
		ID: o.ID, Number: o.Number, Status: o.Status, DeliveryDate: o.DeliveryDate,
		Items: storeItems(o.Items), TotalTiyin: o.SubtotalTiyin, CustomerName: firstWord(o.CustomerName),
		RejectReason: o.RejectReason, CreatedAt: o.CreatedAt, PaymentMethod: o.PaymentMethod,
		History: []storeOrderHistoryEntry{},
	}
	for _, h := range o.History {
		v.History = append(v.History, storeOrderHistoryEntry{To: h.To, At: h.At})
	}
	return v
}

func parseStatuses(q string) ([]qurtorders.Status, bool) {
	var out []qurtorders.Status
	for _, p := range strings.Split(q, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		st := qurtorders.Status(p)
		if !qurtorders.ValidStatus(st) {
			return nil, false
		}
		out = append(out, st)
	}
	return out, true
}

func parseLimit(q string, def, max int) int {
	n, _ := strconv.Atoi(q)
	if n <= 0 || n > max {
		return def
	}
	return n
}

func (s *Server) qurtOrdersReady(w http.ResponseWriter) bool {
	if s.QurtOrderSvc == nil || s.QurtRepo == nil {
		httpError(w, http.StatusServiceUnavailable, errors.New("qurt buyurtmalari sozlanmagan"))
		return false
	}
	return true
}

func (s *Server) registerQurtOrderRoutes(mux *http.ServeMux) {
	// ── Mijoz ilovasi ──────────────────────────────────────────────────

	// GET /qurt/delivery-info — eng yaqin reys: mijoz buyurtma berishdan OLDIN
	// "qachon yetkaziladi" ni ko'radi.
	mux.HandleFunc("GET /qurt/delivery-info", func(w http.ResponseWriter, r *http.Request) {
		if !s.qurtOrdersReady(w) {
			return
		}
		sched := qurtorders.DefaultSchedule()
		if s.QurtOrderSvc.Schedule != nil {
			sched = s.QurtOrderSvc.Schedule()
		}
		date, cutoff := sched.NextDelivery(time.Now())
		days := make([]int, 0, len(sched.Weekdays))
		for _, d := range sched.Weekdays {
			days = append(days, int(d))
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, map[string]any{
			"delivery_date":  date,
			"order_by":       cutoff.Format(time.RFC3339),
			"weekdays":       days, // 0 = yakshanba ... 6 = shanba
			"payment_method": "cash",
		})
	})

	// POST /qurt/orders — savatdan buyurtma (do'konlar bo'yicha bo'linadi).
	// Mijoz FAQAT mahsulot va miqdorni yuboradi: narx, do'kon, manzil va telefon
	// serverdan olinadi.
	mux.HandleFunc("POST /qurt/orders", s.auth([]users.Role{users.RoleCustomer}, func(w http.ResponseWriter, r *http.Request) {
		if !s.qurtOrdersReady(w) {
			return
		}
		uid := claimsFrom(r).Subject
		if rateLimited(w, qurtOrderCreateLimiter, uid) {
			return
		}
		var req struct {
			Items []struct {
				ProductID string `json:"product_id"`
				Qty       int    `json:"qty"`
			} `json:"items"`
			Comment        string `json:"comment"`
			IdempotencyKey string `json:"idempotency_key"`
			PaymentMethod  string `json:"payment_method"`
			// ExpectedTotalTiyin — ekranda ko'rsatilgan jami; farq qilsa 409 total_changed.
			ExpectedTotalTiyin int64 `json:"expected_total_tiyin"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		if pm := strings.TrimSpace(req.PaymentMethod); pm != "" && pm != "cash" {
			httpError(w, http.StatusBadRequest, errors.New("hozircha faqat naqd to'lov mavjud"))
			return
		}
		u, err := s.UserRepo.GetByID(r.Context(), uid)
		if err != nil {
			httpError(w, http.StatusBadRequest, errors.New("foydalanuvchi topilmadi"))
			return
		}
		addr := u.Address
		if addr.Lat == 0 && addr.Lng == 0 || strings.TrimSpace(addr.Text) == "" {
			httpError(w, http.StatusBadRequest, errors.New("avval yetkazib berish manzilini tanlang"))
			return
		}
		if !delivery.Covered(addr.Lat, addr.Lng) {
			httpError(w, http.StatusBadRequest, errOutsideServiceArea)
			return
		}
		stores, err := s.qurtStores(r.Context())
		if err != nil {
			httpError(w, http.StatusInternalServerError, errors.New("server xatosi"))
			return
		}
		lines := make([]qurtorders.Line, 0, len(req.Items))
		for _, it := range req.Items {
			lines = append(lines, qurtorders.Line{ProductID: it.ProductID, Qty: it.Qty})
		}
		created, err := s.QurtOrderSvc.Create(r.Context(), qurtorders.CreateInput{
			CustomerID: uid, CustomerName: u.Name, CustomerPhone: u.Phone,
			Address: qurtorders.Address{
				Text: addr.Text, Entrance: addr.Entrance, Floor: addr.Floor, Apartment: addr.Apartment,
				Intercom: addr.Intercom, Comment: addr.Comment, Lat: addr.Lat, Lng: addr.Lng,
			},
			Comment: req.Comment, Lines: lines, IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
			ActiveStores: stores, ExpectedTotalTiyin: req.ExpectedTotalTiyin,
		})
		if err != nil {
			qurtOrderError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"orders": created})
	}))

	// GET /qurt/orders — mijozning o'z buyurtmalari.
	mux.HandleFunc("GET /qurt/orders", s.auth([]users.Role{users.RoleCustomer}, func(w http.ResponseWriter, r *http.Request) {
		if !s.qurtOrdersReady(w) {
			return
		}
		list, err := s.QurtOrderSvc.Repo.List(r.Context(), qurtorders.Filter{
			CustomerID: claimsFrom(r).Subject, Limit: parseLimit(r.URL.Query().Get("limit"), 50, 100),
		})
		if err != nil {
			qurtOrderError(w, err)
			return
		}
		if list == nil {
			list = []*qurtorders.Order{}
		}
		for i, o := range list {
			list[i] = forCustomer(o)
		}
		writeJSON(w, http.StatusOK, map[string]any{"orders": list})
	}))

	ownOrder := func(r *http.Request) func(*qurtorders.Order) error {
		uid := claimsFrom(r).Subject
		return func(o *qurtorders.Order) error {
			if o.CustomerID != uid {
				return qurtorders.ErrNotFound // boshqaning buyurtmasi mavjud emasdek
			}
			return nil
		}
	}

	// GET /qurt/orders/{id}
	mux.HandleFunc("GET /qurt/orders/{id}", s.auth([]users.Role{users.RoleCustomer}, func(w http.ResponseWriter, r *http.Request) {
		if !s.qurtOrdersReady(w) {
			return
		}
		o, err := s.QurtOrderSvc.Repo.Get(r.Context(), r.PathValue("id"))
		if err == nil {
			err = ownOrder(r)(o)
		}
		if err != nil {
			qurtOrderError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, forCustomer(o))
	}))

	// POST /qurt/orders/{id}/cancel — mijoz faqat do'kon ko'rmagan (`new`) buyurtmani bekor qiladi.
	mux.HandleFunc("POST /qurt/orders/{id}/cancel", s.auth([]users.Role{users.RoleCustomer}, func(w http.ResponseWriter, r *http.Request) {
		if !s.qurtOrdersReady(w) {
			return
		}
		var req struct {
			Reason string `json:"reason"`
		}
		_ = decodeOptionalJSON(r, &req)
		o, err := s.QurtOrderSvc.Transition(r.Context(), r.PathValue("id"), qurtorders.StatusCancelled,
			qurtorders.ActorCustomer, claimsFrom(r).Subject, req.Reason, ownOrder(r))
		if err != nil {
			qurtOrderError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, forCustomer(o))
	}))

	// ── POS serveri (do'kon) ───────────────────────────────────────────

	ownStoreOrder := func(storeID string) func(*qurtorders.Order) error {
		return func(o *qurtorders.Order) error {
			if o.StoreID != storeID {
				return qurtorders.ErrNotFound
			}
			return nil
		}
	}

	// GET /merchant/qurt/orders?status=new,confirmed&limit=
	mux.HandleFunc("GET /merchant/qurt/orders", func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.merchantQurtStore(w, r)
		if !ok || !s.qurtOrdersReady(w) {
			return
		}
		statuses, valid := parseStatuses(r.URL.Query().Get("status"))
		if !valid {
			httpError(w, http.StatusBadRequest, errors.New("noma'lum holat"))
			return
		}
		list, err := s.QurtOrderSvc.Repo.List(r.Context(), qurtorders.Filter{
			StoreID: id.EntityID, Statuses: statuses, Limit: parseLimit(r.URL.Query().Get("limit"), 100, 200),
		})
		if err != nil {
			qurtOrderError(w, err)
			return
		}
		views := make([]storeOrderView, 0, len(list))
		for _, o := range list {
			views = append(views, toStoreView(o))
		}
		writeJSON(w, http.StatusOK, map[string]any{"orders": views})
	})

	// GET /merchant/qurt/orders/{id}
	mux.HandleFunc("GET /merchant/qurt/orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.merchantQurtStore(w, r)
		if !ok || !s.qurtOrdersReady(w) {
			return
		}
		o, err := s.QurtOrderSvc.Repo.Get(r.Context(), r.PathValue("id"))
		if err == nil {
			err = ownStoreOrder(id.EntityID)(o)
		}
		if err != nil {
			qurtOrderError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toStoreView(o))
	})

	// POST /merchant/qurt/orders/{id}/transition {to, reason} — do'kon: confirmed | rejected | packed.
	mux.HandleFunc("POST /merchant/qurt/orders/{id}/transition", func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.merchantQurtStore(w, r)
		if !ok || !s.qurtOrdersReady(w) {
			return
		}
		var req struct {
			To     string `json:"to"`
			Reason string `json:"reason"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		o, err := s.QurtOrderSvc.Transition(r.Context(), r.PathValue("id"), qurtorders.Status(req.To),
			qurtorders.ActorStore, id.UserID, req.Reason, ownStoreOrder(id.EntityID))
		if err != nil {
			qurtOrderError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toStoreView(o))
	})

	// ── Admin (logistika) ──────────────────────────────────────────────

	admin := []users.Role{users.RoleAdmin}

	// GET /admin/qurt/orders?status=&date=&store_id=&limit= — to'liq ma'lumot (manzil, telefon).
	mux.HandleFunc("GET /admin/qurt/orders", s.auth(admin, func(w http.ResponseWriter, r *http.Request) {
		if !s.qurtOrdersReady(w) {
			return
		}
		q := r.URL.Query()
		statuses, valid := parseStatuses(q.Get("status"))
		if !valid {
			httpError(w, http.StatusBadRequest, errors.New("noma'lum holat"))
			return
		}
		list, err := s.QurtOrderSvc.Repo.List(r.Context(), qurtorders.Filter{
			Statuses: statuses, DeliveryDate: strings.TrimSpace(q.Get("date")),
			StoreID: strings.TrimSpace(q.Get("store_id")), CourierID: strings.TrimSpace(q.Get("courier_id")),
			CashStatus: qurtorders.CashStatus(strings.TrimSpace(q.Get("cash"))), Limit: parseLimit(q.Get("limit"), 200, 500),
		})
		if err != nil {
			qurtOrderError(w, err)
			return
		}
		if list == nil {
			list = []*qurtorders.Order{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"orders": list})
	}))

	// POST /admin/qurt/orders/{id}/transition {to, reason}
	mux.HandleFunc("POST /admin/qurt/orders/{id}/transition", s.auth(admin, func(w http.ResponseWriter, r *http.Request) {
		if !s.qurtOrdersReady(w) {
			return
		}
		var req struct {
			To     string `json:"to"`
			Reason string `json:"reason"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		o, err := s.QurtOrderSvc.Transition(r.Context(), r.PathValue("id"), qurtorders.Status(req.To),
			qurtorders.ActorAdmin, claimsFrom(r).Subject, req.Reason, nil)
		if err != nil {
			qurtOrderError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, o)
	}))

	// GET /admin/qurt/batches — reyslar (yetkazish sanalari) va ulardagi buyurtmalar
	// soni holat bo'yicha. Reys = yetkazish sanasi; alohida jadval yo'q.
	mux.HandleFunc("GET /admin/qurt/batches", s.auth(admin, func(w http.ResponseWriter, r *http.Request) {
		if !s.qurtOrdersReady(w) {
			return
		}
		active := []qurtorders.Status{
			qurtorders.StatusNew, qurtorders.StatusConfirmed, qurtorders.StatusPacked,
			qurtorders.StatusCollected, qurtorders.StatusOutForDelivery, qurtorders.StatusFailed,
		}
		list, err := s.QurtOrderSvc.Repo.List(r.Context(), qurtorders.Filter{Statuses: active, Limit: 1000})
		if err != nil {
			qurtOrderError(w, err)
			return
		}
		type batch struct {
			Date    string         `json:"date"`
			Total   int            `json:"total"`
			ByState map[string]int `json:"by_status"`
			Sum     int64          `json:"total_tiyin"`
		}
		byDate := map[string]*batch{}
		for _, o := range list {
			b := byDate[o.DeliveryDate]
			if b == nil {
				b = &batch{Date: o.DeliveryDate, ByState: map[string]int{}}
				byDate[o.DeliveryDate] = b
			}
			b.Total++
			b.ByState[string(o.Status)]++
			b.Sum += o.TotalTiyin
		}
		out := make([]*batch, 0, len(byDate))
		for _, b := range byDate {
			out = append(out, b)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
		writeJSON(w, http.StatusOK, map[string]any{"batches": out})
	}))

	// POST /admin/qurt/batches/{date}/transition {from, to} — reysdagi hamma buyurtmani
	// birdaniga o'tkazadi. Faqat logistika qadamlari.
	mux.HandleFunc("POST /admin/qurt/batches/{date}/transition", s.auth(admin, func(w http.ResponseWriter, r *http.Request) {
		if !s.qurtOrdersReady(w) {
			return
		}
		date := r.PathValue("date")
		if _, err := time.Parse("2006-01-02", date); err != nil {
			httpError(w, http.StatusBadRequest, errors.New("sana noto'g'ri (YYYY-MM-DD)"))
			return
		}
		var req struct {
			From string `json:"from"`
			To   string `json:"to"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		from, to := qurtorders.Status(req.From), qurtorders.Status(req.To)
		allowed := map[[2]qurtorders.Status]bool{
			{qurtorders.StatusPacked, qurtorders.StatusCollected}:         true,
			{qurtorders.StatusCollected, qurtorders.StatusOutForDelivery}: true,
			{qurtorders.StatusOutForDelivery, qurtorders.StatusDelivered}: true,
			{qurtorders.StatusFailed, qurtorders.StatusOutForDelivery}:    true,
		}
		if !allowed[[2]qurtorders.Status{from, to}] {
			httpError(w, http.StatusBadRequest, errors.New("reys uchun bu o'tish mumkin emas"))
			return
		}
		done, skipped, err := s.QurtOrderSvc.TransitionDate(r.Context(), date, from, to,
			qurtorders.ActorAdmin, claimsFrom(r).Subject)
		if err != nil {
			qurtOrderError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]int{"done": done, "skipped": skipped})
	}))
}

// forCustomer — mijozga kuryer va naqd pul hisobi maydonlari ko'rsatilmaydi (ichki ma'lumot).
func forCustomer(o *qurtorders.Order) *qurtorders.Order {
	cp := *o
	cp.CourierID, cp.CourierAssignedAt = "", nil
	cp.CashTiyin, cp.CashStatus, cp.CashSettledAt, cp.CashSettledBy = 0, "", nil, ""
	return &cp
}

// decodeOptionalJSON — tana bo'sh bo'lishi mumkin (masalan, sababsiz bekor qilish).
func decodeOptionalJSON(r *http.Request, dst any) error {
	if r.Body == nil || r.ContentLength == 0 {
		return nil
	}
	return json.NewDecoder(r.Body).Decode(dst)
}
