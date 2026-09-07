package notify

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"strings"
	"testing"
)

// Eskiz multipart yig'uvchisining testlari (bug.md 13-band).
//
// ┌─ NEGA BU MUHIM ────────────────────────────────────────────────────┐
// Yig'uvchi qiymatlarni buferga XOM holda yozardi. Qiymat ichida
// `\r\n----ondex-eskiz-boundary\r\n` bo'lsa, so'rovga qo'shimcha
// maydon kiritish mumkin edi.
//
// Zarar hozir yo'q (matn serverda yasaladi), lekin bu LATENT: restoran
// nomi yoki foydalanuvchi kiritgan matn SMS'ga qo'shilgan kunda teshik
// darhol ochilardi. Shuning uchun tekshiruv YUBORUVCHI QATLAMDA —
// chaqiruvchi uni unutsa ham himoya ishlaydi (`email.go: guardHeader`
// bilan bir xil naqsh).
// └────────────────────────────────────────────────────────────────────┘

// parseParts — yasalgan tanani HAQIQIY multipart parser bilan o'qiydi.
//
// ┌─ NEGA SATRDAN QIDIRISH EMAS ───────────────────────────────────────┐
// Birinchi urinishda test tanadagi chegara SATRINI sanagan edi va u
// noto'g'ri o'lchov bo'lib chiqdi: inyeksiya zararsizlantirilgandan
// keyin ham matn ichida `----ondex-eskiz-boundary` HARFLARI qoladi
// (u endi shunchaki matn, chegara emas). Muhimi — parser nechta
// MAYDON ko'rishi. Shuning uchun test aynan shuni o'lchaydi.
// └────────────────────────────────────────────────────────────────────┘
func parseParts(t *testing.T, contentType string, body []byte) map[string]string {
	t.Helper()
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("Content-Type o'qilmadi: %v", err)
	}
	mr := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	out := map[string]string{}
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatalf("multipart o'qishda xato: %v", err)
		}
		data, err := io.ReadAll(p)
		if err != nil {
			t.Fatalf("qism o'qilmadi: %v", err)
		}
		out[p.FormName()] = string(data)
	}
}

// ASOSIY REGRESSIYA: chegara satrini o'z ichiga olgan qiymat qo'shimcha
// MAYDON yarata olmasligi kerak.
func TestMultipartFormRejectsBoundaryInjection(t *testing.T) {
	var buf bytes.Buffer
	// Hujumchining qiymati: o'z maydonini "yopib", yangisini ochmoqchi.
	evil := "OnDex kodi: 1234\r\n" +
		"------ondex-eskiz-boundary\r\n" +
		"Content-Disposition: form-data; name=\"from\"\r\n\r\n" +
		"SOXTA\r\n"
	ct := multipartForm(&buf, map[string]string{"message": evil})

	parts := parseParts(t, ct, buf.Bytes())
	if len(parts) != 1 {
		t.Fatalf("inyeksiya o'tdi: %d ta maydon (1 kutilgan): %v", len(parts), parts)
	}
	if _, ok := parts["from"]; ok {
		t.Fatalf("soxta `from` maydoni yaratildi: %v", parts)
	}
	// Qiymat bitta qatorga aylanishi kerak — CR/LF olib tashlangan.
	if strings.ContainsAny(parts["message"], "\r\n") {
		t.Fatalf("qiymatda CR/LF qoldi: %q", parts["message"])
	}
}

// Maydon NOMI ham tozalanishi kerak — u ham chaqiruvchidan keladi.
func TestMultipartFormSanitizesFieldName(t *testing.T) {
	var buf bytes.Buffer
	ct := multipartForm(&buf, map[string]string{"mes\r\nsage": "salom"})

	parts := parseParts(t, ct, buf.Bytes())
	if len(parts) != 1 {
		t.Fatalf("nom orqali qo'shimcha maydon kiritildi: %v", parts)
	}
	if _, ok := parts["message"]; !ok {
		t.Fatalf("nom kutilganidek tozalanmadi: %v", parts)
	}
}

// Oddiy qiymat o'zgarishsiz o'tishi kerak — tuzatish ishlayotgan
// SMS'ni buzmasin.
func TestMultipartFormKeepsNormalValue(t *testing.T) {
	var buf bytes.Buffer
	ct := multipartForm(&buf, map[string]string{
		"message":      "OnDex kodi: 1234",
		"mobile_phone": "998900000000",
	})

	parts := parseParts(t, ct, buf.Bytes())
	if parts["message"] != "OnDex kodi: 1234" {
		t.Fatalf("matn o'zgardi: %q", parts["message"])
	}
	if parts["mobile_phone"] != "998900000000" {
		t.Fatalf("raqam o'zgardi: %q", parts["mobile_phone"])
	}
	if len(parts) != 2 {
		t.Fatalf("%d ta maydon (2 kutilgan): %v", len(parts), parts)
	}
}

func TestStripCRLF(t *testing.T) {
	cases := map[string]string{
		"oddiy":           "oddiy",
		"ikki\r\nqator":   "ikkiqator",
		"faqat\nLF":       "faqatLF",
		"faqat\rCR":       "faqatCR",
		"\r\n\r\nboshida": "boshida",
		"oxirida\r\n\r\n": "oxirida",
		"ko'p\n\n\nqator": "ko'pqator",
	}
	for in, want := range cases {
		if got := stripCRLF(in); got != want {
			t.Errorf("stripCRLF(%q) = %q, kutilgan %q", in, got, want)
		}
	}
}
