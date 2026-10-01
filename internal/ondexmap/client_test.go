package ondexmap

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRouteParsesGeoJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/route" || r.URL.Query().Get("from_lat") == "" {
			t.Errorf("kutilmagan so'rov: %s", r.URL)
		}
		_, _ = w.Write([]byte(`{"distance_m":1234.5,"duration_s":99,"geometry":{"type":"LineString","coordinates":[[71.2,41.0],[71.21,41.01]]}}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).Route(context.Background(), Point{41.0, 71.2}, Point{41.01, 71.21})
	if err != nil {
		t.Fatal(err)
	}
	// GeoJSON [lng,lat] — Point{Lat,Lng} ga to'g'ri o'girilishi shart.
	if len(got.Points) != 2 || got.Points[0].Lat != 41.0 || got.Points[0].Lng != 71.2 {
		t.Fatalf("nuqtalar noto'g'ri: %+v", got.Points)
	}
	if got.DistanceM != 1234.5 || got.DurationS != 99 {
		t.Fatalf("masofa/vaqt noto'g'ri: %+v", got)
	}
}

func TestRouteEmptyGeometryIsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"distance_m":1,"duration_s":1,"geometry":{"coordinates":[]}}`))
	}))
	defer srv.Close()
	if _, err := New(srv.URL).Route(context.Background(), Point{41, 71}, Point{41.1, 71.1}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("bo'sh geometriya ErrUnavailable bo'lishi kerak: %v", err)
	}
}

func TestStatusMapping(t *testing.T) {
	for code, want := range map[int]error{
		http.StatusNotFound:            ErrNotCovered,
		http.StatusBadRequest:          ErrNotCovered,
		http.StatusBadGateway:          ErrUnavailable,
		http.StatusTooManyRequests:     ErrUnavailable,
		http.StatusInternalServerError: ErrUnavailable,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(code)
		}))
		_, err := New(srv.URL).Resolve(context.Background(), 41, 71.2)
		srv.Close()
		if !errors.Is(err, want) {
			t.Errorf("status %d: %v, kutilgan %v", code, err, want)
		}
	}
}

func TestResolveEmptyTextIsNotCovered(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"text":"  "}`))
	}))
	defer srv.Close()
	if _, err := New(srv.URL).Resolve(context.Background(), 41, 71.2); !errors.Is(err, ErrNotCovered) {
		t.Fatalf("bo'sh matn ErrNotCovered bo'lishi kerak: %v", err)
	}
}

func TestSearchPassesBias(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("q") != "navoiy" || q.Get("lat") != "41.003000" || q.Get("limit") != "5" {
			t.Errorf("parametrlar noto'g'ri: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"results":[{"id":"a1","name":"Navoiy ko'chasi","label":"Ko'cha","near":"Chust","lat":41.0,"lng":71.2}]}`))
	}))
	defer srv.Close()
	res, err := New(srv.URL).Search(context.Background(), "navoiy", &Point{41.003, 71.236}, 5)
	if err != nil || len(res) != 1 || res[0].Name != "Navoiy ko'chasi" {
		t.Fatalf("natija noto'g'ri: %+v, %v", res, err)
	}
}

func TestBreakerOpensAndRecovers(t *testing.T) {
	var hits atomic.Int32
	healthy := atomic.Bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		if !healthy.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"text":"Chust"}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	clock := time.Now()
	c.now = func() time.Time { return clock }

	for i := 0; i < breakerThreshold; i++ {
		_, _ = c.Resolve(context.Background(), 41, 71.2)
	}
	before := hits.Load()
	if _, err := c.Resolve(context.Background(), 41, 71.2); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("to'xtatgich ochiq: %v", err)
	}
	if hits.Load() != before {
		t.Fatal("to'xtatgich ochiq paytda so'rov yuborilmasligi kerak")
	}

	clock = clock.Add(breakerCooldown + time.Second)
	healthy.Store(true)
	if _, err := c.Resolve(context.Background(), 41, 71.2); err != nil {
		t.Fatalf("muddat o'tgach xizmat tiklanishi kerak: %v", err)
	}
}

func TestNotCoveredDoesNotTripBreaker(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := New(srv.URL)
	for i := 0; i < breakerThreshold*3; i++ {
		if _, err := c.Resolve(context.Background(), 41, 71.2); !errors.Is(err, ErrNotCovered) {
			t.Fatalf("%d: %v", i, err)
		}
	}
}
