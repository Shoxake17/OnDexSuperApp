package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/couriers"
	"chustapp/internal/orders"
	"chustapp/internal/users"
)

func (s *Server) registerAdminRoutes(mux *http.ServeMux) {

	// ---------- Superadmin API ----------

	// POST /admin/restaurants — restoran + unga kirish akkaunti bir amalda.
	// Restoranlar o'zi ro'yxatdan o'tmaydi: akkauntni faqat superadmin yaratadi,
	// restoran o'z paneliga shu telefon raqami bilan (SMS kod) kiradi.
	mux.HandleFunc("POST /admin/restaurants", s.auth([]users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Name      string  `json:"name"`
				Address   string  `json:"address"`
				Lat       float64 `json:"lat"`
				Lng       float64 `json:"lng"`
				Phone     string  `json:"phone"`      // akkaunt telefoni (majburiy)
				StaffName string  `json:"staff_name"` // akkaunt egasi ismi
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			if req.Name == "" || req.Phone == "" {
				httpError(w, http.StatusBadRequest, errors.New("name va phone majburiy"))
				return
			}
			phone, err := users.NormalizePhone(req.Phone)
			if err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			if _, err := s.UserRepo.GetByPhone(r.Context(), phone); err == nil {
				httpError(w, http.StatusConflict, errors.New("bu telefon raqam allaqachon ro'yxatda"))
				return
			}
			rest := catalog.Restaurant{
				ID: NewID(), Name: req.Name, Address: req.Address,
				Lat: req.Lat, Lng: req.Lng, Open: true,
			}
			if err := s.CatalogRepo.SaveRestaurant(r.Context(), &rest); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			s.Cache.Del(r.Context(), restaurantsCacheKey)
			account := users.User{
				ID: NewID(), Phone: phone, Name: req.StaffName,
				Role: users.RoleRestaurant, EntityID: rest.ID, CreatedAt: time.Now(),
			}
			if err := s.UserRepo.Create(r.Context(), &account); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{"restaurant": rest, "account": account})
		}))

	mux.HandleFunc("DELETE /admin/restaurants/{id}", s.auth([]users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			id := r.PathValue("id")
			if _, err := s.CatalogRepo.GetRestaurant(r.Context(), id); err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			active, err := s.OrderRepo.HasActiveByRestaurant(r.Context(), id)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			if active {
				httpError(w, http.StatusConflict,
					errors.New("bu restoranning faol buyurtmalari bor — avval ular yakunlanishi kerak"))
				return
			}
			// Aksiyalar restorandan OLDIN o'chiriladi.
			//
			// Sabab: Postgres'da `promotions.restaurant_id` restoranga
			// FOREIGN KEY. Avval bu qadam yo'q edi — natijada aksiyasi
			// bor restoranni o'chirish FK buzilishiga tushib,
			// superadminga tushunarsiz 500 xatosi qaytarardi va restoran
			// UMUMAN o'chirilmasdi. Aksiya o'chirilmasa, buyurtma
			// yaratish yo'lida ham mavjud bo'lmagan restoranga ishora
			// qiluvchi "yetim" aksiya qolib ketardi.
			if n, err := s.PromotionsRepo.DeleteByRestaurant(r.Context(), id); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			} else if n > 0 {
				slog.Info("restoran bilan birga aksiyalar o'chirildi", "restaurant", id, "count", n)
			}
			if err := s.CatalogRepo.DeleteRestaurant(r.Context(), id); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			s.Cache.Del(r.Context(), restaurantsCacheKey, menuCacheKey(id))
			deletedUsers, err := s.UserRepo.DeleteByRoleEntity(r.Context(), users.RoleRestaurant, id)
			if err != nil {
				slog.Error("restoran akkauntini o'chirishda xato", "restaurant", id, "err", err)
			}
			// Akkauntni bazadan o'chirish YETARLI EMAS: `auth()` faqat
			// imzoni tekshiradi, shuning uchun xodim qo'lidagi token
			// akkauntsiz ham 30 kun ishlayverardi. Endi u ham darhol
			// bekor qilinadi.
			for _, uid := range deletedUsers {
				s.Revoked.Revoke(r.Context(), uid)
			}
			slog.Info("restoran o'chirildi", "restaurant", id, "by", claimsFrom(r).Subject)
			writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
		}))

	// POST /admin/restaurants/{id} — restoran ma'lumotlarini tahrirlash
	// (nomi, manzili, joylashuvi, logo, cover). Superadmin panelidagi
	// "Tahrirlash" oynasi shu yerga to'liq holatni yuboradi.
	mux.HandleFunc("POST /admin/restaurants/{id}", s.auth([]users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			rest, err := s.CatalogRepo.GetRestaurant(r.Context(), r.PathValue("id"))
			if err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			var req struct {
				Name     string  `json:"name"`
				Address  string  `json:"address"`
				Lat      float64 `json:"lat"`
				Lng      float64 `json:"lng"`
				LogoURL  string  `json:"logo_url"`
				CoverURL string  `json:"cover_url"`
				Tags     string  `json:"tags"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			if req.Name == "" {
				httpError(w, http.StatusBadRequest, errors.New("name majburiy"))
				return
			}
			rest.Name = req.Name
			rest.Address = req.Address
			rest.Lat = req.Lat
			rest.Lng = req.Lng
			rest.LogoURL = req.LogoURL
			rest.CoverURL = req.CoverURL
			rest.Tags = req.Tags
			if err := s.CatalogRepo.SaveRestaurant(r.Context(), rest); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			s.Cache.Del(r.Context(), restaurantsCacheKey)
			writeJSON(w, http.StatusOK, rest)
		}))

	// POST /admin/restaurants/{id}/open  {"open":true|false}
	mux.HandleFunc("POST /admin/restaurants/{id}/open", s.auth([]users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			rest, err := s.CatalogRepo.GetRestaurant(r.Context(), r.PathValue("id"))
			if err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			var req struct {
				Open bool `json:"open"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			rest.Open = req.Open
			if err := s.CatalogRepo.SaveRestaurant(r.Context(), rest); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			s.Cache.Del(r.Context(), restaurantsCacheKey)
			writeJSON(w, http.StatusOK, rest)
		}))

	// GET /admin/couriers — barcha kuryerlar (telefon raqamlari bilan)
	mux.HandleFunc("GET /admin/couriers", s.auth([]users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			list, err := s.CourierRepo.ListAll(r.Context())
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			courierUsers, err := s.UserRepo.ListByRole(r.Context(), users.RoleCourier)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			phoneByEntity := make(map[string]string, len(courierUsers))
			for _, u := range courierUsers {
				phoneByEntity[u.EntityID] = u.Phone
			}
			type row struct {
				couriers.Courier
				Phone string `json:"phone"`
			}
			out := make([]row, 0, len(list))
			for _, c := range list {
				out = append(out, row{Courier: *c, Phone: phoneByEntity[c.ID]})
			}
			writeJSON(w, http.StatusOK, out)
		}))

	// POST /admin/couriers/{id}/approve  {"approved":true|false}
	mux.HandleFunc("POST /admin/couriers/{id}/approve", s.auth([]users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Approved bool `json:"approved"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			courierID := r.PathValue("id")
			if err := s.CourierRepo.SetApproved(r.Context(), courierID, req.Approved); err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			// Blok qilinganda darhol offline ham qilamiz
			if !req.Approved {
				s.CourierRepo.SetAvailable(r.Context(), courierID, false)
				// ...va sessiyasini bekor qilamiz. Busiz "blokladim"
				// degan amal yarim choraki bo'lardi: kuryer offline
				// qilinsa ham, qo'lidagi token bilan API'ga murojaat
				// qilishda (o'z ma'lumotlarini o'qish, joylashuv
				// yuborish) davom eta olardi.
				if list, err := s.UserRepo.ListByRole(r.Context(), users.RoleCourier); err == nil {
					for _, u := range list {
						if u.EntityID == courierID {
							s.Revoked.Revoke(r.Context(), u.ID)
						}
					}
				} else {
					slog.Error("kuryer sessiyasini bekor qilib bo'lmadi", "courier", courierID, "err", err)
				}
			}
			writeJSON(w, http.StatusOK, map[string]bool{"approved": req.Approved})
		}))

	// GET /admin/orders — so'nggi buyurtmalar
	mux.HandleFunc("GET /admin/orders", s.auth([]users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			list, err := s.OrderRepo.ListRecent(r.Context(), 100)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, list)
		}))

	// GET /admin/stats — boshqaruv paneli ko'rsatkichlari
	mux.HandleFunc("GET /admin/stats", s.auth([]users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			recent, err := s.OrderRepo.ListRecent(r.Context(), 500)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			now := time.Now()
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			byStatus := map[orders.Status]int{}
			var ordersToday, deliveredToday int
			var revenueToday int64
			for _, o := range recent {
				byStatus[o.Status]++
				if o.CreatedAt.After(today) {
					ordersToday++
					if o.Status == orders.StatusDelivered {
						deliveredToday++
						revenueToday += o.TotalTiyin
					}
				}
			}
			allCouriers, err := s.CourierRepo.ListAll(r.Context())
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			var online, pending int
			for _, c := range allCouriers {
				if c.Available && c.Approved {
					online++
				}
				if !c.Approved {
					pending++
				}
			}
			restaurants, _ := s.CatalogRepo.ListRestaurants(r.Context())
			writeJSON(w, http.StatusOK, map[string]any{
				"orders_today":        ordersToday,
				"delivered_today":     deliveredToday,
				"revenue_today_tiyin": revenueToday,
				"by_status":           byStatus,
				"couriers_online":     online,
				"couriers_pending":    pending,
				"restaurants_total":   len(restaurants),
			})
		}))
}
