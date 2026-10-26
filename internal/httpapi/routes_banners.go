package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"chustapp/internal/banners"
	"chustapp/internal/users"
)

// registerBannerRoutes — reklama/aksiya bannerlari (`internal/banners`,
// Yandex Eats/Go uslubi): admin panelidan boshqariladi, mijoz ilovasi
// ularni ochiq (auth shart emas) o'qiydi.
//
// `s.BannerRepo == nil` bo'lsa (Postgres ulanmagan, lekin xotira
// zaxirasi ham berilmagan — amalda bo'lmasligi kerak) marshrutlar
// RO'YXATGA OLINMAYDI: ilova 404 aniq javob beradi, "ishlamayapti"
// deb chalkashtirmaydi.
func (s *Server) registerBannerRoutes(mux *http.ServeMux) {
	if s.BannerRepo == nil {
		return
	}

	// GET /promo-banners — ochiq, faqat FAOL bannerlar, tartib bo'yicha.
	// Mijoz ilovasining restoran sahifasidagi reklama bloki shundan
	// o'qiydi (`CatalogScreen._PromoBannerSection`).
	//
	// `?for_me=1` — `GET /restaurants` bilan AYNAN bir xil shartnoma
	// (`service_area.go`, `visibleBanners`): restoranga bog'langan
	// banner faqat o'sha restoran yetkaza oladigan shahardagi mijozga
	// ko'rinadi — Toshkentdagi restoran banneri Chustlik mijozga
	// chiqmaydi.
	mux.HandleFunc("GET /promo-banners", publicCache(5, 30, s.optionalAuth(
		func(w http.ResponseWriter, r *http.Request) {
			list, err := s.BannerRepo.ListActive(r.Context())
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, s.visibleBanners(r, list))
		})))

	// GET /admin/banners — HAMMASI (faol va nofaol) — admin panel ro'yxati.
	mux.HandleFunc("GET /admin/banners", s.auth([]users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			list, err := s.BannerRepo.List(r.Context())
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, list)
		}))

	// POST /admin/banners — yangi banner. Rasm OLDINDAN
	// `POST /uploads?type=cover` orqali yuklanadi (admin panel shuni
	// chaqiradi) — bu yerga faqat tayyor URL keladi, alohida yuklash
	// yo'li TAKRORLANMAYDI.
	mux.HandleFunc("POST /admin/banners", s.auth([]users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				ImageURL     string `json:"image_url"`
				RestaurantID string `json:"restaurant_id"`
				Active       bool   `json:"active"`
				SortOrder    int    `json:"sort_order"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			if req.ImageURL == "" {
				httpError(w, http.StatusBadRequest, errors.New("image_url majburiy"))
				return
			}
			now := time.Now()
			b := &banners.Banner{
				ID:           NewID(),
				ImageURL:     req.ImageURL,
				RestaurantID: req.RestaurantID,
				Active:       req.Active,
				SortOrder:    req.SortOrder,
				CreatedAt:    now,
				UpdatedAt:    now,
			}
			if err := s.BannerRepo.Create(r.Context(), b); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusCreated, b)
		}))

	// POST /admin/banners/{id} — tahrirlash (rasm, havola, holat, tartib).
	mux.HandleFunc("POST /admin/banners/{id}", s.auth([]users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			b, err := s.BannerRepo.Get(r.Context(), r.PathValue("id"))
			if err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			var req struct {
				ImageURL     string `json:"image_url"`
				RestaurantID string `json:"restaurant_id"`
				Active       bool   `json:"active"`
				SortOrder    int    `json:"sort_order"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			if req.ImageURL == "" {
				httpError(w, http.StatusBadRequest, errors.New("image_url majburiy"))
				return
			}
			b.ImageURL = req.ImageURL
			b.RestaurantID = req.RestaurantID
			b.Active = req.Active
			b.SortOrder = req.SortOrder
			b.UpdatedAt = time.Now()
			if err := s.BannerRepo.Update(r.Context(), b); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, b)
		}))

	// DELETE /admin/banners/{id}
	mux.HandleFunc("DELETE /admin/banners/{id}", s.auth([]users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			if err := s.BannerRepo.Delete(r.Context(), r.PathValue("id")); err != nil {
				if errors.Is(err, banners.ErrNotFound) {
					httpError(w, http.StatusNotFound, err)
					return
				}
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
		}))
}
