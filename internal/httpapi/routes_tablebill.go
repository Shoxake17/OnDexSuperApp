package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"chustapp/internal/modules/tablebill"
	"chustapp/internal/payments"
	"chustapp/internal/tables"
	"chustapp/internal/users"
)

// ┌─ STOLDA HISOBNI BO'LISH (RS-5) ───────────────────────────────────────┐
// Mijoz (stol QR tokeni — "shu stoldaman" dalili):
//   GET  /tables/bill?token=...      ochiq hisob (yo'q bo'lsa ochiladi)
//   GET  /bills/{id}                 hisob holati (ID tasodifiy, faqat token orqali bilinadi)
//   POST /bills/{id}/shares          {kind: equal|items, parts, items, tip_percent, channel}
// Ofitsiant / restoran:
//   GET  /waiter/bills, GET /restaurants/{id}/bills   ochiq hisoblar
//   POST /bills/{id}/close           {method: cash|terminal} — qolganini qabul qildim
// └──────────────────────────────────────────────────────────────────────┘

func billStatus(err error) int {
	switch {
	case errors.Is(err, tablebill.ErrNotFound), errors.Is(err, tables.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, tablebill.ErrInvalid):
		return http.StatusBadRequest
	case errors.Is(err, tablebill.ErrConflict), errors.Is(err, tablebill.ErrNothingDue),
		errors.Is(err, tablebill.ErrPendingPayments), errors.Is(err, payments.ErrAlreadyPaid):
		return http.StatusConflict
	case errors.Is(err, tablebill.ErrPaymentDisabled), errors.Is(err, payments.ErrBillsDisabled),
		errors.Is(err, payments.ErrNotConfigured), errors.Is(err, tables.ErrInactive):
		return http.StatusUnprocessableEntity
	}
	return http.StatusInternalServerError
}

func (s *Server) registerTableBillRoutes(mux *http.ServeMux) {
	customer := []users.Role{users.RoleCustomer}
	staff := []users.Role{users.RoleRestaurant, users.RoleWaiter, users.RoleAdmin}
	ready := func(w http.ResponseWriter) bool {
		if s.TableBillSvc == nil || s.TableSvc == nil {
			httpError(w, http.StatusServiceUnavailable, errors.New("stol hisobi sozlanmagan"))
			return false
		}
		return true
	}
	fail := func(w http.ResponseWriter, err error) { httpError(w, billStatus(err), err) }

	mux.HandleFunc("GET /tables/bill", s.auth(customer, func(w http.ResponseWriter, r *http.Request) {
		if !ready(w) {
			return
		}
		t, err := s.TableSvc.Resolve(r.Context(), r.URL.Query().Get("token"))
		if err != nil {
			fail(w, err)
			return
		}
		v, err := s.TableBillSvc.Current(r.Context(), t.RestaurantID, t.ID, t.DisplayLabel())
		if err != nil {
			fail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
	}))

	// GET /orders/{id}/bill — mehmon o'z stol buyurtmasidan hisobni ochadi
	// (QR token ilovada saqlanmagan bo'lsa ham). Faqat buyurtma egasi.
	mux.HandleFunc("GET /orders/{id}/bill", s.auth(customer, func(w http.ResponseWriter, r *http.Request) {
		o, err := s.OrderRepo.GetByID(r.Context(), r.PathValue("id"))
		if err != nil || o.CustomerID != claimsFrom(r).Subject || !o.IsDineIn() {
			fail(w, tablebill.ErrNotFound)
			return
		}
		if !ready(w) {
			return
		}
		v, err := s.TableBillSvc.Current(r.Context(), o.RestaurantID, o.TableID, o.TableLabel)
		if err != nil {
			fail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
	}))

	mux.HandleFunc("GET /bills/{id}", s.auth(customer, func(w http.ResponseWriter, r *http.Request) {
		if !ready(w) {
			return
		}
		v, err := s.TableBillSvc.ViewOf(r.Context(), r.PathValue("id"))
		if err != nil {
			fail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
	}))

	mux.HandleFunc("POST /bills/{id}/shares", s.auth(customer, func(w http.ResponseWriter, r *http.Request) {
		if !ready(w) {
			return
		}
		var req struct {
			Kind       string              `json:"kind"`
			Parts      int                 `json:"parts"`
			Items      []tablebill.ItemRef `json:"items"`
			TipPercent int                 `json:"tip_percent"`
			Channel    string              `json:"channel"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		v, err := s.TableBillSvc.ViewOf(r.Context(), r.PathValue("id"))
		if err != nil {
			fail(w, err)
			return
		}
		res, err := s.TableBillSvc.CreateShare(r.Context(), v.Bill.RestaurantID, tablebill.ShareInput{
			BillID: v.Bill.ID, PayerID: claimsFrom(r).Subject, Kind: req.Kind, Parts: req.Parts,
			Items: req.Items, TipPercent: req.TipPercent, Channel: req.Channel,
		})
		if err != nil {
			fail(w, err)
			return
		}
		pay := map[string]any{"payment_id": res.Pay.PaymentID, "pay_url": res.Pay.PayURL, "expires_at": res.Pay.ExpiresAt}
		if s.Payments != nil {
			pay["return_url"] = s.Payments.ReturnURL()
		}
		writeJSON(w, http.StatusCreated, map[string]any{"share": res.Share, "pay": pay})
	}))

	list := func(w http.ResponseWriter, r *http.Request, rid string) {
		bills, err := s.TableBillSvc.OpenBills(r.Context(), rid)
		if err != nil {
			fail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, bills)
	}
	mux.HandleFunc("GET /waiter/bills", s.auth([]users.Role{users.RoleWaiter}, func(w http.ResponseWriter, r *http.Request) {
		if !ready(w) {
			return
		}
		list(w, r, claimsFrom(r).EntityID)
	}))
	mux.HandleFunc("GET /restaurants/{id}/bills", s.auth([]users.Role{users.RoleRestaurant, users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			// Egalik AVVAL: begona restoran hech qachon 404 dan boshqa javob olmasin.
			rid, ok := entityIDFor(claimsFrom(r), r.PathValue("id"))
			if !ok {
				fail(w, tablebill.ErrNotFound)
				return
			}
			if !ready(w) {
				return
			}
			list(w, r, rid)
		}))

	// GET /restaurants/{id}/tips?from=RFC3339&to=RFC3339 — choy pullari ofitsiant bo'yicha.
	mux.HandleFunc("GET /restaurants/{id}/tips", s.auth([]users.Role{users.RoleRestaurant, users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			rid, ok := entityIDFor(claimsFrom(r), r.PathValue("id"))
			if !ok {
				fail(w, tablebill.ErrNotFound)
				return
			}
			if !ready(w) {
				return
			}
			from, err1 := time.Parse(time.RFC3339, r.URL.Query().Get("from"))
			to, err2 := time.Parse(time.RFC3339, r.URL.Query().Get("to"))
			if err1 != nil || err2 != nil {
				fail(w, tablebill.ErrInvalid)
				return
			}
			tips, err := s.TableBillSvc.Tips(r.Context(), rid, from, to)
			if err != nil {
				fail(w, err)
				return
			}
			writeJSON(w, http.StatusOK, tips)
		}))

	mux.HandleFunc("POST /bills/{id}/close", s.auth(staff, func(w http.ResponseWriter, r *http.Request) {
		if !ready(w) {
			return
		}
		var req struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&req); err != nil {
			fail(w, tablebill.ErrInvalid)
			return
		}
		c := claimsFrom(r)
		rid := c.EntityID
		if c.Role == users.RoleAdmin {
			v, err := s.TableBillSvc.ViewOf(r.Context(), r.PathValue("id"))
			if err != nil {
				fail(w, err)
				return
			}
			rid = v.Bill.RestaurantID
		}
		v, err := s.TableBillSvc.CloseByStaff(r.Context(), rid, r.PathValue("id"), req.Method, c.Subject)
		if err != nil {
			fail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
	}))
}
