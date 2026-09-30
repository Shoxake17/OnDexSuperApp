package httpapi

import (
	"context"
	"errors"
	"net/http"

	"chustapp/internal/catalog"
	"chustapp/internal/delivery"
	"chustapp/internal/users"
)

// errOutsideServiceArea — manzil hech bir xizmat shahrida emas. Matn
// shaharlarni sanamaydi: ro'yxat `delivery.Cities` da, xabar esa
// shahar qo'shilganda eskirib qolmasligi kerak.
var errOutsideServiceArea = errors.New("bu manzilga hozircha yetkazmaymiz — xizmat hududidan tashqarida")

// checkRestaurantServes — restoran mijoz manzili joylashgan SHAHARGA
// yetkaza oladimi (`delivery.CheckServes` izohiga qarang).
//
// Yetkazish buyurtmasining IKKALA kirish nuqtasi — `POST /orders` va AI
// agent qoralamasi — shu funksiyadan o'tadi: qoidalar ajralib ketmasin.
func (s *Server) checkRestaurantServes(ctx context.Context, restaurantID string, addrLat, addrLng float64) error {
	rest, err := s.CatalogRepo.GetRestaurant(ctx, restaurantID)
	if err != nil {
		return err
	}
	return delivery.CheckServes(rest.Lat, rest.Lng, addrLat, addrLng)
}

// visibleRestaurants — mijoz BUYURTMA BERA OLADIGAN restoranlar.
//
// ┌─ NEGA FILTR AYNAN `delivery.Serves` BILAN ─────────────────────────┐
// U `CheckServes` ning o'zi — buyurtma yaratishda ishlatiladigan
// funksiya. Ikki joyda ikki xil shart yozilsa ro'yxatda ko'rinadigan,
// lekin buyurtma qabul qilmaydigan restoran paydo bo'lardi: odam
// savatni to'ldirib, faqat oxirida "boshqa shahar" xatosini ko'rardi.
// └────────────────────────────────────────────────────────────────────┘
//
// ┌─ KIM HAMMASINI KO'RADI ────────────────────────────────────────────┐
// Manzili yo'q foydalanuvchi (kirmagan, yoki kirgan-u hali manzil
// tanlamagan) TO'LIQ ro'yxatni ko'radi. Sabab: bosh sahifa bo'sh
// ochilmasligi kerak — odam avval nimalar borligini ko'rib, keyin
// manzil tanlaydi. Buyurtma bosqichida tekshiruv baribir ishlaydi.
//
// Bu maxfiylik chegarasi EMAS: restoran ro'yxati ochiq ma'lumot.
// └────────────────────────────────────────────────────────────────────┘
//
// ┌─ ⚠️ XODIM ROLLARI FILTRLANMAYDI ───────────────────────────────────┐
// `GET /restaurants` ni admin paneli HAM o'qiydi
// (`apps/admin_panel/lib/api.dart`). Adminning shaxsiy yetkazish
// manzili (masalan Chustda) saqlangan bo'lsa, filtr uning
// PANELIDAN boshqa shaharlarning restoranlarini yashirib qo'yardi —
// ular boshqarib bo'lmas holga kelardi.
//
// Shuning uchun filtr faqat OVQAT BUYURTMA QILADIGAN rollarga
// qo'llanadi. Yangi rol qo'shilsa u ATAYLAB filtrlanmaydi: bilmasdan
// biror panelni sindirgandan ko'ra, bilmasdan ortiqcha ko'rsatgan
// yaxshiroq (buyurtma bosqichida tekshiruv baribir bor).
// └────────────────────────────────────────────────────────────────────┘
func (s *Server) visibleRestaurants(r *http.Request, list []*catalog.Restaurant) []*catalog.Restaurant {
	c := claimsFrom(r)
	if c == nil || c.Subject == "" {
		return list
	}
	if c.Role != users.RoleCustomer && c.Role != users.RoleCourier {
		return list
	}
	u, err := s.UserRepo.GetByID(r.Context(), c.Subject)
	if err != nil || !delivery.ValidCoords(u.Address.Lat, u.Address.Lng) {
		return list
	}
	out := make([]*catalog.Restaurant, 0, len(list))
	for _, rest := range list {
		if delivery.Serves(rest.Lat, rest.Lng, u.Address.Lat, u.Address.Lng) {
			out = append(out, rest)
		}
	}
	return out
}

// writeServesError — `checkRestaurantServes` xatosini HTTP javobiga
// aylantiradi: boshqa shahar — mijozning xatosi (400), qolgani — ichki.
func writeServesError(w http.ResponseWriter, err error) {
	if errors.Is(err, delivery.ErrOtherCity) {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	httpError(w, http.StatusInternalServerError, err)
}
