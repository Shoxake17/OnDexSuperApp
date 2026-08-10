package notify

import (
	"strings"
	"testing"
)

// Xat FORMATIDAGI xatolar jimgina o'tib ketadi: server "yuborildi"
// deydi, foydalanuvchi esa buzilgan yoki kesilgan xat oladi. Shuning
// uchun tuzilish shu yerda tekshiriladi.

func TestBuildMessagePlainOnly(t *testing.T) {
	msg := buildMessage("OnDex", "a@ondex.uz", "b@example.com", "Mavzu", "matn", "")
	if strings.Contains(msg, "multipart/alternative") {
		t.Fatal("HTML yo'q bo'lsa multipart bo'lmasligi kerak")
	}
	if !strings.Contains(msg, `Content-Type: text/plain; charset="UTF-8"`) {
		t.Fatal("text/plain sarlavhasi yo'q")
	}
	if !strings.Contains(msg, "matn") {
		t.Fatal("tana yo'q")
	}
}

// TARTIB MUHIM: RFC 2046 bo'yicha OXIRGI qism "eng boyi" deb
// qaraladi. Matn birinchi, HTML ikkinchi bo'lmasa ko'p mijozlar
// bezatilgan xat o'rniga oddiy matnni ko'rsatadi.
func TestBuildMessageMultipartOrder(t *testing.T) {
	msg := buildMessage("OnDex", "a@ondex.uz", "b@example.com", "Mavzu",
		"matnli nusxa", "<html>bezatilgan</html>")

	if !strings.Contains(msg, "multipart/alternative") {
		t.Fatal("multipart/alternative yo'q")
	}
	iText := strings.Index(msg, `Content-Type: text/plain`)
	iHTML := strings.Index(msg, `Content-Type: text/html`)
	if iText < 0 || iHTML < 0 {
		t.Fatal("ikkala qism ham bo'lishi kerak")
	}
	if iText > iHTML {
		t.Fatal("matn HTML'dan OLDIN kelishi kerak (aks holda mijoz matnni ko'rsatadi)")
	}
	// Yopuvchi chegara bo'lmasa ba'zi mijozlar xatni buzilgan deb biladi.
	if !strings.Contains(msg, "--ondex-") || !strings.HasSuffix(strings.TrimSpace(msg), "--") {
		t.Fatal("chegara (boundary) to'g'ri yopilmagan")
	}
}

func TestBuildMessageHasDeliverabilityHeaders(t *testing.T) {
	msg := buildMessage("OnDex", "a@ondex.uz", "b@example.com", "Mavzu", "matn", "<b>x</b>")
	// Message-ID yo'qligi spam balini oshiradi.
	if !strings.Contains(msg, "Message-ID: <") {
		t.Fatal("Message-ID yo'q")
	}
	if !strings.Contains(msg, "@ondex.uz>") {
		t.Fatal("Message-ID domeni jo'natuvchi domeniga mos emas")
	}
	if !strings.Contains(msg, "Auto-Submitted: auto-generated") {
		t.Fatal("Auto-Submitted yo'q — avtojavoblar qaytadi va xat rassilka deb qaraladi")
	}
}

// Yakka "." qatori SMTP'da xat OXIRI deb qabul qilinadi — himoyasiz
// xat o'sha joyda kesilib qolardi (RFC 5321).
func TestDotStuffing(t *testing.T) {
	if got := dotStuff("qator\r\n.yashirin"); !strings.Contains(got, "\r\n..yashirin") {
		t.Fatalf("CRLF dan keyingi nuqta ikkilanmadi: %q", got)
	}
	if got := dotStuff("qator\n.yashirin"); !strings.Contains(got, "\n..yashirin") {
		t.Fatalf("LF dan keyingi nuqta ikkilanmadi: %q", got)
	}
}

// SMTP header injection: manzil yoki mavzudagi CR/LF hujumchiga o'z
// sarlavhasini (masalan `Bcc:`) qo'shish imkonini berardi.
func TestGuardHeaderRejectsInjection(t *testing.T) {
	bad := [][2]string{
		{"a@b.uz\r\nBcc: qurbon@x.uz", "Mavzu"},
		{"a@b.uz\nBcc: qurbon@x.uz", "Mavzu"},
		{"a@b.uz", "Mavzu\r\nBcc: qurbon@x.uz"},
		{"", "Mavzu"},
		{"emailemas", "Mavzu"},
	}
	for _, c := range bad {
		if err := guardHeader(c[0], c[1]); err == nil {
			t.Fatalf("qabul qilinmasligi kerak edi: to=%q subject=%q", c[0], c[1])
		}
	}
	if err := guardHeader("a@b.uz", "Oddiy mavzu"); err != nil {
		t.Fatalf("to'g'ri qiymat rad etildi: %v", err)
	}
}

func TestLogEmailGuards(t *testing.T) {
	if err := (LogEmail{}).Send("a@b.uz\r\nBcc: x@y.uz", "M", "t", ""); err == nil {
		t.Fatal("dev yuboruvchi ham injection'ni rad etishi kerak")
	}
}
