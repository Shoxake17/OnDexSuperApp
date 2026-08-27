// Taom uchun 3D model (AI generatsiyasi) endpointlari.
//
// ┌─ XAVFSIZLIK: NEGA HAMMASI SERVER ORQALI ──────────────────────────┐
// Tripo API kaliti FAQAT serverda (`.env` → `TRIPO_API_KEY`). Restoran
// paneli — Flutter Windows ilovasi, ya'ni uning `.exe` fayliga qo'yilgan
// kalitni istalgan odam ajratib olib, boshqa loyihaning hisobidan
// kredit sarflay olardi.
//
// Shuning uchun panel FAQAT o'z serveriga murojaat qiladi, server esa
// tashqi xizmat bilan gaplashadi. Kalit hech qachon tarmoqqa mijozga
// qarab ketmaydi.
// └───────────────────────────────────────────────────────────────────┘
package httpapi

import (
	"errors"
	"net/http"

	"chustapp/internal/catalog"
	"chustapp/internal/model3d"
	"chustapp/internal/users"
)

func (s *Server) registerModel3DRoutes(mux *http.ServeMux) {
	staff := []users.Role{users.RoleRestaurant, users.RoleAdmin}

	// POST /restaurants/{id}/products/{productId}/model3d
	// Generatsiyani boshlaydi. TEZ qaytadi (202): model bir necha
	// daqiqada tayyor bo'ladi va panel WebSocket orqali xabar oladi.
	mux.HandleFunc("POST /restaurants/{id}/products/{productId}/model3d", s.auth(staff,
		func(w http.ResponseWriter, r *http.Request) {
			p, restaurantID, ok := s.productForStaff(w, r)
			if !ok {
				return
			}

			// ┌─ TEZLIK CHEGARASI ────────────────────────────────────┐
			// Har chaqiruv TASHQI XIZMATDA PUL sarflaydi (~$0.20-0.35).
			// Chegarasiz: buzilgan panel (yoki o'g'irlangan token)
			// menyudagi hamma taomni qayta-qayta generatsiya qilib,
			// hisobni bir necha daqiqada bo'shatib qo'yardi.
			//
			// Kalit — RESTORAN bo'yicha (IP emas): bitta restoranning
			// bir nechta xodimi bo'lishi mumkin, lekin hisob bitta.
			// └───────────────────────────────────────────────────────┘
			if s.Model3DLimiter != nil && !s.Model3DLimiter.Allow("model3d:"+restaurantID) {
				httpError(w, http.StatusTooManyRequests,
					errors.New("juda ko'p so'rov — biroz kuting va qayta urining"))
				return
			}

			if err := s.Model3D.Start(r.Context(), p); err != nil {
				httpError(w, model3dStatus(err), err)
				return
			}
			writeJSON(w, http.StatusAccepted, map[string]any{
				"status":     p.Model3DStatus,
				"product_id": p.ID,
			})
		}))

	// DELETE /restaurants/{id}/products/{productId}/model3d
	// Modelni taomdan uzadi (masalan natija sifatsiz chiqqan bo'lsa).
	mux.HandleFunc("DELETE /restaurants/{id}/products/{productId}/model3d", s.auth(staff,
		func(w http.ResponseWriter, r *http.Request) {
			p, _, ok := s.productForStaff(w, r)
			if !ok {
				return
			}
			if err := s.Model3D.Remove(r.Context(), p); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]bool{"removed": true})
		}))
}

// productForStaff — yo'ldagi taomni oladi va CHAQIRUVCHI unga haqli
// ekanini tekshiradi.
//
// Uchta tekshiruv bir joyda (ikkala endpoint ham shuni ishlatadi —
// takrorlanmasin):
//  1. xizmat umuman yoqilganmi (`Model3D == nil` → 503);
//  2. restoran O'Z restoranini so'rayaptimi;
//  3. taom AYNAN shu restoranga tegishlimi.
//
// Uchinchisi shart: ikkinchisisiz ham restoran o'z ID'sini yozib,
// boshqa restoranning taom ID'sini qo'shib yuborishi mumkin edi.
func (s *Server) productForStaff(w http.ResponseWriter, r *http.Request) (*catalog.Product, string, bool) {
	if s.Model3D == nil {
		httpError(w, http.StatusServiceUnavailable,
			errors.New("3D model xizmati sozlanmagan"))
		return nil, "", false
	}

	restaurantID := r.PathValue("id")
	productID := r.PathValue("productId")
	claims := claimsFrom(r)
	if claims.Role == users.RoleRestaurant && claims.EntityID != restaurantID {
		httpError(w, http.StatusForbidden,
			errors.New("boshqa restoran menyusini o'zgartirib bo'lmaydi"))
		return nil, "", false
	}

	products, err := s.CatalogRepo.GetProductsByIDs(r.Context(), []string{productID})
	if err != nil {
		httpError(w, http.StatusInternalServerError, err)
		return nil, "", false
	}
	if len(products) == 0 {
		httpError(w, http.StatusNotFound, catalog.ErrNotFound)
		return nil, "", false
	}
	if products[0].RestaurantID != restaurantID {
		httpError(w, http.StatusForbidden,
			errors.New("bu taom sizning restoraningizga tegishli emas"))
		return nil, "", false
	}
	return products[0], restaurantID, true
}

// model3dStatus — domen xatosini HTTP kodiga o'giradi.
//
// Matn `model3d` paketida yozilgan (foydalanuvchiga tushunarli),
// bu yerda faqat kod tanlanadi — xabar ikki joyda takrorlanmaydi.
func model3dStatus(err error) int {
	switch {
	case errors.Is(err, model3d.ErrNoImage):
		return http.StatusBadRequest
	case errors.Is(err, model3d.ErrBusy):
		return http.StatusConflict
	case errors.Is(err, model3d.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, model3d.ErrNoCredit):
		// 402: bu server nosozligi EMAS, hisobdagi mablag' holati.
		// 5xx bo'lsa xabar yashirilardi (`httpError`) va restoran
		// "server xatosi" ni ko'rib, sababini bilmasdan qayta-qayta
		// urinaverardi.
		return http.StatusPaymentRequired
	case errors.Is(err, model3d.ErrProviderBusy):
		return http.StatusTooManyRequests
	default:
		return http.StatusBadGateway // tashqi xizmat javob bermadi
	}
}
