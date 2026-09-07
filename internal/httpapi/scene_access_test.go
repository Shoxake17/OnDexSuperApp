package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/users"
)

// Bu fayl bitta qoidani qo'riqlaydi: 3D maket havolasi
// AUTENTIFIKATSIYASIZ javobda CHIQMASLIGI kerak.
//
// Maket ~100-220 MB va ichida bajariladigan kod bor. Ilgari u ochiq
// `GET /restaurants` javobida turardi, ya'ni javobni o'qigan har kim
// (bot ham) uni cheksiz yuklab olardi.

const testSceneURL = "https://pub-test.r2.dev/scenes/bookcafe.pck"

func sceneTestRestaurant() *catalog.Restaurant {
	return &catalog.Restaurant{
		ID:            "r1",
		Name:          "Book Cafe",
		Scene3DURL:    testSceneURL,
		Scene3DSHA256: strings.Repeat("a", 64),
		Scene3DBytes:  115271748,
	}
}

func sceneTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	tokens := users.NewTokenIssuer("test-secret-test-secret-test-sec", time.Hour)
	tok, err := tokens.Issue(&users.User{ID: "u1", Role: users.RoleCustomer})
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Deps: Deps{
		Tokens:             tokens,
		MediaPublicBaseURL: "https://pub-test.r2.dev",
	}}
	return s, tok
}

// Token yo'q -> uchala maydon ham javobda BO'LMASLIGI kerak.
func TestSceneFieldsHiddenWithoutToken(t *testing.T) {
	s, _ := sceneTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/restaurants", nil)

	got := s.sceneViewOne(r, sceneTestRestaurant())

	if got.Scene3DURL != "" {
		t.Errorf("maket MANZILI ochiq javobda qoldi: %q", got.Scene3DURL)
	}
	if got.Scene3DSHA256 != "" {
		t.Errorf("maket XESHI ochiq javobda qoldi: %q", got.Scene3DSHA256)
	}
	if got.Scene3DBytes != 0 {
		t.Errorf("maket HAJMI ochiq javobda qoldi: %d", got.Scene3DBytes)
	}

	// JSON darajasida ham tekshiramiz: `omitempty` tufayli kalitlar
	// umuman bo'lmasligi kerak. Struct'ni tekshirish yetarli emas —
	// mijoz JSON ko'radi.
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"scene_3d_url", "scene_3d_sha256", "scene_3d_bytes"} {
		if strings.Contains(string(b), key) {
			t.Errorf("JSON da %q kaliti bor: %s", key, b)
		}
	}
}

// Token bor -> maydonlar ko'rinadi (imzolovchi yo'q, shuning uchun
// saqlangan manzil o'zgarishsiz qaytadi).
func TestSceneFieldsVisibleWithToken(t *testing.T) {
	s, tok := sceneTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/restaurants", nil)
	r.Header.Set("Authorization", "Bearer "+tok)

	got := s.sceneViewOne(r, sceneTestRestaurant())

	if got.Scene3DURL != testSceneURL {
		t.Errorf("kirgan foydalanuvchi maketni ko'rishi kerak, olindi: %q", got.Scene3DURL)
	}
	if got.Scene3DBytes == 0 {
		t.Error("hajm ko'rsatilishi kerak — ilova 'ncha MB yuklanadi' deb aytadi")
	}
}

// Yaroqsiz token — token YO'Q bilan bir xil natija berishi kerak.
func TestSceneFieldsHiddenWithBrokenToken(t *testing.T) {
	s, _ := sceneTestServer(t)
	for _, bad := range []string{"Bearer ", "Bearer abc", "Basic xyz", "Bearer a.b.c"} {
		r := httptest.NewRequest(http.MethodGet, "/restaurants", nil)
		r.Header.Set("Authorization", bad)
		if got := s.sceneViewOne(r, sceneTestRestaurant()); got.Scene3DURL != "" {
			t.Errorf("yaroqsiz token (%q) bilan maket ko'rindi", bad)
		}
	}
}

// Ro'yxat yo'li ham bir xil qoidaga bo'ysunsin — `GET /restaurants`
// aynan shu yo'ldan o'tadi va u eng ko'p so'raladigan endpoint.
func TestSceneFieldsHiddenInListWithoutToken(t *testing.T) {
	s, _ := sceneTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/restaurants", nil)

	list := s.sceneViewMany(r, []*catalog.Restaurant{sceneTestRestaurant(), sceneTestRestaurant()})
	if len(list) != 2 {
		t.Fatalf("ikkita restoran kutilgandi, olindi %d", len(list))
	}
	for i, rest := range list {
		if rest.Scene3DURL != "" {
			t.Errorf("ro'yxatdagi %d-restoranda maket ochiq qoldi", i)
		}
	}
}
