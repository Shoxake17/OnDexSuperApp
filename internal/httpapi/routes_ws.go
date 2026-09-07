package httpapi

import (
	"chustapp/internal/users"
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
			ticket := s.WsTickets.Issue(ws.TicketClaims{
				Subject:  claims.Subject,
				EntityID: claims.EntityID,
				// Rol AYNAN shu yerda, tekshirilgan tokendan olinadi
				// (izoh: `ws.TicketClaims.Role`).
				Role: string(claims.Role),
			})
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

	// adminTopicIfAdmin — ma'muriyat kanaliga obuna FAQAT `admin`
	// roliga ochiladi.
	//
	// Rol ikkala shoxda ham SERVER manbasidan keladi: Bearer shoxida
	// tekshirilgan JWT'dan, bilet shoxida esa bilet yaratilganda
	// saqlangan qiymatdan (`ws.TicketClaims.Role`). Mijoz uni hech
	// qayerda ayta olmaydi — aks holda bu kanal orqali PLATFORMADAGI
	// BARCHA buyurtmalar oqimi begonaga ochilardi.
	// (adminTopicIfAdmin va wsSubscriptionKeys — shu faylning oxirida,
	// `*Server` metodlari sifatida: ular sof funksiyalar va WebSocket
	// ko'tarmasdan sinaladi — `routes_ws_test.go`.)

	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		// restaurantID bo'lsa, foydalanuvchi kalitlariga QO'SHIMCHA shu
		// restoranning OCHIQ mavzusiga obuna qilinadi — menyu ekrani
		// ochiq turganda restoran mahsulot/aksiya o'zgartirsa, DARHOL
		// (refresh'siz) bilinishi uchun (menu_screen.dart'ga qarang).
		//
		// ┌─ TUZATILGAN NOSOZLIK (bug.md 29-band) ─────────────────────┐
		// Bu yerda AVVAL `restaurantTopic(restaurantID)` turardi —
		// ya'ni XODIM kanali, tekshiruvsiz, so'rov parametridan.
		// Restoran ID'lari `GET /restaurants` da ochiq berilgani uchun
		// istalgan mijoz raqibning butun buyurtma oqimini (summa,
		// stol, kuryer) kuzata olardi; kuryer ID'si ham shu fazoda
		// bo'lgani uchun kuryerning taklif oqimi — restoran manzili va
		// KOORDINATASI — ham ochiq edi.
		//
		// Endi bu parametr FAQAT ochiq kanalga obuna qiladi. Xodim
		// kanaliga obuna quyida, tokendagi `EntityID` dan keladi —
		// ya'ni serverdan, mijozdan emas.
		// └────────────────────────────────────────────────────────────┘
		//
		// Kuryer GPS joylashuvi ham SHAXSIY ma'lumot: `?order_id=` ni
		// bilgan HAR KIM emas, faqat o'sha buyurtmaning HAQIQIY mijozi
		// obuna bo'la oladi (`wsSubscriptionKeys` ichidagi egalik
		// tekshiruvi).
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			claims, err := s.Tokens.Parse(strings.TrimPrefix(h, "Bearer "))
			if err != nil {
				httpError(w, http.StatusUnauthorized, err)
				return
			}
			s.Hub.Serve(w, r, s.wsSubscriptionKeys(r,
				claims.Subject, claims.EntityID, string(claims.Role))...)
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
		// ┌─ TUZATILGAN NOSOZLIK ─────────────────────────────────────┐
		// Bu yerda AVVAL xom ID'lar turardi:
		//
		//	keys := append([]string{claims.Subject, claims.EntityID}, ...)
		//
		// Yuqoridagi Bearer shoxi esa `userTopic(...)`/
		// `restaurantTopic(...)` ishlatadi va xabarlar ham AYNAN
		// o'sha prefiksli kalitlarga yuboriladi (`notify/topic.go`:
		// "u:<id>", "e:food:<id>").
		//
		// Natijada bilet orqali ulanadigan HAR QANDAY klient —
		// brauzer, Telegram Mini App, Flutter web panellari —
		// hech qanday jonli xabar OLMASDI: obuna kalitlari hech
		// qachon mos kelmasdi. Xato jimgina edi, chunki ulanish
		// muvaffaqiyatli o'rnatilardi.
		//
		// Endi ikkala shox ham bir xil funksiyalardan o'tadi.
		// └───────────────────────────────────────────────────────────┘
		s.Hub.Serve(w, r, s.wsSubscriptionKeys(r,
			claims.Subject, claims.EntityID, claims.Role)...)
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

// adminTopicIfAdmin — ma'muriyat kanaliga obuna FAQAT `admin` roliga
// ochiladi.
//
// Rol ikkala shoxda ham SERVER manbasidan keladi: Bearer shoxida
// tekshirilgan JWT'dan, bilet shoxida esa bilet yaratilganda saqlangan
// qiymatdan (`ws.TicketClaims.Role`). Mijoz uni hech qayerda ayta
// olmaydi — aks holda bu kanal orqali PLATFORMADAGI BARCHA buyurtmalar
// oqimi begonaga ochilardi.
func adminTopicIfAdmin(role string) []string {
	if role != string(users.RoleAdmin) {
		return nil
	}
	return []string{adminTopic()}
}

// wsSubscriptionKeys — `GET /ws` ulanishi obuna bo'ladigan kalitlar.
//
// Ikkala shox (Bearer va bilet) AYNAN shu funksiyadan o'tadi: avval
// kalitlar ikki joyda alohida yig'ilardi va shoxlar bir-biridan
// jimgina farq qilib qolardi (`?ticket=` shoxi umuman xom ID
// ishlatardi — hech kim jonli xabar olmasdi).
//
// `userID`, `entityID` va `role` — HAR DOIM server manbasidan
// (tekshirilgan JWT yoki bilet). So'rov parametrlaridan faqat
// `restaurant_id` (ochiq kanal) va `order_id` (egalik tekshiruvi
// bilan) olinadi — bug.md 29-band shu joyda edi.
func (s *Server) wsSubscriptionKeys(r *http.Request, userID, entityID, role string) []string {
	keys := []string{
		userTopic(userID),
		restaurantTopic(entityID), // kuryer/restoran/xodim ISH kanali
	}
	// ?restaurant_id= — FAQAT ochiq kanal (menyu/aksiya/3D signali).
	// Bu ma'lumot GET endpointlarida allaqachon ochiq.
	if restaurantID := r.URL.Query().Get("restaurant_id"); restaurantID != "" {
		keys = append(keys, restaurantPublicTopic(restaurantID))
	}
	// ?order_id= — kuryerning JONLI GPS'i shaxsiy ma'lumot: faqat shu
	// buyurtmaning HAQIQIY mijozi obuna bo'la oladi.
	if orderID := r.URL.Query().Get("order_id"); orderID != "" && s.OrderRepo != nil {
		o, err := s.OrderRepo.GetByID(r.Context(), orderID)
		if err == nil && o.CustomerID == userID {
			keys = append(keys, orderTopic(orderID))
		}
	}
	return append(keys, adminTopicIfAdmin(role)...)
}
