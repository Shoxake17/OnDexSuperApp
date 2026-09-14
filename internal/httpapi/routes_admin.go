package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/couriers"
	"chustapp/internal/delivery"
	"chustapp/internal/orders"
	"chustapp/internal/users"
)

// isAllowedSceneURL — 3D maket manzili bizning saqlashimizdami.
//
// ┌─ NEGA CHEKLOV ─────────────────────────────────────────────────────┐
// Maket fayli mijoz telefoniga yuklanadi va ichida bajariladigan kod
// bor. Ixtiyoriy manzilga ruxsat berilsa, admin panelga kirgan kishi
// (yoki uning hisobini egallagan) mijozlarga begona kod tarqata
// olardi.
//
// Manzil `R2_PUBLIC_URL` dan olinadi — ya'ni qayerga yuklaganimiz va
// qayerdan berishimiz BIR joyda belgilangan.
// └────────────────────────────────────────────────────────────────────┘
func isAllowedSceneURL(raw string) bool {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("R2_PUBLIC_URL")), "/")
	if base == "" {
		// R2 sozlanmagan bo'lsa 3D ni yoqib bo'lmaydi. Jimgina
		// ruxsat berishdan ko'ra rad etish xavfsizroq.
		return false
	}
	if !strings.HasPrefix(base, "https://") {
		return false
	}
	return strings.HasPrefix(raw, base+"/")
}

// deref — nil ko'rsatkichdan bo'sh satr.
func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// isSHA256Hex — 64 ta o'n oltilik belgi.
func isSHA256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

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
			// Koordinata tekshiruvi (bug.md 25-band, 1-qism): avval
			// umuman tekshirilmasdi va `(0,0)` yoki `(999,999)`
			// restoran yaratish mumkin edi — bunday restoranga kuryer
			// hech qachon topilmasdi (`distanceKM` ma'nosiz qiymat
			// beradi) va dispatch JIMGINA ishlamay qolardi.
			if !delivery.ValidCoords(req.Lat, req.Lng) {
				httpError(w, http.StatusBadRequest,
					errors.New("lat/lng noto'g'ri — xaritadan nuqta tanlang"))
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
				// ┌─ KOMPENSATSIYA (bug.md 25-band, 2-qism) ───────────┐
				// Avval bu yerda faqat 500 qaytarilardi va restoran
				// XODIMSIZ qolib ketardi — unga kirishning yo'li yo'q
				// va uni faqat bazadan qo'lda tozalash mumkin edi.
				//
				// Haqiqiy tranzaksiya MUMKIN EMAS: restoran Mongo'da,
				// akkaunt Postgres'da (`single-source-of-truth`
				// arxitekturasi). Shuning uchun kompensatsiya —
				// yaratilgan restoranni orqaga o'chirish.
				//
				// O'chirish ham yiqilsa, ID logga yoziladi: superadmin
				// uni qo'lda topa oladi. Jimgina qoldirish eng yomon
				// variant bo'lardi.
				// └────────────────────────────────────────────────────┘
				if delErr := s.CatalogRepo.DeleteRestaurant(r.Context(), rest.ID); delErr != nil {
					slog.Error("restoran yaratildi, lekin akkaunt yaratilmadi VA orqaga o'chirib ham bo'lmadi — QO'LDA tozalash kerak",
						"restaurant", rest.ID, "akkaunt_xatosi", err, "ochirish_xatosi", delErr)
				} else {
					slog.Warn("akkaunt yaratilmadi — restoran orqaga o'chirildi",
						"restaurant", rest.ID, "err", err)
				}
				s.Cache.Del(r.Context(), restaurantsCacheKey)
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
				// Kind — muassasa turi. Ko'rsatkich: eski admin panel uni
				// yubormaydi va mavjud tur jimgina o'chib ketmasin.
				Kind *string `json:"kind"`
				// 0 = ko'rsatkich yo'q (mijoz tomonida chip chizilmaydi).
				Rating        float64 `json:"rating"`
				RatingCount   int     `json:"rating_count"`
				ETAMinMinutes int     `json:"eta_min_minutes"`
				ETAMaxMinutes int     `json:"eta_max_minutes"`
				// ┌─ KO'RSATKICH, ODDIY QIYMAT EMAS ──────────────────┐
				// Bu endpointga TO'LIQ holat yuboriladi, lekin eski
				// mijozlar (admin panelning oldingi versiyasi) 3D
				// maydonlarini umuman bilmaydi.
				//
				// Oddiy `string` bo'lganda ular bo'sh kelardi va
				// mavjud maket JIMGINA O'CHIB KETARDI — aynan shu
				// yuz berdi: panel restoranni saqlashi bilan Book
				// Cafe maketi yo'qoldi.
				//
				// Ko'rsatkich bilan "yuborilmadi" (nil) va
				// "bo'shatilsin" ("") ajraladi.
				// └───────────────────────────────────────────────────┘
				Scene3DURL    *string `json:"scene_3d_url"`
				Scene3DSHA256 *string `json:"scene_3d_sha256"`
				Scene3DBytes  *int64  `json:"scene_3d_bytes"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			if req.Name == "" {
				httpError(w, http.StatusBadRequest, errors.New("name majburiy"))
				return
			}
			// ┌─ NEGA BU YERDA HAM TEKSHIRILADI ──────────────────────────┐
			// Bazada CHECK cheklovlari bor, lekin ular buzilganda pgx
			// xatosi 500 bo'lib chiqardi va admin sababni tushunmasdi.
			// Bu yerda 400 + aniq matn beriladi; baza esa oxirgi
			// himoya bo'lib qoladi (boshqa yo'l bilan yozishga qarshi).
			// └───────────────────────────────────────────────────────────┘
			if req.Rating < 0 || req.Rating > 5 {
				httpError(w, http.StatusBadRequest,
					errors.New("reyting 0 va 5 orasida bo'lishi kerak (0 = ko'rsatilmaydi)"))
				return
			}
			if req.RatingCount < 0 || req.ETAMinMinutes < 0 || req.ETAMaxMinutes < 0 {
				httpError(w, http.StatusBadRequest, errors.New("manfiy qiymat bo'lmaydi"))
				return
			}
			if req.ETAMaxMinutes < req.ETAMinMinutes {
				httpError(w, http.StatusBadRequest,
					errors.New("yetkazishning eng ko'p vaqti eng kamidan kichik bo'lmasin"))
				return
			}
			// ┌─ URL SXEMASI (bug.md 24-band) ─────────────────────────┐
			// Avval bu maydonlar UMUMAN tekshirilmasdi: `javascript:`,
			// `data:` yoki begona domen bemalol saqlanardi va mijoz
			// ilovasida rasm/havola sifatida ishlatilardi.
			// └────────────────────────────────────────────────────────┘
			if !isSafeMediaURL(req.LogoURL) || !isSafeMediaURL(req.CoverURL) {
				httpError(w, http.StatusBadRequest,
					errors.New("logo/muqova manzili yaroqsiz (faqat https:// yoki ichki yo'l)"))
				return
			}
			// ┌─ KOORDINATA TEKSHIRUVI (bug.md 25-band) ───────────────┐
			// `Lat`/`Lng` umuman tekshirilmasdi: `(0,0)` yoki
			// `(999,999)` restoran yaratish mumkin edi. Bunday restoran
			// kuryer taqsimotini buzadi — `distanceKM` ma'nosiz qiymat
			// beradi va hech bir kuryer mos kelmaydi, dispatch esa
			// JIMGINA ishlamay qoladi.
			//
			// `POST /me/address` va `POST /couriers/{id}/location`
			// ikkalasi ham allaqachon tekshiradi; bu yerda
			// qo'llanmagandi.
			//
			// `CheckPoint` EMAS, `ValidCoords`: restoran xizmat
			// hududidan tashqarida bo'lishi MUMKIN (masalan yangi
			// shahar ochilayotganda) — bu admin qarori. Tekshiriladigan
			// narsa faqat koordinataning o'zi ma'noli ekani.
			// └────────────────────────────────────────────────────────┘
			if !delivery.ValidCoords(req.Lat, req.Lng) {
				httpError(w, http.StatusBadRequest,
					errors.New("lat/lng noto'g'ri — xaritadan nuqta tanlang"))
				return
			}
			rest.Name = req.Name
			rest.Address = req.Address
			rest.Lat = req.Lat
			rest.Lng = req.Lng
			rest.LogoURL = req.LogoURL
			rest.CoverURL = req.CoverURL
			rest.Tags = req.Tags
			if req.Kind != nil {
				kind, err := catalog.ParseRestaurantKind(*req.Kind)
				if err != nil {
					httpError(w, http.StatusBadRequest, err)
					return
				}
				rest.Kind = kind
			}
			rest.Rating = req.Rating
			rest.RatingCount = req.RatingCount
			rest.ETAMinMinutes = req.ETAMinMinutes
			rest.ETAMaxMinutes = req.ETAMaxMinutes

			// ┌─ 3D MAKET UCHUN TEKSHIRUV ────────────────────────────────┐
			// Maket fayli ichida BAJARILADIGAN kod bor (Godot
			// skriptlari). Ilova uni yuklab olib ishga tushiradi.
			//
			// Shuning uchun manzil ixtiyoriy bo'la olmaydi: faqat
			// bizning R2 domenimiz qabul qilinadi va SHA-256 majburiy.
			// Aks holda admin panelga kirgan kishi mijozlarning
			// telefoniga begona kod yuborishi mumkin bo'lardi.
			// └───────────────────────────────────────────────────────────┘
			// Uchtasi BIRGA keladi yoki umuman kelmaydi: yarim
			// yangilanish (manzil yangi, xesh eski) ilovada "fayl
			// buzilgan" xatosiga olib kelardi.
			if req.Scene3DURL != nil || req.Scene3DSHA256 != nil ||
				req.Scene3DBytes != nil {

				scene := strings.TrimSpace(deref(req.Scene3DURL))
				digest := strings.ToLower(strings.TrimSpace(deref(req.Scene3DSHA256)))
				var size int64
				if req.Scene3DBytes != nil {
					size = *req.Scene3DBytes
				}

				if scene != "" {
					if !isAllowedSceneURL(scene) {
						httpError(w, http.StatusBadRequest,
							errors.New("maket manzili faqat R2 domenida bo'lishi kerak"))
						return
					}
					if !isSHA256Hex(digest) {
						httpError(w, http.StatusBadRequest,
							errors.New("maket uchun to'g'ri SHA-256 majburiy"))
						return
					}
					if size <= 0 {
						httpError(w, http.StatusBadRequest,
							errors.New("maket hajmi ko'rsatilishi kerak"))
						return
					}
				}
				rest.Scene3DURL = scene
				rest.Scene3DSHA256 = digest
				rest.Scene3DBytes = size
			}

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
			// userIDByEntity — kuryer yozuvidan uning AKKAUNTIGA o'tish.
			// Panelda o'chirish tugmasi shu ID bilan ishlaydi
			// (`DELETE /admin/users/{id}`): o'chirish akkauntga
			// tegishli amal, kuryer yozuvi esa uning ergashuvchisi.
			userIDByEntity := make(map[string]string, len(courierUsers))
			for _, u := range courierUsers {
				phoneByEntity[u.EntityID] = u.Phone
				userIDByEntity[u.EntityID] = u.ID
			}
			type row struct {
				couriers.Courier
				Phone  string `json:"phone"`
				UserID string `json:"user_id,omitempty"`
			}
			out := make([]row, 0, len(list))
			for _, c := range list {
				out = append(out, row{
					Courier: *c,
					Phone:   phoneByEntity[c.ID],
					UserID:  userIDByEntity[c.ID],
				})
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
				if err := s.CourierRepo.SetAvailable(r.Context(), courierID, false); err != nil {
					slog.Error("bloklangan kuryerni offline qilib bo'lmadi", "courier", courierID, "err", err)
				}
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
	//
	// ┌─ NEGA NOMLAR QO'SHILADI ───────────────────────────────────────┐
	// Avval bu yerdan XOM buyurtma obyekti qaytardi va admin panel
	// jadvalida `restaurant_id`/`courier_id` — ya'ni o'n oltilik ID —
	// ko'rinardi. Mijoz esa umuman ko'rsatilmasdi: `customer_id` bor
	// edi, lekin unga mos ustun yo'q edi.
	//
	// Panel har qatorga alohida so'rov yuborishi mumkin edi, lekin 100
	// ta buyurtma uchun bu 300 ta qo'shimcha so'rov degani. Shuning
	// uchun nomlar SHU YERDA to'ldiriladi — `GET /me/orders` dagi bilan
	// bir xil naqsh.
	//
	// Kesh takroriy qidiruvni yo'q qiladi: bir necha restoran va
	// kuryerga tegishli 100 ta buyurtma odatda 5-10 ta qidiruv bilan
	// to'ladi.
	// └────────────────────────────────────────────────────────────────┘
	mux.HandleFunc("GET /admin/orders", s.auth([]users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			list, err := s.OrderRepo.ListRecent(r.Context(), 100)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}

			type found struct {
				name  string
				found bool
			}
			restCache := map[string]*found{}
			courierCache := map[string]*found{}
			userCache := map[string][2]string{} // ism, telefon

			out := make([]map[string]any, 0, len(list))
			for _, o := range list {
				entry := map[string]any{
					"id":            o.ID,
					"order_number":  o.OrderNumber,
					"created_at":    o.CreatedAt,
					"status":        o.Status,
					"type":          o.Type,
					"table_label":   o.TableLabel,
					"total_tiyin":   o.TotalTiyin,
					"customer_id":   o.CustomerID,
					"restaurant_id": o.RestaurantID,
					"courier_id":    o.CourierID,
				}

				if o.RestaurantID != "" {
					f, ok := restCache[o.RestaurantID]
					if !ok {
						f = &found{}
						if rest, err := s.CatalogRepo.GetRestaurant(r.Context(), o.RestaurantID); err == nil && rest != nil {
							f.name = rest.Name
							f.found = true
						}
						restCache[o.RestaurantID] = f
					}
					if f.found {
						entry["restaurant_name"] = f.name
					}
				}

				if o.CourierID != "" {
					f, ok := courierCache[o.CourierID]
					if !ok {
						f = &found{}
						if c, err := s.CourierRepo.GetByID(r.Context(), o.CourierID); err == nil && c != nil {
							f.name = c.Name
							f.found = true
						}
						courierCache[o.CourierID] = f
					}
					if f.found {
						entry["courier_name"] = f.name
					}
				}

				if o.CustomerID != "" {
					who, ok := userCache[o.CustomerID]
					if !ok {
						if u, err := s.UserRepo.GetByID(r.Context(), o.CustomerID); err == nil && u != nil {
							who = [2]string{u.Name, u.Phone}
						}
						userCache[o.CustomerID] = who
					}
					entry["customer_name"] = who[0]
					// ┌─ TELEFON NEGA KERAK ────────────────────────────┐
					// Ism ixtiyoriy va ko'p mijozda bo'sh bo'ladi
					// (ro'yxatdan o'tishda faqat raqam so'raladi).
					// Faqat ismga tayansak jadval yana bo'sh ko'rinardi.
					// Telefon esa HAR DOIM bor — u login identifikatori.
					// └─────────────────────────────────────────────────┘
					entry["customer_phone"] = who[1]
				}

				out = append(out, entry)
			}
			writeJSON(w, http.StatusOK, out)
		}))

	// GET /admin/accounts?role=restaurant|courier|waiter
	//
	// ┌─ NEGA KERAK ───────────────────────────────────────────────────┐
	// Superadmin panelida restoran yoki kuryer ustiga bosilganda uning
	// PANELDA/ILOVADA nima qilgani ko'rsatilishi kerak (PostHog seans
	// yozuvi).
	//
	// Lekin PostHog'da "odam" — bu RESTORAN emas, uning XODIM AKKAUNTI:
	// tahlil `ApiClient.me()` da `identify(userId: <user.ID>)` bilan
	// bog'lanadi. Restoranlar ro'yxati esa katalogdan keladi va unda
	// akkaunt ID si umuman yo'q.
	//
	// Shu endpoint o'sha bog'lanishni beradi: `entity_id` (restoran yoki
	// kuryer) -> `user_id` (PostHog dagi odam).
	//
	// Telefon/ism ham qaytadi — panelda kimga tegishli ekanini
	// ko'rsatish uchun. Parol xeshi `json:"-"` bilan himoyalangan
	// (`users.User` izohiga qarang), ya'ni bu yerdan chiqib keta olmaydi.
	// └────────────────────────────────────────────────────────────────┘
	mux.HandleFunc("GET /admin/accounts", s.auth([]users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			var role users.Role
			switch r.URL.Query().Get("role") {
			case "restaurant":
				role = users.RoleRestaurant
			case "courier":
				role = users.RoleCourier
			case "waiter":
				role = users.RoleWaiter
			default:
				httpError(w, http.StatusBadRequest,
					errors.New("role: restaurant, courier yoki waiter"))
				return
			}
			list, err := s.UserRepo.ListByRole(r.Context(), role)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			out := make([]map[string]any, 0, len(list))
			for _, u := range list {
				if u == nil || u.EntityID == "" {
					continue
				}
				out = append(out, map[string]any{
					"entity_id": u.EntityID,
					"user_id":   u.ID,
					"name":      u.Name,
					"phone":     u.Phone,
				})
			}
			writeJSON(w, http.StatusOK, out)
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
