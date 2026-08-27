package httpapi

import (
	"strings"
	"testing"

	"chustapp/internal/catalog"
)

func str(s string) *string { return &s }

// Kitobda o'qiladigan biror narsa BO'LISHI kerak. PDF qo'shilgandan
// keyin bu shart uchta manbadan biri bilan qanoatlantiriladi va
// ularning har biri alohida tekshiriladi: chegara faqat matn uchun
// ishlab qolsa, PDF li kitob saqlanmay qolardi.
func TestBookRequestNeedsSomethingToRead(t *testing.T) {
	cases := []struct {
		name string
		req  bookRequest
		ok   bool
	}{
		{"faqat nom", bookRequest{Title: str("Kitob")}, false},
		{"matn bilan", bookRequest{Title: str("Kitob"), Text: str("salom")}, true},
		{"PDF bilan", bookRequest{Title: str("Kitob"), PDFURL: str("https://x/y.pdf")}, true},
		{"sahifalar bilan", bookRequest{
			Title: str("Kitob"),
			Pages: &[]string{"https://x/1.webp"},
		}, true},
		// Skanerlangan kitob: PDF bor, matn ajratilmagan. Javonda
		// muqovasi bilan turishi kerak, saqlash rad etilmasin.
		{"PDF bor, matn bo'sh", bookRequest{
			Title:  str("Skaner"),
			PDFURL: str("https://x/y.pdf"),
			Text:   str(""),
		}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.req.applyTo(&catalog.Book{})
			if c.ok && err != nil {
				t.Errorf("kutilmagan xato: %v", err)
			}
			if !c.ok && err == nil {
				t.Error("xato kutilgandi, lekin qabul qilindi")
			}
		})
	}
}

func TestBookRequestValidation(t *testing.T) {
	long := strings.Repeat("a", booksMaxURL+1)
	cases := []struct {
		name string
		req  bookRequest
	}{
		{"nom bo'sh", bookRequest{Title: str("   "), Text: str("x")}},
		{"nom uzun", bookRequest{Title: str(strings.Repeat("a", booksMaxTitle+1)), Text: str("x")}},
		{"muallif uzun", bookRequest{
			Title:  str("K"),
			Author: str(strings.Repeat("a", booksMaxAuthor+1)),
			Text:   str("x"),
		}},
		{"muqova manzili uzun", bookRequest{Title: str("K"), CoverURL: str(long), Text: str("x")}},
		{"PDF manzili uzun", bookRequest{Title: str("K"), PDFURL: str(long), Text: str("x")}},
		{"matn juda katta", bookRequest{
			Title: str("K"),
			Text:  str(strings.Repeat("a", catalog.MaxBookTextBytes+1)),
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.req.applyTo(&catalog.Book{}); err == nil {
				t.Error("xato kutilgandi, lekin qabul qilindi")
			}
		})
	}
}

// PATCH da YUBORILMAGAN maydon o'zgarmasligi kerak. Bu bir marta
// buzilgan va mavjud matnni o'chirib yuborgan — shuning uchun PDF
// maydoni ham xuddi shu qoidaga bo'ysunishi tekshiriladi.
func TestBookRequestPartialUpdateKeepsFields(t *testing.T) {
	existing := &catalog.Book{
		Title:    "Eski nom",
		Author:   "Muallif",
		CoverURL: "https://x/cover.webp",
		PDFURL:   "https://x/book.pdf",
		Text:     "eski matn",
		Active:   true,
	}
	req := bookRequest{Title: str("Yangi nom")}
	if err := req.applyTo(existing); err != nil {
		t.Fatalf("kutilmagan xato: %v", err)
	}
	if existing.Title != "Yangi nom" {
		t.Errorf("nom yangilanmadi: %q", existing.Title)
	}
	for _, c := range []struct{ name, got, want string }{
		{"muallif", existing.Author, "Muallif"},
		{"muqova", existing.CoverURL, "https://x/cover.webp"},
		{"pdf", existing.PDFURL, "https://x/book.pdf"},
		{"matn", existing.Text, "eski matn"},
	} {
		if c.got != c.want {
			t.Errorf("%s o'chib ketdi: %q, kutilgani %q", c.name, c.got, c.want)
		}
	}
}
