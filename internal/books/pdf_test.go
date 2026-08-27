package books

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// minimalPDF — matnli eng kichik yaroqli PDF.
//
// Fikstura fayl sifatida emas, kod ichida: sinov repozitoriyadan
// tashqaridagi hech narsaga bog'liq bo'lmasligi kerak.
func minimalPDF(t *testing.T, text string) []byte {
	t.Helper()

	content := "BT /F1 12 Tf 72 720 Td (" + text + ") Tj ET"
	var b bytes.Buffer
	offsets := make([]int, 0, 6)

	b.WriteString("%PDF-1.4\n")
	add := func(s string) {
		offsets = append(offsets, b.Len())
		b.WriteString(s)
	}
	add("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	add("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")
	add("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] " +
		"/Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>\nendobj\n")
	add("4 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n")
	add("5 0 obj\n<< /Length " + itoa(len(content)) + " >>\nstream\n" + content + "\nendstream\nendobj\n")

	xref := b.Len()
	b.WriteString("xref\n0 6\n0000000000 65535 f \n")
	for _, off := range offsets {
		b.WriteString(pad10(off) + " 00000 n \n")
	}
	b.WriteString("trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n" + itoa(xref) + "\n%%EOF\n")
	return b.Bytes()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}

func pad10(n int) string {
	s := itoa(n)
	for len(s) < 10 {
		s = "0" + s
	}
	return s
}

func TestExtractText(t *testing.T) {
	// Fikstura ATAYLAB uzun: `hasProse` juda qisqa matnni "nasr emas"
	// deb rad etadi va bu to'g'ri xatti-harakat. Bir necha so'zli
	// fikstura sinovni o'tkazsa, chegara aslida sinalmagan bo'lardi.
	sentence := "Kitob matni shu yerda turadi va sahifalarga bolinadi. "
	raw := minimalPDF(t, strings.Repeat(sentence, 4))
	text, pages, err := ExtractText(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("ExtractText xato: %v", err)
	}
	if pages != 1 {
		t.Errorf("sahifa soni = %d, kutilgani 1", pages)
	}
	if !strings.Contains(text, "Kitob matni shu yerda turadi") {
		t.Errorf("matn topilmadi, olingani: %q", text)
	}
}

// Sahifa raqamlaridan boshqa hech narsasi yo'q PDF - skanerlangan
// kitobning aynan o'zi. `ExtractText` uni matnli kitob deb qabul
// qilmasligi kerak.
func TestExtractTextScannedBook(t *testing.T) {
	var body string
	for i := 1; i <= 40; i++ {
		body += fmt.Sprintf("%d ", i)
	}
	raw := minimalPDF(t, body)
	_, _, err := ExtractText(bytes.NewReader(raw), int64(len(raw)))
	if err != ErrNoText {
		t.Errorf("xato = %v, kutilgani ErrNoText", err)
	}
}

// PDF tahlilchilari buzilgan faylda PANIC qilishga moyil. Handler
// ichidagi tutilmagan panic butun jarayonni yiqitardi, shuning uchun
// tiklash aynan shu yerda tekshiriladi.
func TestExtractTextDoesNotPanic(t *testing.T) {
	cases := map[string][]byte{
		"bo'sh":             {},
		"faqat imzo":        []byte("%PDF-1.4\n"),
		"kesilgan":          minimalPDF(t, "Salom")[:40],
		"axlat":             bytes.Repeat([]byte{0xFF, 0x00, 0xAB}, 300),
		"yolg'on startxref": []byte("%PDF-1.4\ntrailer\n<< /Size 6 >>\nstartxref\n999999999\n%%EOF\n"),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			// Yiqilmasligi yetarli; xato qaytarishi kutiladi.
			if _, _, err := ExtractText(bytes.NewReader(raw), int64(len(raw))); err == nil {
				t.Log("xato qaytmadi (yiqilmagani muhim)")
			}
		})
	}
}

// Skanerlangan kitob — matnsiz PDF. Bu XATO EMAS, alohida holat:
// foydalanuvchiga aynan shu sabab aytilishi kerak.
func TestExtractTextNoText(t *testing.T) {
	raw := minimalPDF(t, "")
	_, _, err := ExtractText(bytes.NewReader(raw), int64(len(raw)))
	if err != ErrNoText {
		t.Errorf("xato = %v, kutilgani ErrNoText", err)
	}
}

// Skanerlangan kitobda matn qatlami BO'SH BO'LMAYDI: har sahifada
// bosilgan raqam qoladi. Haqiqiy holat (2026-08-27) — "Vavilonlik eng
// boy odam" dan 1169 belgi ajratildi va hammasi "1 2 3 4 5 ..." edi.
// Bo'shlik tekshiruvi buni o'tkazib yuborardi.
func TestHasProse(t *testing.T) {
	pageNumbers := ""
	for i := 1; i <= 120; i++ {
		pageNumbers += fmt.Sprintf("%d\n \n \n\n", i)
	}

	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"faqat sahifa raqamlari", pageNumbers, false},
		{"bo'sh", "", false},
		{"bir-ikki harf raqamlar orasida", "1\n2\nа\n3\n4\nb\n5\n", false},
		{"raqamli jadval", "12 345 | 67 890 | 11 222 | 33 444 | 55 666 | 77 888", false},
		{"oddiy nasr", strings.Repeat("Kitob matni shu yerda turadi. ", 5), true},
		{"kirill nasr", strings.Repeat("Китоб матни шу ерда туради. ", 5), true},
		// Raqamlari ko'p, lekin nasri ham bor sahifa - rad etilmasin.
		{"sana va nasr", "2026-07-24 00:29:10\n\n" +
			strings.Repeat("O'zbekiston Respublikasi Ichki ishlar vazirligi. ", 3), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := hasProse(c.in); got != c.want {
				t.Errorf("hasProse() = %v, kutilgani %v", got, c.want)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	cases := []struct{ in, want string }{
		// Apostrof: o'zbek matnida harf o'rnida turadi, yo'qotib
		// bo'lmaydi.
		{"O\x19zbekiston", "O'zbekiston"},
		// Qolgan boshqaruv belgilari olib tashlanadi.
		{"a\x16b\x00c", "abc"},
		{"a\r\nb", "a\nb"},
		{"a\n\n\n\n\nb", "a\n\nb"},
		{"a    b", "a b"},
		// Tab va yangi qator saqlanadi.
		{"a\tb", "a\tb"},
	}
	for _, c := range cases {
		if got := normalize(c.in); got != c.want {
			t.Errorf("normalize(%q) = %q, kutilgani %q", c.in, got, c.want)
		}
	}
}

// Ko'p baytli belgini O'RTASIDAN kesish yaroqsiz UTF-8 hosil qiladi va
// JSON kodlashda `<?>` bo'lib chiqadi. Kirill/o'zbek matnida bu deyarli
// har kesishda uchraydi.
func TestTruncateAtRune(t *testing.T) {
	s := "Ўзбекистон" // har harf 2 bayt
	for n := 1; n <= len(s); n++ {
		got := truncateAtRune(s, n)
		if len(got) > n {
			t.Fatalf("truncateAtRune(_, %d) uzunligi %d — chegaradan oshdi", n, len(got))
		}
		if !isValidUTF8(got) {
			t.Fatalf("truncateAtRune(_, %d) yaroqsiz UTF-8 berdi: %q", n, got)
		}
	}
}

func isValidUTF8(s string) bool {
	return strings.ToValidUTF8(s, "�") == s
}
