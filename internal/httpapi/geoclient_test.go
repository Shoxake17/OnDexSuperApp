package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Tashqi geo klientining testlari (bug.md 21-band).
//
// ┌─ NEGA BU MUHIM ────────────────────────────────────────────────────┐
// Oltita geo chaqiruvi `http.DefaultClient` ni ishlatardi, uning
// `Timeout` maydoni esa NOL — cheksiz. Tashqi xizmat osilib qolsa,
// goroutine va mijoz ulanishi cheksiz band bo'lardi; bir necha o'nlab
// bunday so'rov serverni bo'g'ardi.
//
// Test buni "sekin server" bilan HAQIQATAN o'lchaydi — konstantani
// o'qib solishtirish yetarli emas, chunki xato aynan klient
// TANLASHDA edi (`DefaultClient` vs. o'z klientimiz).
// └────────────────────────────────────────────────────────────────────┘

// Klientda timeout borligi: sekin server javob bermasa, chaqiruv
// abadiy kutmasligi kerak.
func TestGeoClientHasTimeout(t *testing.T) {
	if geoClient.Timeout <= 0 {
		t.Fatal("geoClient timeout'siz — `http.DefaultClient` bilan bir xil xato")
	}
	if geoClient == http.DefaultClient {
		t.Fatal("geoClient — `http.DefaultClient` ning o'zi")
	}
}

// Sekin server: chaqiruv timeout bilan tugashi kerak, osilib
// qolmasligi. Chegara vaqtincha qisqartiriladi — test 10 soniya
// kutib o'tirmasin.
func TestFetchGeoJSONTimesOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Klient ketguncha ushlab turamiz.
		<-r.Context().Done()
	}))
	defer srv.Close()

	slow := &http.Client{Timeout: 150 * time.Millisecond}
	orig := geoClient
	geoClient = slow
	defer func() { geoClient = orig }()

	done := make(chan error, 1)
	go func() {
		var out map[string]any
		done <- fetchGeoJSON(context.Background(), srv.URL, &out)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("sekin server xato bermadi")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("chaqiruv timeout bilan tugamadi — cheksiz kutyapti")
	}
}

// So'rov konteksti bekor qilinsa (mijoz ulanishni uzdi), tashqi
// chaqiruv ham darhol to'xtashi kerak — bu timeout'dan ALOHIDA
// ikkinchi chegara.
func TestFetchGeoJSONHonoursContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		var out map[string]any
		done <- fetchGeoJSON(ctx, srv.URL, &out)
	}()
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("bekor qilingan kontekst xato bermadi")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("kontekst bekor qilinganda chaqiruv to'xtamadi")
	}
}

// Javob hajmi cheklanadi: buzilgan yoki zararli javob xotirani
// to'ldirmasin. Chegaradan oshgan JSON o'qilmaydi.
func TestFetchGeoJSONLimitsResponseSize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Chegaradan ancha katta, ATAYLAB tugallanmagan JSON:
		// `LimitReader` uni kessa, dekodlash xato beradi.
		w.Write([]byte(`{"a":"` + strings.Repeat("x", geoMaxResponseBytes+1024)))
	}))
	defer srv.Close()

	var out map[string]any
	if err := fetchGeoJSON(context.Background(), srv.URL, &out); err == nil {
		t.Fatal("chegaradan katta javob xatosiz o'qildi")
	}
}

// Halol javob esa odatdagidek o'qilishi kerak — tuzatish ishlayotgan
// oqimni buzmasin.
func TestFetchGeoJSONReadsNormalResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"OK","results":[{"formatted_address":"Chust"}]}`))
	}))
	defer srv.Close()

	var data geocodeResponse
	if err := fetchGeoJSON(context.Background(), srv.URL, &data); err != nil {
		t.Fatalf("halol javob o'qilmadi: %v", err)
	}
	if data.Status != "OK" || len(data.Results) != 1 {
		t.Fatalf("javob noto'g'ri o'qildi: %+v", data)
	}
}

// Noto'g'ri URL panic bermasligi kerak: avval
// `req, _ := http.NewRequestWithContext(...)` xatoni tashlab
// yuborardi va `req` nil bo'lsa keyingi qator panic berardi.
func TestFetchGeoJSONRejectsBadURLWithoutPanic(t *testing.T) {
	var out map[string]any
	// Boshqaruv belgisi bo'lgan URL — `http.NewRequest` xato beradi.
	if err := fetchGeoJSON(context.Background(), "http://ex\x7fample", &out); err == nil {
		t.Fatal("noto'g'ri URL xato bermadi")
	}
}
