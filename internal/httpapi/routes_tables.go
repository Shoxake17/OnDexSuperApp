// Stollar (QR kod) va affitsiantlar — restoran o'zi boshqaradigan
// resurslar.
//
// ┌─ HUQUQ MODELI ────────────────────────────────────────────────────┐
// Bu yerdagi HAMMA endpoint `RoleRestaurant` yoki `RoleAdmin` uchun.
// Restoran FAQAT O'Z resurslarini ko'radi: `EntityID` tokendan
// olinadi, so'rov tanasidan EMAS. Shuning uchun restoran boshqa
// restoranning stollarini so'rashga urinsa 404 oladi.
//
// Affitsiant (`RoleWaiter`) bu yerga umuman kira olmaydi — u faqat
// buyurtmalarni ko'radi va "berildi" deb belgilaydi.
// └───────────────────────────────────────────────────────────────────┘
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"chustapp/internal/tables"
	"chustapp/internal/users"
)

// entityIDFor — so'rovdagi restoran ID'sini tekshiradi.
//
// Admin istalgan restoran bilan ishlay oladi; restoran akkaunti esa
// FAQAT o'zi bilan. Bu tekshiruv bitta joyda: har bir handler ichida
// qo'lda yozilsa, birortasida unutilishi mumkin edi.
func entityIDFor(c *users.Claims, requested string) (string, bool) {
	if c.Role == users.RoleAdmin {
		if requested == "" {
			return "", false
		}
		return requested, true
	}
	if c.EntityID == "" || (requested != "" && requested != c.EntityID) {
		return "", false
	}
	return c.EntityID, true
}

func (s *Server) registerTableRoutes(mux *http.ServeMux) {
	staff := []users.Role{users.RoleRestaurant, users.RoleAdmin}

	// requireTables — TableSvc ulanmagan bo'lsa aniq xato beradi.
	requireTables := func(w http.ResponseWriter) bool {
		if s.TableSvc == nil {
			httpError(w, http.StatusServiceUnavailable,
				errors.New("stol buyurtmalari sozlanmagan"))
			return false
		}
		return true
	}

	// ownTable — stol shu restoranga tegishliligini tekshiradi.
	//
	// Stol ID'si URL'da keladi, ya'ni uni istalgan restoran taxmin
	// qilib yoki boshqa yo'l bilan bilib olishi mumkin. Egalik
	// tekshirilmasa, bir restoran boshqasining stolini o'chirib yoki
	// QR tokenini yangilab yuborardi (butun zal QR kodlari bir
	// zumda ishlamay qolardi).
	ownTable := func(w http.ResponseWriter, r *http.Request) (*tables.Table, bool) {
		t, err := s.TableSvc.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			httpError(w, http.StatusNotFound, tables.ErrNotFound)
			return nil, false
		}
		claims := claimsFrom(r)
		if claims.Role != users.RoleAdmin && t.RestaurantID != claims.EntityID {
			// 404, 403 EMAS — begona stol ID'sining MAVJUDLIGI ham
			// oshkor bo'lmasin.
			httpError(w, http.StatusNotFound, tables.ErrNotFound)
			return nil, false
		}
		return t, true
	}

	// ---------- Stollar ----------

	// GET /restaurants/{id}/tables — restoranning stollari.
	mux.HandleFunc("GET /restaurants/{id}/tables", s.auth(staff,
		func(w http.ResponseWriter, r *http.Request) {
			if !requireTables(w) {
				return
			}
			restaurantID, ok := entityIDFor(claimsFrom(r), r.PathValue("id"))
			if !ok {
				httpError(w, http.StatusNotFound, tables.ErrNotFound)
				return
			}
			list, err := s.TableSvc.List(r.Context(), restaurantID)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			out := make([]map[string]any, 0, len(list))
			for _, t := range list {
				out = append(out, s.tableWithQR(r, t))
			}
			writeJSON(w, http.StatusOK, out)
		}))

	// POST /restaurants/{id}/tables  {"label":"5"}
	mux.HandleFunc("POST /restaurants/{id}/tables", s.auth(staff,
		func(w http.ResponseWriter, r *http.Request) {
			if !requireTables(w) {
				return
			}
			restaurantID, ok := entityIDFor(claimsFrom(r), r.PathValue("id"))
			if !ok {
				httpError(w, http.StatusNotFound, tables.ErrNotFound)
				return
			}
			var req struct {
				Label string `json:"label"`
				Zone  string `json:"zone"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			t, err := s.TableSvc.CreateInZone(r.Context(), restaurantID, req.Zone, req.Label)
			if err != nil {
				status := http.StatusBadRequest
				if errors.Is(err, tables.ErrDuplicate) {
					status = http.StatusConflict
				}
				httpError(w, status, err)
				return
			}
			writeJSON(w, http.StatusCreated, s.tableWithQR(r, t))
		}))

	// PATCH /tables/{id}  {"label":"6"} yoki {"active":false}
	mux.HandleFunc("PATCH /tables/{id}", s.auth(staff,
		func(w http.ResponseWriter, r *http.Request) {
			if !requireTables(w) {
				return
			}
			t, ok := ownTable(w, r)
			if !ok {
				return
			}
			var req struct {
				Label  *string `json:"label"`
				Zone   *string `json:"zone"`
				Active *bool   `json:"active"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			var err error
			if req.Label != nil {
				t, err = s.TableSvc.Rename(r.Context(), t.ID, *req.Label)
				if err != nil {
					status := http.StatusBadRequest
					if errors.Is(err, tables.ErrDuplicate) {
						status = http.StatusConflict
					}
					httpError(w, status, err)
					return
				}
			}
			if req.Zone != nil {
				t, err = s.TableSvc.SetZone(r.Context(), t.ID, *req.Zone)
				if err != nil {
					status := http.StatusBadRequest
					if errors.Is(err, tables.ErrDuplicate) {
						status = http.StatusConflict
					}
					httpError(w, status, err)
					return
				}
			}
			if req.Active != nil {
				t, err = s.TableSvc.SetActive(r.Context(), t.ID, *req.Active)
				if err != nil {
					httpError(w, http.StatusBadRequest, err)
					return
				}
			}
			writeJSON(w, http.StatusOK, s.tableWithQR(r, t))
		}))

	// Eslatma: `POST /tables/{id}/regenerate` endpointi ATAYLAB YO'Q.
	// QR kod menyu varaqasiga chop etilgan va stolda abadiy turadi —
	// tokenni almashtirish butun zaldagi varaqalarni bir zumda
	// ishlamas holga keltirardi (`internal/tables/table.go` dagi
	// batafsil izohga qarang). QR surati tarqalib ketsa, stol
	// `PATCH /tables/{id}` orqali `active: false` qilinadi.

	mux.HandleFunc("DELETE /tables/{id}", s.auth(staff,
		func(w http.ResponseWriter, r *http.Request) {
			if !requireTables(w) {
				return
			}
			t, ok := ownTable(w, r)
			if !ok {
				return
			}
			if err := s.TableSvc.Delete(r.Context(), t.ID); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
		}))

	// GET /tables/resolve?token=... — QR koddan stolni aniqlash.
	//
	// Mini App ochilganda chaqiriladi: qaysi restoran menyusini
	// ko'rsatish kerakligini bilish uchun.
	//
	// AUTENTIFIKATSIYA TALAB QILINADI (rol muhim emas): usiz istalgan
	// odam tokenlarni birma-bir sinab, qaysi biri haqiqiy ekanini
	// aniqlay olardi. Token 32 bayt bo'lgani uchun bu amalda
	// imkonsiz, lekin himoya bir qatlamga tayanmasligi kerak.
	mux.HandleFunc("GET /tables/resolve", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			if !requireTables(w) {
				return
			}
			t, err := s.TableSvc.Resolve(r.Context(), r.URL.Query().Get("token"))
			if err != nil {
				status := http.StatusNotFound
				if errors.Is(err, tables.ErrInactive) {
					status = http.StatusBadRequest
				}
				httpError(w, status, err)
				return
			}
			// Javobda token QAYTMAYDI — chaqiruvchi uni allaqachon
			// biladi, qaytarish esa uni loglarga/keshlarga yoyardi.
			resp := map[string]any{
				"table_id":      t.ID,
				"table_label":   t.DisplayLabel(),
				"table_number":  t.Label,
				"zone":          t.Zone,
				"restaurant_id": t.RestaurantID,
			}
			if rest, err := s.CatalogRepo.GetRestaurant(r.Context(), t.RestaurantID); err == nil {
				resp["restaurant_name"] = rest.Name
				resp["restaurant_open"] = rest.Open
			}
			writeJSON(w, http.StatusOK, resp)
		}))

	// ---------- Affitsiantlar ----------

	// GET /restaurants/{id}/waiters
	mux.HandleFunc("GET /restaurants/{id}/waiters", s.auth(staff,
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID, ok := entityIDFor(claimsFrom(r), r.PathValue("id"))
			if !ok {
				httpError(w, http.StatusNotFound, errors.New("restoran topilmadi"))
				return
			}
			list, err := s.waitersOf(r, restaurantID)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, list)
		}))

	// POST /restaurants/{id}/waiters  {"phone":"+998...","name":"Ali"}
	//
	// Akkaunt YARATILADI, lekin parol o'rnatilmaydi: affitsiant o'z
	// ilovasida SMS kod bilan kiradi (mavjud oqim). Shu sababli
	// restoran hech qachon xodimning parolini bilmaydi.
	mux.HandleFunc("POST /restaurants/{id}/waiters", s.auth(staff,
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID, ok := entityIDFor(claimsFrom(r), r.PathValue("id"))
			if !ok {
				httpError(w, http.StatusNotFound, errors.New("restoran topilmadi"))
				return
			}
			var req struct {
				Phone string `json:"phone"`
				Name  string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			phone, err := users.NormalizePhone(req.Phone)
			if err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			name := strings.TrimSpace(req.Name)
			if name == "" {
				httpError(w, http.StatusBadRequest, errors.New("ism bo'sh bo'lishi mumkin emas"))
				return
			}
			// ┌─ MAVJUD AKKAUNT ─────────────────────────────────────┐
			// Raqam allaqachon ro'yxatda bo'lsa YANGI akkaunt
			// yaratilmaydi va MAVJUDI ham o'zgartirilmaydi.
			//
			// Nega: aks holda restoran istalgan telefon raqamini
			// kiritib, o'sha odamning akkauntini o'z affitsiantiga
			// AYLANTIRIB yuborardi — jabrlanuvchi o'z buyurtmalari
			// o'rniga restoran buyurtmalarini ko'rib qolardi. Bu
			// akkauntni egallashning to'g'ridan-to'g'ri yo'li.
			// └───────────────────────────────────────────────────────┘
			if existing, err := s.UserRepo.GetByPhone(r.Context(), phone); err == nil {
				if existing.Role == users.RoleWaiter && existing.EntityID == restaurantID {
					httpError(w, http.StatusConflict,
						errors.New("bu affitsiant allaqachon qo'shilgan"))
					return
				}
				httpError(w, http.StatusConflict,
					errors.New("bu telefon raqam boshqa akkauntga biriktirilgan"))
				return
			}
			account := users.User{
				ID: NewID(), Phone: phone, Name: name,
				Role: users.RoleWaiter, EntityID: restaurantID,
				CreatedAt: time.Now(),
			}
			if err := s.UserRepo.Create(r.Context(), &account); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusCreated, account)
		}))

	// DELETE /restaurants/{id}/waiters/{waiterID}
	mux.HandleFunc("DELETE /restaurants/{id}/waiters/{waiterID}", s.auth(staff,
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID, ok := entityIDFor(claimsFrom(r), r.PathValue("id"))
			if !ok {
				httpError(w, http.StatusNotFound, errors.New("restoran topilmadi"))
				return
			}
			waiterID := r.PathValue("waiterID")
			u, err := s.UserRepo.GetByID(r.Context(), waiterID)
			if err != nil || u.Role != users.RoleWaiter || u.EntityID != restaurantID {
				httpError(w, http.StatusNotFound, errors.New("affitsiant topilmadi"))
				return
			}
			// ┌─ O'CHIRISH EMAS, ROLNI QAYTARISH ────────────────────┐
			// Affitsiantni ishdan bo'shatish uning SHAXSIY
			// akkauntini yo'q qilmasligi kerak: o'sha odam bir
			// vaqtning o'zida oddiy mijoz ham bo'lishi mumkin va
			// uning buyurtmalar tarixi, manzili, sevimlilari
			// akkauntga bog'langan.
			//
			// Shuning uchun rol `customer` ga qaytariladi va
			// `EntityID` tozalanadi — restoran ma'lumotlariga
			// kirish shu zahoti yopiladi.
			// └───────────────────────────────────────────────────────┘
			if err := s.UserRepo.UpdateRole(r.Context(), waiterID, users.RoleCustomer, ""); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			// Rolni bazada o'zgartirish YETARLI EMAS: `auth()` faqat
			// imzoni tekshiradi va ESKI tokendagi rol `waiter` bo'lib
			// qolaveradi — u 30 kun ishlayverardi. Sessiyani bekor
			// qilish uni darhol to'xtatadi (restoran o'chirilganda
			// ham xuddi shu qadam qo'yiladi — routes_admin.go).
			s.Revoked.Revoke(r.Context(), waiterID)
			writeJSON(w, http.StatusOK, map[string]bool{"removed": true})
		}))
}

// waitersOf — restoranning affitsiantlari.
//
// `ListByRole` + filtr: bitta restoranda affitsiantlar soni o'nlab,
// shuning uchun alohida repository metodi va migratsiya shart emas
// (`restaurantPhoneFor` bilan bir xil mulohaza — authz.go).
func (s *Server) waitersOf(r *http.Request, restaurantID string) ([]*users.User, error) {
	all, err := s.UserRepo.ListByRole(r.Context(), users.RoleWaiter)
	if err != nil {
		return nil, err
	}
	out := make([]*users.User, 0, 4)
	for _, u := range all {
		if u.EntityID == restaurantID {
			out = append(out, u)
		}
	}
	return out, nil
}

// tableWithQR — stolga QR havolasini qo'shib qaytaradi.
//
// ┌─ QR ICHIDA NIMA BO'LADI ──────────────────────────────────────────┐
//
//	https://t.me/<bot>/<short_name>?startapp=<token>
//
// `<short_name>` — BotFather'dagi Mini App qisqa nomi
// (`miniAppShortName`, hozir `ondex`). U BotFather'dagi nom bilan
// AYNAN mos bo'lishi shart, aks holda havola bot profilini ochadi
// va stol oqimi umuman boshlanmaydi.
//
// Kamera shu havolani ochadi → Telegram → Mini App. Telegram
// `startapp` qiymatini `initData` ning IMZOLANGAN qismiga
// (`start_param`) joylaydi, ya'ni server uni qalbakilashtirib
// bo'lmaydigan manba sifatida qabul qiladi.
//
// Token AYNAN shu javobda beriladi (`Table.QRToken` da `json:"-"`
// turadi) — chunki bu endpoint faqat restoran egasiga ochiq va u
// tokenni QR chop etish uchun bilishi SHART.
// └───────────────────────────────────────────────────────────────────┘
func (s *Server) tableWithQR(r *http.Request, t *tables.Table) map[string]any {
	out := map[string]any{
		"id":            t.ID,
		"restaurant_id": t.RestaurantID,
		"zone":          t.Zone,
		"label":         t.Label,
		"active":        t.Active,
		"created_at":    t.CreatedAt,
		"qr_token":      t.QRToken,
	}
	if s.Telegram != nil {
		if bot, err := s.Telegram.BotUsername(r.Context()); err == nil && bot != "" {
			out["qr_link"] = fmt.Sprintf("https://t.me/%s/%s?startapp=%s",
				bot, miniAppShortName(), t.QRToken)
		}
	}
	return out
}

// miniAppShortName — Mini App'ning BotFather'dagi qisqa nomi
// (`t.me/<bot>/<short_name>`).
//
// ┌─ NEGA SOZLAMA, QATTIQ YOZILGAN QIYMAT EMAS ───────────────────────┐
// Bu yerda avval `app` QATTIQ yozilgan edi, BotFather'da esa ilova
// `ondex` nomi bilan yaratilgan. Natijada har bir stol QR kodi
// MAVJUD BO'LMAGAN manzilga ishora qilardi: havola ochilardi-yu,
// Mini App o'rniga oddiy bot profili chiqardi va stol oqimi
// boshlanmasdi. Xato hech qayerda ko'rinmasdi — server ham,
// Telegram ham xato bermaydi, chunki havola sintaktik jihatdan
// to'g'ri.
//
// Qisqa nomni BotFather'da o'zgartirib bo'lmaydi (faqat o'chirib
// qayta yaratish), shuning uchun moslashish SHU tomonda.
//
// Standart qiymat ATAYLAB haqiqiy production nomi: agar o'zgaruvchi
// compose'ning `environment:` ro'yxatiga qo'shilmay qolsa
// (`GOOGLE_MAPS_API_KEY` bilan aynan shunday bo'lgan), tizim baribir
// to'g'ri ishlaydi.
// └───────────────────────────────────────────────────────────────────┘
func miniAppShortName() string {
	if v := strings.TrimSpace(os.Getenv("TELEGRAM_MINIAPP_SHORT_NAME")); v != "" {
		return v
	}
	return "ondex"
}
