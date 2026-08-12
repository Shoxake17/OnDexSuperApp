// Affitsiant ilovasining API yuzasi.
//
// ┌─ NEGA ALOHIDA ENDPOINT ───────────────────────────────────────────┐
// Affitsiant `GET /restaurants/{id}/orders` dan foydalana OLMAYDI: u
// restoranning HAMMA buyurtmalarini (yetkazish ham) va har biriga
// mijoz telefon raqamini qo'shib qaytaradi. Affitsiantga bularning
// hech biri kerak emas.
//
// Bu yerda esa faqat kerakli minimum: qaysi stol, nechta kishi, nima
// buyurtma qilingan va holati. Mijoz telefoni, manzili, koordinatasi
// UMUMAN yo'q — eng kam huquq (least privilege) prinsipi.
// └───────────────────────────────────────────────────────────────────┘
package httpapi

import (
	"errors"
	"net/http"

	"chustapp/internal/orders"
	"chustapp/internal/users"
)

// waiterOrderView — affitsiant KO'RADIGAN maydonlar.
//
// Ataylab `orders.Order` ni to'g'ridan-to'g'ri qaytarmaymiz: unda
// `CustomerID`, `DeliveryLat/Lng`, `DeliveryAddress` bor va yangi
// maydon qo'shilganda u avtomatik ravishda affitsiantga ham
// ko'rinib ketardi. Aniq ro'yxat esa "sukut bo'yicha yopiq".
func waiterOrderView(o *orders.Order) map[string]any {
	return map[string]any{
		"id":           o.ID,
		"order_number": o.OrderNumber,
		"table_label":  o.TableLabel,
		"party_size":   o.PartySize,
		"items":        o.Items,
		"total_tiyin":  o.TotalTiyin,
		"status":       o.Status,
		"created_at":   o.CreatedAt,
		"updated_at":   o.UpdatedAt,
		"ready_at":     o.ReadyAt,
	}
}

func (s *Server) registerWaiterRoutes(mux *http.ServeMux) {
	// GET /waiter/orders — affitsiant ilovasining asosiy ro'yxati.
	//
	// Qaytadi: shu restoranning FAOL stol buyurtmalari. Yakunlangan
	// (served/cancelled/rejected) buyurtmalar ro'yxatni to'ldirib
	// yubormasligi uchun chiqarib tashlanadi — affitsiantga "hozir
	// nima qilish kerak" ko'rinishi muhim.
	mux.HandleFunc("GET /waiter/orders", s.auth([]users.Role{users.RoleWaiter},
		func(w http.ResponseWriter, r *http.Request) {
			claims := claimsFrom(r)
			if claims.EntityID == "" {
				httpError(w, http.StatusForbidden,
					errors.New("akkaunt hech qaysi restoranga biriktirilmagan"))
				return
			}
			list, err := s.OrderRepo.ListByRestaurant(r.Context(), claims.EntityID, 100)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			out := make([]map[string]any, 0, len(list))
			for _, o := range list {
				if !o.IsDineIn() || o.IsTerminal() {
					continue
				}
				out = append(out, waiterOrderView(o))
			}
			writeJSON(w, http.StatusOK, out)
		}))

	// GET /waiter/me — affitsiant qaysi restoranda ishlashini bilishi
	// uchun (ilova sarlavhasida restoran nomi ko'rsatiladi).
	mux.HandleFunc("GET /waiter/me", s.auth([]users.Role{users.RoleWaiter},
		func(w http.ResponseWriter, r *http.Request) {
			claims := claimsFrom(r)
			resp := map[string]any{
				"restaurant_id": claims.EntityID,
			}
			if rest, err := s.CatalogRepo.GetRestaurant(r.Context(), claims.EntityID); err == nil {
				resp["restaurant_name"] = rest.Name
				resp["restaurant_logo_url"] = rest.LogoURL
			}
			writeJSON(w, http.StatusOK, resp)
		}))
}
