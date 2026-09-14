// Joylar (stol, kabina, VIP xona... — QR kod) va affitsiantlar —
// restoran o'zi boshqaradigan resurslar.
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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

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

// recentScansWithOrders — "So'nggi skanerlangan QR kodlar" uchun oxirgi
// buyurtmasi qo'shiladigan joylar soni (panel ulardan 4-5 tasini
// ko'rsatadi; har biri bitta indeks qidiruvi).
const recentScansWithOrders = 10

// tableErrStatus — domen xatosini HTTP holatiga aylantiradi. Tanilmagan
// xato (baza, tarmoq) 500: foydalanuvchi xatosi deb 400 berilsa, haqiqiy
// nosozlik monitoringda ko'rinmay qolardi.
func tableErrStatus(err error) int {
	switch {
	case errors.Is(err, tables.ErrDuplicate):
		return http.StatusConflict
	case errors.Is(err, tables.ErrLimitReached):
		return http.StatusUnprocessableEntity
	case errors.Is(err, tables.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, tables.ErrEmptyLabel), errors.Is(err, tables.ErrLabelTooLong),
		errors.Is(err, tables.ErrEmptyZone), errors.Is(err, tables.ErrZoneTooLong),
		errors.Is(err, tables.ErrControlChars), errors.Is(err, tables.ErrBadCapacity),
		errors.Is(err, tables.ErrBadBatch), errors.Is(err, tables.ErrUnknownKind):
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
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
	// yopib yuborardi (butun zal QR kodlari bir zumda ishlamay qolardi).
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

	// ---------- Joylar ----------

	// GET /tables/kinds — joy turlari (stol, kabina, VIP xona...).
	mux.HandleFunc("GET /tables/kinds", s.auth(staff,
		func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, tables.Kinds())
		}))

	// GET /restaurants/{id}/tables — restoranning joylari, JONLI holati
	// bilan (band/bo'sh/tozalanmoqda — `tables.StatusOf`).
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
			occ, err := s.tableOccupancy(r.Context(), restaurantID, list)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			out := make([]map[string]any, 0, len(list))
			for _, t := range list {
				view := s.tableWithQR(r, t)
				active := occ.active[t.ID]
				if active == nil {
					active = []tables.OrderSnapshot{}
				}
				var last *tables.OrderSnapshot
				if o, ok := occ.latest[t.ID]; ok {
					last = &o
					view["last_order"] = o
				}
				view["status"] = tables.StatusOf(t, active, last)
				view["active_orders"] = active
				out = append(out, view)
			}
			writeJSON(w, http.StatusOK, out)
		}))

	// POST /restaurants/{id}/tables  {"label":"5","zone":"Ayvon","kind":"cabin","capacity":6}
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
				Label    string `json:"label"`
				Zone     string `json:"zone"`
				Kind     string `json:"kind"`
				Capacity *int   `json:"capacity"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			t, err := s.TableSvc.CreateTable(r.Context(), restaurantID, tables.Spec{
				Zone: req.Zone, Kind: req.Kind, Label: req.Label, Capacity: req.Capacity,
			})
			if err != nil {
				httpError(w, tableErrStatus(err), err)
				return
			}
			writeJSON(w, http.StatusCreated, s.tableWithQR(r, t))
		}))

	// POST /restaurants/{id}/tables/batch
	//   {"kind":"cabin","zone":"Asosiy zal","prefix":"","from":1,"count":10,"capacity":4}
	//
	// Bir nechta ketma-ket raqamli joy BIRDANIGA: hammasi yoki hech biri
	// (`tables.Service.CreateBatch`). Chegara — bitta so'rovda
	// `tables.MaxBatch`, restoranda jami `tables.MaxTablesPerRestaurant`.
	mux.HandleFunc("POST /restaurants/{id}/tables/batch", s.auth(staff,
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
				Zone     string `json:"zone"`
				Kind     string `json:"kind"`
				Prefix   string `json:"prefix"`
				Capacity *int   `json:"capacity"`
				From     int    `json:"from"`
				Count    int    `json:"count"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			created, err := s.TableSvc.CreateBatch(r.Context(), restaurantID, tables.BatchSpec{
				Zone: req.Zone, Kind: req.Kind, Prefix: req.Prefix,
				Capacity: req.Capacity, From: req.From, Count: req.Count,
			})
			if err != nil {
				httpError(w, tableErrStatus(err), err)
				return
			}
			out := make([]map[string]any, 0, len(created))
			for _, t := range created {
				out = append(out, s.tableWithQR(r, t))
			}
			writeJSON(w, http.StatusCreated, out)
		}))

	// PATCH /tables/{id}
	//   {"label":"6","zone":"Ayvon","kind":"cabin","capacity":8,"active":false,"cleaning":true}
	//
	// Hamma o'zgarish BITTA yozuvda (`tables.Service.Edit`): avval
	// maydonma-maydon saqlanardi va ikkinchisi xato bersa birinchisi
	// allaqachon yozilib qolardi. `"capacity": null` — sig'imni olib
	// tashlash.
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
				Label    *string         `json:"label"`
				Zone     *string         `json:"zone"`
				Kind     *string         `json:"kind"`
				Capacity json.RawMessage `json:"capacity"`
				Active   *bool           `json:"active"`
				Cleaning *bool           `json:"cleaning"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			patch := tables.Patch{
				Label: req.Label, Zone: req.Zone, Kind: req.Kind,
				Active: req.Active, Cleaning: req.Cleaning,
			}
			if len(req.Capacity) > 0 {
				if string(req.Capacity) == "null" {
					patch.ClearCapacity = true
				} else {
					var c int
					if err := json.Unmarshal(req.Capacity, &c); err != nil {
						httpError(w, http.StatusBadRequest, tables.ErrBadCapacity)
						return
					}
					patch.Capacity = &c
				}
			}
			updated, err := s.TableSvc.Edit(r.Context(), t.ID, patch)
			if err != nil {
				httpError(w, tableErrStatus(err), err)
				return
			}
			writeJSON(w, http.StatusOK, s.tableWithQR(r, updated))
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
			// Band joy o'chirilmaydi: undagi mehmonlarning buyurtmasi yo'q
			// joyga bog'lanib qolardi va QR yana ishlatilmay turib yo'qolardi.
			if s.TableOrders != nil {
				active, err := s.TableOrders.ActiveDineInOrders(r.Context(), t.RestaurantID)
				if err != nil {
					httpError(w, http.StatusInternalServerError, err)
					return
				}
				for _, o := range active {
					if o.TableID == t.ID {
						httpError(w, http.StatusConflict,
							errors.New("band joyni o'chirib bo'lmaydi — avval undagi buyurtmalarni yakunlang"))
						return
					}
				}
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
			// Skanerlash vaqti — panelning "So'nggi skanerlangan QR
			// kodlar" bloki uchun. Faqat MUVAFFAQIYATLI yechilgan token
			// yoziladi (soxta tokenlar hech narsa qoldirmaydi). Xato
			// mijozni to'xtatmaydi: bu yordamchi ma'lumot.
			_ = s.TableSvc.MarkScanned(r.Context(), t.ID)

			// Javobda token QAYTMAYDI — chaqiruvchi uni allaqachon
			// biladi, qaytarish esa uni loglarga/keshlarga yoyardi.
			resp := map[string]any{
				"table_id":      t.ID,
				"table_label":   t.DisplayLabel(),
				"table_number":  t.Label,
				"table_kind":    t.Kind.Normalized(),
				"zone":          t.Zone,
				"restaurant_id": t.RestaurantID,
			}
			if rest, err := s.CatalogRepo.GetRestaurant(r.Context(), t.RestaurantID); err == nil {
				resp["restaurant_name"] = rest.Name
				resp["restaurant_open"] = rest.Open
			}
			writeJSON(w, http.StatusOK, resp)
		}))

	// Affitsiantlar endi "Xodimlar" bo'limida boshqariladi
	// (`routes_staff.go`, `internal/staff`): ilovaga kirish xodim
	// yozuviga ergashadi va ta'til/ishdan bo'shatishda darhol yopiladi.
}

// tableOccupancy — joylar holati uchun buyurtmalar.
type tableOccupancy struct {
	active map[string][]tables.OrderSnapshot
	latest map[string]tables.OrderSnapshot
}

// tableOccupancy — faol stol buyurtmalari (hamma joy uchun bitta so'rov)
// va KERAKLI joylarning oxirgi buyurtmasi: "tozalanmoqda" belgisi
// borlar (belgi eskirganini bilish uchun) va eng so'nggi
// skanerlanganlar. `TableOrders` ulanmagan bo'lsa bo'sh — holatlar
// buyurtmasiz hisoblanadi.
func (s *Server) tableOccupancy(ctx context.Context, restaurantID string, list []*tables.Table) (tableOccupancy, error) {
	occ := tableOccupancy{
		active: map[string][]tables.OrderSnapshot{},
		latest: map[string]tables.OrderSnapshot{},
	}
	if s.TableOrders == nil || len(list) == 0 {
		return occ, nil
	}
	active, err := s.TableOrders.ActiveDineInOrders(ctx, restaurantID)
	if err != nil {
		return occ, err
	}
	for _, o := range active {
		occ.active[o.TableID] = append(occ.active[o.TableID], o)
	}

	seen := map[string]bool{}
	var ids []string
	for _, t := range list {
		if t.CleaningSince != nil {
			seen[t.ID] = true
			ids = append(ids, t.ID)
		}
	}
	scanned := make([]*tables.Table, 0, len(list))
	for _, t := range list {
		if t.LastScannedAt != nil {
			scanned = append(scanned, t)
		}
	}
	sort.Slice(scanned, func(i, j int) bool { return scanned[i].LastScannedAt.After(*scanned[j].LastScannedAt) })
	for i, t := range scanned {
		if i >= recentScansWithOrders {
			break
		}
		if !seen[t.ID] {
			seen[t.ID] = true
			ids = append(ids, t.ID)
		}
	}
	if len(ids) > 0 {
		latest, err := s.TableOrders.LatestDineInOrders(ctx, restaurantID, ids)
		if err != nil {
			return occ, err
		}
		occ.latest = latest
	}
	return occ, nil
}

// tableWithQR — joyga QR havolasini qo'shib qaytaradi.
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
// Havolaning o'zgaruvchan qismi faqat token — u esa abadiy. Ya'ni
// joyning nomi, turi, zali yoki sig'imi o'zgarsa ham QR tasviri
// piksel-piksel bir xil qoladi.
//
// Token AYNAN shu javobda beriladi (`Table.QRToken` da `json:"-"`
// turadi) — chunki bu endpoint faqat restoran egasiga ochiq va u
// tokenni QR chop etish uchun bilishi SHART.
// └───────────────────────────────────────────────────────────────────┘
func (s *Server) tableWithQR(r *http.Request, t *tables.Table) map[string]any {
	kind := t.Kind.Normalized()
	out := map[string]any{
		"id":              t.ID,
		"restaurant_id":   t.RestaurantID,
		"zone":            t.Zone,
		"kind":            kind,
		"kind_title":      kind.Title(),
		"label":           t.Label,
		"display_label":   t.DisplayLabel(),
		"capacity":        t.Capacity,
		"active":          t.Active,
		"cleaning_since":  t.CleaningSince,
		"last_scanned_at": t.LastScannedAt,
		"created_at":      t.CreatedAt,
		"qr_token":        t.QRToken,
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
