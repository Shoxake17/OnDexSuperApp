package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"chustapp/internal/catalog"
	"chustapp/internal/delivery"
	"chustapp/internal/orders"
	"chustapp/internal/users"
)

func (s *Server) registerOrderRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /orders", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Items          []catalog.ItemRequest `json:"items"`
				IdempotencyKey string                `json:"idempotency_key"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			if len(req.IdempotencyKey) > 128 {
				httpError(w, http.StatusBadRequest, errors.New("idempotency_key juda uzun"))
				return
			}

			// MANZIL MIJOZDAN OLINMAYDI — serverdagi saqlangan manzildan
			// o'qiladi. Sabab: avval `delivery_lat/lng` so'rov tanasidan
			// kelardi, ya'ni mijoz istalgan koordinatani (hatto xizmat
			// hududidan tashqarisini) yuborishi mumkin edi va kiritilgan
			// podyezd/kvartira/izoh buyurtmaga UMUMAN qo'shilmasdi —
			// kuryer ularni ko'rmasdi. Endi manba bitta: /me/address.
			u, err := s.UserRepo.GetByID(r.Context(), claimsFrom(r).Subject)
			if err != nil {
				httpError(w, http.StatusBadRequest, errors.New("foydalanuvchi topilmadi"))
				return
			}
			addr := u.Address
			if addr.Lat == 0 && addr.Lng == 0 {
				httpError(w, http.StatusBadRequest,
					errors.New("avval yetkazib berish manzilini tanlang"))
				return
			}
			// XIZMAT HUDUDI — ishonchli tekshiruv AYNAN shu yerda.
			// Frontenddagi tekshiruv faqat qulaylik uchun; API to'g'ridan
			// -to'g'ri chaqirilsa ham hudud tashqarisiga buyurtma
			// yaratilmaydi.
			if !delivery.Covered(addr.Lat, addr.Lng) {
				httpError(w, http.StatusBadRequest,
					errors.New("bu manzilga hozircha yetkazmaymiz — faqat Chust shahri"))
				return
			}

			restaurantID, items, err := s.CatalogSvc.PriceOrder(r.Context(), req.Items)
			if err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			o := orders.Order{
				CustomerID:   claimsFrom(r).Subject,
				RestaurantID: restaurantID,
				Items:        items,
				DeliveryLat:  addr.Lat,
				DeliveryLng:  addr.Lng,
				// Buyurtma vaqtidagi NUSXA — mijoz keyin profil manzilini
				// o'zgartirsa ham kuryer aynan shu ma'lumotni ko'radi.
				DeliveryAddress: orders.Address{
					Text:      addr.Text,
					Entrance:  addr.Entrance,
					Floor:     addr.Floor,
					Apartment: addr.Apartment,
					Intercom:  addr.Intercom,
					Comment:   addr.Comment,
				},
				IdempotencyKey: req.IdempotencyKey,
			}
			created, err := s.OrderSvc.Create(r.Context(), &o)
			if err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, created)
		}))

	// POST /restaurants/{id}/quote — mijoz ilovasi checkout'dan OLDIN
	// (savat o'zgarganda) chaqiradigan HAQIQIY narxlash — real buyurtma
	// yaratmaydi, faqat oldindan ko'rsatadi. orders.Service.Quote AYNAN
	// Create() bilan bir xil priceCart() funksiyasini chaqiradi, shuning
	// uchun bu yerda ko'rsatilgan raqam buyurtma yaratilganda haqiqatan
	// yozilgan raqam bilan HAR DOIM bir xil.
	mux.HandleFunc("POST /restaurants/{id}/quote", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID := r.PathValue("id")
			var req struct {
				Items []catalog.ItemRequest `json:"items"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			pricedRestaurantID, items, err := s.CatalogSvc.PriceOrder(r.Context(), req.Items)
			if err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			if pricedRestaurantID != restaurantID {
				httpError(w, http.StatusBadRequest, errors.New("mahsulotlar boshqa restoranga tegishli"))
				return
			}
			quote, err := s.OrderSvc.Quote(r.Context(), restaurantID, items, claimsFrom(r).Subject)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, quote)
		}))

	// GET /me/orders — login qilgan foydalanuvchining o'z buyurtmalari
	// tarixi ("Buyurtmalarim" bo'limi). Har biriga restoran nomi/logotipi
	// qo'shib beriladi (ProductSearchResult'dagi kabi enrichment naqshi) —
	// aks holda mijoz ilovasi har bir buyurtma uchun alohida restoran
	// so'rovi yuborishga majbur bo'lardi.
	mux.HandleFunc("GET /me/orders", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			list, err := s.OrderRepo.ListByCustomer(r.Context(), claimsFrom(r).Subject, 50)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			restaurantCache := map[string]*catalog.Restaurant{}
			out := make([]map[string]any, 0, len(list))
			for _, o := range list {
				rest, ok := restaurantCache[o.RestaurantID]
				if !ok {
					rest, _ = s.CatalogRepo.GetRestaurant(r.Context(), o.RestaurantID)
					restaurantCache[o.RestaurantID] = rest
				}
				entry := map[string]any{
					"id":             o.ID,
					"restaurant_id":  o.RestaurantID,
					"items":          o.Items,
					"subtotal_tiyin": o.SubtotalTiyin,
					"discount_tiyin": o.DiscountTiyin,
					"promotion_name": o.PromotionName,
					"total_tiyin":    o.TotalTiyin,
					"status":         o.Status,
					"courier_id":     o.CourierID,
					"created_at":     o.CreatedAt,
				}
				if rest != nil {
					entry["restaurant_name"] = rest.Name
					entry["restaurant_logo_url"] = rest.LogoURL
				}
				out = append(out, entry)
			}
			writeJSON(w, http.StatusOK, out)
		}))

	// GET /orders/{id} — faqat aloqador tomonlar ko'ra oladi
	mux.HandleFunc("GET /orders/{id}", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			claims := claimsFrom(r)
			o, err := s.OrderSvc.Get(r.Context(), r.PathValue("id"))
			if err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			if !canSeeOrder(claims, o) {
				// ATAYLAB 404 (403 emas): aks holda javob kodi
				// buyurtma MAVJUDLIGINI oshkor qilardi — begona ID
				// uchun 403, yo'q ID uchun 404. Bu hujumchiga
				// mavjud buyurtma ID'larini ajratib olish imkonini
				// beruvchi "oracle" edi. Endi ikkala holat ham bir
				// xil ko'rinadi.
				httpError(w, http.StatusNotFound, orders.ErrNotFound)
				return
			}
			out := redactForCourierBeforePickup(claims, o)
			// restaurant_phone/customer_phone — kuryer ilovasida "Qo'ng'iroq
			// qilish" tugmalari uchun. FAQAT kuryer/admin'ga qo'shiladi
			// (restoran o'zi, mijoz esa hozircha bu funksiyani so'ramagan) va
			// FAQAT shu buyurtma orqali (`canSeeOrder` allaqachon egalikni
			// tekshirgan) — hech qaysi raqam biror ochiq endpointda UMUMAN
			// yo'q, faqat shu kontekstda ko'rinadi.
			if claims.Role == users.RoleCourier || claims.Role == users.RoleAdmin {
				if phone := restaurantPhoneFor(r.Context(), s.UserRepo, o.RestaurantID); phone != "" {
					out = withExtraField(out, "restaurant_phone", phone)
				}
				// customer_phone FAQAT picked_up bosqichidan keyin (kuryer
				// uchun) — xuddi taomlar/mijoz manzili kabi, kuryer
				// restoranga bormasdan mijoz bilan bog'lanmasin degan
				// bir xil xavfsizlik falsafasi. Admin uchun bosqichdan
				// qat'i nazar (u redaksiyaga umuman uchramaydi).
				redactedForCourier := claims.Role == users.RoleCourier &&
					(o.Status == orders.StatusAccepted || o.Status == orders.StatusPreparing || o.Status == orders.StatusReady)
				if !redactedForCourier {
					if phone := customerPhoneFor(r.Context(), s.UserRepo, o.CustomerID); phone != "" {
						out = withExtraField(out, "customer_phone", phone)
					}
				}
			}
			writeJSON(w, http.StatusOK, out)
		}))

	// Eslatma: alohida "pickup-code"/"verify-pickup" endpointlari endi
	// YO'Q — kuryer "ready" -> "picked_up" o'tishini oddiy
	// `/orders/{id}/transition {"to":"picked_up"}` orqali TO'G'RIDAN-TO'G'RI
	// o'zi qiladi (pastga, statemachine.go'dagi StatusReady qoidasiga
	// qarang). Buyurtmani olib ketishda kuryer restoran xodimiga buyurtma
	// raqamining OXIRGI 4 xonasini og'zaki aytadi — dastur darajasida
	// tekshirilmaydi.

	// POST /orders/{id}/transition  {"to":"accepted","preparation_minutes":20}
	// aktor tokendagi roldan aniqlanadi. "accepted"ga o'tishda
	// preparation_minutes MAJBURIY — dispatch shu payt AVTOMATIK ishga
	// tushadi (qo'lda "Kuryer chaqirish" tugmasisiz).
	mux.HandleFunc("POST /orders/{id}/transition", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				To                 orders.Status `json:"to"`
				PreparationMinutes int           `json:"preparation_minutes"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			if req.To == orders.StatusAccepted && req.PreparationMinutes <= 0 {
				httpError(w, http.StatusBadRequest,
					errors.New("preparation_minutes (tayyorlash vaqti, daqiqada) majburiy va musbat bo'lishi kerak"))
				return
			}
			claims := claimsFrom(r)
			o, err := s.OrderSvc.Get(r.Context(), r.PathValue("id"))
			if err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			if !ownsOrderAction(claims, o) {
				// 404 — qarang: GET /orders/{id}dagi izoh (mavjudlik
				// oshkor bo'lmasligi uchun).
				httpError(w, http.StatusNotFound, orders.ErrNotFound)
				return
			}
			o, err = s.OrderSvc.ChangeStatus(r.Context(), o.ID, req.To, roleToActor(claims.Role))
			if err != nil {
				var terr *orders.TransitionError
				switch {
				case errors.As(err, &terr), errors.Is(err, orders.ErrConflict):
					// ErrConflict — optimistik parallel boshqaruv: shu
					// buyurtmani AYNAN shu payt boshqa so'rov ham
					// o'zgartirmoqchi bo'lgan va bir necha qayta urinishdan
					// keyin ham ziddiyat davom etgan (juda kamdan-kam).
					// 409 — mijoz/restoran/kuryer ilovasi holatni qayta
					// yuklab, qayta urinishi kerak.
					httpError(w, http.StatusConflict, err)
				default:
					httpError(w, http.StatusBadRequest, err)
				}
				return
			}
			if req.To == orders.StatusAccepted {
				if updated, perr := s.OrderSvc.SetPreparationTime(r.Context(), o.ID, req.PreparationMinutes); perr != nil {
					// Tayyorlash vaqtini saqlab bo'lmadi (masalan
					// optimistik qulf bir necha marta to'qnashdi).
					// MUHIM: dispatch baribir ISHGA TUSHIRILADI.
					//
					// Avval bu holatda `else` shoxi bajarilmasdi va
					// buyurtma "accepted" holatida, kuryersiz
					// ABADIY osilib qolardi — hech qanday qayta
					// urinish mexanizmi uni olib ketmasdi (restoran
					// panelida "qayta urinish" tugmasi yo'q).
					// `PreparationMinutes` faqat ETA moslashtirish
					// uchun kerak, dispatch uchun SHART emas —
					// shuning uchun so'rovdagi qiymat bilan davom
					// etamiz.
					slog.Error("tayyorlash vaqtini saqlashda xato — dispatch baribir boshlanadi",
						"order", o.ID, "err", perr)
				} else {
					o = updated
				}
				oID, rID, prep := o.ID, o.RestaurantID, req.PreparationMinutes
				safeGo("dispatch:"+oID, func() { s.dispatchOrder(oID, rID, prep) })
			}
			// Buyurtma yakunlandi — kuryer yana bo'sh
			if o.IsTerminal() && o.CourierID != "" {
				if err := s.CourierRepo.SetAvailable(r.Context(), o.CourierID, true); err != nil {
					slog.Error("kuryerni bo'shatishda xato", "courier", o.CourierID, "err", err)
				}
				// Muvaffaqiyatli yetkazilgan bo'lsa — tajriba hisoblagichi +1
				// (haqiqiy, obyektiv mezon; ScoreCandidates shundan foydalanadi).
				if o.Status == orders.StatusDelivered {
					if err := s.CourierRepo.IncrementCompletedOrders(r.Context(), o.CourierID); err != nil {
						slog.Error("tajriba hisoblagichini oshirishda xato", "courier", o.CourierID, "err", err)
					}
				}
			}
			writeJSON(w, http.StatusOK, o)
		}))
}
