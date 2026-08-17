package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"chustapp/internal/couriers"
	"chustapp/internal/orders"
	"chustapp/internal/users"
)

// ┌─ ODAMLAR BO'LIMI (superadmin) ────────────────────────────────────┐
// Mijozlar va affitsiantlar ro'yxati hamda akkauntni o'chirish.
//
// NEGA ALOHIDA FAYL: `routes_admin.go` restoran/kuryer/hisobot
// endpointlari bilan allaqachon to'lgan. Bu yerdagi uchala handler
// bir mavzuga tegishli (odamlar va ularning akkauntlari) va bir xil
// xavfsizlik qoidalariga bo'ysunadi.
//
// MAXFIYLIK: bu endpointlar telefon raqami va emailni qaytaradi, ya'ni
// SHAXSIY MA'LUMOT. Uchalasi ham `users.RoleAdmin` bilan yopilgan va
// boshqa hech qaysi rol ularga yeta olmaydi. `apps/web` proksisining
// ruxsat ro'yxatida (`/api/proxy`) `admin/*` yo'llari YO'Q — ya'ni
// mini-app ichidagi skript bu yerga umuman murojaat qila olmaydi.
// └───────────────────────────────────────────────────────────────────┘

// personRow — panelda bitta qator. Mijoz ham, affitsiant ham bir xil
// shaklda qaytadi: panelda ikkala jadval bir xil kod bilan chiziladi.
type personRow struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	FirstName string    `json:"first_name,omitempty"`
	LastName  string    `json:"last_name,omitempty"`
	Phone     string    `json:"phone,omitempty"`
	Email     string    `json:"email,omitempty"`
	CreatedAt time.Time `json:"created_at"`

	// TelegramLinked — akkaunt Telegram bilan bog'langanmi.
	//
	// TELEGRAM ID'NING O'ZI ATAYLAB QAYTARILMAYDI: u admin uchun
	// hech qanday qaror qabul qilishda kerak emas, lekin oshkor
	// qilinsa foydalanuvchini Telegram'da to'g'ridan-to'g'ri topish
	// mumkin bo'lardi. Bayroq yetarli.
	TelegramLinked bool `json:"telegram_linked"`

	// RestaurantID/RestaurantName — faqat affitsiantlar uchun.
	RestaurantID   string `json:"restaurant_id,omitempty"`
	RestaurantName string `json:"restaurant_name,omitempty"`

	// Devices — qaysi ilovadan kirgani (TMA / mobil ilova / brauzer)
	// va ilova versiyasi. Manba: `user_devices` (migration 0034).
	Devices []users.Device `json:"devices"`
}

func (s *Server) registerAdminUserRoutes(mux *http.ServeMux) {

	// GET /admin/customers — mijozlar ro'yxati va umumiy soni.
	mux.HandleFunc("GET /admin/customers", s.auth([]users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			list, err := s.UserRepo.ListByRole(r.Context(), users.RoleCustomer)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			rows := s.toPersonRows(r.Context(), list)
			writeJSON(w, http.StatusOK, map[string]any{
				"count": len(rows),
				"items": rows,
			})
		}))

	// GET /admin/waiters — affitsiantlar: ismi, qaysi raqamdan kirishi
	// va qaysi restoranga biriktirilgani.
	mux.HandleFunc("GET /admin/waiters", s.auth([]users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			list, err := s.UserRepo.ListByRole(r.Context(), users.RoleWaiter)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			rows := s.toPersonRows(r.Context(), list)

			// Restoran nomlari BITTA so'rov bilan (har qator uchun
			// alohida `GetRestaurant` N+1 bo'lardi).
			if restaurants, err := s.CatalogRepo.ListRestaurants(r.Context()); err == nil {
				nameByID := make(map[string]string, len(restaurants))
				for _, rest := range restaurants {
					nameByID[rest.ID] = rest.Name
				}
				for i := range rows {
					rows[i].RestaurantName = nameByID[rows[i].RestaurantID]
				}
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"count": len(rows),
				"items": rows,
			})
		}))

	// DELETE /admin/users/{id} — akkauntni o'chirish.
	//
	// ┌─ NIMA SODIR BO'LADI ──────────────────────────────────────────┐
	//  1. Sessiya BEKOR QILINADI (`revoke`) — qo'lidagi token shu
	//     zahoti ishlamay qoladi, 30 kunlik muddatini kutmasdan.
	//  2. WebSocket orqali `account_deleted` yuboriladi — ilova ochiq
	//     bo'lsa DARHOL kirish ekraniga qaytadi (foydalanuvchi
	//     keyingi so'rovni kutmaydi).
	//  3. Akkaunt va unga bog'langan shaxsiy ma'lumot o'chiriladi
	//     (sevimlilar, bildirishnomalar, push tokenlari, qurilma
	//     yozuvlari — `PgUserRepo.Delete`).
	//  4. Kuryer bo'lsa kuryer yozuvi ham ro'yxatlardan olinadi
	//     (`SoftDelete` — sabab o'sha yerda).
	//
	// TARTIB MUHIM: avval bekor qilish, keyin o'chirish. Teskarisi
	// bo'lsa, oradagi soniyalarda akkaunt bazada yo'q, LEKIN qo'lidagi
	// token hamon amal qilardi (`auth()` bazaga qaramaydi).
	// └───────────────────────────────────────────────────────────────┘
	mux.HandleFunc("DELETE /admin/users/{id}", s.auth([]users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			id := r.PathValue("id")
			admin := claimsFrom(r)

			// O'ZINI o'chirish — bloklanadi. Bu shunchaki qulaylik
			// emas: superadmin akkaunti faqat `BOOTSTRAP_ADMIN_PHONE`
			// orqali beriladi, ya'ni tasodifan o'chirilsa panelga
			// kirishning yo'li serverga qo'l bilan tegishdan o'tadi.
			if id == admin.Subject {
				httpError(w, http.StatusBadRequest,
					errors.New("o'z akkauntingizni o'chira olmaysiz"))
				return
			}

			u, err := s.UserRepo.GetByID(r.Context(), id)
			if err != nil {
				httpError(w, http.StatusNotFound, users.ErrUserNotFound)
				return
			}

			// Boshqa superadminni ham o'chirib bo'lmaydi. Rol faqat
			// `BOOTSTRAP_ADMIN_PHONE` orqali beriladi (API orqali
			// umuman berilmaydi), shuning uchun uni panel orqali
			// olib tashlash ham noto'g'ri bo'lardi: bitta buzilgan
			// admin sessiyasi qolgan hammasini o'chirib tashlay olardi.
			if u.Role == users.RoleAdmin {
				httpError(w, http.StatusForbidden,
					errors.New("superadmin akkauntini panel orqali o'chirib bo'lmaydi"))
				return
			}

			if err := s.blockIfBusy(r.Context(), u); err != nil {
				httpError(w, http.StatusConflict, err)
				return
			}

			// 1-qadam: sessiyani bekor qilish.
			//
			// `nil` tekshiruvi `auth()` dagi bilan bir xil sababdan:
			// bog'liqlik berilmagan yig'ilishda (masalan qisqartirilgan
			// test serveri) bu yer PANIKA qilib, butun so'rovni
			// tushunarsiz 500 ga aylantirardi.
			if s.Revoked != nil {
				s.Revoked.Revoke(r.Context(), u.ID)
			}

			// 2-qadam: ilova ochiq bo'lsa darhol chiqarib yuborish.
			if s.Hub != nil {
				s.Hub.Send(userTopic(u.ID), map[string]any{
					"type":   "account_deleted",
					"reason": "admin",
				})
			}

			// 3-qadam: kuryer yozuvi (bo'lsa) ro'yxatlardan olinadi.
			// Akkauntdan OLDIN: bu qadam yiqilsa akkaunt hali joyida
			// bo'ladi va amal butunlay qaytariladi (superadmin qayta
			// urinadi). Teskarisi "egasiz kuryer" qoldirardi.
			if u.Role == users.RoleCourier && u.EntityID != "" {
				err := s.CourierRepo.SoftDelete(r.Context(), u.EntityID)
				// `ErrNoCourier` — yozuv allaqachon yo'q yoki allaqachon
				// o'chirilgan. Bu XATO EMAS va rad etilmasligi kerak:
				// birinchi urinishda kuryer yozuvi o'chib, akkaunt
				// o'chirishda nosozlik bo'lgan bo'lsa (masalan DB uzildi),
				// superadmin qayta bosadi — o'shanda bu qadam "topilmadi"
				// qaytaradi va amal MANGU takrorlanmaydigan holatga
				// tushib qolardi.
				if err != nil && !errors.Is(err, couriers.ErrNoCourier) {
					slog.Error("kuryer yozuvini o'chirib bo'lmadi",
						"courier", u.EntityID, "err", err)
					httpError(w, http.StatusInternalServerError,
						errors.New("kuryer yozuvini o'chirib bo'lmadi — akkaunt tegilmadi"))
					return
				}
			}

			// 4-qadam: akkaunt va shaxsiy ma'lumotlar.
			if err := s.UserRepo.Delete(r.Context(), u.ID); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}

			// AUDIT: kim, kimni, qachon. Bu yozuv shaxsiy ma'lumotni
			// (telefon/email) O'Z ICHIGA OLMAYDI — o'chirilgan
			// akkauntning maxfiy ma'lumoti loglarda qolib ketmasligi
			// kerak, aks holda "o'chirdik" degan da'vo yolg'on bo'lardi.
			slog.Info("akkaunt o'chirildi",
				"user", u.ID, "role", u.Role, "by", admin.Subject)

			writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
		}))
}

// blockIfBusy — akkauntni o'chirish HOZIR xavfsizmi.
//
// Yakunlanmagan buyurtma o'rtasida akkauntni o'chirish jonli oqimni
// buzadi: kuryer manzilni ko'rsatuvchi mijozsiz qoladi yoki buyurtma
// hech kim bajara olmaydigan holatda osilib qoladi. Shu sabab bu
// hollarda 409 qaytariladi — superadmin avval buyurtmani yakunlaydi.
func (s *Server) blockIfBusy(ctx context.Context, u *users.User) error {
	switch u.Role {
	case users.RoleCourier:
		if u.EntityID == "" {
			return nil
		}
		if _, err := s.OrderRepo.GetActiveByCourier(ctx, u.EntityID); err == nil {
			return errors.New("bu kuryer hozir buyurtma yetkazyapti — avval o'sha buyurtma yakunlansin")
		} else if !errors.Is(err, orders.ErrNotFound) {
			return err
		}
	case users.RoleCustomer:
		// `ListByCustomer` eng yangilaridan beradi; faol buyurtma
		// har doim shular orasida bo'ladi.
		list, err := s.OrderRepo.ListByCustomer(ctx, u.ID, 20)
		if err != nil {
			return err
		}
		for _, o := range list {
			if !orders.IsTerminal(o.Status) {
				return errors.New("bu mijozning yakunlanmagan buyurtmasi bor — avval u yopilsin")
			}
		}
	}
	return nil
}

// toPersonRows — foydalanuvchilar ro'yxatini panel ko'rinishiga
// aylantiradi va qurilma ma'lumotini BITTA so'rov bilan biriktiradi.
//
// Eng yangi ro'yxatdan o'tganlar YUQORIDA: superadmin odatda "kim
// yangi qo'shildi" degan savol bilan keladi.
func (s *Server) toPersonRows(ctx context.Context, list []*users.User) []personRow {
	rows := make([]personRow, 0, len(list))
	ids := make([]string, 0, len(list))
	for _, u := range list {
		ids = append(ids, u.ID)
		rows = append(rows, personRow{
			ID:             u.ID,
			Name:           u.Name,
			FirstName:      u.FirstName,
			LastName:       u.LastName,
			Phone:          u.Phone,
			Email:          u.Email,
			CreatedAt:      u.CreatedAt,
			TelegramLinked: u.TelegramID != 0,
			RestaurantID:   u.EntityID,
			Devices:        []users.Device{},
		})
	}

	// `Devices` sozlanmagan bo'lsa (xotira rejimi yoki nosozlik)
	// ro'yxat baribir qaytadi — qurilma ustuni bo'sh ko'rinadi.
	// Butun sahifani xatoga chiqarish nomutanosib bo'lardi.
	if s.Devices != nil {
		if byUser, err := s.Devices.ListByUsers(ctx, ids); err == nil {
			for i := range rows {
				if d := byUser[rows[i].ID]; len(d) > 0 {
					rows[i].Devices = d
				}
			}
		} else {
			slog.Warn("qurilma ma'lumotini o'qib bo'lmadi", "err", err)
		}
	}

	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].CreatedAt.After(rows[j].CreatedAt)
	})
	return rows
}
