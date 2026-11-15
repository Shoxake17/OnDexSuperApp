package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func hdr(kv ...string) http.Header {
	h := http.Header{}
	for i := 0; i+1 < len(kv); i += 2 {
		h.Set(kv[i], kv[i+1])
	}
	return h
}

func TestCheckSecurityHeaders(t *testing.T) {
	goodHTML := hdr("Strict-Transport-Security", "max-age=63072000; preload", "X-Content-Type-Options", "nosniff",
		"Content-Security-Policy", "default-src 'self'; frame-ancestors 'self'", "Referrer-Policy", "no-referrer")
	goodAPI := hdr("Strict-Transport-Security", "max-age=63072000", "X-Content-Type-Options", "nosniff",
		"Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
	cases := []struct {
		name, kind string
		h          http.Header
		wantErr    string // bo'sh — o'tishi kerak
	}{
		{"html to'liq", "html", goodHTML, ""},
		{"html XFO bilan", "html", hdr("Strict-Transport-Security", "max-age=31536000", "X-Content-Type-Options", "nosniff",
			"Content-Security-Policy", "default-src 'self'", "X-Frame-Options", "DENY", "Referrer-Policy", "same-origin"), ""},
		{"api to'liq", "api", goodAPI, ""},
		{"static", "static", hdr("Strict-Transport-Security", "max-age=31536000", "X-Content-Type-Options", "nosniff"), ""},
		{"plane holati: CSP yo'q", "html", hdr("Strict-Transport-Security", "max-age=31536000", "X-Content-Type-Options", "nosniff",
			"X-Frame-Options", "DENY"), "CSP yo'q"},
		{"HSTS qisqa", "html", hdr("Strict-Transport-Security", "max-age=300", "X-Content-Type-Options", "nosniff",
			"Content-Security-Policy", "frame-ancestors 'none'", "Referrer-Policy", "no-referrer"), "max-age=300"},
		{"framing ochiq", "html", hdr("Strict-Transport-Security", "max-age=31536000", "X-Content-Type-Options", "nosniff",
			"Content-Security-Policy", "default-src 'self'", "Referrer-Policy", "no-referrer"), "framing"},
		{"api ochiq CSP", "api", hdr("Strict-Transport-Security", "max-age=31536000", "X-Content-Type-Options", "nosniff",
			"Content-Security-Policy", "default-src 'self'", "X-Frame-Options", "DENY"), "default-src 'none'"},
		{"cdn hozirgi holati", "static", hdr(), "HSTS yo'q; nosniff yo'q"},
	}
	for _, c := range cases {
		err := CheckSecurityHeaders(c.h, c.kind)
		if c.wantErr == "" && err != nil {
			t.Errorf("%s: kutilmagan xato %v", c.name, err)
		}
		if c.wantErr != "" && (err == nil || !strings.Contains(err.Error(), c.wantErr)) {
			t.Errorf("%s: %q kutilgandi, keldi %v", c.name, c.wantErr, err)
		}
		if err != nil && !isPermanent(err) {
			t.Errorf("%s: sarlavha xatosi qayta urinilmasligi kerak", c.name)
		}
	}
}

func TestParseHeaderTargets(t *testing.T) {
	if ts, err := ParseHeaderTargets(""); err != nil || ts != nil {
		t.Fatalf("bo'sh: %v %v", ts, err)
	}
	if ts, _ := ParseHeaderTargets("default"); len(ts) != len(DefaultHeaderTargets) {
		t.Fatal("default ro'yxati")
	}
	ts, err := ParseHeaderTargets("a=https://x.uz/p:api, b=https://y.uz:8443/:html")
	if err != nil || len(ts) != 2 || ts[0].URL != "https://x.uz/p" || ts[1].URL != "https://y.uz:8443/" || ts[1].Kind != "html" {
		t.Fatalf("tahlil: %+v %v", ts, err)
	}
	for _, bad := range []string{"a=https://x.uz", "=https://x.uz:api", "a=https://x.uz:foo"} {
		if _, err := ParseHeaderTargets(bad); err == nil {
			t.Errorf("%q rad etilishi kerak", bad)
		}
	}
}

// Server bilan: redirect kuzatilmaydi, 5xx alohida aytiladi, natija nomi hdr.<nom>.
func TestCheckHeadersAgainstServer(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=63072000")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(404)
	}))
	defer ok.Close()
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(502) }))
	defer down.Close()
	r := newRunner(Config{Timeout: 5 * time.Second, Headers: []HeaderTarget{
		{"cdn", ok.URL + "/", "static"}, {"git", down.URL + "/", "html"},
	}}, nil)
	for _, tg := range r.cfg.Headers {
		r.checkHeaders(tg)
	}
	got := map[string]Result{}
	for _, res := range r.results {
		got[res.Check] = res
	}
	if !got["hdr.cdn"].OK {
		t.Fatalf("cdn o'tishi kerak: %+v", got["hdr.cdn"])
	}
	if got["hdr.git"].OK || !strings.Contains(got["hdr.git"].Err, "502") {
		t.Fatalf("git 502 aytilishi kerak: %+v", got["hdr.git"])
	}
}
