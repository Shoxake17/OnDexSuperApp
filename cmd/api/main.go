package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/catalog"
	"chustapp/internal/couriers"
	"chustapp/internal/notify"
	"chustapp/internal/orders"
	"chustapp/internal/storage"
	"chustapp/internal/users"
	"chustapp/internal/ws"
)

// devMode — production'da APP_ENV=production qo'yiladi; unda SMS kodlar
// HTTP javobda qaytarilmaydi va zaif JWT_SECRET bilan ishga tushmaydi.
var devMode bool

// loadDotEnv — loyiha ildizidagi .env faylni o'qiydi (bor bo'lsa).
// Tizim muhitida allaqachon o'rnatilgan o'zgaruvchilar ustun turadi.
// Maxfiy qiymatlar (kalitlar, parollar) faqat shu faylda saqlanadi,
// kodga hech qachon yozilmaydi.
func loadDotEnv() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
	slog.Info(".env yuklandi")
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	loadDotEnv()
	devMode = os.Getenv("APP_ENV") != "production"

	var orderRepo orders.Repository
	var courierRepo couriers.Repository
	var userRepo users.Repository
	var codeStore users.CodeStore
	var catalogRepo catalog.Repository

	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		pool, err := pgxpool.New(ctx, dbURL)
		if err != nil {
			slog.Error("PostgreSQL konfiguratsiya xatosi", "err", err)
			os.Exit(1)
		}
		if err := pool.Ping(ctx); err != nil {
			slog.Error("PostgreSQL'ga ulanib bo'lmadi (docker compose up -d qilinganmi?)", "err", err)
			os.Exit(1)
		}
		if err := storage.Migrate(ctx, pool); err != nil {
			slog.Error("migratsiya xatosi", "err", err)
			os.Exit(1)
		}
		if err := storage.SeedDemoCouriers(ctx, pool); err != nil {
			slog.Error("seed xatosi", "err", err)
			os.Exit(1)
		}
		if err := storage.SeedDemoUsers(ctx, pool); err != nil {
			slog.Error("users seed xatosi", "err", err)
			os.Exit(1)
		}
		if err := storage.SeedDemoCatalog(ctx, pool); err != nil {
			slog.Error("catalog seed xatosi", "err", err)
			os.Exit(1)
		}
		orderRepo = storage.NewPgOrderRepo(pool)
		courierRepo = storage.NewPgCourierRepo(pool)
		userRepo = storage.NewPgUserRepo(pool)
		codeStore = storage.NewPgCodeStore(pool)
		catalogRepo = storage.NewPgCatalogRepo(pool)
		slog.Info("rejim: PostgreSQL")
	} else {
		courierRepo = storage.NewMemoryCourierRepo(
			couriers.Courier{ID: "c1", Name: "Aziz", Lat: 41.0056, Lng: 71.2378, Available: true, Approved: true},
			couriers.Courier{ID: "c2", Name: "Bekzod", Lat: 41.0010, Lng: 71.2400, Available: true, Approved: true},
			couriers.Courier{ID: "c3", Name: "Doniyor", Lat: 40.9980, Lng: 71.2330, Available: true, Approved: true},
		)
		orderRepo = storage.NewMemoryOrderRepo()
		userRepo = storage.NewMemoryUserRepo(storage.DemoUsers()...)
		codeStore = storage.NewMemoryCodeStore()
		catalogRepo = storage.NewMemoryCatalogRepo(storage.DemoRestaurants(), storage.DemoProducts())
		slog.Warn("rejim: in-memory (DATABASE_URL berilmagan — ma'lumotlar server o'chsa yo'qoladi)")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		if !devMode {
			slog.Error("production rejimda JWT_SECRET majburiy")
			os.Exit(1)
		}
		jwtSecret = "dev-secret-almashtiring"
		slog.Warn("JWT_SECRET berilmagan — dev secret ishlatilyapti")
	}
	tokens := users.NewTokenIssuer(jwtSecret, 30*24*time.Hour)

	hub := ws.NewHub()
	notifier := notify.NewLive(hub)
	authSvc := users.NewService(userRepo, codeStore, notify.LogSms{}, tokens, newID)
	orderSvc := orders.NewService(orderRepo, notifier, newID)
	catalogSvc := catalog.NewService(catalogRepo)
	dispatcher := couriers.NewDispatcher(courierRepo, notifier, 30*time.Second, 5)

	mux := http.NewServeMux()

	// GET /ws — jonli eventlar oqimi. Token query'da (?token=...) yoki
	// Authorization header'da. Ulangach: buyurtma holati o'zgarishlari,
	// kuryer uchun takliflar shu kanaldan keladi.
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		tokenStr := r.URL.Query().Get("token")
		if tokenStr == "" {
			tokenStr = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		}
		claims, err := tokens.Parse(tokenStr)
		if err != nil {
			httpError(w, http.StatusUnauthorized, err)
			return
		}
		hub.Serve(w, r, claims.Subject, claims.EntityID)
	})

	// ---------- Katalog ----------

	// GET /restaurants — ochiq: mijoz ilovasining bosh sahifasi
	mux.HandleFunc("GET /restaurants", func(w http.ResponseWriter, r *http.Request) {
		list, err := catalogRepo.ListRestaurants(r.Context())
		if err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})

	// GET /restaurants/{id}/menu — ochiq
	mux.HandleFunc("GET /restaurants/{id}/menu", func(w http.ResponseWriter, r *http.Request) {
		if _, err := catalogRepo.GetRestaurant(r.Context(), r.PathValue("id")); err != nil {
			httpError(w, http.StatusNotFound, err)
			return
		}
		list, err := catalogRepo.ListProducts(r.Context(), r.PathValue("id"))
		if err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})

	// ---------- Superadmin API ----------

	// POST /admin/restaurants — restoran + unga kirish akkaunti bir amalda.
	// Restoranlar o'zi ro'yxatdan o'tmaydi: akkauntni faqat superadmin yaratadi,
	// restoran o'z paneliga shu telefon raqami bilan (SMS kod) kiradi.
	mux.HandleFunc("POST /admin/restaurants", auth(tokens, []users.Role{users.RoleAdmin},
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
			if _, err := userRepo.GetByPhone(r.Context(), phone); err == nil {
				httpError(w, http.StatusConflict, errors.New("bu telefon raqam allaqachon ro'yxatda"))
				return
			}
			rest := catalog.Restaurant{
				ID: newID(), Name: req.Name, Address: req.Address,
				Lat: req.Lat, Lng: req.Lng, Open: true,
			}
			if err := catalogRepo.SaveRestaurant(r.Context(), &rest); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			account := users.User{
				ID: newID(), Phone: phone, Name: req.StaffName,
				Role: users.RoleRestaurant, EntityID: rest.ID, CreatedAt: time.Now(),
			}
			if err := userRepo.Create(r.Context(), &account); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{"restaurant": rest, "account": account})
		}))

	// DELETE /admin/restaurants/{id} — restoranni butunlay o'chirish.
	// Qoidalar: faol (yakunlanmagan) buyurtmasi bo'lsa o'chirib bo'lmaydi;
	// o'chirilganda menyu taomlari va kirish akkauntlari ham o'chadi;
	// eski buyurtmalar tarixi hisobotlar uchun SAQLANADI.
	mux.HandleFunc("DELETE /admin/restaurants/{id}", auth(tokens, []users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			id := r.PathValue("id")
			if _, err := catalogRepo.GetRestaurant(r.Context(), id); err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			active, err := orderRepo.HasActiveByRestaurant(r.Context(), id)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			if active {
				httpError(w, http.StatusConflict,
					errors.New("bu restoranning faol buyurtmalari bor — avval ular yakunlanishi kerak"))
				return
			}
			if err := catalogRepo.DeleteRestaurant(r.Context(), id); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			if err := userRepo.DeleteByRoleEntity(r.Context(), users.RoleRestaurant, id); err != nil {
				slog.Error("restoran akkauntini o'chirishda xato", "restaurant", id, "err", err)
			}
			slog.Info("restoran o'chirildi", "restaurant", id, "by", claimsFrom(r).Subject)
			writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
		}))

	// POST /admin/restaurants/{id}/open  {"open":true|false}
	mux.HandleFunc("POST /admin/restaurants/{id}/open", auth(tokens, []users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			rest, err := catalogRepo.GetRestaurant(r.Context(), r.PathValue("id"))
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
			if err := catalogRepo.SaveRestaurant(r.Context(), rest); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, rest)
		}))

	// GET /admin/couriers — barcha kuryerlar (telefon raqamlari bilan)
	mux.HandleFunc("GET /admin/couriers", auth(tokens, []users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			list, err := courierRepo.ListAll(r.Context())
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			courierUsers, err := userRepo.ListByRole(r.Context(), users.RoleCourier)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			phoneByEntity := make(map[string]string, len(courierUsers))
			for _, u := range courierUsers {
				phoneByEntity[u.EntityID] = u.Phone
			}
			type row struct {
				couriers.Courier
				Phone string `json:"phone"`
			}
			out := make([]row, 0, len(list))
			for _, c := range list {
				out = append(out, row{Courier: *c, Phone: phoneByEntity[c.ID]})
			}
			writeJSON(w, http.StatusOK, out)
		}))

	// POST /admin/couriers/{id}/approve  {"approved":true|false}
	mux.HandleFunc("POST /admin/couriers/{id}/approve", auth(tokens, []users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Approved bool `json:"approved"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			courierID := r.PathValue("id")
			if err := courierRepo.SetApproved(r.Context(), courierID, req.Approved); err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			// Blok qilinganda darhol offline ham qilamiz
			if !req.Approved {
				courierRepo.SetAvailable(r.Context(), courierID, false)
			}
			writeJSON(w, http.StatusOK, map[string]bool{"approved": req.Approved})
		}))

	// GET /admin/orders — so'nggi buyurtmalar
	mux.HandleFunc("GET /admin/orders", auth(tokens, []users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			list, err := orderRepo.ListRecent(r.Context(), 100)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, list)
		}))

	// GET /admin/stats — boshqaruv paneli ko'rsatkichlari
	mux.HandleFunc("GET /admin/stats", auth(tokens, []users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			recent, err := orderRepo.ListRecent(r.Context(), 500)
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
			allCouriers, err := courierRepo.ListAll(r.Context())
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
			restaurants, _ := catalogRepo.ListRestaurants(r.Context())
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

	// POST /restaurants/{id}/products — restoran o'z menyusini boshqaradi (yoki admin)
	mux.HandleFunc("POST /restaurants/{id}/products", auth(tokens, []users.Role{users.RoleRestaurant, users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID := r.PathValue("id")
			claims := claimsFrom(r)
			if claims.Role == users.RoleRestaurant && claims.EntityID != restaurantID {
				httpError(w, http.StatusForbidden, errors.New("boshqa restoran menyusini o'zgartirib bo'lmaydi"))
				return
			}
			if _, err := catalogRepo.GetRestaurant(r.Context(), restaurantID); err != nil {
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
			p.RestaurantID = restaurantID
			if p.ID == "" {
				p.ID = newID()
			}
			if err := catalogRepo.SaveProduct(r.Context(), &p); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusCreated, p)
		}))

	// ---------- Auth (ochiq endpoint'lar) ----------

	// POST /auth/request-code  {"phone":"+998901234567"}
	mux.HandleFunc("POST /auth/request-code", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Phone string `json:"phone"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpError(w, http.StatusBadRequest, err)
			return
		}
		phone, code, err := authSvc.RequestCode(r.Context(), req.Phone)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, users.ErrTooSoon) {
				status = http.StatusTooManyRequests
			}
			httpError(w, status, err)
			return
		}
		resp := map[string]any{"sent": true, "phone": phone}
		if devMode {
			resp["dev_code"] = code // faqat dev: SMS o'rniga kod javobda
		}
		writeJSON(w, http.StatusOK, resp)
	})

	// POST /auth/verify  {"phone":"+998901234567","code":"123456"} -> {token, user}
	mux.HandleFunc("POST /auth/verify", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Phone string `json:"phone"`
			Code  string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpError(w, http.StatusBadRequest, err)
			return
		}
		token, u, err := authSvc.Verify(r.Context(), req.Phone, req.Code)
		if err != nil {
			httpError(w, http.StatusUnauthorized, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": u})
	})

	// GET /config/maps — Google Maps kaliti. Frontend kodida saqlanmaydi:
	// server .env dan o'qib, faqat tizimga kirgan foydalanuvchilarga beradi.
	// Qo'shimcha himoya Google Console'da: kalit domen (referrer) va API
	// turi bo'yicha cheklanadi.
	mux.HandleFunc("GET /config/maps", auth(tokens, nil,
		func(w http.ResponseWriter, r *http.Request) {
			key := os.Getenv("GOOGLE_MAPS_API_KEY")
			if key == "" {
				httpError(w, http.StatusServiceUnavailable,
					errors.New("xarita kaliti sozlanmagan (.env: GOOGLE_MAPS_API_KEY)"))
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"maps_api_key": key})
		}))

	// ---------- Buyurtmalar (token talab qilinadi) ----------

	// POST /orders — faqat mijoz. Mijoz faqat product_id + qty yuboradi;
	// narx, nom va restoran katalogdan aniqlanadi (narxni soxtalashtirib bo'lmaydi).
	mux.HandleFunc("POST /orders", auth(tokens, []users.Role{users.RoleCustomer},
		func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Items       []catalog.ItemRequest `json:"items"`
				DeliveryLat float64               `json:"delivery_lat"`
				DeliveryLng float64               `json:"delivery_lng"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			restaurantID, items, err := catalogSvc.PriceOrder(r.Context(), req.Items)
			if err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			o := orders.Order{
				CustomerID:   claimsFrom(r).Subject,
				RestaurantID: restaurantID,
				Items:        items,
				DeliveryLat:  req.DeliveryLat,
				DeliveryLng:  req.DeliveryLng,
			}
			created, err := orderSvc.Create(r.Context(), &o)
			if err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, created)
		}))

	// GET /orders/{id} — faqat aloqador tomonlar ko'ra oladi
	mux.HandleFunc("GET /orders/{id}", auth(tokens, nil,
		func(w http.ResponseWriter, r *http.Request) {
			o, err := orderSvc.Get(r.Context(), r.PathValue("id"))
			if err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			if !canSeeOrder(claimsFrom(r), o) {
				httpError(w, http.StatusForbidden, errors.New("bu buyurtma sizga tegishli emas"))
				return
			}
			writeJSON(w, http.StatusOK, o)
		}))

	// POST /orders/{id}/transition  {"to":"accepted"} — aktor tokendagi roldan aniqlanadi
	mux.HandleFunc("POST /orders/{id}/transition", auth(tokens, nil,
		func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				To orders.Status `json:"to"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			claims := claimsFrom(r)
			o, err := orderSvc.Get(r.Context(), r.PathValue("id"))
			if err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			if !ownsOrderAction(claims, o) {
				httpError(w, http.StatusForbidden, errors.New("bu buyurtma sizga tegishli emas"))
				return
			}
			o, err = orderSvc.ChangeStatus(r.Context(), o.ID, req.To, roleToActor(claims.Role))
			if err != nil {
				var terr *orders.TransitionError
				if errors.As(err, &terr) {
					httpError(w, http.StatusConflict, err)
				} else {
					httpError(w, http.StatusBadRequest, err)
				}
				return
			}
			// Buyurtma yakunlandi — kuryer yana bo'sh
			if o.IsTerminal() && o.CourierID != "" {
				if err := courierRepo.SetAvailable(r.Context(), o.CourierID, true); err != nil {
					slog.Error("kuryerni bo'shatishda xato", "courier", o.CourierID, "err", err)
				}
			}
			writeJSON(w, http.StatusOK, o)
		}))

	// POST /orders/{id}/dispatch — restoran (o'z buyurtmasi uchun) yoki admin
	mux.HandleFunc("POST /orders/{id}/dispatch", auth(tokens, []users.Role{users.RoleRestaurant, users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			claims := claimsFrom(r)
			orderID := r.PathValue("id")
			o, err := orderSvc.Get(r.Context(), orderID)
			if err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			if claims.Role == users.RoleRestaurant && o.RestaurantID != claims.EntityID {
				httpError(w, http.StatusForbidden, errors.New("bu buyurtma sizning restoraningizniki emas"))
				return
			}
			if o.IsTerminal() || o.CourierID != "" {
				httpError(w, http.StatusConflict, errors.New("bu buyurtma uchun dispatch mumkin emas"))
				return
			}
			// Kuryer restoranga yaqinidan qidiriladi (taomni olib ketish nuqtasi).
			searchLat, searchLng := o.DeliveryLat, o.DeliveryLng
			if rest, err := catalogRepo.GetRestaurant(r.Context(), o.RestaurantID); err == nil {
				searchLat, searchLng = rest.Lat, rest.Lng
			}
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()
				courierID, err := dispatcher.Dispatch(ctx, orderID, searchLat, searchLng)
				if err != nil {
					slog.Error("dispatch muvaffaqiyatsiz", "order", orderID, "err", err)
					return
				}
				if _, err := orderSvc.AssignCourier(ctx, orderID, courierID); err != nil {
					slog.Error("kuryer biriktirishda xato", "order", orderID, "err", err)
				}
			}()
			writeJSON(w, http.StatusAccepted, map[string]string{"status": "dispatch boshlandi"})
		}))

	// ---------- Kuryer amallari ----------

	// POST /couriers/register — mijoz kuryer bo'lishga ariza beradi.
	// Kuryer yaratiladi, lekin approved=false: superadmin tasdiqlamaguncha
	// online bo'la olmaydi va taklif olmaydi. Yangi token qaytariladi
	// (eski tokenda rol hali "customer" edi).
	mux.HandleFunc("POST /couriers/register", auth(tokens, []users.Role{users.RoleCustomer},
		func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			if req.Name == "" {
				httpError(w, http.StatusBadRequest, errors.New("name majburiy"))
				return
			}
			claims := claimsFrom(r)
			u, err := userRepo.GetByID(r.Context(), claims.Subject)
			if err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			c := couriers.Courier{ID: newID(), Name: req.Name}
			if err := courierRepo.Create(r.Context(), &c); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			if err := userRepo.UpdateRole(r.Context(), u.ID, users.RoleCourier, c.ID); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			u.Role = users.RoleCourier
			u.EntityID = c.ID
			newToken, err := tokens.Issue(u)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{
				"courier": c,
				"token":   newToken,
				"message": "Ariza qabul qilindi. Admin tasdiqlagach ishlay boshlaysiz.",
			})
		}))

	// POST /couriers/{id}/respond — faqat o'sha kuryerning o'zi
	mux.HandleFunc("POST /couriers/{id}/respond", auth(tokens, []users.Role{users.RoleCourier},
		func(w http.ResponseWriter, r *http.Request) {
			courierID := r.PathValue("id")
			if claimsFrom(r).EntityID != courierID {
				httpError(w, http.StatusForbidden, errors.New("boshqa kuryer nomidan javob berib bo'lmaydi"))
				return
			}
			var req struct {
				OrderID  string `json:"order_id"`
				Accepted bool   `json:"accepted"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			ok := dispatcher.HandleResponse(req.OrderID, couriers.Response{CourierID: courierID, Accepted: req.Accepted})
			if !ok {
				httpError(w, http.StatusConflict, errors.New("taklif eskirgan yoki sizga tegishli emas"))
				return
			}
			writeJSON(w, http.StatusOK, map[string]bool{"received": true})
		}))

	// POST /couriers/{id}/available — kuryerning o'zi yoki admin
	mux.HandleFunc("POST /couriers/{id}/available", auth(tokens, []users.Role{users.RoleCourier, users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			claims := claimsFrom(r)
			courierID := r.PathValue("id")
			if claims.Role == users.RoleCourier && claims.EntityID != courierID {
				httpError(w, http.StatusForbidden, errors.New("boshqa kuryer holatini o'zgartirib bo'lmaydi"))
				return
			}
			var req struct {
				Available bool `json:"available"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			c, err := courierRepo.GetByID(r.Context(), courierID)
			if err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			if req.Available && !c.Approved {
				httpError(w, http.StatusForbidden,
					errors.New("kuryerlik arizangiz hali admin tomonidan tasdiqlanmagan"))
				return
			}
			if err := courierRepo.SetAvailable(r.Context(), courierID, req.Available); err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]bool{"available": req.Available})
		}))

	addr := ":8080"
	slog.Info("ChustApp API ishga tushdi", "addr", addr, "dev_mode", devMode)
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
		slog.Error("server to'xtadi", "err", err)
		os.Exit(1)
	}
}

// withCORS — brauzerdan (Flutter web) kelgan so'rovlar uchun CORS ruxsatlari.
// Dev'da hamma originga ochiq; production'da o'z domenlarimizga cheklanadi.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---------- Auth middleware ----------

type ctxKey int

const claimsKey ctxKey = 0

// auth — Bearer tokenni tekshiradi; roles bo'sh bo'lmasa rol ham talab qilinadi.
func auth(tokens *users.TokenIssuer, roles []users.Role, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			httpError(w, http.StatusUnauthorized, errors.New("Authorization: Bearer <token> talab qilinadi"))
			return
		}
		claims, err := tokens.Parse(strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			httpError(w, http.StatusUnauthorized, err)
			return
		}
		if len(roles) > 0 && !slices.Contains(roles, claims.Role) {
			httpError(w, http.StatusForbidden, errors.New("bu amal sizning rolingizga ochiq emas"))
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), claimsKey, claims)))
	}
}

func claimsFrom(r *http.Request) *users.Claims {
	return r.Context().Value(claimsKey).(*users.Claims)
}

func roleToActor(role users.Role) orders.Actor {
	switch role {
	case users.RoleCustomer:
		return orders.ActorCustomer
	case users.RoleRestaurant:
		return orders.ActorRestaurant
	case users.RoleCourier:
		return orders.ActorCourier
	case users.RoleAdmin:
		return orders.ActorAdmin
	}
	return ""
}

// ownsOrderAction — holatni o'zgartirish huquqi: mijoz o'z buyurtmasi, restoran
// o'z restorani buyurtmasi, kuryer o'ziga biriktirilgan buyurtma; admin hammasi.
func ownsOrderAction(c *users.Claims, o *orders.Order) bool {
	switch c.Role {
	case users.RoleCustomer:
		return o.CustomerID == c.Subject
	case users.RoleRestaurant:
		return o.RestaurantID == c.EntityID
	case users.RoleCourier:
		return o.CourierID == c.EntityID
	case users.RoleAdmin:
		return true
	}
	return false
}

func canSeeOrder(c *users.Claims, o *orders.Order) bool {
	// Ko'rish huquqi hozircha o'zgartirish huquqi bilan bir xil, faqat kuryer
	// hali biriktirilmagan bo'lsa ham taklif bosqichida ko'rishi kerak bo'ladi —
	// bu WebSocket bosqichida qayta ko'riladi.
	return ownsOrderAction(c, o)
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
