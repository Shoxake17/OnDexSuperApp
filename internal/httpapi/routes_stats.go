package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/orders"
	"chustapp/internal/stats"
	"chustapp/internal/users"
)

// statsCacheTTL — bir xil so'rov (restoran + davr + o'lcham) shuncha vaqt
// bazaga qayta bormaydi. Yangi buyurtma statistikada eng ko'pi bilan
// shuncha kechikib ko'rinadi.
const statsCacheTTL = 60 * time.Second

// statsQueryTimeout — bir so'rov uchun bazaga ajratilgan vaqt.
const statsQueryTimeout = 20 * time.Second

func statsCacheKey(restaurantID string, q stats.Query) string {
	return "cache:stats:" + restaurantID + ":" + q.From.Format("2006-01-02") + ":" +
		q.To.Format("2006-01-02") + ":" + string(q.Granularity)
}

// orderHistoryResponse — `GET /restaurants/{id}/orders/history` javobi.
type orderHistoryResponse struct {
	Orders []map[string]any `json:"orders"`
	// NextCursor — bo'sh bo'lsa ro'yxat tugagan.
	NextCursor string `json:"next_cursor"`
	// Summary — faqat birinchi sahifada (kursorsiz so'rovda).
	Summary *stats.Lifetime `json:"summary,omitempty"`
}

func (s *Server) registerStatsRoutes(mux *http.ServeMux) {
	// GET /restaurants/{id}/stats?from=YYYY-MM-DD&to=YYYY-MM-DD&granularity=day|week|month
	//
	// ┌─ XAVFSIZLIK ───────────────────────────────────────────────────┐
	//  * Faqat restoran xodimi (O'Z restorani) va admin. Affitsiant
	//    ATAYLAB yo'q: tushum — moliyaviy ma'lumot.
	//  * Egalik tekshiruvi HAMMA narsadan oldin — hatto "xizmat
	//    ulanmagan" javobidan ham. Aks holda begona restoran ID'si
	//    bilan kelgan so'rov 503 olib, bu ID mavjudligi haqida signal
	//    berardi (`TestTenancyMatrix` shuni tekshiradi).
	//  * Javobda shaxsiy ma'lumot YO'Q: mijoz ID'lari faqat sanash
	//    uchun ishlatiladi, hech biri qaytarilmaydi.
	//  * Kesh o'tkazib yuborilganda (haqiqiy baza ishi) xodim bo'yicha
	//    tezlik chegarasi bor — bir so'rov bir yillik davrni o'qishi
	//    mumkin.
	// └────────────────────────────────────────────────────────────────┘
	mux.HandleFunc("GET /restaurants/{id}/stats", s.auth([]users.Role{users.RoleRestaurant, users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID := r.PathValue("id")
			claims := claimsFrom(r)
			if claims.Role == users.RoleRestaurant && claims.EntityID != restaurantID {
				httpError(w, http.StatusForbidden, errors.New("boshqa restoran statistikasini ko'rib bo'lmaydi"))
				return
			}
			if s.StatsSource == nil {
				httpError(w, http.StatusServiceUnavailable, errors.New("statistika xizmati ulanmagan"))
				return
			}

			qv := r.URL.Query()
			q, err := stats.ParseQuery(qv.Get("from"), qv.Get("to"), qv.Get("granularity"), time.Now())
			if err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}

			// Admin mavjud bo'lmagan ID bilan kelsa, "hammasi nol" degan
			// ishonarli, lekin yolg'on javob o'rniga aniq 404.
			if !s.restaurantExists(w, r, restaurantID) {
				return
			}

			key := statsCacheKey(restaurantID, q)
			var cached stats.Result
			if s.Cache.GetJSON(r.Context(), key, &cached) {
				writeJSON(w, http.StatusOK, cached)
				return
			}

			if ok, wait := s.statsLimiter.AllowWithWait(claims.Subject); !ok {
				tooManyRequests(w, wait)
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), statsQueryTimeout)
			defer cancel()

			rows, err := s.StatsSource.StatRows(ctx, restaurantID, q.PrevFrom(), q.End(), q.From, stats.MaxRows)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			if len(rows) > stats.MaxRows {
				httpError(w, http.StatusUnprocessableEntity, stats.ErrTooManyRows)
				return
			}
			first, err := s.StatsSource.FirstOrderAt(ctx, restaurantID, stats.CustomerIDs(rows))
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}

			res := stats.Compute(q, rows, first)
			s.Cache.SetJSON(r.Context(), key, res, statsCacheTTL)
			writeJSON(w, http.StatusOK, res)
		}))

	// GET /restaurants/{id}/orders/history?status=all|in_progress|completed|cancelled&limit=30&cursor=...&from=YYYY-MM-DD&to=YYYY-MM-DD
	//
	// "Barcha buyurtmalar" sahifasi: eng yangisidan, kursor bilan
	// sahifalab. `from`/`to` berilmasa — restoran ochilgandan beri.
	// Birinchi sahifada shu davr xulosasi (`summary`) ham qaytadi.
	//
	// ┌─ XAVFSIZLIK ───────────────────────────────────────────────────┐
	//  * Rollar va egalik — `GET /restaurants/{id}/orders` bilan AYNAN
	//    bir xil, mijoz telefoni ham o'sha darajada qo'shiladi
	//    (`restaurantOrderViews`). Affitsiant yo'q: unga faqat o'z
	//    stollari ko'rinadi (`routes_waiter.go`).
	//  * Egalik tekshiruvi 503/400 dan OLDIN (`TestTenancyMatrix`).
	//  * Kursor ichida sir yo'q va u faqat "shu vaqtdan eski" shartini
	//    beradi — restoran sharti har doim tokendan tekshirilgan
	//    yo'ldan olinadi, ya'ni soxta kursor begona buyurtma ochmaydi.
	//  * Xulosa KESHLANMAYDI: chiplardagi sonlar ro'yxat bilan bir
	//    paytdagi holatni ko'rsatsin. Buning o'rniga tezlik chegarasi.
	// └────────────────────────────────────────────────────────────────┘
	mux.HandleFunc("GET /restaurants/{id}/orders/history", s.auth([]users.Role{users.RoleRestaurant, users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID := r.PathValue("id")
			claims := claimsFrom(r)
			if claims.Role == users.RoleRestaurant && claims.EntityID != restaurantID {
				httpError(w, http.StatusForbidden, errors.New("boshqa restoran buyurtmalarini ko'rib bo'lmaydi"))
				return
			}
			if s.HistorySource == nil {
				httpError(w, http.StatusServiceUnavailable, errors.New("buyurtmalar tarixi xizmati ulanmagan"))
				return
			}

			qv := r.URL.Query()
			status, err := stats.ParseHistoryStatus(qv.Get("status"))
			if err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			limit, err := stats.ParseHistoryLimit(qv.Get("limit"))
			if err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			after, err := stats.ParseCursor(qv.Get("cursor"))
			if err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			period, err := stats.ParseHistoryPeriod(qv.Get("from"), qv.Get("to"), time.Now())
			if err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			query := stats.HistoryQuery{Status: status, After: after, Period: period}

			if !s.restaurantExists(w, r, restaurantID) {
				return
			}
			if ok, wait := s.historyLimiter.AllowWithWait(claims.Subject); !ok {
				tooManyRequests(w, wait)
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), statsQueryTimeout)
			defer cancel()

			// limit+1: keyingi sahifa bor-yo'qligini qo'shimcha COUNT'siz bilish.
			list, err := s.HistorySource.OrderHistory(ctx, restaurantID, query, limit+1)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			var resp orderHistoryResponse
			if len(list) > limit {
				list = list[:limit]
				resp.NextCursor = stats.CursorOf(list[len(list)-1]).Encode()
			}
			resp.Orders = s.restaurantOrderViews(ctx, list)

			if after == nil {
				l, err := s.HistorySource.Lifetime(ctx, restaurantID, query.Period)
				if err != nil {
					httpError(w, http.StatusInternalServerError, err)
					return
				}
				resp.Summary = &l
			}
			writeJSON(w, http.StatusOK, resp)
		}))
}

// restaurantExists — yo'q bo'lsa 404 (yoki 500) yozib `false` qaytaradi.
func (s *Server) restaurantExists(w http.ResponseWriter, r *http.Request, restaurantID string) bool {
	if _, err := s.CatalogRepo.GetRestaurant(r.Context(), restaurantID); err != nil {
		if errors.Is(err, catalog.ErrNotFound) {
			httpError(w, http.StatusNotFound, errors.New("restoran topilmadi"))
			return false
		}
		httpError(w, http.StatusInternalServerError, err)
		return false
	}
	return true
}

// restaurantOrderViews — restoran xodimi (yoki admin) O'Z buyurtmalarini
// ko'rayotgandagi shakl: har bir buyurtmaga mijoz telefoni va (bo'lsa)
// kuryer ismi qo'shiladi.
//
// FAQAT egalik tekshiruvidan KEYIN chaqiriladi — bu maydonlar ochiq
// endpointlarda hech qachon ko'rinmaydi (`customerPhoneFor` izohiga
// qarang). Bir sahifada bir mijozning bir nechta buyurtmasi bo'lsa, u
// bir marta so'raladi.
func (s *Server) restaurantOrderViews(ctx context.Context, list []*orders.Order) []map[string]any {
	type courierName struct {
		name  string
		found bool
	}
	phones := map[string]string{}
	couriersByID := map[string]courierName{}

	out := make([]map[string]any, 0, len(list))
	for _, o := range list {
		phone, ok := phones[o.CustomerID]
		if !ok {
			phone = customerPhoneFor(ctx, s.UserRepo, o.CustomerID)
			phones[o.CustomerID] = phone
		}
		entry := withExtraField(o, "customer_phone", phone)
		if o.CourierID != "" {
			c, ok := couriersByID[o.CourierID]
			if !ok {
				if found, err := s.CourierRepo.GetByID(ctx, o.CourierID); err == nil {
					c = courierName{name: found.Name, found: true}
				}
				couriersByID[o.CourierID] = c
			}
			if c.found {
				entry = withExtraField(entry, "courier_name", c.name)
			}
		}
		out = append(out, entry)
	}
	return out
}
