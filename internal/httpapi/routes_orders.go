package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"chustapp/internal/catalog"
	"chustapp/internal/delivery"
	"chustapp/internal/orders"
	"chustapp/internal/tables"
	"chustapp/internal/users"
)

// maxPartySize — "nechta kishi" uchun yuqori chegara.
//
// Bu maydon narxga ta'sir qilmaydi, lekin restoran panelida va
// affitsiant ilovasida ko'rsatiladi. Cheklovsiz bo'lsa, mijoz
// 2000000000 yozib qo'yishi va o'sha ekranlarni buzishi mumkin edi.
// 50 — eng katta banket stoli uchun ham yetarli.
const maxPartySize = 50

// createDineInOrder — stoldagi QR kod orqali berilgan buyurtma.
//
// ┌─ YETKAZISHDAN FARQI ──────────────────────────────────────────────┐
//   - manzil so'ralmaydi va xizmat hududi tekshirilmaydi (mijoz
//     restoranning O'ZIDA o'tiribdi);
//   - kuryer dispatch'i ISHGA TUSHMAYDI;
//   - to'lov NAQD bo'lsa affitsiantga to'lanadi; karta tanlansa
//     yetkazishdagi bilan bir xil oqim (oldindan to'lov).
//
// └───────────────────────────────────────────────────────────────────┘
func (s *Server) createDineInOrder(
	w http.ResponseWriter, r *http.Request,
	items []catalog.ItemRequest, tableToken string, partySize int, idempotencyKey string,
	paymentMethod orders.PaymentMethod,
) {
	if s.TableSvc == nil {
		httpError(w, http.StatusServiceUnavailable,
			errors.New("stol buyurtmalari sozlanmagan"))
		return
	}
	table, err := s.TableSvc.Resolve(r.Context(), strings.TrimSpace(tableToken))
	if err != nil {
		switch {
		case errors.Is(err, tables.ErrInactive):
			httpError(w, http.StatusBadRequest,
				errors.New("bu stol vaqtincha faol emas — xodimga murojaat qiling"))
		default:
			// Noto'g'ri token va mavjud bo'lmagan token — BIR XIL javob.
			// Farqlansa, tokenlarni birma-bir sinab ko'rish (enumeration)
			// osonlashardi.
			httpError(w, http.StatusBadRequest,
				errors.New("QR kod yaroqsiz — qaytadan skanerlang"))
		}
		return
	}

	if partySize < 0 || partySize > maxPartySize {
		httpError(w, http.StatusBadRequest,
			fmt.Errorf("odamlar soni 1 dan %d gacha bo'lishi kerak", maxPartySize))
		return
	}

	restaurantID, priced, err := s.CatalogSvc.PriceOrder(r.Context(), items)
	if err != nil {
		httpError(w, http.StatusBadRequest, err)
		return
	}

	// ┌─ ★ ASOSIY XAVFSIZLIK TEKSHIRUVI ──────────────────────────────┐
	// Savat MIJOZDAN keladi, stol esa IMZOLANGAN tokendan. Ikkalasi
	// bir xil restoranga tegishli ekani tekshirilmasa, "A" restorani
	// stolida o'tirgan odam "B" restoranining taomlarini buyurtma
	// qilardi: buyurtma B'ning oshxonasiga tushardi, lekin A'ning
	// affitsianti uni "5-stol" deb ko'rardi.
	// └───────────────────────────────────────────────────────────────┘
	if restaurantID != table.RestaurantID {
		httpError(w, http.StatusBadRequest,
			errors.New("savatdagi taomlar bu restoranga tegishli emas"))
		return
	}
	// Restoran "To'lov usullari" sozlamasi — serverda majburiy.
	if err := s.CatalogSvc.CheckPayment(r.Context(), restaurantID, paymentMethod); err != nil {
		httpError(w, http.StatusBadRequest, err)
		return
	}

	o := orders.Order{
		CustomerID:     claimsFrom(r).Subject,
		RestaurantID:   restaurantID,
		Items:          priced,
		Type:           orders.TypeDineIn,
		TableID:        table.ID,
		TableLabel:     table.DisplayLabel(),
		PartySize:      partySize,
		IdempotencyKey: idempotencyKey,
		// DeliveryLat/Lng va DeliveryAddress ATAYLAB bo'sh: stol
		// buyurtmasida yetkazish manzili degan tushuncha yo'q.
	}
	setPaymentMethod(&o, paymentMethod)
	created, err := s.OrderSvc.Create(r.Context(), &o)
	if err != nil {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) registerOrderRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /orders", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Items          []catalog.ItemRequest `json:"items"`
				IdempotencyKey string                `json:"idempotency_key"`
				// TableToken — stoldagi QR kod ichidagi sir. Bo'lsa,
				// bu STOL buyurtmasi (dine_in). Mijoz uni o'zi
				// yozmaydi — u Telegram tomonidan IMZOLANGAN
				// `initData` ichidagi `start_param` dan keladi
				// (routes_auth.go, TMA oqimi).
				TableToken string `json:"table_token"`
				// PartySize — nechta kishi. Faqat restoran uchun
				// ma'lumot (idish-tovoq, non, joy), narxga ta'sir
				// qilmaydi.
				PartySize int `json:"party_size"`
				// PaymentMethod — "cash" (standart) yoki "card".
				// Kartada buyurtma TO'LOV KUTIB turadi va restoranga
				// ko'rinmaydi; mijoz `POST /orders/{id}/pay` orqali
				// to'lov havolasini oladi.
				PaymentMethod string `json:"payment_method"`
				// ExpectedTotalTiyin — mijozga EKRANDA ko'rsatilgan
				// jami. Berilsa (>0) va hozirgi hisob undan farq
				// qilsa, buyurtma YARATILMAYDI va 409 qaytadi.
				//
				// Ixtiyoriy — eski klientlar buni yubormaydi va ular
				// uchun hech narsa o'zgarmaydi.
				ExpectedTotalTiyin int64 `json:"expected_total_tiyin"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			// Karta tanlangan bo'lsa-yu to'lov tizimi sozlanmagan
			// bo'lsa — buyurtma YARATILMAYDI. Aks holda u hech qachon
			// to'lanmaydigan holatda osilib qolardi.
			// Noma'lum to'lov usuli RAD ETILADI (bug.md 81-band):
			// avval u jimgina naqdga aylanardi va klientdagi xato
			// hech qayerda ko'rinmasdi.
			paymentMethod, err := paymentMethodFromRequest(req.PaymentMethod)
			if err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			if paymentMethod.RequiresPrepayment() && s.Payments == nil {
				httpError(w, http.StatusServiceUnavailable,
					errors.New("karta orqali to'lov hozircha mavjud emas"))
				return
			}
			if len(req.IdempotencyKey) > 128 {
				httpError(w, http.StatusBadRequest, errors.New("idempotency_key juda uzun"))
				return
			}

			if strings.TrimSpace(req.TableToken) != "" {
				s.createDineInOrder(w, r, req.Items, req.TableToken,
					req.PartySize, req.IdempotencyKey, paymentMethod)
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
				httpError(w, http.StatusBadRequest, errOutsideServiceArea)
				return
			}

			restaurantID, items, err := s.CatalogSvc.PriceOrder(r.Context(), req.Items)
			if err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			// Restoran va manzil BITTA shaharda bo'lishi shart: kuryer
			// restorandan 7 km ichida qidiriladi, ya'ni boshqa shahardagi
			// manzilga buyurtma hech qachon yetkazilmasdi.
			if err := s.checkRestaurantServes(r.Context(), restaurantID, addr.Lat, addr.Lng); err != nil {
				writeServesError(w, err)
				return
			}
			// Restoran "To'lov usullari" sozlamasi — serverda majburiy.
			if err := s.CatalogSvc.CheckPayment(r.Context(), restaurantID, paymentMethod); err != nil {
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
			o.Type = orders.TypeDelivery
			setPaymentMethod(&o, paymentMethod)
			created, err := s.OrderSvc.CreateExpecting(r.Context(), &o,
				req.ExpectedTotalTiyin)
			if err != nil {
				// Narx ekranda ko'rsatilgandan keyin o'zgargan —
				// buyurtma yaratilmadi. 409 (Conflict) va YANGI summa
				// qaytadi: ilova uni ko'rsatib qayta so'raydi.
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
			// Kuryer ismi — "Buyurtmalarim" ro'yxatida ham ID emas, ism
			// (faqat ismi, `courierPublicName`).
			courierNames := map[string]string{}
			out := make([]map[string]any, 0, len(list))
			for _, o := range list {
				rest, ok := restaurantCache[o.RestaurantID]
				if !ok {
					rest, _ = s.CatalogRepo.GetRestaurant(r.Context(), o.RestaurantID)
					restaurantCache[o.RestaurantID] = rest
				}
				entry := map[string]any{
					"id": o.ID,
					// Buyurtma raqami — "Buyurtmalarim" kartochkasining
					// tepasida ko'rsatiladi. Bu maydon YETISHMAS EDI va
					// mijoz ilovasi raqam o'rniga "—" chizardi.
					"order_number":   o.OrderNumber,
					"restaurant_id":  o.RestaurantID,
					"items":          o.Items,
					"subtotal_tiyin": o.SubtotalTiyin,
					"discount_tiyin": o.DiscountTiyin,
					"promotion_name": o.PromotionName,
					"total_tiyin":    o.TotalTiyin,
					"status":         o.Status,
					"courier_id":     o.CourierID,
					"created_at":     o.CreatedAt,
					// ┌─ BUYURTMA TURI ─────────────────────────────┐
					// Bu maydonlar YETISHMAS EDI va "Buyurtmalarim"
					// ro'yxati stol buyurtmasini yetkazish buyurtmasi
					// deb ko'rsatardi: `ready` holati uchun "Tayyor —
					// kuryer kutilmoqda", bosqichlar esa "Yo'lda" va
					// "Yetkazildi" (kuryer bu buyurtmaga umuman
					// chaqirilmagan bo'lsa ham).
					//
					// Tafsilot ekrani to'g'ri ishlardi, chunki u
					// `GET /orders/{id}` dan TO'LIQ obyektni oladi —
					// xato faqat shu qisqartirilgan ro'yxatda edi.
					//
					// `Type` bo'sh bo'lsa `delivery` demak
					// (`Type.Normalized()`), ya'ni eski buyurtmalar va
					// eski klientlar buzilmaydi.
					// └─────────────────────────────────────────────┘
					"type":        o.Type,
					"table_label": o.TableLabel,
					"party_size":  o.PartySize,
				}
				if rest != nil {
					entry["restaurant_name"] = rest.Name
					entry["restaurant_logo_url"] = rest.LogoURL
				}
				if o.CourierID != "" && !o.IsDineIn() && s.CourierRepo != nil {
					name, ok := courierNames[o.CourierID]
					if !ok {
						if c, err := s.CourierRepo.GetByID(r.Context(), o.CourierID); err == nil {
							name = courierPublicName(c.Name)
						}
						courierNames[o.CourierID] = name
					}
					entry["courier_name"] = name
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
			out := s.withCourierInfo(r.Context(), claims, o, redactForCourierBeforePickup(claims, o))
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
				// Tayyorlash vaqti IKKALA turda ham saqlanadi — mijoz
				// "taxminan 15 daqiqa" degan ma'lumotni stolda ham
				// ko'radi.
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
				// ┌─ DISPATCH FAQAT YETKAZISHDA ──────────────────────┐
				// Stol buyurtmasida kuryer umuman kerak emas. Bu
				// shart bo'lmasa, restoran "qabul qildim" bosishi
				// bilan dispatch ishga tushardi va u kuryer
				// topilmaguncha QAYTA-QAYTA urinaverardi
				// (couriers/dispatch.go) — mavjud bo'lmagan yetkazish
				// uchun kuryerlarni bezovta qilib, ularni "band"
				// holatiga o'tkazib qo'yardi.
				// └───────────────────────────────────────────────────┘
				if !o.IsDineIn() {
					oID, rID, prep := o.ID, o.RestaurantID, req.PreparationMinutes
					safeGo("dispatch:"+oID, func() { s.dispatchOrder(oID, rID, prep) })
				}
			}
			// Buyurtma yakunlandi — ochiq qidiruv va kuryerlardagi
			// takliflar DARHOL yopiladi (keyingi tsiklni kutmasdan).
			if o.IsTerminal() {
				s.stopDispatch(o.ID)
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

	// POST /orders/{id}/dispatch/retry — "Kuryer topilmadi" holatida
	// restoran kuryer qidiruvini QAYTA boshlaydi (yangi muddat bilan).
	//
	// Bekor qilish uchun alohida endpoint YO'Q: oddiy
	// `transition {"to":"cancelled"}` — holat mashinasi tayyor yetkazish
	// buyurtmasini restoranga FAQAT shu holatda bekor qildiradi
	// (`orders.ValidateOrderTransition`). Ikki yo'l bo'lsa, qoidalar
	// ajralib ketardi.
	mux.HandleFunc("POST /orders/{id}/dispatch/retry", s.auth([]users.Role{users.RoleRestaurant, users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			o, err := s.OrderSvc.Get(r.Context(), r.PathValue("id"))
			if err != nil {
				if errors.Is(err, orders.ErrNotFound) {
					httpError(w, http.StatusNotFound, orders.ErrNotFound)
					return
				}
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			if !ownsOrderAction(claimsFrom(r), o) {
				// 404 — begona buyurtmaning mavjudligi oshkor bo'lmasin
				// (GET /orders/{id} dagi izoh).
				httpError(w, http.StatusNotFound, orders.ErrNotFound)
				return
			}
			o, err = s.OrderSvc.RestartCourierSearch(r.Context(), o.ID)
			switch {
			case errors.Is(err, orders.ErrDispatchNotRestartable), errors.Is(err, orders.ErrConflict):
				// Holat shu oraliqda o'zgargan (kuryer topildi, bekor
				// qilindi, boshqa xodim allaqachon bosdi) — panel
				// ro'yxatni yangilaydi.
				httpError(w, http.StatusConflict, err)
				return
			case err != nil:
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			s.launchDispatch(o)
			s.publishDispatchState("dispatch_state", o)
			writeJSON(w, http.StatusOK, o)
		}))
}
