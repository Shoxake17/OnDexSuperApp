// Affitsiant ilovasining API yuzasi.
//
// ┌─ NEGA ALOHIDA ENDPOINT ───────────────────────────────────────────┐
// Affitsiant `GET /restaurants/{id}/orders` dan foydalana OLMAYDI: u
// restoranning HAMMA buyurtmalarini (yetkazish ham) va har biriga
// mijoz telefon raqamini qo'shib qaytaradi. Affitsiantga bularning
// hech biri kerak emas.
//
// Bu yerda esa faqat kerakli minimum: qaysi stol, nechta kishi, nima
// buyurtma qilingan va holati. Mijoz telefoni, manzili, koordinatasi
// UMUMAN yo'q — eng kam huquq (least privilege) prinsipi.
// └───────────────────────────────────────────────────────────────────┘
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"chustapp/internal/catalog"
	"chustapp/internal/orders"
	"chustapp/internal/tables"
	"chustapp/internal/users"
)

const (
	// maxWaiterOrderBody — affitsiant buyurtmasi so'rovi tanasining
	// chegarasi. 100 tur taom ham bir necha KB; katta tana faqat
	// xotirani band qilish urinishi bo'ladi.
	maxWaiterOrderBody = 64 << 10
	// maxWaiterIdempotencyKey — mijoz ilovasidagi bilan bir xil chegara.
	maxWaiterIdempotencyKey = 128
)

// waiterOrderView — affitsiant KO'RADIGAN maydonlar.
//
// Ataylab `orders.Order` ni to'g'ridan-to'g'ri qaytarmaymiz: unda
// `CustomerID`, `DeliveryLat/Lng`, `DeliveryAddress` bor va yangi
// maydon qo'shilganda u avtomatik ravishda affitsiantga ham
// ko'rinib ketardi. Aniq ro'yxat esa "sukut bo'yicha yopiq".
func waiterOrderView(o *orders.Order) map[string]any {
	return map[string]any{
		"id":           o.ID,
		"order_number": o.OrderNumber,
		"table_id":     o.TableID,
		"table_label":  o.TableLabel,
		"party_size":   o.PartySize,
		"items":        o.Items,
		"total_tiyin":  o.TotalTiyin,
		"status":       o.Status,
		"created_at":   o.CreatedAt,
		"updated_at":   o.UpdatedAt,
		"ready_at":     o.ReadyAt,
		// Buyurtmani affitsiant kiritganmi (mijoz QR orqali emas). KIM
		// kiritgani (foydalanuvchi ID'si) ATAYLAB berilmaydi.
		"placed_by_waiter": o.PlacedBy != "",
	}
}

// waiterTableView — affitsiant KO'RADIGAN joy maydonlari.
//
// ┌─ QR TOKEN YO'Q ───────────────────────────────────────────────────┐
// Restoran javobidagi (`tableWithQR`) `qr_token` va `qr_link` bu yerda
// UMUMAN yo'q. Token — sir: uni bilgan odam o'sha stol nomidan
// istalgan joydan buyurtma bera oladi. Affitsiantga u kerak emas —
// buyurtmani u o'z sessiyasi bilan, stol ID'si orqali kiritadi va
// server stol shu restoranga tegishliligini o'zi tekshiradi.
// └───────────────────────────────────────────────────────────────────┘
func waiterTableView(t *tables.Table, status tables.Status, activeOrders int) map[string]any {
	kind := t.Kind.Normalized()
	return map[string]any{
		"id":            t.ID,
		"zone":          t.Zone,
		"kind":          kind,
		"kind_title":    kind.Title(),
		"label":         t.Label,
		"display_label": t.DisplayLabel(),
		"capacity":      t.Capacity,
		"active":        t.Active,
		"status":        status,
		"active_orders": activeOrders,
	}
}

func (s *Server) registerWaiterRoutes(mux *http.ServeMux) {
	// GET /waiter/orders — affitsiant ilovasining asosiy ro'yxati.
	//
	// Qaytadi: shu restoranning FAOL stol buyurtmalari. Yakunlangan
	// (served/cancelled/rejected) buyurtmalar ro'yxatni to'ldirib
	// yubormasligi uchun chiqarib tashlanadi — affitsiantga "hozir
	// nima qilish kerak" ko'rinishi muhim.
	mux.HandleFunc("GET /waiter/orders", s.auth([]users.Role{users.RoleWaiter},
		func(w http.ResponseWriter, r *http.Request) {
			claims := claimsFrom(r)
			if claims.EntityID == "" {
				httpError(w, http.StatusForbidden,
					errors.New("akkaunt hech qaysi restoranga biriktirilmagan"))
				return
			}
			list, err := s.OrderRepo.ListByRestaurant(r.Context(), claims.EntityID, 100)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			out := make([]map[string]any, 0, len(list))
			for _, o := range list {
				if !o.IsDineIn() || o.IsTerminal() {
					continue
				}
				out = append(out, waiterOrderView(o))
			}
			writeJSON(w, http.StatusOK, out)
		}))

	// GET /waiter/tables — restoranning BARCHA joylari va jonli holati
	// (bo'sh/band/tozalanmoqda/yopiq). Restoran ID'si FAQAT tokendan
	// olinadi — affitsiant boshqa restoran joylarini so'ray olmaydi.
	mux.HandleFunc("GET /waiter/tables", s.auth([]users.Role{users.RoleWaiter},
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID := claimsFrom(r).EntityID
			if restaurantID == "" {
				httpError(w, http.StatusForbidden,
					errors.New("akkaunt hech qaysi restoranga biriktirilmagan"))
				return
			}
			if s.TableSvc == nil {
				httpError(w, http.StatusServiceUnavailable, errors.New("stol buyurtmalari sozlanmagan"))
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
				active := occ.active[t.ID]
				var last *tables.OrderSnapshot
				if o, ok := occ.latest[t.ID]; ok {
					last = &o
				}
				out = append(out, waiterTableView(t, tables.StatusOf(t, active, last), len(active)))
			}
			writeJSON(w, http.StatusOK, out)
		}))

	// POST /waiter/orders — affitsiant stolda turgan mehmon uchun
	// buyurtma kiritadi.
	//
	//	{"table_id":"...","party_size":3,"items":[{"product_id":"...","qty":2}],
	//	 "idempotency_key":"..."}
	//
	// ┌─ XAVFSIZLIK QOIDALARI ────────────────────────────────────────────┐
	//  1. Rol — faqat affitsiant; ishdan bo'shatilgan xodimning sessiyasi
	//     bekor qilinadi va roli qaytariladi (`staff.Accounts`), ya'ni
	//     u bu yerga yeta olmaydi.
	//  2. Restoran — FAQAT tokendagi `EntityID`. Stol ham, savatdagi
	//     HAR BIR taom ham shu restoranniki bo'lishi shart: aks holda
	//     affitsiant boshqa restoran oshxonasiga buyurtma yubora olardi.
	//  3. Narx — mijoz yo'lidagi kabi FAQAT serverdagi katalogdan
	//     (`CatalogSvc.PriceOrder`), ilova yuborgan summa ishlatilmaydi.
	//  4. Yopiq joyga (ta'mir, mavsumiy ayvon) buyurtma kiritilmaydi.
	//  5. `idempotency_key` MAJBURIY: zalda tarmoq zaif, affitsiant javob
	//     kelmasa qayta bosadi — oshxonaga ikkita bir xil buyurtma
	//     tushmasligi kerak. Kalit xodim ID'si bilan birlashtiriladi, ya'ni
	//     ikki affitsiantning kalitlari hech qachon to'qnashmaydi.
	// └───────────────────────────────────────────────────────────────────┘
	mux.HandleFunc("POST /waiter/orders", s.auth([]users.Role{users.RoleWaiter},
		func(w http.ResponseWriter, r *http.Request) {
			claims := claimsFrom(r)
			restaurantID := claims.EntityID
			if restaurantID == "" {
				httpError(w, http.StatusForbidden,
					errors.New("akkaunt hech qaysi restoranga biriktirilmagan"))
				return
			}
			if s.TableSvc == nil {
				httpError(w, http.StatusServiceUnavailable, errors.New("stol buyurtmalari sozlanmagan"))
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxWaiterOrderBody)
			var req struct {
				TableID            string                `json:"table_id"`
				PartySize          int                   `json:"party_size"`
				Items              []catalog.ItemRequest `json:"items"`
				IdempotencyKey     string                `json:"idempotency_key"`
				ExpectedTotalTiyin int64                 `json:"expected_total_tiyin"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, errors.New("so'rov noto'g'ri"))
				return
			}
			key := strings.TrimSpace(req.IdempotencyKey)
			if key == "" || len(key) > maxWaiterIdempotencyKey {
				httpError(w, http.StatusBadRequest, errors.New("idempotency_key majburiy (1-128 belgi)"))
				return
			}
			if req.PartySize < 0 || req.PartySize > maxPartySize {
				httpError(w, http.StatusBadRequest, errors.New("mehmonlar soni noto'g'ri"))
				return
			}

			table, err := s.TableSvc.Get(r.Context(), strings.TrimSpace(req.TableID))
			if err != nil || table.RestaurantID != restaurantID {
				// Begona va mavjud bo'lmagan stol — BIR XIL javob: boshqa
				// restoran stol ID'larining mavjudligi oshkor bo'lmasin.
				httpError(w, http.StatusNotFound, tables.ErrNotFound)
				return
			}
			if !table.Active {
				httpError(w, http.StatusBadRequest, errors.New("bu joy vaqtincha yopilgan"))
				return
			}

			pricedRestaurant, items, err := s.CatalogSvc.PriceOrder(r.Context(), req.Items)
			if err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			if pricedRestaurant != restaurantID {
				httpError(w, http.StatusBadRequest, errors.New("savatdagi taomlar bu restoranga tegishli emas"))
				return
			}

			o := orders.Order{
				RestaurantID: restaurantID,
				Items:        items,
				Type:         orders.TypeDineIn,
				TableID:      table.ID,
				TableLabel:   table.DisplayLabel(),
				PartySize:    req.PartySize,
				PlacedBy:     claims.Subject,
				// Mijoz akkaunti yo'q, shuning uchun kalit xodimga
				// bog'lanadi (`customer_id` bo'sh qatorlar orasida
				// to'qnashmasin).
				IdempotencyKey: "waiter:" + claims.Subject + ":" + key,
			}
			// To'lov joyida — xodimning qo'liga (naqd yoki terminal).
			// Restoranning "To'lov usullari" sozlamasi mijozning O'ZI
			// rasmiylashtiradigan buyurtmasi uchun; xodim buyurtmasida
			// onlayn oldindan to'lov bosqichi umuman yo'q.
			setPaymentMethod(&o, orders.PaymentCash)

			created, err := s.OrderSvc.CreateExpecting(r.Context(), &o, req.ExpectedTotalTiyin)
			if err != nil {
				var changed *orders.TotalChangedError
				if errors.As(err, &changed) {
					writeJSON(w, http.StatusConflict, map[string]any{
						"error":       "narx o'zgardi — yangi summani tasdiqlang",
						"code":        "total_changed",
						"total_tiyin": changed.ActualTiyin,
					})
					return
				}
				httpError(w, http.StatusBadRequest, err)
				return
			}
			// Takroriy so'rov (bir xil kalit) eski buyurtmani qaytaradi —
			// u boshqa stolga tegishli bo'lsa, bu kalit qayta ishlatilgan.
			if created.TableID != table.ID || created.PlacedBy != claims.Subject {
				httpError(w, http.StatusConflict, errors.New("bu idempotency_key boshqa buyurtmada ishlatilgan"))
				return
			}
			writeJSON(w, http.StatusCreated, waiterOrderView(created))
		}))

	// GET /waiter/me — affitsiant qaysi restoranda ishlashini bilishi
	// uchun (ilova sarlavhasida restoran nomi ko'rsatiladi).
	mux.HandleFunc("GET /waiter/me", s.auth([]users.Role{users.RoleWaiter},
		func(w http.ResponseWriter, r *http.Request) {
			claims := claimsFrom(r)
			resp := map[string]any{
				"restaurant_id": claims.EntityID,
			}
			if rest, err := s.CatalogRepo.GetRestaurant(r.Context(), claims.EntityID); err == nil {
				resp["restaurant_name"] = rest.Name
				resp["restaurant_logo_url"] = rest.LogoURL
			}
			writeJSON(w, http.StatusOK, resp)
		}))
}
