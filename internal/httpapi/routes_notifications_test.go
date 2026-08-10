package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"chustapp/internal/storage"
	"chustapp/internal/users"
)

// Push tokeni endpointlari — HTTP DARAJASIDA.
//
// NEGA ALOHIDA KERAK: do'kon qatlamining testlari `SaveToken` ni
// to'g'ridan-to'g'ri chaqiradi va handler mantig'iga (autentifikatsiya,
// uzunlik tekshiruvi, `PushTokens == nil` holati) umuman tegmaydi.
// Aynan shunday bo'shliq tufayli Telegram handlerida bug qolib ketgan
// edi — testlar yashil, ilova ishlamas holatda
// (`routes_auth_telegram_test.go` izohiga qarang).

// recordingTokenStore — `platform` ni ham eslab qoladigan ombor.
// `storage.MemoryTokenStore` uni ataylab tashlab yuboradi, shuning
// uchun uzunlik cheklovini u bilan tekshirib bo'lmaydi.
type recordingTokenStore struct {
	mu       sync.Mutex
	owner    map[string]string // token -> userID
	platform map[string]string // token -> platform
}

func newRecordingTokenStore() *recordingTokenStore {
	return &recordingTokenStore{
		owner:    map[string]string{},
		platform: map[string]string{},
	}
}

func (s *recordingTokenStore) SaveToken(_ context.Context, userID, token, platform string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.owner[token] = userID
	s.platform[token] = platform
	return nil
}

func (s *recordingTokenStore) DeleteToken(_ context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.owner, token)
	delete(s.platform, token)
	return nil
}

func (s *recordingTokenStore) TokensFor(_ context.Context, userID string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for t, u := range s.owner {
		if u == userID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (s *recordingTokenStore) ownerOf(token string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.owner[token]
}

func (s *recordingTokenStore) platformOf(token string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.platform[token]
}

// pushTestServer — token ombori ulangan (yoki `nil` bo'lgan) server va
// ikki foydalanuvchi uchun amaldagi JWT.
func pushTestServer(t *testing.T, store *recordingTokenStore) (
	http.Handler, map[string]string) {
	t.Helper()

	userRepo := storage.NewMemoryUserRepo()
	tokens := users.NewTokenIssuer("test-secret", time.Hour)

	jwts := map[string]string{}
	for _, id := range []string{"u1", "u2"} {
		u := &users.User{ID: id, Phone: "+9989012345" + id[1:],
			Role: users.RoleCustomer, PhoneVerified: true}
		if err := userRepo.Create(context.Background(), u); err != nil {
			t.Fatal(err)
		}
		jwt, err := tokens.Issue(u)
		if err != nil {
			t.Fatal(err)
		}
		jwts[id] = jwt
	}

	deps := Deps{
		UserRepo:      userRepo,
		Tokens:        tokens,
		Notifications: storage.NewMemoryNotificationStore(),
		DevMode:       true,
	}
	// `nil` interfeys qiymatini ATAYLAB shunday beramiz: `store` nil
	// ko'rsatkich bo'lsa ham `Deps.PushTokens` "nil emas" bo'lib
	// qolardi (Go'ning tipli-nil tuzog'i) va `s.PushTokens == nil`
	// tekshiruvi hech qachon ishlamasdi.
	if store != nil {
		deps.PushTokens = store
	}
	return New(deps).Routes(nil), jwts
}

func do(t *testing.T, h http.Handler, method, path, jwt, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	if jwt != "" {
		r.Header.Set("Authorization", "Bearer "+jwt)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// ★ Asosiy oqim: ilova tokenni yuboradi, u egasi bilan saqlanadi.
func TestPushTokenSaved(t *testing.T) {
	store := newRecordingTokenStore()
	h, jwts := pushTestServer(t, store)

	w := do(t, h, "POST", "/me/push-token", jwts["u1"],
		`{"token":"fcm-abc","platform":"android"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("kutilgan 200, keldi %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]bool
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp["saved"] {
		t.Fatalf("saved=false qaytdi: %s", w.Body.String())
	}
	if got := store.ownerOf("fcm-abc"); got != "u1" {
		t.Fatalf("egasi noto'g'ri: %q", got)
	}
	if got := store.platformOf("fcm-abc"); got != "android" {
		t.Fatalf("platform noto'g'ri: %q", got)
	}
}

// ★ XAVFSIZLIK: token YANGI egasiga o'tishi shart.
//
// Bitta telefonda ikki kishi navbat bilan kirsa va token eski egasida
// qolib ketsa, chiqib ketgan odamning telefoniga yangi egasining
// buyurtmalari haqidagi push kelaverardi — ma'lumot sizishi.
func TestPushTokenTransfersToNewOwner(t *testing.T) {
	store := newRecordingTokenStore()
	h, jwts := pushTestServer(t, store)

	do(t, h, "POST", "/me/push-token", jwts["u1"], `{"token":"same-device"}`)
	do(t, h, "POST", "/me/push-token", jwts["u2"], `{"token":"same-device"}`)

	if got := store.ownerOf("same-device"); got != "u2" {
		t.Fatalf("token yangi egasiga o'tmadi: %q", got)
	}
	left, _ := store.TokensFor(context.Background(), "u1")
	if len(left) != 0 {
		t.Fatalf("eski egada token qoldi: %v", left)
	}
}

// ★ XAVFSIZLIK: autentifikatsiyasiz saqlab bo'lmaydi.
//
// Busiz istalgan odam begona qurilmani o'ziga bog'lab, o'sha telefonga
// o'z bildirishnomalarini yuborishi mumkin edi.
func TestPushTokenRequiresAuth(t *testing.T) {
	store := newRecordingTokenStore()
	h, _ := pushTestServer(t, store)

	for _, tc := range []struct{ method, path string }{
		{"POST", "/me/push-token"},
		{"DELETE", "/me/push-token?token=x"},
	} {
		w := do(t, h, tc.method, tc.path, "", `{"token":"x"}`)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: kutilgan 401, keldi %d", tc.method, tc.path, w.Code)
		}
	}
	if store.ownerOf("x") != "" {
		t.Fatal("autentifikatsiyasiz token saqlandi")
	}
}

// Yaroqsiz kirish — 400, do'konga hech narsa yozilmaydi.
func TestPushTokenRejectsBadInput(t *testing.T) {
	store := newRecordingTokenStore()
	h, jwts := pushTestServer(t, store)

	long := `{"token":"` + strings.Repeat("a", 513) + `"}`
	for name, body := range map[string]string{
		"bo'sh":        `{"token":""}`,
		"faqat probel": `{"token":"   "}`,
		"yo'q":         `{}`,
		"juda uzun":    long,
	} {
		w := do(t, h, "POST", "/me/push-token", jwts["u1"], body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: kutilgan 400, keldi %d", name, w.Code)
		}
	}
}

// `platform` kesiladi — lekin so'rov RAD ETILMAYDI.
//
// Bu maydon faqat diagnostika uchun: uni sababli push ro'yxatdan
// o'tmay qolishi tekshiruvning foydasidan ko'ra zararli bo'lardi.
func TestPushTokenClampsPlatform(t *testing.T) {
	store := newRecordingTokenStore()
	h, jwts := pushTestServer(t, store)

	w := do(t, h, "POST", "/me/push-token", jwts["u1"],
		`{"token":"t1","platform":"`+strings.Repeat("x", 5000)+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("kutilgan 200, keldi %d", w.Code)
	}
	if got := len(store.platformOf("t1")); got != 32 {
		t.Fatalf("platform kesilmadi: %d belgi", got)
	}
}

// ★ Chiqish: token o'chiriladi.
//
// O'chirilmasa, chiqib ketgan foydalanuvchining telefoni keyingi
// egasining bildirishnomalarini olishda davom etardi.
func TestPushTokenDeleted(t *testing.T) {
	store := newRecordingTokenStore()
	h, jwts := pushTestServer(t, store)

	do(t, h, "POST", "/me/push-token", jwts["u1"], `{"token":"bye"}`)
	w := do(t, h, "DELETE", "/me/push-token?token=bye", jwts["u1"], "")
	if w.Code != http.StatusOK {
		t.Fatalf("kutilgan 200, keldi %d: %s", w.Code, w.Body.String())
	}
	if store.ownerOf("bye") != "" {
		t.Fatal("token o'chirilmadi")
	}

	// Tokensiz DELETE — 400 (jimgina "ok" EMAS, aks holda ilova
	// o'chirilmaganini bilmasdi).
	if w := do(t, h, "DELETE", "/me/push-token", jwts["u1"], ""); w.Code != http.StatusBadRequest {
		t.Fatalf("tokensiz DELETE: kutilgan 400, keldi %d", w.Code)
	}
}

// Push sozlanmagan (FCM kaliti yo'q) — ilova SINMASLIGI kerak.
//
// `saved:false` qaytadi, 500 EMAS: bildirishnomalar WebSocket va DB
// orqali ishlashda davom etadi, faqat push yo'q.
func TestPushTokenWithoutStore(t *testing.T) {
	h, jwts := pushTestServer(t, nil)

	w := do(t, h, "POST", "/me/push-token", jwts["u1"], `{"token":"t"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("kutilgan 200, keldi %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "true") {
		t.Fatalf("saqlanmagan bo'lsa saved:true qaytmasligi kerak: %s", w.Body.String())
	}
	if w := do(t, h, "DELETE", "/me/push-token?token=t", jwts["u1"], ""); w.Code != http.StatusOK {
		t.Fatalf("DELETE: kutilgan 200, keldi %d", w.Code)
	}
}

// ★ XAVFSIZLIK: bildirishnomalar ro'yxati FAQAT o'ziniki bo'ladi.
func TestNotificationsAreScopedToUser(t *testing.T) {
	store := storage.NewMemoryNotificationStore()
	userRepo := storage.NewMemoryUserRepo()
	tokens := users.NewTokenIssuer("test-secret", time.Hour)
	jwts := map[string]string{}
	for _, id := range []string{"u1", "u2"} {
		u := &users.User{ID: id, Phone: "+9989055555" + id[1:],
			Role: users.RoleCustomer, PhoneVerified: true}
		if err := userRepo.Create(context.Background(), u); err != nil {
			t.Fatal(err)
		}
		jwts[id], _ = tokens.Issue(u)
	}
	h := New(Deps{UserRepo: userRepo, Tokens: tokens,
		Notifications: store, DevMode: true}).Routes(nil)

	w := do(t, h, "GET", "/notifications", jwts["u2"], "")
	if w.Code != http.StatusOK {
		t.Fatalf("kutilgan 200, keldi %d", w.Code)
	}
	// `null` emas, bo'sh massiv — klientda ro'yxatni aylantirish
	// xatoga olib kelmasligi uchun.
	if got := strings.TrimSpace(w.Body.String()); got != "[]" {
		t.Fatalf("bo'sh ro'yxat `[]` bo'lishi kerak, keldi: %s", got)
	}
}
