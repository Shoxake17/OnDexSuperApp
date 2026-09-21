package httpapi

import (
	"context"
	"errors"
	"net/http"

	"chustapp/internal/delivery"
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

// writeServesError — `checkRestaurantServes` xatosini HTTP javobiga
// aylantiradi: boshqa shahar — mijozning xatosi (400), qolgani — ichki.
func writeServesError(w http.ResponseWriter, err error) {
	if errors.Is(err, delivery.ErrOtherCity) {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	httpError(w, http.StatusInternalServerError, err)
}
