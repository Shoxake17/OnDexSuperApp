package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeAPI — production API'ning smoke ishlatadigan qismi. Xatti-harakat
// bayroqlar bilan buziladi (har bir buzilish smoke'da FAIL bo'lishi SHART).
type fakeAPI struct {
	mu            sync.Mutex
	logoutBroken  bool // chiqishdan keyin token tirik qoladi
	jwksLeaksKey  bool // JWKS maxfiy kalitni oshkor qiladi
	refreshBroken bool
	live          map[string]string // access -> role
}

func (f *fakeAPI) handler() http.Handler {
	mux := http.NewServeMux()
	js := func(w http.ResponseWriter, code int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(v)
	}
	roleOf := func(phone string) string {
		if phone == "+998888888888" {
			return "courier"
		}
		return "waiter"
	}
	mux.HandleFunc("GET /restaurants", func(w http.ResponseWriter, _ *http.Request) {
		js(w, 200, []map[string]any{{"id": "r1", "name": "Book Cafe"}})
	})
	mux.HandleFunc("GET /.well-known/jwks.json", func(w http.ResponseWriter, _ *http.Request) {
		k := map[string]any{"kty": "OKP", "alg": "EdDSA", "kid": "k1", "x": "abc"}
		if f.jwksLeaksKey {
			k["d"] = "SIR"
		}
		js(w, 200, map[string]any{"keys": []any{k}})
	})
	mux.HandleFunc("POST /auth/request-code", func(w http.ResponseWriter, _ *http.Request) { js(w, 200, map[string]any{"sent": true}) })
	mux.HandleFunc("POST /v2/auth/merchant/otp/verify", func(w http.ResponseWriter, r *http.Request) {
		var b map[string]string
		_ = json.NewDecoder(r.Body).Decode(&b)
		if b["code"] != "666666" {
			js(w, 401, map[string]any{"error": "kod noto'g'ri"})
			return
		}
		f.mu.Lock()
		a := "eyJ" + b["phone"] + b["client_id"]
		f.live[a] = roleOf(b["phone"])
		f.mu.Unlock()
		js(w, 200, map[string]any{"access_token": a, "refresh_token": "ort_1" + a})
	})
	auth := func(r *http.Request) (string, bool) {
		f.mu.Lock()
		defer f.mu.Unlock()
		role, ok := f.live[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]
		return role, ok
	}
	mux.HandleFunc("GET /me", func(w http.ResponseWriter, r *http.Request) {
		role, ok := auth(r)
		if !ok {
			js(w, 401, map[string]any{"error": "sessiya tugagan"})
			return
		}
		js(w, 200, map[string]any{"role": role, "entity_id": "c1"})
	})
	mux.HandleFunc("GET /waiter/me", func(w http.ResponseWriter, r *http.Request) {
		if role, ok := auth(r); !ok || role != "waiter" {
			js(w, 403, map[string]any{"error": "rol"})
			return
		}
		js(w, 200, map[string]any{})
	})
	mux.HandleFunc("GET /couriers/{id}", func(w http.ResponseWriter, r *http.Request) {
		if role, ok := auth(r); !ok || role != "courier" {
			js(w, 403, map[string]any{"error": "rol"})
			return
		}
		js(w, 200, map[string]any{})
	})
	mux.HandleFunc("POST /v2/auth/token/refresh", func(w http.ResponseWriter, r *http.Request) {
		var b map[string]string
		_ = json.NewDecoder(r.Body).Decode(&b)
		if f.refreshBroken {
			js(w, 401, map[string]any{"error": "sessiya tugagan"})
			return
		}
		old := strings.TrimPrefix(b["refresh_token"], "ort_1")
		f.mu.Lock()
		role := f.live[old]
		na := old + "-2"
		f.live[na] = role
		f.mu.Unlock()
		js(w, 200, map[string]any{"access_token": na, "refresh_token": "ort_2" + na})
	})
	mux.HandleFunc("POST /v2/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if !f.logoutBroken {
			f.mu.Lock()
			delete(f.live, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
			f.mu.Unlock()
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	return mux
}

func run(t *testing.T, f *fakeAPI) map[string]Result {
	t.Helper()
	f.live = map[string]string{}
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)
	res := newRunner(Config{API: srv.URL, Web: srv.URL + "/", POS: srv.URL + "/",
		WaiterPhone: "+998999999999", CourierPhone: "+998888888888", TestCode: "666666",
		Timeout: 5 * time.Second}, srv.Client()).Run()
	out := map[string]Result{}
	for _, r := range res {
		out[r.Check] = r
	}
	return out
}

func TestSmoke_AllGreen(t *testing.T) {
	res := run(t, &fakeAPI{})
	want := []string{"catalog", "jwks", "web", "pos", "pro.login", "pro.bridge", "pro.refresh", "pro.logout",
		"go.login", "go.bridge", "go.refresh", "go.logout"}
	if len(res) != len(want) {
		t.Fatalf("tekshiruvlar %d, kutilgan %d: %v", len(res), len(want), res)
	}
	for _, c := range want {
		if r, ok := res[c]; !ok || !r.OK {
			t.Errorf("%s: %+v", c, r)
		}
	}
}

// Har bir haqiqiy nosozlik smoke'da FAIL bo'lishi shart — aks holda
// u "yashil" deb yolg'on gapiradi.
func TestSmoke_DetectsBreakage(t *testing.T) {
	cases := map[string]struct {
		f     *fakeAPI
		check string
	}{
		"chiqishdan keyin token tirik":      {&fakeAPI{logoutBroken: true}, "pro.logout"},
		"JWKS maxfiy kalitni oshkor qiladi": {&fakeAPI{jwksLeaksKey: true}, "jwks"},
		"refresh ishlamaydi":                {&fakeAPI{refreshBroken: true}, "go.refresh"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if r := run(t, c.f)[c.check]; r.OK {
				t.Fatalf("%s: nosozlik sezilmadi (%+v)", c.check, r)
			}
		})
	}
}

func TestSanitize_NoSecretsInLabels(t *testing.T) {
	in := `kirish: HTTP 401 eyJhbGciOiJFZERTQSJ9.abc ort_SECRETVALUE omfa_xyz.sig +998901234567 "quote"` + "\n"
	got := sanitize(in)
	for _, leak := range []string{"eyJhbG", "SECRETVALUE", "omfa_xyz", "901234567", `"`, "\n"} {
		if strings.Contains(got, leak) {
			t.Errorf("yorliqda %q qoldi: %q", leak, got)
		}
	}
	if len([]rune(sanitize(strings.Repeat("x", 500)))) > 120 {
		t.Error("yorliq kesilmadi")
	}
}

func TestMetricsFormat(t *testing.T) {
	m := Metrics([]Result{{Check: "b", OK: true}, {Check: "a", OK: false, Err: "HTTP 500"}}, time.Unix(1700000000, 0))
	for _, want := range []string{
		`ondex_smoke_check_success{check="a",error="HTTP 500"} 0`,
		`ondex_smoke_check_success{check="b",error=""} 1`,
		"ondex_smoke_failed_checks 1",
		"ondex_smoke_last_run_timestamp_seconds 1700000000",
	} {
		if !strings.Contains(m, want) {
			t.Errorf("metrikada yo'q: %s\n%s", want, m)
		}
	}
}
