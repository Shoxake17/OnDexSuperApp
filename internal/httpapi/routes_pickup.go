package httpapi

import (
	"errors"
	"net/http"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/orders"
	"chustapp/internal/users"
)

// Olib ketish (Borish, Plane 473/474/477).
//
//	POST /orders  {"fulfillment":"pickup", "pickup_at":?, "payment_method":"card|wallet", ...}
//	GET  /orders/{id}/pickup-code   — faqat buyurtma EGASI (mijoz)
//
// Egasi qarori 2026-11-18: to'lov FAQAT oldindan; chegirma restoran
// sozlamasidan; manzil, kuryer va yetkazish haqi YO'Q.

var (
	errPickupDisabled  = errors.New("bu restoranda olib ketish yoqilmagan")
	errPickupPrepay    = errors.New("olib ketish faqat oldindan to'lov bilan (karta yoki OnDex Wallet)")
	errPickupTimeRange = errors.New("olib ketish vaqti hozirdan 12 soat ichida bo'lishi kerak")
)

const pickupMaxAhead = 12 * time.Hour

func (s *Server) createPickupOrder(w http.ResponseWriter, r *http.Request, reqItems []catalog.ItemRequest,
	idempotencyKey string, pm orders.PaymentMethod, pickupAt *time.Time, expectedTotal int64) {

	if !pm.RequiresPrepayment() {
		httpError(w, http.StatusBadRequest, errPickupPrepay)
		return
	}
	if pickupAt != nil {
		now := time.Now()
		if pickupAt.Before(now.Add(-5*time.Minute)) || pickupAt.After(now.Add(pickupMaxAhead)) {
			httpError(w, http.StatusBadRequest, errPickupTimeRange)
			return
		}
		t := pickupAt.UTC()
		pickupAt = &t
	}
	restaurantID, items, err := s.CatalogSvc.PriceOrder(r.Context(), reqItems)
	if err != nil {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	rest, err := s.CatalogRepo.GetRestaurant(r.Context(), restaurantID)
	if err != nil {
		httpError(w, http.StatusBadRequest, errors.New("restoran topilmadi"))
		return
	}
	if !rest.PickupEnabled() {
		httpError(w, http.StatusBadRequest, errPickupDisabled)
		return
	}
	if err := s.CatalogSvc.CheckPayment(r.Context(), restaurantID, pm); err != nil {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	o := orders.Order{
		CustomerID:            claimsFrom(r).Subject,
		RestaurantID:          restaurantID,
		Items:                 items,
		Type:                  orders.TypePickup,
		PickupAt:              pickupAt,
		PickupDiscountPercent: rest.PickupDiscount(),
		IdempotencyKey:        idempotencyKey,
	}
	setPaymentMethod(&o, pm)
	created, err := s.OrderSvc.CreateExpecting(r.Context(), &o, expectedTotal)
	if err != nil {
		var changed *orders.TotalChangedError
		if errors.As(err, &changed) {
			writeJSON(w, http.StatusConflict, map[string]any{
				"error":       "narx o'zgardi — yangi summani tasdiqlang",
				"code":        "total_changed",
				"total_tiyin": changed.ActualTiyin,
			})
			return
		}
		httpError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) registerPickupRoutes(mux *http.ServeMux) {
	// Kodni FAQAT buyurtma egasi ko'radi. Restoran uni mijozdan og'zaki
	// oladi — aks holda "kod bilan berish" ma'nosiz bo'lardi.
	mux.HandleFunc("GET /orders/{id}/pickup-code", s.auth([]users.Role{users.RoleCustomer},
		func(w http.ResponseWriter, r *http.Request) {
			o, err := s.OrderSvc.Get(r.Context(), r.PathValue("id"))
			if err != nil || o.CustomerID != claimsFrom(r).Subject || !o.IsPickup() {
				httpError(w, http.StatusNotFound, orders.ErrNotFound)
				return
			}
			if o.IsTerminal() || len(s.PickupCodeKey) == 0 {
				httpError(w, http.StatusConflict, errors.New("buyurtma yakunlangan"))
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusOK, map[string]any{"code": s.pickupCode(o.ID)})
		}))
}
