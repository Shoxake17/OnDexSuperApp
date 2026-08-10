package httpapi

import (
	"chustapp/internal/ws"
	"errors"
	"net/http"
	"strings"
)

func (s *Server) registerWsRoutes(mux *http.ServeMux) {
	// POST /ws/ticket — GET /ws uchun qisqa muddatli (30s), bir martalik
	// ulanish bileti. Nega kerak: brauzer JavaScript WebSocket API'si
	// handshake'da maxsus header (Authorization) qo'yishga umuman ruxsat
	// bermaydi — shuning uchun avval bu yerda (oddiy HTTP, Authorization
	// header orqali, xavfsiz) qisqa bilet olinadi, keyin WebSocket'ga shu
	// bilet bilan ulaniladi. Uzoq muddatli (30 kunlik) asosiy JWT hech
	// qachon URL'da/loglarda ko'rinmaydi (internal/ws/tickets.go'ga qarang).
	mux.HandleFunc("POST /ws/ticket", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			claims := claimsFrom(r)
			ticket := s.WsTickets.Issue(ws.TicketClaims{Subject: claims.Subject, EntityID: claims.EntityID})
			writeJSON(w, http.StatusCreated, map[string]string{"ticket": ticket})
		}))

	// GET /ws — jonli eventlar oqimi. Brauzer bo'lmagan (native mobil)
	// klientlar handshake'da Authorization header qo'ya oladi — shular
	// uchun to'g'ridan-to'g'ri header qabul qilinadi. Brauzer klientlari
	// (buni qila olmaydi) avval POST /ws/ticket orqali bilet olib, uni
	// ?ticket=... sifatida yuboradi — xom uzoq muddatli token endi
	// HECH QACHON query'da yurmaydi.
	// restaurantTopic — menyu/aksiya jonli yangilanishlari uchun umumiy
	// kalit (foydalanuvchiga emas, RESTORANga bog'liq) — shu restoran
	// menyusini ochib turgan HAMMA mijozlarga bir vaqtda yetkaziladi
	// (pastdagi ?restaurant_id= parametriga qarang).

	// orderTopic — kuryerning JONLI GPS joylashuvini shu buyurtmani kuzatib
	// turgan mijozga yetkazish uchun (pastdagi ?order_id= parametriga va
	// POST /couriers/{id}/location handler'idagi s.Hub.Send chaqiruviga
	// qarang).

	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		// restaurantID bo'lsa, foydalanuvchi kalitlariga QO'SHIMCHA shu
		// restoranning umumiy mavzusiga ham obuna qilinadi — menyu ekrani
		// ochiq turganda restoran mahsulot/aksiya o'zgartirsa, DARHOL
		// (refresh'siz) bilinishi uchun (menu_screen.dart'ga qarang). Bu
		// PUBLIC ma'lumot (menyu/aksiya GET endpointlari ham ochiq),
		// shuning uchun egalik tekshiruvi shart emas.
		var baseTopics []string
		if restaurantID := r.URL.Query().Get("restaurant_id"); restaurantID != "" {
			baseTopics = append(baseTopics, restaurantTopic(restaurantID))
		}
		orderID := r.URL.Query().Get("order_id")
		// orderTopicIfOwned — kuryer GPS joylashuvi SHAXSIY ma'lumot —
		// buyurtma ID'sini bilgan HAR KIM emas, faqat o'sha buyurtmaning
		// HAQIQIY mijozi shu mavzuga obuna bo'lishi mumkin (aks holda
		// tasodifiy/taxmin qilingan order_id bilan begona odam boshqa
		// mijozning kuryerini kuzatib turishi mumkin bo'lardi).
		orderTopicIfOwned := func(customerID string) []string {
			if orderID == "" {
				return nil
			}
			o, err := s.OrderRepo.GetByID(r.Context(), orderID)
			if err != nil || o.CustomerID != customerID {
				return nil
			}
			return []string{orderTopic(orderID)}
		}
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			claims, err := s.Tokens.Parse(strings.TrimPrefix(h, "Bearer "))
			if err != nil {
				httpError(w, http.StatusUnauthorized, err)
				return
			}
			keys := append([]string{
				userTopic(claims.Subject),
				restaurantTopic(claims.EntityID), // kuryer/restoran ish kanali
			}, baseTopics...)
			keys = append(keys, orderTopicIfOwned(claims.Subject)...)
			s.Hub.Serve(w, r, keys...)
			return
		}
		ticket := r.URL.Query().Get("ticket")
		if ticket == "" {
			httpError(w, http.StatusUnauthorized,
				errors.New("Authorization header yoki ?ticket= (avval POST /ws/ticket orqali olinadi) talab qilinadi"))
			return
		}
		claims, ok := s.WsTickets.Consume(ticket)
		if !ok {
			httpError(w, http.StatusUnauthorized, errors.New("bilet yaroqsiz yoki muddati tugagan"))
			return
		}
		keys := append([]string{claims.Subject, claims.EntityID}, baseTopics...)
		keys = append(keys, orderTopicIfOwned(claims.Subject)...)
		s.Hub.Serve(w, r, keys...)
	})

	// ---------- Katalog ----------

	// GET /restaurants — ochiq: mijoz ilovasining bosh sahifasi. Eng ko'p
	// so'raladigan endpoint (har mijoz ilova ochganda) — shuning uchun
	// Redis'da qisqa muddatga (30s) keshlanadi. Restoran ma'lumoti
	// o'zgarganda (yaratish/tahrirlash/ochiq-yopiq) kesh darhol tozalanadi
	// (restaurantsCacheKey konstantasiga qarang), shuning uchun 30s —
	// "eng yomon holatda shuncha eskirishi mumkin" chegarasi, oddiy TTL
	// emas.
}
