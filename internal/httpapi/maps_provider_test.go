package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"chustapp/internal/mapprovider"
	"chustapp/internal/ondexmap"
	"chustapp/internal/revoke"
	"chustapp/internal/storage"
	"chustapp/internal/tracking"
	"chustapp/internal/users"
	"chustapp/internal/ws"
)

// fakeOndex — OnDexMap o'rniga: javoblari sozlanadi, chaqiruvlar sanaladi.
type fakeOndex struct {
	calls  atomic.Int32
	search []ondexmap.Match
	place  ondexmap.Place
	route  ondexmap.Route
	err    error // barcha amallar uchun
}

func (f *fakeOndex) Search(context.Context, string, *ondexmap.Point, int) ([]ondexmap.Match, error) {
	f.calls.Add(1)
	return f.search, f.err
}

func (f *fakeOndex) Resolve(context.Context, float64, float64) (ondexmap.Place, error) {
	f.calls.Add(1)
	return f.place, f.err
}

func (f *fakeOndex) Route(context.Context, ondexmap.Point, ondexmap.Point) (ondexmap.Route, error) {
	f.calls.Add(1)
	return f.route, f.err
}

func fptr(v float64) *float64 { return &v }

// mapsServer — GOOGLE_MAP rejimi va OnDexMap soxtasi bilan server.
func mapsServer(t *testing.T, mode mapprovider.Mode, od ondexmap.API) (http.Handler, string) {
	t.Helper()
	ctx := context.Background()
	repo := storage.NewMemoryUserRepo()
	tokens := users.NewTokenIssuer("test-secret", time.Hour)
	u := &users.User{ID: "u-maps", Phone: "+998900000401", Role: users.RoleCustomer, PhoneVerified: true, CreatedAt: time.Now()}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	tok, err := tokens.Issue(u)
	if err != nil {
		t.Fatal(err)
	}
	h := New(Deps{
		UserRepo:     repo,
		Tokens:       tokens,
		Revoked:      revoke.New(nil, time.Hour),
		Hub:          ws.NewHub(nil),
		MapMode:      mode,
		OndexMap:     od,
		OndexMapBase: "https://maps.example.test/",
		DevMode:      true,
	}).Routes(nil)
	return h, tok
}

func decodeMap(t *testing.T, body string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("JSON emas: %v — %s", err, body)
	}
	return m
}

func TestMapsConfigByMode(t *testing.T) {
	t.Run("google rejimi + kalit", func(t *testing.T) {
		t.Setenv("GOOGLE_MAPS_API_KEY", "gkey")
		h, tok := mapsServer(t, mapprovider.Google, &fakeOndex{})
		w := do(t, h, "GET", "/config/maps", tok, "")
		if w.Code != http.StatusOK {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
		m := decodeMap(t, w.Body.String())
		if m["provider"] != "google" || m["maps_api_key"] != "gkey" || m["google_available"] != true {
			t.Fatalf("noto'g'ri: %v", m)
		}
		od, _ := m["ondexmap"].(map[string]any)
		if od["api_base"] != "https://maps.example.test" || od["style_url"] != "https://maps.example.test/tiles/style.json" {
			t.Fatalf("ondexmap bloki: %v", od)
		}
	})
	t.Run("google rejimi + kalit yo'q = 503 (ilgarigidek)", func(t *testing.T) {
		t.Setenv("GOOGLE_MAPS_API_KEY", "")
		h, tok := mapsServer(t, mapprovider.Google, &fakeOndex{})
		if w := do(t, h, "GET", "/config/maps", tok, ""); w.Code != http.StatusServiceUnavailable {
			t.Fatalf("503 kutilgan: %d", w.Code)
		}
	})
	t.Run("ondexmap rejimi + kalit yo'q = 200, Google yo'q", func(t *testing.T) {
		t.Setenv("GOOGLE_MAPS_API_KEY", "")
		h, tok := mapsServer(t, mapprovider.OndexMap, &fakeOndex{})
		w := do(t, h, "GET", "/config/maps", tok, "")
		if w.Code != http.StatusOK {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
		m := decodeMap(t, w.Body.String())
		if m["provider"] != "ondexmap" || m["google_available"] != false {
			t.Fatalf("noto'g'ri: %v", m)
		}
		if _, has := m["maps_api_key"]; has {
			t.Fatal("kalit bo'lmasa maps_api_key umuman yuborilmasligi kerak")
		}
	})
	t.Run("ondexmap rejimi lekin mijoz yo'q = Google", func(t *testing.T) {
		t.Setenv("GOOGLE_MAPS_API_KEY", "gkey")
		h, tok := mapsServer(t, mapprovider.OndexMap, nil)
		m := decodeMap(t, do(t, h, "GET", "/config/maps", tok, "").Body.String())
		if m["provider"] != "google" {
			t.Fatalf("mijozsiz OnDexMap rejimi Google bo'lishi kerak: %v", m)
		}
		if _, has := m["ondexmap"]; has {
			t.Fatal("mijoz yo'q — ondexmap bloki ham yo'q")
		}
	})
	t.Run("tokensiz = 401", func(t *testing.T) {
		h, _ := mapsServer(t, mapprovider.Google, nil)
		if w := do(t, h, "GET", "/config/maps", "", ""); w.Code != http.StatusUnauthorized {
			t.Fatalf("401 kutilgan: %d", w.Code)
		}
	})
}

func TestGoogleModeNeverTouchesOndexMap(t *testing.T) {
	t.Setenv("GOOGLE_GEOCODING_API_KEY", "")
	od := &fakeOndex{place: ondexmap.Place{Text: "X"}}
	h, tok := mapsServer(t, mapprovider.Google, od)
	for _, p := range []string{
		"/geocode/reverse?lat=41&lng=71.2",
		"/geocode/autocomplete?input=chust",
		"/geocode/route?origin_lat=41&origin_lng=71.2&dest_lat=41.01&dest_lng=71.21",
	} {
		do(t, h, "GET", p, tok, "")
	}
	if od.calls.Load() != 0 {
		t.Fatalf("Google rejimida OnDexMap chaqirilmasligi kerak: %d marta", od.calls.Load())
	}
}

func TestReverseOndexThenFallback(t *testing.T) {
	t.Setenv("GOOGLE_GEOCODING_API_KEY", "")
	t.Run("OnDexMap manzil beradi", func(t *testing.T) {
		od := &fakeOndex{place: ondexmap.Place{Text: "Navoiy ko'chasi, Chust"}}
		h, tok := mapsServer(t, mapprovider.OndexMap, od)
		w := do(t, h, "GET", "/geocode/reverse?lat=41&lng=71.2", tok, "")
		if w.Code != http.StatusOK || decodeMap(t, w.Body.String())["address"] != "Navoiy ko'chasi, Chust" {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
	})
	t.Run("OnDexMap yiqilsa Google zanjiriga o'tadi (kalit yo'q = 503)", func(t *testing.T) {
		od := &fakeOndex{err: ondexmap.ErrUnavailable}
		h, tok := mapsServer(t, mapprovider.OndexMap, od)
		w := do(t, h, "GET", "/geocode/reverse?lat=41&lng=71.2", tok, "")
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("zaxira zanjiriga o'tishi kerak edi (503): %d %s", w.Code, w.Body.String())
		}
		if od.calls.Load() != 1 {
			t.Fatalf("OnDexMap bir marta so'ralishi kerak: %d", od.calls.Load())
		}
	})
	t.Run("OnDexMap 'bu nuqta yo'q' desa ham Google zanjiriga", func(t *testing.T) {
		od := &fakeOndex{err: ondexmap.ErrNotCovered}
		h, tok := mapsServer(t, mapprovider.OndexMap, od)
		if w := do(t, h, "GET", "/geocode/reverse?lat=41&lng=71.2", tok, ""); w.Code != http.StatusServiceUnavailable {
			t.Fatalf("%d", w.Code)
		}
	})
}

func TestAutocompleteOndexAndPlaceRoundTrip(t *testing.T) {
	od := &fakeOndex{search: []ondexmap.Match{
		{Name: "Navoiy ko'chasi", Near: "Chust", Lat: fptr(41.001), Lng: fptr(71.231)},
		{Name: "Bog'ishamol", Near: "Chust", BBox: &[4]float64{71.20, 41.00, 71.22, 41.02}}, // faqat bbox
		{Name: "Koordinatasiz"},                                          // o'tkazib yuboriladi
		{Name: "Tashqarida", Lat: fptr(10), Lng: fptr(10)},               // O'zbekistondan tashqari
		{Name: "  ", Lat: fptr(41.0), Lng: fptr(71.2)},                   // bo'sh nom
		{Name: "Chust", Near: "Chust", Lat: fptr(41.0), Lng: fptr(71.2)}, // near == name — takrorlanmaydi
	}}
	h, tok := mapsServer(t, mapprovider.OndexMap, od)

	w := do(t, h, "GET", "/geocode/autocomplete?input="+url.QueryEscape("navoiy"), tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	var list []map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("3 ta yaroqli taklif kutilgan: %+v", list)
	}
	if list[0]["description"] != "Navoiy ko'chasi, Chust" || list[2]["description"] != "Chust" {
		t.Fatalf("matnlar: %+v", list)
	}
	for _, it := range list {
		if !strings.HasPrefix(it["place_id"], ondexPlacePrefix) {
			t.Fatalf("place_id prefiksi: %q", it["place_id"])
		}
	}

	// place_id -> /geocode/place: tashqi so'rovsiz, koordinata va matn qaytadi.
	w = do(t, h, "GET", "/geocode/place?place_id="+url.QueryEscape(list[0]["place_id"]), tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("place: %d %s", w.Code, w.Body.String())
	}
	got := decodeMap(t, w.Body.String())
	if got["lat"] != 41.001 || got["lng"] != 71.231 || got["address"] != "Navoiy ko'chasi, Chust" {
		t.Fatalf("place javobi: %v", got)
	}
	// bbox markazi: janub/shimol o'rtasi, g'arb/sharq o'rtasi.
	w = do(t, h, "GET", "/geocode/place?place_id="+url.QueryEscape(list[1]["place_id"]), tok, "")
	got = decodeMap(t, w.Body.String())
	if got["lat"] != 41.01 || got["lng"] != 71.21 {
		t.Fatalf("bbox markazi: %v", got)
	}
}

func TestAutocompleteOndexFailureFallsBackAndEmptyIsNotAnError(t *testing.T) {
	t.Setenv("GOOGLE_GEOCODING_API_KEY", "")
	h, tok := mapsServer(t, mapprovider.OndexMap, &fakeOndex{search: nil})
	w := do(t, h, "GET", "/geocode/autocomplete?input=zzz", tok, "")
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("bo'sh natija [] bo'lishi kerak (xato emas): %d %s", w.Code, w.Body.String())
	}
	h, tok = mapsServer(t, mapprovider.OndexMap, &fakeOndex{err: ondexmap.ErrUnavailable})
	if w := do(t, h, "GET", "/geocode/autocomplete?input=zzz", tok, ""); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("OnDexMap yiqilsa Google zanjiriga o'tishi kerak: %d", w.Code)
	}
}

func TestPlaceRejectsForgedOndexIDs(t *testing.T) {
	h, tok := mapsServer(t, mapprovider.Google, nil)
	for _, id := range []string{
		"ondex:",
		"ondex:41.0,71.2",              // matn yo'q
		"ondex:41.0,71.2:",             // bo'sh matn
		"ondex:abc,71.2:x",             // son emas
		"ondex:10.0,10.0:Tashqarida",   // O'zbekistondan tashqari
		"ondex:NaN,71.2:x",             // NaN
		"ondex:41.0:x",                 // lng yo'q
		"ondex:41.0,71.2:\n\r\x00\x01", // faqat nazorat belgilari
	} {
		w := do(t, h, "GET", "/geocode/place?place_id="+url.QueryEscape(id), tok, "")
		if w.Code != http.StatusBadRequest {
			t.Errorf("%q: 400 kutilgan, keldi %d", id, w.Code)
		}
	}
	// Matn ichidagi ':' saqlanadi, uzun matn kesiladi.
	long := strings.Repeat("a", 500)
	w := do(t, h, "GET", "/geocode/place?place_id="+url.QueryEscape("ondex:41.0,71.2:A: B "+long), tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("%d", w.Code)
	}
	addr, _ := decodeMap(t, w.Body.String())["address"].(string)
	if !strings.HasPrefix(addr, "A: B ") || len([]rune(addr)) != maxPlaceTextRunes {
		t.Fatalf("matn: %d belgi, %q", len([]rune(addr)), addr[:10])
	}
}

func TestRouteOndexSynthesizesStepsAndModes(t *testing.T) {
	// 600 m shimolga, keyin 600 m sharqqa (o'ng burilish). distance_m = 1200.
	first := leg(stepsOrigin, 1, 0, 600, 20)
	corner := first[len(first)-1]
	pts := append([]tracking.Point{stepsOrigin}, first...)
	pts = append(pts, leg(corner, 0, 1, 600, 20)...)
	var rp []ondexmap.Point
	for _, p := range pts {
		rp = append(rp, ondexmap.Point{Lat: p.Lat, Lng: p.Lng})
	}
	od := &fakeOndex{route: ondexmap.Route{Points: rp, DistanceM: 1200, DurationS: 100}}
	h, tok := mapsServer(t, mapprovider.OndexMap, od)

	type resp struct {
		Points []map[string]float64 `json:"points"`
		Dist   int                  `json:"distance_meters"`
		Dur    int                  `json:"duration_seconds"`
		Steps  []RouteStep          `json:"steps"`
	}
	get := func(mode string) resp {
		w := do(t, h, "GET", "/geocode/route?origin_lat=41&origin_lng=71.2&dest_lat=41.01&dest_lng=71.21&mode="+mode, tok, "")
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", mode, w.Code, w.Body.String())
		}
		var r resp
		if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
		return r
	}

	drv := get("driving")
	if drv.Dist != 1200 || drv.Dur != 100 || len(drv.Points) != len(pts) {
		t.Fatalf("driving: %+v", drv)
	}
	if len(drv.Steps) != 1 || drv.Steps[0].Maneuver != "right" {
		t.Fatalf("burilish: %+v", drv.Steps)
	}
	// Piyoda 4 km/h: 1200 m → 1080 s. Velosiped 15 km/h: 288 s.
	if w := get("walking"); w.Dur != 1080 {
		t.Fatalf("walking vaqti: %d", w.Dur)
	}
	if b := get("bicycling"); b.Dur != 288 {
		t.Fatalf("bicycling vaqti: %d", b.Dur)
	}
}

func TestRouteOndexFailureWithoutGoogleKeepsRealReason(t *testing.T) {
	t.Setenv("GOOGLE_GEOCODING_API_KEY", "")
	q := "/geocode/route?origin_lat=41&origin_lng=71.2&dest_lat=41.01&dest_lng=71.21"

	h, tok := mapsServer(t, mapprovider.OndexMap, &fakeOndex{err: ondexmap.ErrNotCovered})
	if w := do(t, h, "GET", q, tok, ""); w.Code != http.StatusNotFound {
		t.Fatalf("yo'q = 404 kutilgan: %d", w.Code)
	}
	h, tok = mapsServer(t, mapprovider.OndexMap, &fakeOndex{err: ondexmap.ErrUnavailable})
	if w := do(t, h, "GET", q, tok, ""); w.Code != http.StatusBadGateway {
		t.Fatalf("yiqilgan = 502 kutilgan (503 'kalit yo'q' emas): %d", w.Code)
	}
}

func TestMapsFallbackReport(t *testing.T) {
	h, tok := mapsServer(t, mapprovider.OndexMap, &fakeOndex{})
	if w := do(t, h, "POST", "/config/maps/fallback", tok, `{"surface":"address","reason":"tile xatosi\nQALBAKI"}`); w.Code != http.StatusNoContent {
		t.Fatalf("204 kutilgan: %d %s", w.Code, w.Body.String())
	}
	if w := do(t, h, "POST", "/config/maps/fallback", tok, `{"surface":"nomalum"}`); w.Code != http.StatusNoContent {
		t.Fatalf("noma'lum surface ham 204: %d", w.Code)
	}
	if w := do(t, h, "POST", "/config/maps/fallback", tok, `{xato`); w.Code != http.StatusBadRequest {
		t.Fatalf("buzuq JSON = 400: %d", w.Code)
	}
	if w := do(t, h, "POST", "/config/maps/fallback", "", `{"surface":"web"}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("tokensiz = 401: %d", w.Code)
	}
}

func TestCleanLogTextStripsControlChars(t *testing.T) {
	if got := cleanLogText("a\nb\r\tc\x00d", 50); got != "abcd" {
		t.Fatalf("%q", got)
	}
	if got := cleanLogText(strings.Repeat("ё", 10), 4); got != "ёёёё" {
		t.Fatalf("%q", got)
	}
}
