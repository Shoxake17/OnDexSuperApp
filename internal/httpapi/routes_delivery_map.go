package httpapi

import (
	"bytes"
	"embed"
	"errors"
	"net/http"
	"strings"

	"chustapp/internal/users"
)

// ┌─ RS-1: RESTORAN PANELIDA YETKAZISH XARITASI ──────────────────────────┐
// GET /restaurants/{id}/delivery-map — restoranning yo'ldagi yetkazmalari:
// kuryer ismi, jonli joylashuvi, buyurtma raqami/holati va borish nuqtasi.
// Faqat shu restoran (yoki admin); yakunlangan buyurtma va kuryersizlari
// kirmaydi (`courierLocationShared` bilan bir xil qoida).
//
// GET /courier-map — xarita SAHIFASI (OnDexMap, MapLibre — Google kaliti
// kerak emas). Sahifada sir YO'Q va u o'zi API chaqirmaydi: ma'lumotni
// panel `postMessage` bilan uzatadi (web — iframe, Windows — WebView2).
// Shuning uchun sahifaga token kerak emas va u auth'siz beriladi.
// └──────────────────────────────────────────────────────────────────────┘

//go:embed assets/courier_map.html
var courierMapFS embed.FS

func (s *Server) registerDeliveryMapRoutes(mux *http.ServeMux) {
	staff := []users.Role{users.RoleRestaurant, users.RoleAdmin}

	mux.HandleFunc("GET /restaurants/{id}/delivery-map", s.auth(staff,
		func(w http.ResponseWriter, r *http.Request) {
			rid, ok := entityIDFor(claimsFrom(r), r.PathValue("id"))
			if !ok {
				httpError(w, http.StatusNotFound, errors.New("topilmadi"))
				return
			}
			out := map[string]any{"couriers": []map[string]any{}}
			if s.CatalogRepo != nil {
				if rest, err := s.CatalogRepo.GetRestaurant(r.Context(), rid); err == nil {
					out["restaurant"] = map[string]any{"name": rest.Name, "lat": rest.Lat, "lng": rest.Lng}
				}
			}
			list, err := s.OrderRepo.ListByRestaurant(r.Context(), rid, 200)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			couriers := []map[string]any{}
			for _, o := range list {
				if !courierLocationShared(o) || s.CourierRepo == nil {
					continue
				}
				cr, err := s.CourierRepo.GetByID(r.Context(), o.CourierID)
				if err != nil {
					continue
				}
				c := map[string]any{
					"order_id": o.ID, "order_number": o.OrderNumber, "status": o.Status,
					"courier_name": strings.TrimSpace(cr.Name),
					"delivery":     map[string]float64{"lat": o.DeliveryLat, "lng": o.DeliveryLng},
				}
				if cr.Lat != 0 || cr.Lng != 0 {
					c["location"] = map[string]float64{"lat": cr.Lat, "lng": cr.Lng}
				}
				couriers = append(couriers, c)
			}
			out["couriers"] = couriers
			writeJSON(w, http.StatusOK, out)
		}))

	page, err := courierMapFS.ReadFile("assets/courier_map.html")
	if err != nil {
		panic("courier_map.html embed qilinmadi: " + err.Error())
	}
	// OnDexMap uslubi sahifaga build/ishga tushishda yoziladi (sir emas).
	page = bytes.ReplaceAll(page, []byte("__STYLE_URL__"), []byte(s.ondexMapBase()+"/tiles/style.json"))
	head, tail, found := bytes.Cut(page, []byte(cspNoncePlaceholder))
	if !found {
		panic("courier_map.html ichida " + cspNoncePlaceholder + " yo'q")
	}
	ondexConnect := "https://*.ondex.uz"
	if o := originOf(s.ondexMapBase()); o != "" {
		ondexConnect += " " + o
	}
	mux.HandleFunc("GET /courier-map", func(w http.ResponseWriter, r *http.Request) {
		nonce, err := randomNonce()
		if err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		// map-picker bilan bir xil tor siyosat; farqi — freym qilinadi:
		// web panel (merchant.ondex.uz/r/) uni iframe'da ochadi.
		w.Header().Set("Content-Security-Policy",
			"default-src 'none'; "+
				"script-src 'nonce-"+nonce+"' 'strict-dynamic'; "+
				"style-src 'unsafe-inline' 'self'; "+
				"img-src 'self' data: blob: https://*.ondex.uz; "+
				"connect-src 'self' "+ondexConnect+"; "+
				"font-src 'self' https://*.ondex.uz; "+
				"worker-src 'self' blob:; child-src blob:; "+
				"frame-ancestors 'self' "+s.panelWebOrigin()+"; base-uri 'none'")
		// Global `X-Frame-Options: DENY` olib tashlanadi — freymlashni
		// yuqoridagi `frame-ancestors` boshqaradi (faqat panel manbasi).
		w.Header().Del("X-Frame-Options")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(head)
		_, _ = w.Write([]byte(nonce))
		_, _ = w.Write(tail)
	})
}
