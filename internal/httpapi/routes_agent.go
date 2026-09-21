// Tashqi AI agentlar uchun API (`/agent/v1/...`).
//
// ┌─ NEGA ALOHIDA, IXCHAM YUZA ────────────────────────────────────────┐
// Agentga oddiy mijoz API'sini ochib qo'yish mumkin edi, lekin bu ikki
// sababdan noto'g'ri:
//
//  1. XAVFSIZLIK. Mijoz API'si keng: manzil o'zgartirish, parol
//     qo'yish, qurilma tokeni, to'lov havolasi. Agentga bularning
//     birortasi ham kerak emas. Yuza qanchalik kichik bo'lsa,
//     tekshirish shunchalik oson.
//
//  2. TOKEN NARXI. Javoblar TIL MODELIGA boradi va har bir ortiqcha
//     maydon pul turadi hamda modelni chalg'itadi. Shu sababli bu
//     yerda IXCHAM ko'rinishlar qaytariladi (`agentRestaurant`,
//     `agentProduct`) — to'liq katalog obyektlari emas.
//
// └────────────────────────────────────────────────────────────────────┘
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"chustapp/internal/agentapi"
	"chustapp/internal/catalog"
	"chustapp/internal/delivery"
	"chustapp/internal/notify"
	"chustapp/internal/orders"
)

// ── Ixcham ko'rinishlar (LLM uchun) ──

type agentRestaurant struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Open    bool    `json:"open"`
	Address string  `json:"address,omitempty"`
	Tags    string  `json:"tags,omitempty"`
	Rating  float64 `json:"rating,omitempty"`
	ETAMin  int     `json:"eta_min_minutes,omitempty"`
	ETAMax  int     `json:"eta_max_minutes,omitempty"`
}

func toAgentRestaurant(r *catalog.Restaurant) agentRestaurant {
	return agentRestaurant{
		ID: r.ID, Name: r.Name, Open: r.AcceptingOrdersNow(), Address: r.Address,
		Tags: r.Tags, Rating: r.Rating,
		ETAMin: r.ETAMinMinutes, ETAMax: r.ETAMaxMinutes,
	}
}

type agentProduct struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category,omitempty"`
	// PriceTiyin — mijoz HAQIQATDA to'laydigan narx (chegirma
	// qo'llanilgan). Asl narxni ham berish modelni chalkashtirardi:
	// u ikkitasidan qaysi birini aytishni bilmasdi.
	PriceTiyin  int64  `json:"price_tiyin"`
	Description string `json:"description,omitempty"`
}

// agentDescriptionMax — tavsif uzunligi chegarasi.
//
// To'liq tavsiflar menyu bo'yicha yig'ilganda modelning kontekstini
// to'ldirib yuboradi va javob sifatini pasaytiradi.
const agentDescriptionMax = 160

func toAgentProduct(p *catalog.Product) agentProduct {
	price := p.PriceTiyin
	if p.DiscountPriceTiyin > 0 && p.DiscountPriceTiyin < p.PriceTiyin {
		price = p.DiscountPriceTiyin
	}
	desc := p.Description
	if len([]rune(desc)) > agentDescriptionMax {
		desc = string([]rune(desc)[:agentDescriptionMax])
	}
	return agentProduct{
		ID: p.ID, Name: p.Name, Category: p.Category,
		PriceTiyin: price, Description: desc,
	}
}

type agentOrder struct {
	ID          string `json:"id"`
	OrderNumber string `json:"order_number"`
	Status      string `json:"status"`
	// StatusText — modelga TAYYOR o'zbekcha ibora. Model holat
	// kodlarini o'zi tarjima qilganda har safar boshqacha (va ba'zan
	// noto'g'ri) aytardi.
	StatusText   string `json:"status_text"`
	RestaurantID string `json:"restaurant_id"`
	TotalTiyin   int64  `json:"total_tiyin"`
	Type         string `json:"type,omitempty"`
	CreatedAt    string `json:"created_at"`
	// Cancellable — agent bekor qila oladimi. Modelga "urinib
	// ko'r" deyishdan ko'ra oldindan aytish arzonroq.
	Cancellable bool `json:"cancellable"`
}

func orderStatusTextUz(st orders.Status) string {
	switch st {
	case orders.StatusCreated:
		return "Restoran hali qabul qilmadi"
	case orders.StatusAccepted:
		return "Restoran qabul qildi"
	case orders.StatusPreparing:
		return "Tayyorlanmoqda"
	case orders.StatusReady:
		return "Tayyor"
	case orders.StatusPickedUp:
		return "Kuryer yo'lda"
	case orders.StatusDelivered:
		return "Yetkazildi"
	case orders.StatusRejected:
		return "Restoran rad etdi"
	case orders.StatusCancelled:
		return "Bekor qilindi"
	}
	return string(st)
}

// agentCancellable — mijoz (va demak uning nomidan agent) bekor qila
// oladigan holatlar.
//
// Ro'yxat ATAYLAB tor: taom tayyorlanib bo'lgach yoki kuryer olgach
// bekor qilish restoranning zararini keltirib chiqaradi. Haqiqiy
// tekshiruv baribir holat mashinasida (`orders`), bu yerdagisi faqat
// modelga oldindan xabar berish uchun.
func agentCancellable(o *orders.Order) bool {
	return o.Status == orders.StatusCreated || o.Status == orders.StatusAccepted
}

func toAgentOrder(o *orders.Order) agentOrder {
	return agentOrder{
		ID: o.ID, OrderNumber: o.OrderNumber, Status: string(o.Status),
		StatusText: orderStatusTextUz(o.Status), RestaurantID: o.RestaurantID,
		TotalTiyin: o.TotalTiyin, Type: string(o.Type),
		CreatedAt:   o.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Cancellable: agentCancellable(o),
	}
}

// ── Marshrutlar ──

func (s *Server) registerAgentRoutes(mux *http.ServeMux) {
	// GET /agent/v1/ping — kalit ishlayaptimi (integratsiyani
	// sozlashda birinchi chaqiriladigan endpoint).
	mux.HandleFunc("GET /agent/v1/ping", s.agentPartner(
		func(w http.ResponseWriter, r *http.Request) {
			p := partnerFrom(r)
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":          true,
				"partner":     p.Name,
				"environment": p.Environment,
				"scopes":      p.Scopes,
			})
		}))

	s.registerAgentLinkRoutes(mux)
	s.registerAgentCatalogRoutes(mux)
	s.registerAgentOrderRoutes(mux)
}

// ── Ulanish oqimi ──

func (s *Server) registerAgentLinkRoutes(mux *http.ServeMux) {
	// POST /agent/v1/link/start — "foydalanuvchi meni OnDex bilan
	// bog'lasin". Javobda foydalanuvchiga ko'rsatiladigan KOD va
	// ilovani ochadigan HAVOLA bo'ladi.
	mux.HandleFunc("POST /agent/v1/link/start", s.agentPartner(
		func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				// ExternalRef — sherikdagi foydalanuvchi ID'si.
				// Biz uchun ma'nosiz satr; sherik javobni o'z
				// foydalanuvchisiga bog'lash uchun ishlatadi.
				ExternalRef string   `json:"external_ref"`
				Scopes      []string `json:"scopes"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			p := partnerFrom(r)
			// Ruxsat so'ralmasa — sherikning butun tavani so'raladi.
			if len(req.Scopes) == 0 {
				req.Scopes = p.Scopes
			}
			link, secret, code, err := s.AgentSvc.StartLink(
				r.Context(), p, req.ExternalRef, req.Scopes)
			if err != nil {
				httpError(w, agentStatus(err), err)
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{
				"link_id":     link.ID,
				"link_secret": secret,
				"user_code":   code,
				// DeepLink — mijoz ilovasini to'g'ridan-to'g'ri
				// rozilik ekranida ochadi.
				"deep_link":  "ondex://agent-link?code=" + code,
				"scopes":     link.Scopes,
				"expires_at": link.ExpiresAt,
				"instructions": "Foydalanuvchi OnDex ilovasida " +
					"Profil → Ulangan ilovalar bo'limiga kirib shu kodni kiritsin.",
			})
		}))

	// POST /agent/v1/link/poll — natijani so'rash.
	//
	// Tasdiqlangan bo'lsa grant tokeni AYNAN SHU YERDA, BIR MARTA
	// beriladi. Sherik uni o'z tomonida xavfsiz saqlashi shart.
	mux.HandleFunc("POST /agent/v1/link/poll", s.agentPartner(
		func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				LinkID string `json:"link_id"`
				Secret string `json:"link_secret"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			status, token, g, err := s.AgentSvc.PollLink(
				r.Context(), partnerFrom(r), req.LinkID, req.Secret)
			if err != nil {
				// ErrLinkPending — XATO EMAS, normal holat: 202 va
				// "hali kutilmoqda". Sherik shu javobni ko'rib
				// pollingni davom ettiradi.
				if errors.Is(err, agentapi.ErrLinkPending) {
					writeJSON(w, http.StatusAccepted, map[string]any{"status": status})
					return
				}
				httpError(w, agentStatus(err), err)
				return
			}
			resp := map[string]any{"status": status}
			if token != "" {
				resp["grant_token"] = token
				resp["scopes"] = g.Scopes
				resp["per_order_limit_tiyin"] = g.PerOrderLimitTiyin
				resp["daily_limit_tiyin"] = g.DailyLimitTiyin
			}
			writeJSON(w, http.StatusOK, resp)
		}))

	// GET /agent/v1/me — grant kimga tegishli va nima qila oladi.
	//
	// TELEFON RAQAMI VA TO'LIQ MANZIL QAYTARILMAYDI. Agentga
	// yetkazish manzili BOR-YO'QLIGI kifoya — buyurtma berishdan
	// oldin "manzilingiz tanlanmagan" deb aytishi uchun. Manzil
	// matnining o'zi unga kerak emas va u LLM konteksti orqali
	// sherikning loglariga tushib ketardi.
	mux.HandleFunc("GET /agent/v1/me", s.agentAuth(agentapi.ScopeProfileRead,
		func(w http.ResponseWriter, r *http.Request) {
			g := grantFrom(r)
			u, err := s.UserRepo.GetByID(r.Context(), g.UserID)
			if err != nil {
				httpError(w, http.StatusNotFound, errors.New("foydalanuvchi topilmadi"))
				return
			}
			hasAddress := u.Address.Lat != 0 || u.Address.Lng != 0
			writeJSON(w, http.StatusOK, map[string]any{
				"name":                    u.Name,
				"has_delivery_address":    hasAddress,
				"address_in_service_area": hasAddress && delivery.Covered(u.Address.Lat, u.Address.Lng),
				"scopes":                  g.Scopes,
				"per_order_limit_tiyin":   g.PerOrderLimitTiyin,
				"daily_limit_tiyin":       g.DailyLimitTiyin,
			})
		}))
}

// ── Katalog ──

func (s *Server) registerAgentCatalogRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /agent/v1/restaurants", s.agentAuth(agentapi.ScopeCatalogRead,
		func(w http.ResponseWriter, r *http.Request) {
			list, err := s.CatalogRepo.ListRestaurants(r.Context())
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			out := make([]agentRestaurant, 0, len(list))
			for _, rest := range list {
				out = append(out, toAgentRestaurant(rest))
			}
			writeJSON(w, http.StatusOK, out)
		}))

	mux.HandleFunc("GET /agent/v1/restaurants/{id}/menu", s.agentAuth(agentapi.ScopeCatalogRead,
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID := r.PathValue("id")
			if _, err := s.CatalogRepo.GetRestaurant(r.Context(), restaurantID); err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			list, err := s.CatalogRepo.ListProducts(r.Context(), restaurantID)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			out := make([]agentProduct, 0, len(list))
			for _, p := range list {
				// Mavjud bo'lmagan taom modelga UMUMAN ko'rsatilmaydi:
				// aks holda u shuni taklif qilib, keyin qoralamada
				// xato olardi va foydalanuvchiga tushunarsiz javob
				// berardi.
				if !p.Available {
					continue
				}
				out = append(out, toAgentProduct(p))
			}
			writeJSON(w, http.StatusOK, out)
		}))

	// GET /agent/v1/products/search?q=... — "menga lag'mon buyur"
	// so'roviga eng qisqa yo'l: butun menyularni o'qib chiqish shart
	// emas.
	mux.HandleFunc("GET /agent/v1/products/search", s.agentAuth(agentapi.ScopeCatalogRead,
		func(w http.ResponseWriter, r *http.Request) {
			q := strings.TrimSpace(r.URL.Query().Get("q"))
			if q == "" {
				writeJSON(w, http.StatusOK, []agentProduct{})
				return
			}
			list, err := s.CatalogRepo.SearchProducts(r.Context(), q)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			out := make([]map[string]any, 0, len(list))
			for _, p := range list {
				if !p.Available {
					continue
				}
				item := toAgentProduct(&p.Product)
				// Qidiruvda restoran ID SHART: model taomni topgach
				// darhol qoralama tuza olishi kerak. Restoran nomi va
				// ochiqligi ham shu yerda — busiz model har bir
				// natija uchun alohida so'rov yuborardi, yoki yopiq
				// restorandan buyurtma qilishga urinardi.
				out = append(out, map[string]any{
					"id": item.ID, "name": item.Name, "category": item.Category,
					"price_tiyin":     item.PriceTiyin,
					"restaurant_id":   p.RestaurantID,
					"restaurant_name": p.RestaurantName,
					"restaurant_open": p.RestaurantOpen,
				})
			}
			writeJSON(w, http.StatusOK, out)
		}))
}

// ── Buyurtmalar ──

func (s *Server) registerAgentOrderRoutes(mux *http.ServeMux) {
	// POST /agent/v1/orders/draft — narxlangan qoralama.
	mux.HandleFunc("POST /agent/v1/orders/draft", s.agentAuth(agentapi.ScopeOrdersCreate,
		agentWrite(func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Items []catalog.ItemRequest `json:"items"`
				// PaymentMethod — hozircha FAQAT naqd.
				// Karta to'lovi agent orqali ATAYLAB ochilmagan:
				// u to'lov havolasini talab qiladi va uni til
				// modeli orqali o'tkazish fishing uchun tayyor
				// kanal bo'lardi.
				PaymentMethod string `json:"payment_method"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			if pm := strings.TrimSpace(req.PaymentMethod); pm != "" && pm != "cash" {
				httpError(w, http.StatusBadRequest,
					errors.New("agent orqali faqat naqd to'lov mumkin"))
				return
			}
			p, g := partnerFrom(r), grantFrom(r)

			// Manzil TEKSHIRUVI qoralamadan OLDIN: manzilsiz
			// foydalanuvchiga narx ko'rsatib, keyin tasdiqda
			// "manzil yo'q" deyish — modelni ham, odamni ham
			// chalg'itardi.
			if err := s.agentDeliveryReady(r.Context(), g.UserID); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			d, err := s.AgentSvc.CreateDraft(r.Context(), p, g, req.Items, "cash")
			if err != nil {
				httpError(w, agentStatus(err), err)
				return
			}
			// Tasdiq kerak bo'lsa foydalanuvchiga darhol xabar
			// yuboriladi — u ilovani o'zi ochib kutib o'tirmasin.
			if d.Status == agentapi.DraftAwaitingUser {
				s.notifyAgentDraft(r.Context(), d)
			}
			writeJSON(w, http.StatusCreated, d)
		})))

	// POST /agent/v1/orders/confirm — qoralamani tasdiqlash.
	mux.HandleFunc("POST /agent/v1/orders/confirm", s.agentAuth(agentapi.ScopeOrdersCreate,
		agentWrite(func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				DraftID string `json:"draft_id"`
				// TotalTiyin — agent KO'RGAN summa. Majburiy:
				// mos kelmasa buyurtma yaratilmaydi.
				TotalTiyin int64 `json:"total_tiyin"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			p, g := partnerFrom(r), grantFrom(r)
			d, err := s.AgentSvc.ConfirmDraft(r.Context(), p, g,
				req.DraftID, req.TotalTiyin, s.placeAgentOrder)
			if err != nil {
				if errors.Is(err, agentapi.ErrAwaitingUser) {
					// Qoralama AYNAN SHU so'rovda tasdiq kutish
					// holatiga o'tgan bo'lsa (kunlik chegara
					// tasdiqlashda yorilgan) — foydalanuvchiga xabar
					// beramiz. Allaqachon kutayotgan qoralamaga
					// takroriy tasdiq kelsa xabar QAYTA yuborilmaydi.
					if errors.Is(err, agentapi.ErrAwaitingUserNew) {
						s.notifyAgentDraft(r.Context(), d)
					}
					// 202 — buyurtma hali yaratilmadi, lekin xato ham
					// emas: foydalanuvchi ilovasida tasdiq kutilmoqda.
					writeJSON(w, http.StatusAccepted, map[string]any{
						"status":   d.Status,
						"draft_id": d.ID,
						"message":  "Foydalanuvchi ilovasida tasdiqlashi kerak",
					})
					return
				}
				httpError(w, agentStatus(err), err)
				return
			}
			writeJSON(w, http.StatusCreated, d)
		})))

	// GET /agent/v1/orders — foydalanuvchining so'nggi buyurtmalari.
	mux.HandleFunc("GET /agent/v1/orders", s.agentAuth(agentapi.ScopeOrdersRead,
		func(w http.ResponseWriter, r *http.Request) {
			g := grantFrom(r)
			// 10 ta — modelga "buyurtmam qayerda?" savoliga javob
			// berish uchun yetarli, kontekstni to'ldirmaydi.
			list, err := s.OrderRepo.ListByCustomer(r.Context(), g.UserID, 10)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			out := make([]agentOrder, 0, len(list))
			for _, o := range list {
				out = append(out, toAgentOrder(o))
			}
			writeJSON(w, http.StatusOK, out)
		}))

	mux.HandleFunc("GET /agent/v1/orders/{id}", s.agentAuth(agentapi.ScopeOrdersRead,
		func(w http.ResponseWriter, r *http.Request) {
			o, err := s.agentOwnOrder(r)
			if err != nil {
				httpError(w, agentStatus(err), err)
				return
			}
			writeJSON(w, http.StatusOK, toAgentOrder(o))
		}))

	// POST /agent/v1/orders/{id}/cancel — bekor qilish.
	mux.HandleFunc("POST /agent/v1/orders/{id}/cancel", s.agentAuth(agentapi.ScopeOrdersCancel,
		agentWrite(func(w http.ResponseWriter, r *http.Request) {
			o, err := s.agentOwnOrder(r)
			if err != nil {
				httpError(w, agentStatus(err), err)
				return
			}
			// Holat mashinasi — YAGONA haqiqat manbai. Bu yerda
			// "bekor qilsa bo'ladimi" qayta hisoblanmaydi, aks holda
			// ikkita qoida bir-biridan uzoqlashardi.
			updated, err := s.OrderSvc.ChangeStatus(r.Context(), o.ID,
				orders.StatusCancelled, orders.ActorCustomer)
			if err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			p, g := partnerFrom(r), grantFrom(r)
			s.AgentSvc.Audit(r.Context(), p, g, "order.cancelled", o.ID, true, clientIP(r))
			writeJSON(w, http.StatusOK, toAgentOrder(updated))
		})))
}

// ── Yordamchilar ──

// agentOwnOrder — buyurtmani o'qiydi va u AYNAN grant egasiga
// tegishli ekanini tekshiradi.
//
// ★ Bu tekshiruv butun modelning tayanchi: usiz agent istalgan
// buyurtma ID'sini so'rab, begona odamning buyurtmasini ko'rardi.
func (s *Server) agentOwnOrder(r *http.Request) (*orders.Order, error) {
	o, err := s.OrderRepo.GetByID(r.Context(), r.PathValue("id"))
	if err != nil {
		return nil, agentapi.ErrNotFound
	}
	if o.CustomerID != grantFrom(r).UserID {
		// Ataylab "topilmadi": begona buyurtmaning MAVJUDLIGI ham
		// oshkor bo'lmasin.
		return nil, agentapi.ErrNotFound
	}
	return o, nil
}

// agentDeliveryReady — foydalanuvchida yetkazish manzili bormi va u
// xizmat hududidami.
func (s *Server) agentDeliveryReady(ctx context.Context, userID string) error {
	u, err := s.UserRepo.GetByID(ctx, userID)
	if err != nil {
		return errors.New("foydalanuvchi topilmadi")
	}
	if u.Address.Lat == 0 && u.Address.Lng == 0 {
		return errors.New("yetkazib berish manzili tanlanmagan — foydalanuvchi uni OnDex ilovasida tanlashi kerak")
	}
	if !delivery.Covered(u.Address.Lat, u.Address.Lng) {
		return errOutsideServiceArea
	}
	return nil
}

// placeAgentOrder — qoralamani HAQIQIY buyurtmaga aylantiradi.
//
// ┌─ ODDIY `POST /orders` BILAN BIR XIL QOIDALAR ──────────────────────┐
// Manzil serverdagi saqlangan qiymatdan olinadi (agentdan EMAS),
// xizmat hududi tekshiriladi, tarkib katalog bo'yicha QAYTA
// narxlanadi. Ya'ni agent yo'li mijoz yo'lidan yumshoqroq emas —
// faqat kirish nuqtasi boshqa.
//
// Idempotentlik kaliti qoralama ID'sidan olinadi: bitta qoralama
// hech qachon ikkita buyurtma yaratmaydi, hatto ikki tasdiq bir
// vaqtda kelib qolsa ham (baza darajasidagi unikal indeks
// ikkinchisini ushlaydi va birinchisining buyurtmasini qaytaradi).
// └────────────────────────────────────────────────────────────────────┘
func (s *Server) placeAgentOrder(ctx context.Context, d *agentapi.Draft) (string, error) {
	u, err := s.UserRepo.GetByID(ctx, d.UserID)
	if err != nil {
		return "", errors.New("foydalanuvchi topilmadi")
	}
	addr := u.Address
	if addr.Lat == 0 && addr.Lng == 0 {
		return "", errors.New("yetkazib berish manzili tanlanmagan")
	}
	if !delivery.Covered(addr.Lat, addr.Lng) {
		return "", errOutsideServiceArea
	}
	restaurantID, items, err := s.CatalogSvc.PriceOrder(ctx, d.ToItemRequests())
	if err != nil {
		return "", err
	}
	// Qoralama tuzilgandan keyin taom boshqa restoranga ko'chgan
	// (yoki katalog o'zgargan) holat — buyurtma yaratilmaydi.
	if restaurantID != d.RestaurantID {
		return "", errors.New("menyu o'zgargan — qoralamani qaytadan yarating")
	}
	// `POST /orders` bilan bir xil: restoran boshqa shaharda bo'lsa
	// buyurtma yaratilmaydi.
	if err := s.checkRestaurantServes(ctx, restaurantID, addr.Lat, addr.Lng); err != nil {
		return "", err
	}
	// Agent buyurtmasi yetkazishda to'lanadi — restoran joyida to'lovni
	// o'chirgan bo'lsa (faqat onlayn karta), buyurtma yaratilmaydi.
	if err := s.CatalogSvc.CheckPayment(ctx, restaurantID, orders.PaymentCash); err != nil {
		return "", err
	}
	o := orders.Order{
		CustomerID:   d.UserID,
		RestaurantID: restaurantID,
		Items:        items,
		Type:         orders.TypeDelivery,
		DeliveryLat:  addr.Lat,
		DeliveryLng:  addr.Lng,
		DeliveryAddress: orders.Address{
			Text:      addr.Text,
			Entrance:  addr.Entrance,
			Floor:     addr.Floor,
			Apartment: addr.Apartment,
			Intercom:  addr.Intercom,
			Comment:   addr.Comment,
		},
		IdempotencyKey: agentapi.IdempotencyKey(d.ID),
	}
	created, err := s.OrderSvc.Create(ctx, &o)
	if err != nil {
		return "", err
	}
	// ★ SUMMA TEKSHIRUVI — OXIRGI TO'SIQ.
	//
	// `Create` tarkibni qayta narxlaydi. Natija foydalanuvchi
	// ko'rgan/agent tasdiqlagan summadan farq qilsa, bu jimgina
	// o'tib ketmasligi kerak: odam bir raqamga rozi bo'lib,
	// boshqasini to'lardi. Buyurtma allaqachon yaratilgani uchun uni
	// darhol bekor qilamiz.
	if created.TotalTiyin != d.TotalTiyin {
		if _, cerr := s.OrderSvc.ChangeStatus(ctx, created.ID,
			orders.StatusCancelled, orders.ActorCustomer); cerr != nil {
			// Bekor qilib bo'lmasa — bu jiddiy holat, log SHART.
			return "", errors.New("narx o'zgargan va buyurtmani bekor qilib bo'lmadi — qo'llab-quvvatlashga murojaat qiling")
		}
		return "", agentapi.ErrTotalMismatch
	}
	return created.ID, nil
}

// notifyAgentDraft — "tasdiqlashingiz kerak" xabari.
//
// `notify.Service` ishlatiladi (xom `Hub` emas): u bir chaqiruvda
// bildirishnomani saqlaydi, ilova ochiq bo'lsa WebSocket orqali
// yuboradi, yopiq bo'lsa push qiladi. Bu yerda aynan push MUHIM —
// tasdiq so'ralayotganda odam OnDex ilovasida emas, sherikning
// ilovasida (yoki telefonini quloqqa tutib) turgan bo'ladi.
func (s *Server) notifyAgentDraft(ctx context.Context, d *agentapi.Draft) {
	if s.Notifier == nil {
		return
	}
	s.Notifier.Notify(ctx, d.UserID, notify.Event{
		Module: notify.ModuleFood,
		Kind:   "agent_draft",
		Title:  d.PartnerName + " buyurtma tayyorladi",
		Body: fmt.Sprintf("%s so'm — tasdiqlashingizni kutmoqda",
			formatSum(d.TotalTiyin)),
		Data: map[string]string{
			"draft_id":    d.ID,
			"total_tiyin": strconv.FormatInt(d.TotalTiyin, 10),
		},
	})
}

// formatSum — tiyinni so'mga aylantiradi ("125 000").
//
// Bildirishnoma matni odam o'qiydigan yagona joy bo'lgani uchun
// raqam xom holda ("12500000 tiyin") ko'rsatilmaydi.
func formatSum(tiyin int64) string {
	sum := strconv.FormatInt(tiyin/100, 10)
	// Uch xonadan guruhlash, orqadan.
	var b strings.Builder
	for i, r := range sum {
		if i > 0 && (len(sum)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}
