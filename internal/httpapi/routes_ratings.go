package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"chustapp/internal/orders"
	"chustapp/internal/ratings"
	"chustapp/internal/users"
)

// registerRatingRoutes — mijoz buyurtmadan keyin restoranni baholaydi
// (`internal/ratings`, marketplace ilovalaridagi "Qalay bo'ldi?" bloki).
//
// `s.RatingsRepo == nil` bo'lsa (amalda bo'lmasligi kerak) marshrutlar
// RO'YXATGA OLINMAYDI — xuddi banner/favorites naqshi kabi.
func (s *Server) registerRatingRoutes(mux *http.ServeMux) {
	if s.RatingsRepo == nil {
		return
	}

	// POST /orders/{id}/rating — FAQAT mijoz, FAQAT o'z buyurtmasi,
	// FAQAT yetkazib berilgan/stolga olib borilgandan KEYIN.
	//
	// ┌─ NEGA TERMINAL HOLAT TALAB QILINADI ────────────────────────────┐
	// Hali yetkazilmagan buyurtmani baholash ma'nosiz (tajriba hali
	// tugamagan) va yomon niyatli mijoz restoranni "oldindan" past
	// baho bilan bosim ostiga olishi mumkin edi.
	// └────────────────────────────────────────────────────────────────┘
	mux.HandleFunc("POST /orders/{id}/rating", s.auth([]users.Role{users.RoleCustomer},
		func(w http.ResponseWriter, r *http.Request) {
			claims := claimsFrom(r)
			o, err := s.OrderSvc.Get(r.Context(), r.PathValue("id"))
			if err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			// 404 — mavjudlik oshkor bo'lmasin (`GET /orders/{id}` dagi
			// izohdagi bir xil falsafa): begona buyurtma ID'si ham,
			// yo'q ID ham bir xil javob beradi.
			if o.CustomerID != claims.Subject {
				httpError(w, http.StatusNotFound, orders.ErrNotFound)
				return
			}
			if o.Status != orders.StatusDelivered && o.Status != orders.StatusServed {
				httpError(w, http.StatusConflict,
					errors.New("buyurtma hali yetkazilmagan/yakunlanmagan — baho qo'yib bo'lmaydi"))
				return
			}
			var req struct {
				Stars int `json:"stars"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			if req.Stars < 1 || req.Stars > 5 {
				httpError(w, http.StatusBadRequest, errors.New("stars 1 dan 5 gacha bo'lishi kerak"))
				return
			}
			rt := &ratings.Rating{
				ID:           NewID(),
				OrderID:      o.ID,
				RestaurantID: o.RestaurantID,
				CustomerID:   claims.Subject,
				Stars:        req.Stars,
				CreatedAt:    time.Now(),
			}
			if err := s.RatingsRepo.Create(r.Context(), rt); err != nil {
				if errors.Is(err, ratings.ErrAlreadyRated) {
					httpError(w, http.StatusConflict, err)
					return
				}
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusCreated, rt)
		}))

	// GET /orders/{id}/rating — mijoz ilovasi "baholandimi" holatini shu
	// yerdan bilib, qayta so'ramaydi. Baho bo'lmasa 404.
	mux.HandleFunc("GET /orders/{id}/rating", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			claims := claimsFrom(r)
			o, err := s.OrderSvc.Get(r.Context(), r.PathValue("id"))
			if err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			if !canSeeOrder(claims, o) {
				httpError(w, http.StatusNotFound, orders.ErrNotFound)
				return
			}
			rt, err := s.RatingsRepo.GetByOrder(r.Context(), o.ID)
			if err != nil {
				if errors.Is(err, ratings.ErrNotFound) {
					httpError(w, http.StatusNotFound, err)
					return
				}
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, rt)
		}))
}
