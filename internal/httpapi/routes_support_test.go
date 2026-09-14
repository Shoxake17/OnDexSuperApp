package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/storage"
	"chustapp/internal/support"
	"chustapp/internal/users"
	"chustapp/internal/ws"
)

func supportServer(t *testing.T) (http.Handler, *support.Service, map[string]string) {
	t.Helper()
	ctx := context.Background()
	userRepo := storage.NewMemoryUserRepo()
	tokens := users.NewTokenIssuer("test-secret", time.Hour)
	jwt := map[string]string{}
	mk := func(key, id string, role users.Role, entityID, phone string) {
		u := &users.User{ID: id, Phone: phone, Role: role, EntityID: entityID, PhoneVerified: true, CreatedAt: time.Now()}
		if err := userRepo.Create(ctx, u); err != nil {
			t.Fatal(err)
		}
		tok, err := tokens.Issue(u)
		if err != nil {
			t.Fatal(err)
		}
		jwt[key] = tok
	}
	mk("a", "u-a", users.RoleRestaurant, testRestA, "+998900000091")
	mk("b", "u-b", users.RoleRestaurant, testRestB, "+998900000092")
	mk("admin", "u-admin", users.RoleAdmin, "", "+998900000093")
	mk("waiter", "u-w", users.RoleWaiter, testRestA, "+998900000094")
	mk("customer", "u-c", users.RoleCustomer, "", "+998900000095")
	mk("courier", "u-k", users.RoleCourier, "cour-1", "+998900000096")

	catalogRepo := storage.NewMemoryCatalogRepo(
		[]catalog.Restaurant{{ID: testRestA, Name: "Book Cafe", Address: "M. Fayozov ko'chasi", Open: true},
			{ID: testRestB, Name: "B", Open: true}}, nil)
	hub := ws.NewHub(nil)
	svc := support.NewService(storage.NewMemorySupportStore(), hub, NewID)
	h := New(Deps{
		UserRepo:       userRepo,
		CatalogRepo:    catalogRepo,
		PromotionsRepo: storage.NewMemoryPromotionsRepo(),
		OrderRepo:      storage.NewMemoryOrderRepo(),
		Tokens:         tokens,
		SupportSvc:     svc,
		Hub:            hub,
		DevMode:        true,
	}).Routes(nil)
	return h, svc, jwt
}

type supportMsgResp struct {
	Items      []support.MessageView `json:"items"`
	NextBefore int64                 `json:"next_before"`
	Thread     support.ThreadView    `json:"thread"`
}

func decodeSupport[T any](t *testing.T, w interface{ Result() *http.Response }, body []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("javob JSON emas: %v — %s", err, body)
	}
	return v
}

func sendSupport(t *testing.T, h http.Handler, path, token, body, clientID string, want int) support.MessageView {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"body": body, "client_id": clientID})
	w := do(t, h, "POST", path, token, string(payload))
	if w.Code != want {
		t.Fatalf("POST %s: kutilgan %d, keldi %d — %s", path, want, w.Code, w.Body.String())
	}
	return decodeSupport[struct {
		Message support.MessageView `json:"message"`
	}](t, w, w.Body.Bytes()).Message
}

func TestSupportRolesAndTenancy(t *testing.T) {
	h, _, jwt := supportServer(t)
	restPath := "/restaurants/" + testRestA + "/support/messages"
	for _, c := range []struct {
		key  string
		want int
	}{
		{"b", http.StatusNotFound},
		{"waiter", http.StatusForbidden},
		{"customer", http.StatusForbidden},
		{"courier", http.StatusForbidden},
		// Admin restoran nomidan yozmaydi — uning yo'li alohida.
		{"admin", http.StatusForbidden},
		{"a", http.StatusOK},
	} {
		if w := do(t, h, "GET", restPath, jwt[c.key], ""); w.Code != c.want {
			t.Errorf("%s GET %s: kutilgan %d, keldi %d", c.key, restPath, c.want, w.Code)
		}
	}
	if w := do(t, h, "GET", restPath, "", ""); w.Code != http.StatusUnauthorized {
		t.Errorf("tokensiz: %d", w.Code)
	}
	for _, p := range []string{"/admin/support/threads", "/admin/support/summary",
		"/admin/support/threads/" + testRestA + "/messages"} {
		for _, key := range []string{"a", "waiter", "customer", "courier"} {
			if w := do(t, h, "GET", p, jwt[key], ""); w.Code != http.StatusForbidden {
				t.Errorf("XAVFSIZLIK: %s %s ga kirdi: %d", key, p, w.Code)
			}
		}
	}
	for _, key := range []string{"a", "waiter", "customer", "courier", "admin"} {
		if w := do(t, h, "GET", "/support/contacts", jwt[key], ""); w.Code != http.StatusOK {
			t.Errorf("%s aloqa ma'lumotlarini o'qiy olmadi: %d", key, w.Code)
		}
	}
	// Admin yo'lida noto'g'ri yoki mavjud bo'lmagan restoran.
	for _, rid := range []string{"yoq", "..%2F..%2Fx", strings.Repeat("a", 65)} {
		if w := do(t, h, "GET", "/admin/support/threads/"+rid+"/messages", jwt["admin"], ""); w.Code != http.StatusNotFound {
			t.Errorf("rid %q: %d", rid, w.Code)
		}
	}
}

func TestSupportChatFlow(t *testing.T) {
	h, _, jwt := supportServer(t)
	restPath := "/restaurants/" + testRestA + "/support/messages"
	adminPath := "/admin/support/threads/" + testRestA + "/messages"

	// Yo'nalish almashtiruvchi (U+202E) va boshqaruv belgisi olib tashlanadi.
	first := sendSupport(t, h, restPath, jwt["a"], "  Salom\xe2\x80\xae, printer\x07 ishlamayapti  ", "client-0001", http.StatusCreated)
	if first.Body != "Salom, printer ishlamayapti" || first.Sender != support.SideRestaurant || first.SenderName != "Book Cafe" {
		t.Fatalf("birinchi xabar: %+v", first)
	}
	// Qayta urinish — ikkinchi xabar YARATILMAYDI.
	again := sendSupport(t, h, restPath, jwt["a"], "Salom, printer ishlamayapti", "client-0001", http.StatusOK)
	if again.ID != first.ID {
		t.Fatalf("takroriy client_id yangi xabar yaratdi: %s != %s", again.ID, first.ID)
	}

	w := do(t, h, "GET", "/admin/support/threads", jwt["admin"], "")
	threads := decodeSupport[struct {
		Items []struct {
			support.ThreadView
			Restaurant supportRestaurantInfo `json:"restaurant"`
		} `json:"items"`
		UnreadTotal int `json:"unread_total"`
	}](t, w, w.Body.Bytes())
	if w.Code != http.StatusOK || len(threads.Items) != 1 || threads.Items[0].Unread != 1 || threads.UnreadTotal != 1 ||
		threads.Items[0].Restaurant.Name != "Book Cafe" || threads.Items[0].MessageCount != 1 {
		t.Fatalf("admin suhbatlari: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "u-a") {
		t.Fatal("XAVFSIZLIK: yuboruvchi akkaunt ID'si javobda")
	}

	reply := sendSupport(t, h, adminPath, jwt["admin"], "Assalomu alaykum! Printer modelini yozing.", "admin-0001", http.StatusCreated)
	if reply.Sender != support.SideAdmin || reply.SenderName != support.AdminName {
		t.Fatalf("admin javobi: %+v", reply)
	}
	// Admin javob yozgani — restoran xabarini ko'rgani.
	if w := do(t, h, "GET", "/admin/support/summary", jwt["admin"], ""); !strings.Contains(w.Body.String(), `"unread":0`) {
		t.Fatalf("admin o'qilmaganlari: %s", w.Body.String())
	}

	w = do(t, h, "GET", restPath, jwt["a"], "")
	got := decodeSupport[supportMsgResp](t, w, w.Body.Bytes())
	if len(got.Items) != 2 || got.Items[0].Seq >= got.Items[1].Seq || got.Thread.Unread != 1 ||
		got.Thread.PeerReadSeq < first.Seq {
		t.Fatalf("restoran ko'rinishi: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "u-admin") {
		t.Fatal("XAVFSIZLIK: admin akkaunt ID'si restoranga ko'rindi")
	}

	// O'qish: chegaradan katta raqam oxirgi xabargacha qisqaradi.
	for _, body := range []string{`{}`, `{"up_to_seq":0}`, `{"up_to_seq":-1}`, `{"up_to_seq":5,"x":1}`, `[]`} {
		if w := do(t, h, "POST", "/restaurants/"+testRestA+"/support/read", jwt["a"], body); w.Code != http.StatusBadRequest {
			t.Errorf("read %s: %d", body, w.Code)
		}
	}
	w = do(t, h, "POST", "/restaurants/"+testRestA+"/support/read", jwt["a"], `{"up_to_seq":999999}`)
	read := decodeSupport[struct {
		Thread support.ThreadView `json:"thread"`
	}](t, w, w.Body.Bytes())
	if w.Code != http.StatusOK || read.Thread.Unread != 0 || read.Thread.ReadSeq != reply.Seq {
		t.Fatalf("o'qish: %d %s", w.Code, w.Body.String())
	}
	sum := do(t, h, "GET", "/restaurants/"+testRestA+"/support/summary", jwt["a"], "")
	if sum.Code != http.StatusOK || !strings.Contains(sum.Body.String(), `"unread":0`) {
		t.Fatalf("summary: %s", sum.Body.String())
	}

	// Begona restoranning suhbati bo'sh.
	w = do(t, h, "GET", "/restaurants/"+testRestB+"/support/messages", jwt["b"], "")
	if other := decodeSupport[supportMsgResp](t, w, w.Body.Bytes()); len(other.Items) != 0 || other.Thread.MessageCount != 0 {
		t.Fatalf("XAVFSIZLIK: B restoran A suhbatini ko'rdi: %s", w.Body.String())
	}

	// Sahifalash va uzilishdan keyin to'ldirish.
	for i, id := range []string{"client-0002", "client-0003", "client-0004"} {
		sendSupport(t, h, restPath, jwt["a"], "Xabar "+string(rune('A'+i)), id, http.StatusCreated)
	}
	w = do(t, h, "GET", restPath+"?limit=2", jwt["a"], "")
	page := decodeSupport[supportMsgResp](t, w, w.Body.Bytes())
	if len(page.Items) != 2 || page.Items[1].Body != "Xabar C" || page.NextBefore != page.Items[0].Seq {
		t.Fatalf("sahifa: %s", w.Body.String())
	}
	w = do(t, h, "GET", restPath+"?limit=2&before="+itoa(page.NextBefore), jwt["a"], "")
	if older := decodeSupport[supportMsgResp](t, w, w.Body.Bytes()); len(older.Items) != 2 || older.Items[1].Seq >= page.Items[0].Seq {
		t.Fatalf("eskiroq sahifa: %s", w.Body.String())
	}
	w = do(t, h, "GET", restPath+"?after="+itoa(reply.Seq), jwt["a"], "")
	if after := decodeSupport[supportMsgResp](t, w, w.Body.Bytes()); len(after.Items) != 3 || after.Items[0].Body != "Xabar A" {
		t.Fatalf("to'ldirish: %s", w.Body.String())
	}
}

func TestSupportValidation(t *testing.T) {
	h, _, jwt := supportServer(t)
	restPath := "/restaurants/" + testRestA + "/support/messages"
	for _, body := range []string{
		`{}`,
		`[]`,
		`{"body":"","client_id":"client-0001"}`,
		"{\"body\":\"   \\u202e  \",\"client_id\":\"client-0001\"}",
		`{"body":"ok","client_id":"short"}`,
		`{"body":"ok","client_id":"bad id with spaces"}`,
		`{"body":"` + strings.Repeat("a", support.MaxBodyLen+1) + `","client_id":"client-0001"}`,
		`{"body":"ok","client_id":"client-0001","sender":"admin"}`,
	} {
		if w := do(t, h, "POST", restPath, jwt["a"], body); w.Code != http.StatusBadRequest {
			t.Errorf("%.60s: %d", body, w.Code)
		}
	}
	for _, q := range []string{"?limit=0", "?limit=101", "?before=-1", "?after=abc", "?before=5&after=3"} {
		if w := do(t, h, "GET", restPath+q, jwt["a"], ""); w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", q, w.Code)
		}
	}
	// Chegaradagi uzunlik o'tadi (2000 belgi, ko'p baytli harflar bilan).
	sendSupport(t, h, restPath, jwt["a"], strings.Repeat("ў", support.MaxBodyLen), "client-long1", http.StatusCreated)
}

func TestSupportRateLimit(t *testing.T) {
	h, _, jwt := supportServer(t)
	restPath := "/restaurants/" + testRestA + "/support/messages"
	limited := false
	for i := 0; i < 20; i++ {
		payload, _ := json.Marshal(map[string]string{"body": "spam", "client_id": "spam-" + itoa(int64(1000+i))})
		w := do(t, h, "POST", restPath, jwt["a"], string(payload))
		if w.Code == http.StatusTooManyRequests {
			if w.Header().Get("Retry-After") == "" {
				t.Fatal("429 da Retry-After yo'q")
			}
			limited = true
			break
		}
		if w.Code != http.StatusCreated {
			t.Fatalf("%d-xabar: %d %s", i, w.Code, w.Body.String())
		}
	}
	if !limited {
		t.Fatal("XAVFSIZLIK: chat yuborishda tezlik chegarasi ishlamadi")
	}
	// Chegara akkaunt bo'yicha: boshqa restoran bunga tushmaydi.
	sendSupport(t, h, "/restaurants/"+testRestB+"/support/messages", jwt["b"], "Salom", "client-b0001", http.StatusCreated)
}

func TestSupportContacts(t *testing.T) {
	h, _, jwt := supportServer(t)
	path := "/admin/support/contacts"
	for _, key := range []string{"a", "waiter", "customer", "courier"} {
		if w := do(t, h, "PUT", path, jwt[key], `{"phone":"+998901234567"}`); w.Code != http.StatusForbidden {
			t.Errorf("XAVFSIZLIK: %s aloqa ma'lumotlarini o'zgartirdi: %d", key, w.Code)
		}
	}
	if w := do(t, h, "GET", "/support/contacts", jwt["a"], ""); !strings.Contains(w.Body.String(), `"configured":false`) {
		t.Fatalf("bo'sh holat: %s", w.Body.String())
	}
	for _, body := range []string{
		`{"phone":"123"}`,
		`{"phone":"+7 999 123 45 67"}`,
		`{"email":"a@b"}`,
		`{"email":"Ali <ali@ondex.uz>"}`,
		`{"email":"ali@ondex.uz\r\nBcc: x@y.uz"}`,
		`{"email":"ali@ондекс.uz"}`,
		`{"telegram":"@ab"}`,
		`{"telegram":"bad name"}`,
		`{"telegram":"javascript:alert(1)"}`,
		`{"telegram":"https://evil.com/ondex_support"}`,
		`{"telegram":"ondex__support"}`,
		"{\"phone_hours\":\"09:00\\u0000\"}",
		`{"email_note":"` + strings.Repeat("a", 61) + `"}`,
		`{"phone":"+998901234567","role":"admin"}`,
	} {
		if w := do(t, h, "PUT", path, jwt["admin"], body); w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d %s", body, w.Code, w.Body.String())
		}
	}
	w := do(t, h, "PUT", path, jwt["admin"], `{"phone":"+998 90 123-45-67","phone_hours":"09:00 – 22:00 (har kuni)",`+
		`"telegram":"https://t.me/OnDex_Support","email":"Support@OnDex.uz","email_note":"24/7 javob beramiz"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("saqlash: %d %s", w.Code, w.Body.String())
	}
	for _, key := range []string{"a", "waiter", "customer", "admin"} {
		w := do(t, h, "GET", "/support/contacts", jwt[key], "")
		c := decodeSupport[support.ContactsView](t, w, w.Body.Bytes())
		if c.Phone != "+998901234567" || c.Telegram != "OnDex_Support" || c.TelegramURL != "https://t.me/OnDex_Support" ||
			c.Email != "support@ondex.uz" || c.PhoneHours != "09:00 – 22:00 (har kuni)" || !c.Configured || c.UpdatedAt == nil {
			t.Fatalf("%s ko'rinishi: %s", key, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "u-admin") {
			t.Fatal("XAVFSIZLIK: tahrirlagan admin ID'si javobda")
		}
	}
	// Tozalash — hammasi bo'sh.
	if w := do(t, h, "PUT", path, jwt["admin"], `{}`); !strings.Contains(w.Body.String(), `"configured":false`) {
		t.Fatalf("tozalash: %s", w.Body.String())
	}
}
