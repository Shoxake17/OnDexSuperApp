// Ikki nuqta orasidagi yo'l (Google Directions API).
package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"chustapp/internal/tracking"
	"chustapp/internal/voice"
)

var (
	errDirectionsKeyMissing = errors.New("geokodlash kaliti sozlanmagan (.env: GOOGLE_GEOCODING_API_KEY)")
	errNoRoute              = errors.New("marshrut topilmadi")
)

// DirectionsResult — yo'l chizig'i, umumiy masofa/vaqt va manevrlar.
type DirectionsResult struct {
	Points          []tracking.Point
	DistanceMeters  int
	DurationSeconds int
	Steps           []RouteStep
}

// RouteStep — aytiladigan manevr va u bajariladigan nuqta (kuryer ilovasining
// ovozli yo'l ko'rsatishi uchun). `Maneuver` — `voice.Maneuver` qiymati.
type RouteStep struct {
	Maneuver       string  `json:"maneuver"`
	Lat            float64 `json:"lat"`
	Lng            float64 `json:"lng"`
	DistanceMeters int     `json:"distance_meters"`
}

// maxRouteSteps — javobdagi eng ko'p manevr (shahar ichi yo'li uchun juda ko'p).
const maxRouteSteps = 200

// DirectionsFunc — yo'l hisoblovchi. Production'da `googleDirections`,
// testlarda soxtasi (`Deps.Directions`).
type DirectionsFunc func(ctx context.Context, from, to tracking.Point, mode string) (DirectionsResult, error)

// directions — sozlangan hisoblovchi yoki Google.
func (s *Server) directions() DirectionsFunc {
	if s.Directions != nil {
		return s.Directions
	}
	return googleDirections
}

// Google Directions so'rovi manbalari (log va o'lchov uchun).
const (
	// directionsCourierRoute — `GET /geocode/route` (kuryer ilovasi:
	// marshrut chizig'i va taklifdagi restorangacha vaqt).
	directionsCourierRoute = "kuryer_ilovasi"
	// directionsTrackingLive — mijoz kuzatuvi: kuryerdan manzilgacha.
	directionsTrackingLive = "kuzatuv_jonli"
	// directionsTrackingPlanned — buyurtmaga biriktiriladigan A→B yo'li.
	directionsTrackingPlanned = "kuzatuv_ab"
)

// directionsLogMsg — har bir PULLIK Google Directions so'rovi shu xabar
// bilan loglanadi. Optimizatsiyani raqam bilan o'lchash uchun: log faylidan
// shu satrlar manba va buyurtma bo'yicha sanaladi.
const directionsLogMsg = "google_directions"

// routeBetween — Google Directions'ga BARCHA so'rovlar shu yerdan o'tadi va
// har biri loglanadi. `ref` — buyurtma ID'si yoki (kuryer ilovasi uchun)
// foydalanuvchi ID'si; koordinata logga YOZILMAYDI (shaxsiy ma'lumot).
func (s *Server) routeBetween(ctx context.Context, source, ref string, from, to tracking.Point, mode string) (DirectionsResult, error) {
	start := time.Now()
	res, err := s.directions()(ctx, from, to, mode)
	slog.Info(directionsLogMsg,
		"manba", source, "ref", ref, "ok", err == nil,
		"ms", time.Since(start).Milliseconds())
	return res, err
}

// googleDirections — Google Directions API.
//
// Server-server chaqiruv: GOOGLE_GEOCODING_API_KEY qayta ishlatiladi
// (Cloud Console'da shu kalitga "Directions API" ham yoqilgan bo'lishi
// kerak). Kalit so'rov manzilida, lekin u faqat serverdan Google'ga ketadi
// — mijozga hech qachon qaytmaydi.
func googleDirections(ctx context.Context, from, to tracking.Point, mode string) (DirectionsResult, error) {
	key := os.Getenv("GOOGLE_GEOCODING_API_KEY")
	if key == "" {
		return DirectionsResult{}, errDirectionsKeyMissing
	}
	switch mode {
	case "walking", "bicycling", "driving":
	default:
		mode = "driving"
	}
	dirURL := "https://maps.googleapis.com/maps/api/directions/json" +
		"?origin=" + strconv.FormatFloat(from.Lat, 'f', -1, 64) + "," + strconv.FormatFloat(from.Lng, 'f', -1, 64) +
		"&destination=" + strconv.FormatFloat(to.Lat, 'f', -1, 64) + "," + strconv.FormatFloat(to.Lng, 'f', -1, 64) +
		"&mode=" + mode + "&language=uz&key=" + key
	var data directionsResponse
	if err := fetchGeoJSON(ctx, dirURL, &data); err != nil {
		return DirectionsResult{}, err
	}
	if data.Status != "OK" || len(data.Routes) == 0 {
		return DirectionsResult{}, fmt.Errorf("%w (status: %s)", errNoRoute, data.Status)
	}
	route := data.Routes[0]
	decoded := decodePolyline(route.OverviewPolyline.Points)
	res := DirectionsResult{Points: make([]tracking.Point, len(decoded))}
	for i, p := range decoded {
		res.Points[i] = tracking.Point{Lat: p.Lat, Lng: p.Lng}
	}
	for _, leg := range route.Legs {
		res.DistanceMeters += leg.Distance.Value
		res.DurationSeconds += leg.Duration.Value
		for _, st := range leg.Steps {
			m := voice.ManeuverFromGoogle(st.Maneuver)
			if m == "" || len(res.Steps) >= maxRouteSteps {
				continue
			}
			res.Steps = append(res.Steps, RouteStep{
				Maneuver: string(m), Lat: st.StartLocation.Lat, Lng: st.StartLocation.Lng,
				DistanceMeters: st.Distance.Value,
			})
		}
	}
	return res, nil
}
