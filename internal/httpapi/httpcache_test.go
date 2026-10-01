package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func cachedHandler(status int, body string) http.HandlerFunc {
	return publicCache(5, 30, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, status, map[string]string{"v": body})
	})
}

func cacheReq(h http.HandlerFunc, method, target string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func TestPublicCache_AnonymousIsPublicWithETag(t *testing.T) {
	rec := cacheReq(cachedHandler(200, "a"), "GET", "/x", nil)
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=5, stale-while-revalidate=30" {
		t.Fatalf("Cache-Control = %q", cc)
	}
	if rec.Header().Get("ETag") == "" {
		t.Fatal("ETag yo'q")
	}
	// Umumiy kesh nusxasi origin'ga bog'liq bo'lmasligi shart.
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q, kutilgan *", got)
	}
	vary := strings.Join(rec.Header().Values("Vary"), ",")
	if strings.Contains(vary, "Origin") || !strings.Contains(vary, "Authorization") {
		t.Fatalf("Vary = %q (Authorization bo'lishi, Origin bo'lmasligi kerak)", vary)
	}
}

func TestPublicCache_AuthenticatedNeverPublic(t *testing.T) {
	for _, tc := range []struct {
		name, target string
		hdr          map[string]string
	}{
		{"bearer", "/x", map[string]string{"Authorization": "Bearer t"}},
		{"for_me", "/x?for_me=1", nil},
	} {
		rec := cacheReq(cachedHandler(200, "a"), "GET", tc.target, tc.hdr)
		cc := rec.Header().Get("Cache-Control")
		if strings.Contains(cc, "public") || !strings.Contains(cc, "private") {
			t.Errorf("%s: Cache-Control = %q (private bo'lishi kerak)", tc.name, cc)
		}
		// Shaxsiy javobda `*` bo'lmaydi: origin aks ettiriladi (CORS middleware).
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got == "*" {
			t.Errorf("%s: shaxsiy javobda ACAO=* bo'lmasligi kerak", tc.name)
		}
	}
}

func TestPublicCache_NotModified(t *testing.T) {
	h := cachedHandler(200, "a")
	etag := cacheReq(h, "GET", "/x", nil).Header().Get("ETag")
	for _, inm := range []string{etag, "W/" + etag, `"zzz", ` + etag, "*"} {
		rec := cacheReq(h, "GET", "/x", map[string]string{"If-None-Match": inm})
		if rec.Code != http.StatusNotModified || rec.Body.Len() != 0 {
			t.Errorf("If-None-Match=%q: kod %d, tana %d bayt", inm, rec.Code, rec.Body.Len())
		}
	}
	if rec := cacheReq(h, "GET", "/x", map[string]string{"If-None-Match": `"boshqa"`}); rec.Code != 200 || rec.Body.Len() == 0 {
		t.Errorf("boshqa ETag: kod %d", rec.Code)
	}
}

func TestPublicCache_ETagFollowsBody(t *testing.T) {
	a := cacheReq(cachedHandler(200, "a"), "GET", "/x", nil).Header().Get("ETag")
	b := cacheReq(cachedHandler(200, "b"), "GET", "/x", nil).Header().Get("ETag")
	if a == b {
		t.Fatal("tana o'zgardi, ETag o'zgarmadi — eskirgan javob 304 bilan qotib qolardi")
	}
}

func TestPublicCache_ErrorsNotCached(t *testing.T) {
	rec := cacheReq(cachedHandler(404, "yo'q"), "GET", "/x", nil)
	if rec.Code != 404 || rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("ETag") != "" {
		t.Fatalf("xato javob: kod %d, CC %q, ETag %q", rec.Code, rec.Header().Get("Cache-Control"), rec.Header().Get("ETag"))
	}
}

func TestPublicCache_HeadHasNoBody(t *testing.T) {
	rec := cacheReq(cachedHandler(200, "a"), "HEAD", "/x", nil)
	if rec.Code != 200 || rec.Body.Len() != 0 || rec.Header().Get("ETag") == "" {
		t.Fatalf("HEAD: kod %d, tana %d", rec.Code, rec.Body.Len())
	}
}

// Haqiqiy CORS middleware bilan: ochiq javob origin'ga bog'liq emas (`*`),
// tokenli javob esa ruxsat etilgan origin'ni aks ettiradi.
func TestPublicCache_WithRealCORS(t *testing.T) {
	const origin = "https://panel.example"
	h := withCORS(http.HandlerFunc(cachedHandler(200, "a")), []string{origin}, false)
	get := func(hdr map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/x", nil)
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	// Native ilova (Origin yo'q) ham, brauzer ham BIR XIL nusxani oladi.
	for name, hdr := range map[string]map[string]string{
		"native":  nil,
		"brauzer": {"Origin": origin},
		"begona":  {"Origin": "https://boshqa.example"},
	} {
		if got := get(hdr).Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Errorf("%s: ACAO = %q, kutilgan *", name, got)
		}
	}
	rec := get(map[string]string{"Origin": origin, "Authorization": "Bearer t"})
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != origin {
		t.Errorf("tokenli: ACAO = %q, kutilgan %q", got, origin)
	}
}
