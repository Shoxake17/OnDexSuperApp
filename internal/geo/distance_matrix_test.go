package geo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fakeGoogleServer — Google Distance Matrix API javobini taqlid qiladi.
// `mode` parametriga qarab turli sonli qatorlar qaytaradi — testda har bir
// mode-guruh UCHUN ALOHIDA so'rov ketganini tekshirish uchun.
func fakeGoogleServer(t *testing.T, handler func(q map[string][]string) any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := handler(r.URL.Query())
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchETAsSingleMode(t *testing.T) {
	srv := fakeGoogleServer(t, func(q map[string][]string) any {
		if q["mode"][0] != "driving" {
			t.Fatalf("kutilgan mode=driving, olindi %v", q["mode"])
		}
		return map[string]any{
			"status": "OK",
			"rows": []map[string]any{
				{"elements": []map[string]any{
					{"status": "OK", "duration": map[string]any{"value": 300}, "distance": map[string]any{"value": 1200}},
				}},
				{"elements": []map[string]any{
					{"status": "OK", "duration": map[string]any{"value": 600}, "distance": map[string]any{"value": 3000}},
				}},
			},
		}
	})

	c := NewClient("test-key")
	c.baseURL = srv.URL

	results, err := c.FetchETAs(context.Background(), LatLng{Lat: 41, Lng: 71}, []Candidate{
		{ID: "c1", Location: LatLng{Lat: 41.001, Lng: 71.001}, Mode: ModeDriving},
		{ID: "c2", Location: LatLng{Lat: 41.002, Lng: 71.002}, Mode: ModeDriving},
	})
	if err != nil {
		t.Fatalf("kutilmagan xato: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("2 ta natija kutilgan edi, olindi %d", len(results))
	}
	if results[0].ID != "c1" || results[0].Duration != 5*time.Minute {
		t.Errorf("c1 uchun noto'g'ri natija: %+v", results[0])
	}
	if results[1].ID != "c2" || results[1].Duration != 10*time.Minute {
		t.Errorf("c2 uchun noto'g'ri natija: %+v", results[1])
	}
}

// TestFetchETAsGroupsByMode — turli transport turidagi nomzodlar alohida
// so'rovlarga (mode bo'yicha guruhlanib) yuborilishini tekshiradi.
func TestFetchETAsGroupsByMode(t *testing.T) {
	var calls []string
	srv := fakeGoogleServer(t, func(q map[string][]string) any {
		mode := q["mode"][0]
		calls = append(calls, mode)
		origins := q["origins"][0]
		n := 1
		for _, ch := range origins {
			if ch == '|' {
				n++
			}
		}
		elements := make([]map[string]any, n)
		for i := range elements {
			elements[i] = map[string]any{"status": "OK", "duration": map[string]any{"value": 60}, "distance": map[string]any{"value": 500}}
		}
		rows := make([]map[string]any, n)
		for i := range rows {
			rows[i] = map[string]any{"elements": []map[string]any{elements[i]}}
		}
		return map[string]any{"status": "OK", "rows": rows}
	})

	c := NewClient("test-key")
	c.baseURL = srv.URL

	results, err := c.FetchETAs(context.Background(), LatLng{Lat: 41, Lng: 71}, []Candidate{
		{ID: "walker", Location: LatLng{Lat: 41.001, Lng: 71.001}, Mode: ModeWalking},
		{ID: "biker", Location: LatLng{Lat: 41.002, Lng: 71.002}, Mode: ModeBicycling},
		{ID: "driver", Location: LatLng{Lat: 41.003, Lng: 71.003}, Mode: ModeDriving},
	})
	if err != nil {
		t.Fatalf("kutilmagan xato: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("3 ta natija kutilgan edi, olindi %d", len(results))
	}
	if len(calls) != 3 {
		t.Fatalf("har bir mode uchun ALOHIDA so'rov kutilgan edi (3 ta), olindi %d: %v", len(calls), calls)
	}
}

func TestFetchETAsNoAPIKey(t *testing.T) {
	c := NewClient("")
	_, err := c.FetchETAs(context.Background(), LatLng{}, []Candidate{{ID: "c1"}})
	if err != ErrNoAPIKey {
		t.Fatalf("ErrNoAPIKey kutilgan edi, olindi: %v", err)
	}
}

func TestFetchETAsEmptyCandidates(t *testing.T) {
	c := NewClient("test-key")
	results, err := c.FetchETAs(context.Background(), LatLng{}, nil)
	if err != nil || results != nil {
		t.Fatalf("bo'sh nomzodlar uchun (nil, nil) kutilgan edi, olindi: %v, %v", results, err)
	}
}

func TestFetchETAsZeroResultsMarkedNotOK(t *testing.T) {
	srv := fakeGoogleServer(t, func(q map[string][]string) any {
		return map[string]any{
			"status": "OK",
			"rows": []map[string]any{
				{"elements": []map[string]any{{"status": "ZERO_RESULTS"}}},
			},
		}
	})
	c := NewClient("test-key")
	c.baseURL = srv.URL

	results, err := c.FetchETAs(context.Background(), LatLng{Lat: 41, Lng: 71}, []Candidate{
		{ID: "unreachable", Location: LatLng{Lat: 50, Lng: 80}, Mode: ModeDriving},
	})
	if err != nil {
		t.Fatalf("kutilmagan xato: %v", err)
	}
	if len(results) != 1 || results[0].OK {
		t.Fatalf("OK=false kutilgan edi, olindi: %+v", results)
	}
}
