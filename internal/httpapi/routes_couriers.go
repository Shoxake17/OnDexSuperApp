package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"chustapp/internal/couriers"
	"chustapp/internal/delivery"
	"chustapp/internal/users"
)

func (s *Server) registerCourierRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /couriers/register", s.auth([]users.Role{users.RoleCustomer},
		func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Name        string               `json:"name"`
				VehicleType couriers.VehicleType `json:"vehicle_type"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			if req.Name == "" {
				httpError(w, http.StatusBadRequest, errors.New("name majburiy"))
				return
			}
			// VehicleType dispatch matching engine uchun MUHIM — Google
			// Distance Matrix'ga qaysi rejim (piyoda/velosiped/mashina) bilan
			// murojaat qilinishini belgilaydi. Berilmasa yoki noto'g'ri
			// bo'lsa xavfsiz standart: moped ("driving").
			if req.VehicleType == "" || !req.VehicleType.Valid() {
				req.VehicleType = couriers.VehicleMoped
			}
			claims := claimsFrom(r)
			u, err := s.UserRepo.GetByID(r.Context(), claims.Subject)
			if err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			c := couriers.Courier{ID: NewID(), Name: req.Name, VehicleType: req.VehicleType, Rating: 5.0}
			if err := s.CourierRepo.Create(r.Context(), &c); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			if err := s.UserRepo.UpdateRole(r.Context(), u.ID, users.RoleCourier, c.ID); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			u.Role = users.RoleCourier
			u.EntityID = c.ID
			newToken, err := s.Tokens.Issue(u)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{
				"courier": c,
				"token":   newToken,
				"message": "Ariza qabul qilindi. Admin tasdiqlagach ishlay boshlaysiz.",
			})
		}))

	// GET /couriers/{id} — kuryer ilovasi o'zining holatini (approved,
	// available, joylashuv) ko'rishi uchun; admin ham istalgan kuryerni
	// ko'ra oladi.
	mux.HandleFunc("GET /couriers/{id}", s.auth([]users.Role{users.RoleCourier, users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			claims := claimsFrom(r)
			courierID := r.PathValue("id")
			if claims.Role == users.RoleCourier && claims.EntityID != courierID {
				httpError(w, http.StatusForbidden, errors.New("boshqa kuryer ma'lumotini ko'rib bo'lmaydi"))
				return
			}
			c, err := s.CourierRepo.GetByID(r.Context(), courierID)
			if err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			writeJSON(w, http.StatusOK, c)
		}))

	// POST /couriers/{id}/location — kuryer ilovasi joriy koordinatani
	// davriy yuboradi (dispatch FindNearby shu qiymatga asoslanadi).
	mux.HandleFunc("POST /couriers/{id}/location", s.auth([]users.Role{users.RoleCourier},
		func(w http.ResponseWriter, r *http.Request) {
			courierID := r.PathValue("id")
			if claimsFrom(r).EntityID != courierID {
				httpError(w, http.StatusForbidden, errors.New("boshqa kuryer joylashuvini o'zgartirib bo'lmaydi"))
				return
			}
			var req struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			// Koordinata tekshiruvi. Avval FAQAT (0,0) rad etilardi —
			// ya'ni NaN, cheksizlik, 91° kenglik yoki "Toshkentdaman"
			// degan soxta nuqta bemalol o'tib ketardi. NaN ayniqsa
			// zararli: u haversine hisobiga tarqalib, dispatch'dagi
			// masofa saralashni butunlay buzadi.
			if !courierLocLimiter.Allow(courierID) {
				httpError(w, http.StatusTooManyRequests, errors.New("joylashuv juda tez-tez yuborilmoqda"))
				return
			}
			if !delivery.ValidCoords(req.Lat, req.Lng) {
				httpError(w, http.StatusBadRequest, errors.New("lat/lng noto'g'ri"))
				return
			}
			if !delivery.InOperationalRange(req.Lat, req.Lng) {
				httpError(w, http.StatusBadRequest, errors.New("joylashuv xizmat mintaqasidan juda uzoq"))
				return
			}
			// "Teleport" tekshiruvi — ketma-ket ikki nuqta orasidagi
			// jismonan imkonsiz tezlik soxtalashtirish belgisi.
			if !s.speedGate.Accept(courierID, req.Lat, req.Lng, time.Now()) {
				slog.Warn("kuryer joylashuvi rad etildi: imkonsiz tezlik",
					"courier_id", courierID, "lat", req.Lat, "lng", req.Lng)
				httpError(w, http.StatusBadRequest, errors.New("joylashuv o'zgarishi juda tez — GPS xatosi"))
				return
			}
			if err := s.CourierRepo.UpdateLocation(r.Context(), courierID, req.Lat, req.Lng); err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			// Kuryer HOZIR faol yetkazib berayotgan buyurtmasi bo'lsa, yangi
			// joylashuv shu buyurtmani kuzatib turgan mijozga DARHOL
			// (refresh'siz) yuboriladi — tracking_screen.dart xaritadagi
			// belgini shundan yangilaydi. Faol buyurtma yo'q bo'lsa (kuryer
			// hozircha bo'sh) — jimgina o'tkazib yuboriladi, xato emas.
			if order, err := s.OrderRepo.GetActiveByCourier(r.Context(), courierID); err == nil {
				s.Hub.Send(orderTopic(order.ID), map[string]any{
					"type": "courier_location", "order_id": order.ID,
					"lat": req.Lat, "lng": req.Lng,
				})
			}
			writeJSON(w, http.StatusOK, map[string]bool{"updated": true})
		}))

	// POST /couriers/{id}/respond — faqat o'sha kuryerning o'zi
	mux.HandleFunc("POST /couriers/{id}/respond", s.auth([]users.Role{users.RoleCourier},
		func(w http.ResponseWriter, r *http.Request) {
			courierID := r.PathValue("id")
			if claimsFrom(r).EntityID != courierID {
				httpError(w, http.StatusForbidden, errors.New("boshqa kuryer nomidan javob berib bo'lmaydi"))
				return
			}
			var req struct {
				OrderID  string `json:"order_id"`
				Accepted bool   `json:"accepted"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			ok := s.Dispatcher.HandleResponse(req.OrderID, couriers.Response{CourierID: courierID, Accepted: req.Accepted})
			if !ok {
				httpError(w, http.StatusConflict, errors.New("taklif eskirgan yoki sizga tegishli emas"))
				return
			}
			writeJSON(w, http.StatusOK, map[string]bool{"received": true})
		}))

	// POST /couriers/{id}/available — kuryerning o'zi yoki admin
	mux.HandleFunc("POST /couriers/{id}/available", s.auth([]users.Role{users.RoleCourier, users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			claims := claimsFrom(r)
			courierID := r.PathValue("id")
			if claims.Role == users.RoleCourier && claims.EntityID != courierID {
				httpError(w, http.StatusForbidden, errors.New("boshqa kuryer holatini o'zgartirib bo'lmaydi"))
				return
			}
			var req struct {
				Available bool `json:"available"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			c, err := s.CourierRepo.GetByID(r.Context(), courierID)
			if err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			if req.Available && !c.Approved {
				httpError(w, http.StatusForbidden,
					errors.New("kuryerlik arizangiz hali admin tomonidan tasdiqlanmagan"))
				return
			}
			if err := s.CourierRepo.SetAvailable(r.Context(), courierID, req.Available); err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]bool{"available": req.Available})
		}))
}
