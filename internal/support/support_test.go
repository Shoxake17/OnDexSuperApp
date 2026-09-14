package support_test

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"chustapp/internal/storage"
	"chustapp/internal/support"
)

func TestNormalizeTelegram(t *testing.T) {
	for in, want := range map[string]string{
		"":                                "",
		"@ondex_support":                  "ondex_support",
		"ondex_support":                   "ondex_support",
		"t.me/OnDex_Support":              "OnDex_Support",
		"https://t.me/ondex_support/":     "ondex_support",
		"HTTPS://T.ME/ondexsupport":       "ondexsupport",
		"https://telegram.me/ondex1":      "ondex1",
		"  @Shoxrux_2026  ":               "Shoxrux_2026",
		"@abcde":                          "abcde",
		"@a2345678901234567890123456789b": "a2345678901234567890123456789b",
	} {
		got, err := support.NormalizeTelegram(in)
		if err != nil || got != want {
			t.Errorf("%q: %q, %v (kutilgan %q)", in, got, err, want)
		}
	}
	for _, in := range []string{"@", "@abcd", "_ondex", "1ondex", "ondex_", "ondex__support", "ondex support",
		"https://evil.com/ondex", "t.me/ondex?start=x", "ondex/../x", "@" + strings.Repeat("a", 33), "онлайн_yordam"} {
		if got, err := support.NormalizeTelegram(in); err == nil {
			t.Errorf("%q qabul qilindi: %q", in, got)
		}
	}
}

func TestNormalizeEmail(t *testing.T) {
	for in, want := range map[string]string{
		"":                        "",
		"support@ondex.uz":        "support@ondex.uz",
		" Shoxrux.T+1@Gmail.com ": "shoxrux.t+1@gmail.com",
		"a@mail.example.co":       "a@mail.example.co",
	} {
		got, err := support.NormalizeEmail(in)
		if err != nil || got != want {
			t.Errorf("%q: %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"a@b", "@ondex.uz", "a@", "a..b@ondex.uz", "Ali <ali@ondex.uz>", "ali@ondex.uz>",
		"ali @ondex.uz", "ali@ondex.uz\nBcc:x@y.uz", "ali@ондекс.uz", "\"ali\"@ondex.uz",
		strings.Repeat("a", 65) + "@ondex.uz", "javascript:alert(1)@x.uz"} {
		if got, err := support.NormalizeEmail(in); err == nil {
			t.Errorf("%q qabul qilindi: %q", in, got)
		}
	}
}

func TestNormalizeBody(t *testing.T) {
	got, err := support.NormalizeBody("\r\n Salom\xe2\x80\xae\xe2\x81\xa6 dunyo\x00\r\nikkinchi\tqator \xf0\x9f\x91\xa8\xe2\x80\x8d\xf0\x9f\x8d\xb3 ")
	if err != nil || got != "Salom dunyo\nikkinchi qator \xf0\x9f\x91\xa8\xe2\x80\x8d\xf0\x9f\x8d\xb3" {
		t.Fatalf("%q, %v", got, err)
	}
	for _, in := range []string{"", "   ", "\xe2\x80\xae\xe2\x80\x8f", strings.Repeat("\xd1\x8f", support.MaxBodyLen+1)} {
		if _, err := support.NormalizeBody(in); err == nil || !support.IsValidation(err) {
			t.Errorf("%.20q qabul qilindi", in)
		}
	}
	// Rasm izohi bo'sh bo'lishi mumkin, lekin chegara bir xil.
	if got, err := support.NormalizeCaption("  \xe2\x80\xae "); err != nil || got != "" {
		t.Fatalf("bo'sh izoh: %q %v", got, err)
	}
	if _, err := support.NormalizeCaption(strings.Repeat("a", support.MaxBodyLen+1)); err == nil {
		t.Fatal("uzun izoh qabul qilindi")
	}
}

type captured struct {
	key string
	v   map[string]any
}

type fakeHub struct {
	mu   sync.Mutex
	sent []captured
}

func (h *fakeHub) Send(key string, v any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sent = append(h.sent, captured{key, v.(map[string]any)})
}

func newService(hub support.Sender) *support.Service {
	n := 0
	return support.NewService(storage.NewMemorySupportStore(), hub, func() string { n++; return "m" + strconv.Itoa(n) }).
		WithClock(func() time.Time { return time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC) })
}

func TestServiceLiveEventsPerSide(t *testing.T) {
	ctx := context.Background()
	hub := &fakeHub{}
	svc := newService(hub)

	if _, _, _, err := svc.Send(ctx, "rest-a", support.Author{Side: support.SideRestaurant, UserID: "u-a", Name: "Book Cafe"},
		"Salom", "client-0001", nil); err != nil {
		t.Fatal(err)
	}
	if len(hub.sent) != 2 || hub.sent[0].key != support.RestaurantTopic("rest-a") || hub.sent[1].key != support.AdminTopic() {
		t.Fatalf("kanallar: %+v", hub.sent)
	}
	restThread := hub.sent[0].v["thread"].(support.ThreadView)
	adminThread := hub.sent[1].v["thread"].(support.ThreadView)
	if restThread.Unread != 0 || adminThread.Unread != 1 {
		t.Fatalf("o'qilmaganlar tomonga mos emas: restoran=%d admin=%d", restThread.Unread, adminThread.Unread)
	}
	// Takroriy yuborish jonli kanalga ikkinchi marta chiqmaydi.
	if _, _, created, _ := svc.Send(ctx, "rest-a", support.Author{Side: support.SideRestaurant, UserID: "u-a"},
		"Salom", "client-0001", nil); created || len(hub.sent) != 2 {
		t.Fatalf("takror: created=%v sent=%d", created, len(hub.sent))
	}
	// Boshqa akkaunt o'sha client_id bilan — alohida xabar (to'qnashuv emas).
	if _, _, created, _ := svc.Send(ctx, "rest-a", support.Author{Side: support.SideAdmin, UserID: "u-admin"},
		"Javob", "client-0001", nil); !created {
		t.Fatal("boshqa yuboruvchining client_id si to'qnashdi")
	}
	// O'qish belgisi faqat oldinga.
	th, err := svc.MarkRead(ctx, "rest-a", support.SideRestaurant, 1)
	if err != nil || th.RestaurantReadSeq != 1 {
		t.Fatalf("%+v %v", th, err)
	}
	before := len(hub.sent)
	if th, _ := svc.MarkRead(ctx, "rest-a", support.SideRestaurant, 1); th.RestaurantReadSeq != 1 || len(hub.sent) != before {
		t.Fatal("o'zgarmagan o'qish jonli kanalga chiqdi")
	}
	if _, err := svc.MarkRead(ctx, "rest-a", support.Side("hacker"), 1); err == nil {
		t.Fatal("noto'g'ri tomon qabul qilindi")
	}
}

func TestServiceImageMessage(t *testing.T) {
	ctx := context.Background()
	hub := &fakeHub{}
	svc := newService(hub)
	author := support.Author{Side: support.SideRestaurant, UserID: "u-a", Name: "Book Cafe"}
	img := &support.ImageInput{Data: []byte("RIFF....WEBPVP8 fake"), Width: 800, Height: 600}

	m, th, created, err := svc.Send(ctx, "rest-a", author, "  ", "image-0001", img)
	if err != nil || !created || m.Body != "" || m.Attachment == nil || m.Attachment.Width != 800 ||
		m.Attachment.ContentType != support.AttachmentContentType || m.Attachment.Size != len(img.Data) {
		t.Fatalf("rasmli xabar: %+v %v", m, err)
	}
	if v := support.ToThreadView(th, support.SideAdmin); !v.LastHasImage || v.LastBody != "Rasm" {
		t.Fatalf("suhbat ko'rinishi: %+v", v)
	}
	// Jonli kanalga faqat metama'lumot chiqadi, baytlar emas.
	raw, _ := json.Marshal(hub.sent[0].v)
	if strings.Contains(string(raw), "WEBPVP8") || !strings.Contains(string(raw), `"attachment":{"id":"`) {
		t.Fatalf("jonli hodisa: %s", raw)
	}
	got, err := svc.Attachment(ctx, "rest-a", m.Attachment.ID)
	if err != nil || string(got.Data) != string(img.Data) {
		t.Fatalf("rasm olish: %v", err)
	}
	for _, c := range []struct{ rid, id string }{{"rest-b", m.Attachment.ID}, {"rest-a", "../etc/passwd"}, {"rest-a", ""}} {
		if _, err := svc.Attachment(ctx, c.rid, c.id); err != support.ErrAttachmentNotFound {
			t.Errorf("XAVFSIZLIK: %+v -> %v", c, err)
		}
	}
	// Chegaralar: juda katta, bo'sh yoki o'lchamsiz rasm.
	for _, bad := range []*support.ImageInput{
		{Data: make([]byte, support.MaxAttachmentBytes+1), Width: 10, Height: 10},
		{Data: nil, Width: 10, Height: 10},
		{Data: []byte("x"), Width: 0, Height: 10},
	} {
		if _, _, _, err := svc.Send(ctx, "rest-a", author, "", "image-bad-"+strconv.Itoa(len(bad.Data)+bad.Width), bad); !support.IsValidation(err) {
			t.Errorf("noto'g'ri rasm qabul qilindi: %v", err)
		}
	}
	// Rasmsiz bo'sh matn hali ham rad etiladi.
	if _, _, _, err := svc.Send(ctx, "rest-a", author, " ", "text-empty-01", nil); !support.IsValidation(err) {
		t.Fatalf("bo'sh matn: %v", err)
	}
}
