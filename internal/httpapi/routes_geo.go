package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"chustapp/internal/tracking"
)

func (s *Server) registerGeoRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /config/maps", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			key := os.Getenv("GOOGLE_MAPS_API_KEY")
			if key == "" {
				httpError(w, http.StatusServiceUnavailable,
					errors.New("xarita kaliti sozlanmagan (.env: GOOGLE_MAPS_API_KEY)"))
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"maps_api_key": key})
		}))

	// GET /geocode/reverse?lat=..&lng=.. — koordinatani manzil matniga
	// aylantiradi. Google Geocoding API'ni SERVERDAN chaqiramiz (brauzerdan
	// to'g'ridan-to'g'ri chaqirish CORS tomonidan bloklanadi — Google bu
	// API'ni faqat server-server foydalanish uchun ochiq qilgan). Shu bilan
	// birga API kaliti hech qachon geokodlash uchun frontend'ga chiqmaydi.
	mux.HandleFunc("GET /geocode/reverse", s.auth(nil,
		rateLimitedGeo(func(w http.ResponseWriter, r *http.Request) {
			lat, errLat := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
			lng, errLng := strconv.ParseFloat(r.URL.Query().Get("lng"), 64)
			if errLat != nil || errLng != nil {
				httpError(w, http.StatusBadRequest, errors.New("lat/lng noto'g'ri"))
				return
			}
			// DIQQAT: bu yerda GOOGLE_MAPS_API_KEY emas, ALOHIDA
			// GOOGLE_GEOCODING_API_KEY ishlatiladi — Google Geocoding API
			// referrer-cheklangan kalitlarni butunlay rad etadi (faqat
			// server-server foydalanish uchun mo'ljallangan), shu bilan
			// birga Maps JavaScript API kaliti xavfsizlik uchun HTTP
			// referrer bilan cheklangan bo'lishi SHART — ikkalasi bitta
			// kalitda birga bo'la olmaydi.
			key := os.Getenv("GOOGLE_GEOCODING_API_KEY")
			if key == "" {
				httpError(w, http.StatusServiceUnavailable,
					errors.New("geokodlash kaliti sozlanmagan (.env: GOOGLE_GEOCODING_API_KEY)"))
				return
			}
			geoURL := "https://maps.googleapis.com/maps/api/geocode/json" +
				"?latlng=" + strconv.FormatFloat(lat, 'f', -1, 64) +
				"," + strconv.FormatFloat(lng, 'f', -1, 64) +
				"&language=uz&key=" + key
			// `fetchGeoJSON` — timeout, xato tekshiruvi va hajm
			// chegarasi bir joyda (`geoclient.go`, bug.md 21-band).
			var data geocodeResponse
			if err := fetchGeoJSON(r.Context(), geoURL, &data); err != nil {
				httpError(w, http.StatusBadGateway, err)
				return
			}
			if data.Status != "OK" && data.Status != "ZERO_RESULTS" {
				slog.Warn("Geocoding API kutilmagan status qaytardi", "status", data.Status)
			}
			address, hasRoute := shortAddress(data)
			// Google aniq ko'cha topa olmasa (faqat mahalla/tuman/shahar
			// darajasi) — navbat bilan Yandex, keyin 2GIS'dan ham so'raymiz
			// (tegishli kalit sozlangan bo'lsagina; bo'lmasa jimgina
			// o'tkazib yuboriladi, xato bermaydi).
			if !hasRoute {
				if yandexAddr, ok := yandexReverseGeocode(r.Context(), lat, lng); ok {
					address = yandexAddr
					hasRoute = true
				}
			}
			if !hasRoute {
				if dgisAddr, ok := dgisReverseGeocode(r.Context(), lat, lng); ok {
					address = dgisAddr
				}
			}
			writeJSON(w, http.StatusOK, map[string]string{"address": address})
		})))

	// GET /geocode/autocomplete?input=.. — manzil qidiruv takliflari (Google
	// Places Autocomplete), Chust atrofiga moslashtirilgan (location+radius) —
	// aks holda boshqa shahardagi bir xil nomli ko'cha ham chiqishi mumkin.
	mux.HandleFunc("GET /geocode/autocomplete", s.auth(nil,
		rateLimitedGeo(func(w http.ResponseWriter, r *http.Request) {
			input := strings.TrimSpace(r.URL.Query().Get("input"))
			if input == "" {
				writeJSON(w, http.StatusOK, []map[string]string{})
				return
			}
			key := os.Getenv("GOOGLE_GEOCODING_API_KEY")
			if key == "" {
				httpError(w, http.StatusServiceUnavailable,
					errors.New("geokodlash kaliti sozlanmagan (.env: GOOGLE_GEOCODING_API_KEY)"))
				return
			}
			acURL := "https://maps.googleapis.com/maps/api/place/autocomplete/json" +
				"?input=" + url.QueryEscape(input) +
				"&language=uz&location=41.0030,71.2360&radius=15000&key=" + key
			var data struct {
				Status      string `json:"status"`
				Predictions []struct {
					Description string `json:"description"`
					PlaceID     string `json:"place_id"`
				} `json:"predictions"`
			}
			if err := fetchGeoJSON(r.Context(), acURL, &data); err != nil {
				httpError(w, http.StatusBadGateway, err)
				return
			}
			// Google xatosini foydalanuvchiga ko'rsatmaymiz (ichki tafsilot
			// sizib chiqmasligi kerak), lekin serverda kuzatib borish uchun
			// log yozamiz — aks holda "bo'sh natija" sababi bilinmay qoladi.
			if data.Status != "OK" && data.Status != "ZERO_RESULTS" {
				slog.Warn("Places Autocomplete kutilmagan status qaytardi",
					"status", data.Status)
			}
			out := make([]map[string]string, 0, len(data.Predictions))
			for _, p := range data.Predictions {
				out = append(out, map[string]string{
					"description": p.Description,
					"place_id":    p.PlaceID,
				})
			}
			writeJSON(w, http.StatusOK, out)
		})))

	// GET /geocode/place?place_id=.. — tanlangan taklif uchun aniq koordinata
	// va to'liq manzil (Google Place Details).
	mux.HandleFunc("GET /geocode/place", s.auth(nil,
		rateLimitedGeo(func(w http.ResponseWriter, r *http.Request) {
			placeID := strings.TrimSpace(r.URL.Query().Get("place_id"))
			if placeID == "" {
				httpError(w, http.StatusBadRequest, errors.New("place_id kerak"))
				return
			}
			key := os.Getenv("GOOGLE_GEOCODING_API_KEY")
			if key == "" {
				httpError(w, http.StatusServiceUnavailable,
					errors.New("geokodlash kaliti sozlanmagan (.env: GOOGLE_GEOCODING_API_KEY)"))
				return
			}
			detURL := "https://maps.googleapis.com/maps/api/place/details/json" +
				"?place_id=" + url.QueryEscape(placeID) +
				"&fields=geometry,formatted_address&language=uz&key=" + key
			var data struct {
				Status string `json:"status"`
				Result struct {
					FormattedAddress string `json:"formatted_address"`
					Geometry         struct {
						Location struct {
							Lat float64 `json:"lat"`
							Lng float64 `json:"lng"`
						} `json:"location"`
					} `json:"geometry"`
				} `json:"result"`
			}
			if err := fetchGeoJSON(r.Context(), detURL, &data); err != nil {
				httpError(w, http.StatusBadGateway, err)
				return
			}
			if data.Status != "OK" {
				httpError(w, http.StatusBadGateway, errors.New("manzil topilmadi"))
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"lat":     data.Result.Geometry.Location.Lat,
				"lng":     data.Result.Geometry.Location.Lng,
				"address": data.Result.FormattedAddress,
			})
		})))

	// GET /geocode/route?origin_lat=..&origin_lng=..&dest_lat=..&dest_lng=..&mode=driving
	// — ikki nuqta orasidagi HAQIQIY yo'l marshrutini (Google Directions
	// API) qaytaradi: xaritada chizish uchun nuqtalar ro'yxati + umumiy
	// masofa/vaqt. Kuryer ilovasi buni kuryerdan restoran/mijozgacha
	// chiziq chizish uchun ishlatadi (Yandex Go/Google Maps navigatordagi
	// kabi) — avval bu funksiya UMUMAN yo'q edi, xarita faqat belgilarni
	// (markerlarni) ko'rsatardi, ular orasida hech qanday chiziq yo'q edi.
	mux.HandleFunc("GET /geocode/route", s.auth(nil,
		rateLimitedGeo(func(w http.ResponseWriter, r *http.Request) {
			originLat, errOLat := strconv.ParseFloat(r.URL.Query().Get("origin_lat"), 64)
			originLng, errOLng := strconv.ParseFloat(r.URL.Query().Get("origin_lng"), 64)
			destLat, errDLat := strconv.ParseFloat(r.URL.Query().Get("dest_lat"), 64)
			destLng, errDLng := strconv.ParseFloat(r.URL.Query().Get("dest_lng"), 64)
			if errOLat != nil || errOLng != nil || errDLat != nil || errDLng != nil {
				httpError(w, http.StatusBadRequest,
					errors.New("origin_lat/origin_lng/dest_lat/dest_lng noto'g'ri"))
				return
			}
			// Google so'rovi `directions.go` da — mijoz kuzatuvi
			// (`GET /orders/{id}/tracking`) ham AYNAN shu funksiyadan o'tadi.
			res, err := s.routeBetween(r.Context(), directionsCourierRoute, claimsFrom(r).Subject,
				tracking.Point{Lat: originLat, Lng: originLng},
				tracking.Point{Lat: destLat, Lng: destLng},
				r.URL.Query().Get("mode"))
			switch {
			case errors.Is(err, errDirectionsKeyMissing):
				httpError(w, http.StatusServiceUnavailable, err)
				return
			case errors.Is(err, errNoRoute):
				httpError(w, http.StatusNotFound, err)
				return
			case err != nil:
				httpError(w, http.StatusBadGateway, err)
				return
			}
			steps := res.Steps
			if steps == nil {
				steps = []RouteStep{}
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"points":           res.Points,
				"distance_meters":  res.DistanceMeters,
				"duration_seconds": res.DurationSeconds,
				// Kuryer ilovasining ovozli yo'l ko'rsatishi uchun.
				"steps": steps,
			})
		})))

	// GET /me — login qilgan foydalanuvchining o'z ma'lumotlari (profil
	// sahifasi uchun: telefon, ism, rol). Parol/kod kabi maxfiy narsa
	// users.User'da umuman saqlanmaydi, shuning uchun to'liq obyektni
	// qaytarish xavfsiz (bir xil ma'lumot /auth/verify'da ham qaytadi).
}
