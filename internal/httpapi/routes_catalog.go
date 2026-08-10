package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/users"
)

func (s *Server) registerCatalogRoutes(mux *http.ServeMux) {
	// GET /restaurants — ochiq: mijoz ilovasining bosh sahifasi. Eng ko'p
	// so'raladigan endpoint (har mijoz ilova ochganda) — shuning uchun
	// Redis'da qisqa muddatga (30s) keshlanadi. Restoran ma'lumoti
	// o'zgarganda (yaratish/tahrirlash/ochiq-yopiq) kesh darhol tozalanadi
	// (restaurantsCacheKey konstantasiga qarang), shuning uchun 30s —
	// "eng yomon holatda shuncha eskirishi mumkin" chegarasi, oddiy TTL
	// emas.
	mux.HandleFunc("GET /restaurants", func(w http.ResponseWriter, r *http.Request) {
		var list []*catalog.Restaurant
		if s.Cache.GetJSON(r.Context(), restaurantsCacheKey, &list) {
			writeJSON(w, http.StatusOK, list)
			return
		}
		list, err := s.CatalogRepo.ListRestaurants(r.Context())
		if err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		s.Cache.SetJSON(r.Context(), restaurantsCacheKey, list, 30*time.Second)
		writeJSON(w, http.StatusOK, list)
	})

	// GET /restaurants/{id} — bitta restoran ma'lumoti (ochiq)
	mux.HandleFunc("GET /restaurants/{id}", func(w http.ResponseWriter, r *http.Request) {
		rest, err := s.CatalogRepo.GetRestaurant(r.Context(), r.PathValue("id"))
		if err != nil {
			httpError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, rest)
	})

	// GET /restaurants/{id}/orders — restoranning o'z buyurtmalari (yoki admin)
	mux.HandleFunc("GET /restaurants/{id}/orders", s.auth([]users.Role{users.RoleRestaurant, users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID := r.PathValue("id")
			claims := claimsFrom(r)
			if claims.Role == users.RoleRestaurant && claims.EntityID != restaurantID {
				httpError(w, http.StatusForbidden, errors.New("boshqa restoran buyurtmalarini ko'rib bo'lmaydi"))
				return
			}
			list, err := s.OrderRepo.ListByRestaurant(r.Context(), restaurantID, 100)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			// Har bir buyurtmaga mijoz telefon raqami VA (biriktirilgan
			// bo'lsa) kuryer ismi qo'shiladi — FAQAT shu yerda (restoran
			// o'z buyurtmalarini ko'rayotganda, egalik tekshiruvi yuqorida
			// allaqachon bajarilgan), xuddi kuryerga picked_up'dan keyin
			// ko'rsatilgan customer_phone bilan bir xil xavfsizlik
			// darajasida (GET /orders/{id}dagi customerPhoneFor'ga
			// qarang) — ochiq/umumiy endpointda bu maydonlar HECH QACHON
			// ko'rinmaydi. Kuryer ismi — restoran panelida "Faol
			// buyurtmalar" ro'yxatida qaysi kuryer ekanini ko'rsatish
			// uchun (avval faqat xom ID ko'rinardi).
			out := make([]map[string]any, 0, len(list))
			for _, o := range list {
				entry := withExtraField(o, "customer_phone", customerPhoneFor(r.Context(), s.UserRepo, o.CustomerID))
				if o.CourierID != "" {
					if c, err := s.CourierRepo.GetByID(r.Context(), o.CourierID); err == nil {
						entry = withExtraField(entry, "courier_name", c.Name)
					}
				}
				out = append(out, entry)
			}
			writeJSON(w, http.StatusOK, out)
		}))

	// POST /restaurants/{id}/open — restoran o'zini ochiq/yopiq qiladi (yoki admin)
	mux.HandleFunc("POST /restaurants/{id}/open", s.auth([]users.Role{users.RoleRestaurant, users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID := r.PathValue("id")
			claims := claimsFrom(r)
			if claims.Role == users.RoleRestaurant && claims.EntityID != restaurantID {
				httpError(w, http.StatusForbidden, errors.New("boshqa restoran holatini o'zgartirib bo'lmaydi"))
				return
			}
			rest, err := s.CatalogRepo.GetRestaurant(r.Context(), restaurantID)
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

	// GET /categories — ochiq. Barcha restoranlar uchun umumiy, standart
	// taom turkumlari ro'yxati (yagona manba — restoran paneli va mijoz
	// ilovasi ikkalasi ham shundan foydalanadi, ular orasida yozilishi
	// farq qilib ketmasligi uchun).
	mux.HandleFunc("GET /categories", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, catalog.PredefinedCategories)
	})

	// GET /products/search?q=... — ochiq. Restoranga bog'liq bo'lmagan
	// holda, turkum yoki nom bo'yicha barcha restoranlardagi mos taomlarni
	// bitta ro'yxatda qaytaradi (mijoz ilovasining turkum filtri uchun).
	mux.HandleFunc("GET /products/search", func(w http.ResponseWriter, r *http.Request) {
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if q == "" {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		list, err := s.CatalogRepo.SearchProducts(r.Context(), q)
		if err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})

	// GET /restaurants/{id}/menu — ochiq. Har mijoz restoran menyusini
	// ochganda so'raladi — Redis'da 30s keshlanadi, mahsulot
	// qo'shilganda/tahrirlanganda/o'chirilganda kesh darhol tozalanadi.
	mux.HandleFunc("GET /restaurants/{id}/menu", func(w http.ResponseWriter, r *http.Request) {
		restaurantID := r.PathValue("id")
		if _, err := s.CatalogRepo.GetRestaurant(r.Context(), restaurantID); err != nil {
			httpError(w, http.StatusNotFound, err)
			return
		}
		var list []*catalog.Product
		if s.Cache.GetJSON(r.Context(), menuCacheKey(restaurantID), &list) {
			writeJSON(w, http.StatusOK, list)
			return
		}
		list, err := s.CatalogRepo.ListProducts(r.Context(), restaurantID)
		if err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		s.Cache.SetJSON(r.Context(), menuCacheKey(restaurantID), list, 30*time.Second)
		writeJSON(w, http.StatusOK, list)
	})
	// POST /restaurants/{id}/products — restoran o'z menyusini boshqaradi (yoki admin)
	mux.HandleFunc("POST /restaurants/{id}/products", s.auth([]users.Role{users.RoleRestaurant, users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID := r.PathValue("id")
			claims := claimsFrom(r)
			if claims.Role == users.RoleRestaurant && claims.EntityID != restaurantID {
				httpError(w, http.StatusForbidden, errors.New("boshqa restoran menyusini o'zgartirib bo'lmaydi"))
				return
			}
			if _, err := s.CatalogRepo.GetRestaurant(r.Context(), restaurantID); err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			var p catalog.Product
			if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			if p.Name == "" || p.PriceTiyin <= 0 {
				httpError(w, http.StatusBadRequest, errors.New("name va musbat price_tiyin majburiy"))
				return
			}
			if p.DiscountPriceTiyin < 0 {
				httpError(w, http.StatusBadRequest, errors.New("discount_price_tiyin manfiy bo'lishi mumkin emas"))
				return
			}
			if p.DiscountPriceTiyin > 0 && p.DiscountPriceTiyin >= p.PriceTiyin {
				httpError(w, http.StatusBadRequest, errors.New("discount_price_tiyin asosiy narxdan kichik bo'lishi kerak"))
				return
			}
			p.PrepTimeText = strings.TrimSpace(p.PrepTimeText)
			if len(p.PrepTimeText) > catalog.MaxPrepTimeTextLength {
				httpError(w, http.StatusBadRequest, errors.New("prep_time_text juda uzun"))
				return
			}
			if p.Stock < 0 {
				httpError(w, http.StatusBadRequest, errors.New("stock manfiy bo'lishi mumkin emas"))
				return
			}
			if p.Weight < 0 {
				httpError(w, http.StatusBadRequest, errors.New("weight manfiy bo'lishi mumkin emas"))
				return
			}
			if p.Weight > 0 {
				if p.WeightUnit == "" {
					p.WeightUnit = "g"
				} else if !catalog.IsAllowedWeightUnit(p.WeightUnit) {
					httpError(w, http.StatusBadRequest, errors.New("weight_unit noto'g'ri (g, kg, ml, l bo'lishi kerak)"))
					return
				}
			} else {
				p.WeightUnit = ""
			}
			p.Description = strings.TrimSpace(p.Description)
			if len(p.Description) > catalog.MaxDescriptionLength {
				httpError(w, http.StatusBadRequest, errors.New("description juda uzun"))
				return
			}
			p.RestaurantID = restaurantID
			if p.ID == "" {
				p.ID = NewID()
			}
			if err := s.CatalogRepo.SaveProduct(r.Context(), &p); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			s.Cache.Del(r.Context(), menuCacheKey(restaurantID))
			s.Hub.Send(restaurantTopic(restaurantID), map[string]any{"type": "menu_updated"})
			writeJSON(w, http.StatusCreated, p)
		}))

	// DELETE /restaurants/{id}/products/{productId} — taomni menyudan o'chirish.
	// Eski buyurtmalarga ta'sir qilmaydi (nom/narx buyurtma vaqtida nusxalanadi).
	mux.HandleFunc("DELETE /restaurants/{id}/products/{productId}", s.auth([]users.Role{users.RoleRestaurant, users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID := r.PathValue("id")
			productID := r.PathValue("productId")
			claims := claimsFrom(r)
			if claims.Role == users.RoleRestaurant && claims.EntityID != restaurantID {
				httpError(w, http.StatusForbidden, errors.New("boshqa restoran menyusini o'zgartirib bo'lmaydi"))
				return
			}
			products, err := s.CatalogRepo.GetProductsByIDs(r.Context(), []string{productID})
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			if len(products) == 0 {
				httpError(w, http.StatusNotFound, catalog.ErrNotFound)
				return
			}
			if products[0].RestaurantID != restaurantID {
				httpError(w, http.StatusForbidden, errors.New("bu taom sizning restoraningizga tegishli emas"))
				return
			}
			if err := s.CatalogRepo.DeleteProduct(r.Context(), productID); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			s.Cache.Del(r.Context(), menuCacheKey(restaurantID))
			s.Hub.Send(restaurantTopic(restaurantID), map[string]any{"type": "menu_updated"})
			writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
		}))

	// ---------- Aksiyalar (promotions) ----------
	// checkout'ga ULANGAN (orders.Service.priceCart + promotions.ApplyBest) —
	// "foydalanish/savdo" ko'rsatkichlari endi HAQIQIY, checkout'da shu
	// aksiya haqiqatan tanlanganda oshadi (orders.Service.Create'ga qarang).

	// GET /restaurants/{id}/active-promotions — OCHIQ (mijoz ilovasi uchun):
	// faqat HOZIR FAOL (ComputeStatus==active) aksiyalarni, faqat
	// mijozga tegishli/xavfsiz maydonlar bilan qaytaradi — restoranning
	// ichki biznes ko'rsatkichlari (UsageCount/SalesTotalTiyin) BU YERDA
	// HECH QACHON chiqarilmaydi.
}
