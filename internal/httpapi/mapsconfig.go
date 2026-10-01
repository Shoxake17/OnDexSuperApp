package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"chustapp/internal/mapprovider"
	"chustapp/internal/ondexmap"
	"chustapp/internal/tracking"
)

// `GOOGLE_MAP` bayrog'i: hamma xarita funksiyasi (manzil qidirish, manzil
// aniqlash, yo'l) qaysi manbadan olinishi shu fayldagi mantiqqa tayanadi.
//
//	GOOGLE_MAP=true  → Google (standart)
//	GOOGLE_MAP=false → OnDexMap; u javob bermasa — Google'ga qaytish
//
// Qoida: OnDexMap rejimida ham foydalanuvchi XATO ko'rmasligi kerak. Shu
// sabab har bir OnDexMap chaqiruvi muvaffaqiyatsiz bo'lsa, o'sha so'rov
// Google zanjiriga o'tadi (agar Google kaliti bor bo'lsa).

const (
	// ondexPlacePrefix — OnDexMap taklifining `place_id` si. Koordinata va
	// manzil matni ID ichida turadi, shuning uchun `/geocode/place` ga
	// qo'shimcha so'rov yoki server holati kerak emas:
	// ondex:<lat>,<lng>:<matn>
	ondexPlacePrefix = "ondex:"

	// ondexSuggestLimit — qidiruv taklifi soni (Google Autocomplete ~5).
	ondexSuggestLimit = 8

	// maxPlaceTextRunes — `place_id` ichidagi manzil matni chegarasi (kiruvchi
	// ma'lumot, shuning uchun cheklanadi).
	maxPlaceTextRunes = 200

	// Yo'lovchi tezligi: OnDexMap marshruti faqat avtomobil uchun (OSRM
	// driving), piyoda/velosiped vaqti masofadan hisoblanadi.
	walkSpeedMS = 4.0 / 3.6
	bikeSpeedMS = 15.0 / 3.6

	// Chust markazi — qidiruvda "yaqinroq" natijani oldinga chiqarish uchun.
	biasLat, biasLng = 41.0030, 71.2360

	// Manzil koordinatasi O'zbekiston ichida bo'lishi shart.
	uzMinLat, uzMaxLat = 37.0, 45.8
	uzMinLng, uzMaxLng = 55.5, 73.5

	// sourceOndexMap — DirectionsResult.Source: yo'l OnDexMap'dan kelgan.
	sourceOndexMap = "ondexmap"
)

// useOndexMap — OnDexMap rejimi yoqilgan VA mijoz sozlangan.
func (s *Server) useOndexMap() bool {
	return s.MapMode == mapprovider.OndexMap && s.OndexMap != nil
}

// mapProvider — ilovalarga aytiladigan faol manba.
func (s *Server) mapProvider() mapprovider.Mode {
	if s.useOndexMap() {
		return mapprovider.OndexMap
	}
	return mapprovider.Google
}

func (s *Server) ondexMapBase() string {
	base := strings.TrimRight(strings.TrimSpace(s.OndexMapBase), "/")
	if base == "" {
		return ondexmap.DefaultBase
	}
	return base
}

// logOndexFallback — OnDexMap YIQILGANDA (ma'lumot yo'qligida emas) Google'ga
// o'tilganini yozadi: bu log "OnDexMap sog'lommi?" degan savolga javob beradi.
func logOndexFallback(surface string, err error) {
	if errors.Is(err, ondexmap.ErrUnavailable) {
		slog.Warn("ondexmap_fallback", "manba", surface, "xato", err)
	}
}

// handleMapsConfig — GET /config/maps: ilovalar qaysi xarita manbasini
// ishlatishini va kerakli kalit/manzillarni shu yerdan oladi.
//
// `maps_api_key` ESKI mijozlar uchun joyida qoldi. Kalit bo'lmasa va rejim
// Google bo'lsa — ilgarigidek 503. OnDexMap rejimida kalit bo'lmasligi
// xato emas (Google faqat zaxira).
func (s *Server) handleMapsConfig(w http.ResponseWriter, _ *http.Request) {
	key := os.Getenv("GOOGLE_MAPS_API_KEY")
	provider := s.mapProvider()
	if key == "" && provider == mapprovider.Google {
		httpError(w, http.StatusServiceUnavailable,
			errors.New("xarita kaliti sozlanmagan (.env: GOOGLE_MAPS_API_KEY)"))
		return
	}
	body := map[string]any{
		"provider":         string(provider),
		"google_available": key != "",
	}
	if key != "" {
		body["maps_api_key"] = key
	}
	if s.OndexMap != nil {
		base := s.ondexMapBase()
		body["ondexmap"] = map[string]string{
			"api_base":  base,
			"style_url": base + "/tiles/style.json",
		}
	}
	writeJSON(w, http.StatusOK, body)
}

// handleMapsFallback — POST /config/maps/fallback: ilova OnDexMap'dan xato
// olib Google'ga o'tganini xabar qiladi (log, 204). Ilova tomonidagi
// qulashlar serverga boshqacha ko'rinmaydi, shuning uchun shu yo'l bilan
// kuzatamiz.
func (s *Server) handleMapsFallback(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Surface string `json:"surface"`
		Reason  string `json:"reason"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	surface := in.Surface
	switch surface {
	case "address", "tracking", "courier", "admin", "web":
	default:
		surface = "other"
	}
	slog.Warn("ondexmap_client_fallback",
		"manba", surface, "sabab", cleanLogText(in.Reason, 160),
		"ref", claimsFrom(r).Subject)
	w.WriteHeader(http.StatusNoContent)
}

// cleanLogText — mijoz yuborgan matnni logga xavfsiz qiladi: nazorat
// belgilari (log qatorini buzadigan) olib tashlanadi va uzunlik cheklanadi.
func cleanLogText(s string, maxRunes int) string {
	var b strings.Builder
	n := 0
	for _, r := range s {
		if unicode.IsControl(r) || r == utf8.RuneError {
			continue
		}
		if n >= maxRunes {
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// ondexReverse — koordinata → manzil (OnDexMap). `ok=false`: javob yo'q yoki
// OnDexMap ishlamayapti — chaqiruvchi Google zanjiriga o'tadi.
func (s *Server) ondexReverse(ctx context.Context, lat, lng float64) (string, bool) {
	if !s.useOndexMap() {
		return "", false
	}
	p, err := s.OndexMap.Resolve(ctx, lat, lng)
	if err != nil {
		logOndexFallback("reverse", err)
		return "", false
	}
	return strings.TrimSpace(p.Text), true
}

// ondexSuggestions — manzil qidiruvi takliflari (OnDexMap). `ok=false` —
// OnDexMap ishlamayapti (Google'ga o'tiladi). Natija bo'sh bo'lishi mumkin
// va bu XATO emas: "topilmadi".
func (s *Server) ondexSuggestions(ctx context.Context, input string) ([]map[string]string, bool) {
	if !s.useOndexMap() {
		return nil, false
	}
	matches, err := s.OndexMap.Search(ctx, input,
		&ondexmap.Point{Lat: biasLat, Lng: biasLng}, ondexSuggestLimit)
	if err != nil {
		logOndexFallback("autocomplete", err)
		return nil, false
	}
	out := make([]map[string]string, 0, len(matches))
	for _, m := range matches {
		lat, lng, ok := m.Center()
		if !ok || !inUzbekistan(lat, lng) || strings.TrimSpace(m.Name) == "" {
			continue
		}
		desc := strings.TrimSpace(m.Name)
		if near := strings.TrimSpace(m.Near); near != "" && near != desc {
			desc += ", " + near
		}
		out = append(out, map[string]string{
			"description": desc,
			"place_id":    makeOndexPlaceID(lat, lng, desc),
		})
	}
	return out, true
}

func inUzbekistan(lat, lng float64) bool {
	return lat >= uzMinLat && lat <= uzMaxLat && lng >= uzMinLng && lng <= uzMaxLng
}

func makeOndexPlaceID(lat, lng float64, text string) string {
	return ondexPlacePrefix +
		strconv.FormatFloat(lat, 'f', 6, 64) + "," +
		strconv.FormatFloat(lng, 'f', 6, 64) + ":" +
		cleanLogText(text, maxPlaceTextRunes)
}

// parseOndexPlaceID — `ondex:<lat>,<lng>:<matn>` ni ochadi. place_id —
// KIRUVCHI ma'lumot: koordinata sonli va O'zbekiston ichida bo'lishi shart.
func parseOndexPlaceID(id string) (lat, lng float64, text string, ok bool) {
	rest, found := strings.CutPrefix(id, ondexPlacePrefix)
	if !found {
		return 0, 0, "", false
	}
	// Matn oxirida va ichida ':' bo'lishi mumkin — faqat birinchisini kesamiz.
	coords, text, found := strings.Cut(rest, ":")
	if !found {
		return 0, 0, "", false
	}
	latS, lngS, found := strings.Cut(coords, ",")
	if !found {
		return 0, 0, "", false
	}
	lat, errLat := strconv.ParseFloat(latS, 64)
	lng, errLng := strconv.ParseFloat(lngS, 64)
	if errLat != nil || errLng != nil || math.IsNaN(lat) || math.IsNaN(lng) || !inUzbekistan(lat, lng) {
		return 0, 0, "", false
	}
	text = cleanLogText(text, maxPlaceTextRunes)
	if text == "" {
		return 0, 0, "", false
	}
	return lat, lng, text, true
}

// directionsOndexThenGoogle — OnDexMap yo'li; xato bo'lsa Google.
func (s *Server) directionsOndexThenGoogle(ctx context.Context, from, to tracking.Point, mode string) (DirectionsResult, error) {
	res, err := ondexDirections(ctx, s.OndexMap, from, to, mode)
	if err == nil {
		return res, nil
	}
	logOndexFallback("route", err)
	if os.Getenv("GOOGLE_GEOCODING_API_KEY") == "" {
		// Zaxira ham yo'q: Google'ning "kalit yo'q" xatosi o'rniga haqiqiy
		// sababni qaytaramiz.
		if errors.Is(err, ondexmap.ErrNotCovered) {
			return DirectionsResult{}, errNoRoute
		}
		return DirectionsResult{}, err
	}
	return googleDirections(ctx, from, to, mode)
}

// ondexDirections — OnDexMap `/v1/route` ni `DirectionsResult` ga aylantiradi.
//
// OnDexMap marshruti faqat avtomobil (OSRM driving) uchun: piyoda va
// velosiped vaqti masofadan hisoblanadi. Burilishlar (ovozli yo'l ko'rsatish)
// OSRM `steps` siz keladi, shuning uchun chiziq shaklidan chiqariladi
// (`synthesizeSteps`).
func ondexDirections(ctx context.Context, api ondexmap.API, from, to tracking.Point, mode string) (DirectionsResult, error) {
	r, err := api.Route(ctx,
		ondexmap.Point{Lat: from.Lat, Lng: from.Lng},
		ondexmap.Point{Lat: to.Lat, Lng: to.Lng})
	if err != nil {
		return DirectionsResult{}, err
	}
	pts := make([]tracking.Point, len(r.Points))
	for i, p := range r.Points {
		pts[i] = tracking.Point{Lat: p.Lat, Lng: p.Lng}
	}
	dur := r.DurationS
	switch mode {
	case "walking":
		dur = r.DistanceM / walkSpeedMS
	case "bicycling":
		dur = r.DistanceM / bikeSpeedMS
	}
	steps := synthesizeSteps(pts)
	if len(steps) > maxRouteSteps {
		steps = steps[:maxRouteSteps]
	}
	return DirectionsResult{
		Points:          pts,
		DistanceMeters:  int(math.Round(r.DistanceM)),
		DurationSeconds: int(math.Round(dur)),
		Steps:           steps,
		Source:          sourceOndexMap,
	}, nil
}
