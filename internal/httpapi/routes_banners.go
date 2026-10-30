package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
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
			// `?placement=menu|restaurant|store`. Parametr YO'Q bo'lsa — `restaurant`:
			// joylashuv paydo bo'lgunicha chiqarilgan ilovalar shu sahifada
			// bannerlarni ko'rsatardi va o'sha xatti-harakat o'zgarmaydi.
			placement := r.URL.Query().Get("placement")
			if placement == "" {
				placement = banners.PlacementRestaurant
			}
			if !banners.ValidPlacement(placement) {
				httpError(w, http.StatusBadRequest,
					errors.New("placement: menu, restaurant yoki store"))
				return
			}
			list, err := s.BannerRepo.ListActive(r.Context())
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			filtered := make([]*banners.Banner, 0, len(list))
			for _, b := range list {
				bp := b.Placement
				if bp == "" { // eski/qo'lda yaratilgan yozuv: joylashuv belgilanmagan = restoran sahifasi
					bp = banners.PlacementRestaurant
				}
				if bp == placement {
					filtered = append(filtered, b)
				}
			}
			writeJSON(w, http.StatusOK, s.visibleBanners(r, filtered))
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
			var req bannerRequest
			if !decodeBannerRequest(w, r, &req) {
				return
			}
			now := time.Now()
			b := &banners.Banner{ID: NewID(), CreatedAt: now, UpdatedAt: now}
			if !applyBannerRequest(w, b, &req) {
				return
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
			var req bannerRequest
			if !decodeBannerRequest(w, r, &req) {
				return
			}
			if !applyBannerRequest(w, b, &req) {
				return
			}
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

// bannerRequest — admin yaratish/tahrirlash so'rovi.
//
// Tahrirlashda ham TO'LIQ holat yuboriladi (admin panel mavjud qiymatlar bilan
// to'ldirilgan oyna ochadi), shuning uchun kelmagan maydon "o'zgarishsiz" EMAS,
// balki bo'sh deb olinadi — xuddi `image_url`/`restaurant_id` da bo'lganidek.
type bannerRequest struct {
	ImageURL     string `json:"image_url"`
	Placement    string `json:"placement"`
	Title        string `json:"title"`
	Subtitle     string `json:"subtitle"`
	ButtonText   string `json:"button_text"`
	Action       string `json:"action"`
	RestaurantID string `json:"restaurant_id"`
	Active       bool   `json:"active"`
	SortOrder    int    `json:"sort_order"`
}

func decodeBannerRequest(w http.ResponseWriter, r *http.Request, req *bannerRequest) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(req); err != nil {
		httpError(w, http.StatusBadRequest, err)
		return false
	}
	return true
}

// applyBannerRequest — so'rovni bannerga o'tkazadi va HAMMA qoidalarni tekshiradi
// (matn uzunligi, joylashuv, havola mosligi, xavfsiz rasm manzili). Xato bo'lsa
// javobni o'zi yozadi va `false` qaytaradi.
func applyBannerRequest(w http.ResponseWriter, b *banners.Banner, req *bannerRequest) bool {
	if strings.TrimSpace(req.ImageURL) == "" {
		httpError(w, http.StatusBadRequest, errors.New("image_url majburiy"))
		return false
	}
	// Rasm manzili mijoz ilovasida `Image.network` ga beriladi: `javascript:`,
	// `data:`, `http://` rad etiladi (`isSafeMediaURL`).
	if !isSafeMediaURL(req.ImageURL) {
		httpError(w, http.StatusBadRequest, errors.New("rasm manzili noto'g'ri"))
		return false
	}
	b.ImageURL = strings.TrimSpace(req.ImageURL)
	b.Placement = req.Placement
	b.Title = req.Title
	b.Subtitle = req.Subtitle
	b.ButtonText = req.ButtonText
	b.Action = req.Action
	b.RestaurantID = req.RestaurantID
	b.Active = req.Active
	b.SortOrder = req.SortOrder
	if err := b.Normalize(); err != nil {
		httpError(w, http.StatusBadRequest, err)
		return false
	}
	return true
}
