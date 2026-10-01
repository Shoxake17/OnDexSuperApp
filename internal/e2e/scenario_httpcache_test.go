package e2e

import (
	"net/http"
	"strings"
	"testing"
)

// Senariy: OCHIQ GET javoblarning HTTP keshi (Cache-Control + ETag + 304).
//
// Xavf ikki tomonlama: (1) kesh yo'q bo'lsa har so'rov serverga yetadi;
// (2) kesh NOTO'G'RI bo'lsa tokenli foydalanuvchi javobi (imzolangan maket
// havolasi) CDN orqali begonaga ko'rinadi. Shuning uchun ochiq javob
// `public`, tokenli javob esa HECH QACHON `public` bo'lmasligi tekshiriladi
// (bundan tashqari har tokenli javobga umumiy invariant qo'llanadi).

func TestPublicHTTPCaching(t *testing.T) {
	w := NewWorld(t, "Ochiq javoblarning HTTP keshi")
	rid := w.Owner.Entity

	paths := []string{
		"/restaurants",
		"/restaurants/" + rid,
		"/restaurants/" + rid + "/menu",
		"/restaurants/" + rid + "/active-promotions",
		"/categories",
	}

	w.Step(t, "anonim javob: public + ETag + Vary", func(t *testing.T) {
		for _, p := range paths {
			r := w.Do(t, nil, "GET", p, nil).Expect(t, 200)
			if cc := r.Header.Get("Cache-Control"); !strings.HasPrefix(cc, "public, max-age=") {
				t.Errorf("%s: Cache-Control = %q", p, cc)
			}
			if r.Header.Get("ETag") == "" {
				t.Errorf("%s: ETag yo'q", p)
			}
			vary := strings.Join(r.Header.Values("Vary"), ",")
			if !strings.Contains(vary, "Authorization") || !strings.Contains(vary, "Origin") {
				t.Errorf("%s: Vary = %q (Origin va Authorization bo'lishi kerak)", p, vary)
			}
		}
	})

	w.Step(t, "tokenli va for_me javoblar public EMAS", func(t *testing.T) {
		for _, p := range paths {
			r := w.Do(t, w.Customer, "GET", p, nil).Expect(t, 200)
			if cc := r.Header.Get("Cache-Control"); strings.Contains(cc, "public") {
				t.Errorf("tokenli %s: Cache-Control = %q", p, cc)
			}
		}
		r := w.Do(t, nil, "GET", "/restaurants?for_me=1", nil).Expect(t, 200)
		if cc := r.Header.Get("Cache-Control"); strings.Contains(cc, "public") {
			t.Errorf("for_me: Cache-Control = %q", cc)
		}
	})

	w.Step(t, "If-None-Match -> 304 tanasiz; boshqa ETag -> 200", func(t *testing.T) {
		url := w.Srv.URL + "/restaurants/" + rid + "/menu"
		etag := w.Do(t, nil, "GET", "/restaurants/"+rid+"/menu", nil).Header.Get("ETag")
		get := func(inm string) *http.Response {
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("If-None-Match", inm)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			return res
		}
		if res := get(etag); res.StatusCode != http.StatusNotModified || res.ContentLength > 0 {
			t.Errorf("mos ETag: kod %d", res.StatusCode)
		}
		if res := get(`"eskirgan"`); res.StatusCode != 200 {
			t.Errorf("boshqa ETag: kod %d", res.StatusCode)
		}
	})

	w.Step(t, "xato javob keshlanmaydi", func(t *testing.T) {
		r := w.Do(t, nil, "GET", "/restaurants/yoq-restoran", nil).Expect(t, 404)
		if cc := r.Header.Get("Cache-Control"); cc != "no-store" {
			t.Errorf("404: Cache-Control = %q", cc)
		}
	})
}
