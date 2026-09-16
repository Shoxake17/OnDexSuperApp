package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"chustapp/internal/couriers"
	"chustapp/internal/delivery"
	"chustapp/internal/orders"
	"chustapp/internal/users"
)

func (s *Server) registerCourierRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /couriers/register", s.auth([]users.Role{users.RoleCustomer},
		func(w http.ResponseWriter, r *http.Request) {
			// ┌─ OnDex KURYERLARI HOZIRCHA YOPIQ ──────────────────────────┐
			// Yetkazib beruvchi akkauntini restoran beradi ("Xodimlar"
			// bo'limi). Ochiq qolsa, istalgan mijoz o'zini kuryer qilib
			// ro'yxatdan o'tkazar, dispatch esa uni hech qachon ko'rmasdi —
			// "tasdiq kutilmoqda" ekranida abadiy qolardi.
			// └────────────────────────────────────────────────────────────┘
			// 410 (403 emas): rol TO'G'RI, xizmatning o'zi o'chiq. 403 rol
			// rad etilishi degani — avtorizatsiya matritsasi (va monitoring)
			// uni shunday o'qiydi.
			if !s.PlatformCouriers {
				httpError(w, http.StatusGone,
					errors.New("OnDex kuryerlari hozircha qabul qilinmaydi — kuryer akkauntini ishlaydigan restoraningiz beradi"))
				return
			}
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
			// Superadmin paneliga jonli xabar: ariza TASDIQ KUTMOQDA.
			//
			// Bu yerda kutish narxi yuqori — kuryer tasdiqlanmaguncha
			// umuman ishlay olmaydi va u ekranga qarab o'tiradi. Avval
			// panel buni faqat 10 soniyalik so'rov siklida ko'rardi.
			s.Hub.Send(adminTopic(), map[string]any{
				"type":       "courier_registered",
				"courier_id": c.ID,
			})
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
			if order, err := s.OrderRepo.GetActiveByCourier(r.Context(), courierID); err == nil && courierLocationShared(order) {
				event := map[string]any{
					"type": "courier_location", "order_id": order.ID,
					"lat": req.Lat, "lng": req.Lng,
				}
				s.Hub.Send(orderTopic(order.ID), event)
				// Mijozning SHAXSIY kanali ham. Mijoz ilovasi butun ilova
				// uchun bitta soket ochadi (`customerLive`) va u
				// `?order_id=` bilan ulanmaydi — faqat yuqoridagi kanalga
				// yuborilganda joylashuv mijozga UMUMAN yetib bormasdi.
				// Bu kanalga faqat mijozning o'z tokeni bilan obuna bo'linadi.
				s.Hub.Send(userTopic(order.CustomerID), event)
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
					errors.New("ilovaga kirishingiz hali ochilmagan yoki yopilgan — restoraningizga murojaat qiling"))
				return
			}
			// ┌─ YETKAZMA O'RTASIDA OFLAYN — TAQIQ (kuryer auditi, 3-band) ─┐
			// Ilova oflaynda buyurtma panelini yashiradi va joylashuv
			// yuborishni to'xtatadi: mijoz xaritasida kuryer qotadi,
			// "yetkazdim" tugmasi ko'rinmaydi. Yetkazma tugagach esa server
			// kuryerni baribir "bo'sh" qiladi (`routes_orders.go`) — ya'ni
			// kuryerning "oflayn" tanlovi jimgina bekor bo'lib, u sezmagan
			// holda taklif ola boshlardi. Admin uchun cheklov yo'q.
			// └─────────────────────────────────────────────────────────────┘
			if !req.Available && claims.Role == users.RoleCourier {
				if _, err := s.OrderRepo.GetActiveByCourier(r.Context(), courierID); err == nil {
					httpError(w, http.StatusConflict,
						errors.New("yakunlanmagan buyurtmangiz bor — avval uni yetkazing, keyin oflayn bo'ling"))
					return
				} else if !errors.Is(err, orders.ErrNotFound) {
					httpError(w, http.StatusInternalServerError, err)
					return
				}
			}
			if err := s.CourierRepo.SetAvailable(r.Context(), courierID, req.Available); err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			// Superadmin panelidagi "Online" ustuni shu eventdan
			// yangilanadi. Hodisa SIYRAK (kuryer smenaga chiqadi/
			// tugatadi), ya'ni kanalni to'ldirmaydi — joylashuv
			// yangilanishlari esa bu yerga ATAYLAB yuborilmaydi
			// (ular sekundiga bir necha marta keladi).
			s.Hub.Send(adminTopic(), map[string]any{
				"type":       "courier_status",
				"courier_id": courierID,
				"available":  req.Available,
			})
			writeJSON(w, http.StatusOK, map[string]bool{"available": req.Available})
		}))

	// GET /couriers/{id}/active-order — kuryerning HOZIRGI (yakunlanmagan)
	// buyurtmasi ID'si yoki `null`.
	//
	// ┌─ NEGA KERAK (kuryer ilovasi auditi, 2-band) ──────────────────────┐
	// Joriy buyurtma avval FAQAT ilova xotirasida turardi. Android fondagi
	// ilovani o'ldirsa (kuryer navigatorga o'tganda odatiy hol), qayta
	// ochilganda "olindi"/"yetkazdim" tugmalari yo'qolardi va buyurtma
	// osilib qolardi.
	//
	// Faqat ID qaytadi — tafsilotni ilova `GET /orders/{id}` dan oladi,
	// ya'ni `picked_up` gacha yashirish qoidasi bitta joyda qoladi.
	// └───────────────────────────────────────────────────────────────────┘
	mux.HandleFunc("GET /couriers/{id}/active-order", s.auth([]users.Role{users.RoleCourier},
		func(w http.ResponseWriter, r *http.Request) {
			courierID := r.PathValue("id")
			if claimsFrom(r).EntityID != courierID {
				httpError(w, http.StatusForbidden, errors.New("boshqa kuryer buyurtmasini ko'rib bo'lmaydi"))
				return
			}
			o, err := s.OrderRepo.GetActiveByCourier(r.Context(), courierID)
			if errors.Is(err, orders.ErrNotFound) {
				writeJSON(w, http.StatusOK, map[string]any{"order_id": nil})
				return
			}
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"order_id": o.ID})
		}))

	// GET /couriers/{id}/offer — kuryerga HOZIR ochiq taklif yoki `null`.
	//
	// ┌─ NEGA KERAK (2026-09-15) ─────────────────────────────────────────┐
	// Taklif avval faqat WebSocket xabari sifatida ilova xotirasida
	// turardi. Kuryer taklifni ko'rib ilovadan chiqsa va 20 soniya ichida
	// qaytsa ham taklif yo'qolardi — server esa uni ochiq deb kutardi.
	// Ilova endi ochilganda, oldinga chiqqanda, WS qayta ulanganda va push
	// bosilganda shu yerdan tiklaydi; qolgan soniyalar ham serverdan.
	// └───────────────────────────────────────────────────────────────────┘
	mux.HandleFunc("GET /couriers/{id}/offer", s.auth([]users.Role{users.RoleCourier},
		func(w http.ResponseWriter, r *http.Request) {
			courierID := r.PathValue("id")
			if claimsFrom(r).EntityID != courierID {
				httpError(w, http.StatusForbidden, errors.New("boshqa kuryer taklifini ko'rib bo'lmaydi"))
				return
			}
			if s.Dispatcher == nil {
				writeJSON(w, http.StatusOK, map[string]any{"offer": nil})
				return
			}
			info, ok := s.Dispatcher.PendingOffer(courierID)
			if !ok {
				writeJSON(w, http.StatusOK, map[string]any{"offer": nil})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"offer": info.Payload(s.Dispatcher.OfferTTL())})
		}))
}
