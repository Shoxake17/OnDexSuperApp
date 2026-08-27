// Package books — kitob fayllari bilan ishlash (hozircha faqat PDF).
package books

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/ledongthuc/pdf"
)

const (
	// MaxPDFBytes — qabul qilinadigan PDF hajmi.
	//
	// Rasm chegarasi (5MB) kitob uchun kam: skanerlanmagan, matnli kitob
	// odatda 1-10 MB. 25 MB — haqiqiy kitoblar uchun keng zaxira, lekin
	// serverni bir so'rov bilan to'ldirib qo'yishga yetmaydi.
	MaxPDFBytes = 25 << 20

	// MaxPages — nechta sahifadan matn olinadi.
	//
	// ┌─ NEGA CHEGARA BOR ──────────────────────────────────────────────┐
	// PDF ichida sahifa soni e'lon qilinadi, lekin uni tekshirib
	// bo'lmaydi: 20 MB fayl "50 000 sahifa" deb da'vo qilishi mumkin.
	// Har sahifani ajratish protsessor vaqti — chegarasiz qoldirilsa
	// bitta yuklash serverni daqiqalab band qiladi.
	//
	// 2000 sahifa — "Urush va tinchlik" hajmidagi kitobdan ham ko'p.
	// └─────────────────────────────────────────────────────────────────┘
	MaxPages = 2000

	// MaxTextBytes — ajratilgan matn chegarasi.
	// `catalog.MaxBookTextBytes` bilan bir xil bo'lishi SHART: matn
	// oxir-oqibat o'sha yerga yoziladi.
	MaxTextBytes = 400 << 10
)

// ErrNoText — PDF o'qildi, lekin ichida matn yo'q.
//
// Odatiy sabab: kitob skanerlangan, ya'ni har sahifa RASM. Bunday
// faylda ajratadigan matn yo'q va bu XATO EMAS — foydalanuvchiga aynan
// shu sabab aytilishi kerak, "fayl buzilgan" degan chalg'ituvchi xabar
// emas.
var ErrNoText = errors.New("bu skanerlangan kitob — ichida matn yo'q, " +
	"sahifalari rasm. Kitob saqlanadi, lekin maketda ko'rinishi uchun " +
	"sahifalarini tayyorlash kerak: go run ./cmd/bookpages -book <id>")

// ExtractText — PDF dan o'qish uchun matn ajratadi.
//
// ┌─ NEGA panic TUTILADI ───────────────────────────────────────────────┐
// PDF — murakkab, ko'p qatlamli format va uni ajratuvchi kutubxonalar
// buzilgan faylda `index out of range` bilan PANIC qilishga moyil.
// Handler ichida tutilmagan panic butun so'rovni emas, jarayonni
// yiqitishi mumkin. Kirish ma'lumoti ISHONCHSIZ (foydalanuvchi
// yuklaydi), shuning uchun tiklash shu yerda, manbada turadi.
// └─────────────────────────────────────────────────────────────────────┘
func ExtractText(r io.ReaderAt, size int64) (text string, pages int, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			text = ""
			pages = 0
			err = fmt.Errorf("PDF o'qib bo'lmadi (fayl buzilgan): %v", rec)
		}
	}()

	doc, err := pdf.NewReader(r, size)
	if err != nil {
		return "", 0, fmt.Errorf("PDF ochilmadi: %w", err)
	}

	total := doc.NumPage()
	if total > MaxPages {
		total = MaxPages
	}

	var sb strings.Builder
	for i := 1; i <= total; i++ {
		p := doc.Page(i)
		if p.V.IsNull() {
			continue
		}
		s, perr := p.GetPlainText(nil)
		if perr != nil {
			// Bitta sahifa ochilmasa butun kitob yo'qotilmaydi —
			// qolganlari o'qilaveradi.
			continue
		}
		sb.WriteString(s)

		// Sahifa chegarasi: maketdagi o'quvchi matnni ekran o'lchamiga
		// qarab O'ZI qaytadan sahifalaydi, shuning uchun PDF sahifalari
		// oddiy xatboshi bilan ajratiladi.
		sb.WriteString("\n\n")

		// Chegaraga yetganda TO'XTAYMIZ — qolgan sahifalarni ajratish
		// baribir tashlab yuboriladigan ish bo'lardi.
		if sb.Len() >= MaxTextBytes {
			break
		}
	}

	out := strings.TrimSpace(normalize(sb.String()))
	if len(out) > MaxTextBytes {
		out = truncateAtRune(out, MaxTextBytes)
	}
	// ┌─ BO'SHLIK TEKSHIRUVI YETARLI EMAS ──────────────────────────────┐
	// Avval bu yerda faqat `out == ""` turardi va skanerlangan kitob
	// tekshiruvdan O'TIB KETARDI: har sahifasi rasm bo'lgan kitobda ham
	// matn qatlami bo'sh bo'lmaydi - sahifa RAQAMI bosilgan bo'ladi.
	//
	// Haqiqiy holat (2026-08-27): "Vavilonlik eng boy odam" dan 1169
	// belgi ajratildi va uning HAMMASI "1 2 3 4 5 ..." edi. Ogohlantirish
	// chiqmadi, kitob saqlandi va maketda faqat raqamlar ko'rindi.
	//
	// Yechim: HARF bormi deb qaraladi. Raqam, tinish belgisi va bo'shliq
	// o'qiladigan matn hosil qilmaydi.
	// └─────────────────────────────────────────────────────────────────┘
	if !hasProse(out) {
		return "", doc.NumPage(), ErrNoText
	}
	return out, doc.NumPage(), nil
}

// hasProse — matnda o'qiladigan gap bormi.
//
// Ikki shart: harflar YETARLICHA ko'p bo'lsin va matnning arzimas
// qismini emas, sezilarli qismini tashkil qilsin. Birinchisi mundarija
// yoki muqova yozuvidan iborat "matn" ni rad etadi, ikkinchisi esa
// sahifa raqamlari orasida qolib ketgan bir-ikki harfni.
func hasProse(s string) bool {
	letters := 0
	solid := 0 // bo'shliqdan boshqa hamma narsa
	for _, r := range s {
		if unicode.IsSpace(r) {
			continue
		}
		solid++
		if unicode.IsLetter(r) {
			letters++
		}
	}
	if letters < minProseLetters {
		return false
	}
	return letters*100 >= solid*minProsePercent
}

const (
	// Kitob uchun juda past chegara: hatto bitta xatboshi ham bundan
	// ko'p harf saqlaydi. Maqsad "kitob to'liqmi" ni emas, "umuman
	// matn bormi" ni aniqlash.
	minProseLetters = 60

	// Raqamli jadval yoki sahifa raqamlari ro'yxati bu chegaradan
	// o'tolmaydi; oddiy nasr esa 90% dan yuqori bo'ladi.
	minProsePercent = 40
)

// normalize — ajratilgan matnni o'qish uchun tozalaydi.
//
// PDF dan chiqqan matnda ko'pincha `\r`, takroriy bo'sh qatorlar,
// qo'sh probel va BOSHQARUV BELGILARI bo'ladi.
func normalize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	// ┌─ NEGA 0x19 APOSTROFGA ALMASHTIRILADI ───────────────────────────┐
	// Haqiqiy PDF larda sinaganda "O'zbekiston" `O\x19zbekiston` bo'lib
	// chiqdi: shrift o'z ichki kodlashida o'ng apostrofni 0x19 da
	// saqlaydi va ajratuvchi uni Unicode ga o'gira olmaydi.
	//
	// Bu TAXMIN, lekin xavfsiz taxmin: 0x19 (End of Medium) haqiqiy
	// matnda hech qachon uchramaydi, ya'ni yo'qotadigan narsa yo'q.
	// Oddiy o'chirish esa "Ozbekiston", "som" kabi buzilgan so'zlar
	// qoldirardi — o'zbek matnida apostrof harf o'rnida turadi.
	// └─────────────────────────────────────────────────────────────────┘
	s = strings.ReplaceAll(s, "\x19", "'")

	// Qolgan boshqaruv belgilari olib tashlanadi. Ular shriftdan
	// o'girilmagan qoldiqlar: ekranda "tofu" kvadrat bo'lib chiqadi
	// va JSON/Mongo uchun ham keraksiz shovqin.
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7F {
			return -1
		}
		return r
	}, s)

	// Yaroqsiz UTF-8 ketma-ketligi Mongo yozuvini ham, JSON javobni ham
	// buzadi — shuning uchun chiqishning to'g'riligi KAFOLATLANADI.
	s = strings.ToValidUTF8(s, "")

	// Uchtadan ortiq ketma-ket yangi qator — bitta xatboshiga.
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
}

// truncateAtRune — matnni bayt chegarasida kesadi, lekin UTF-8 belgini
// O'RTASIDAN emas.
//
// Kirill va o'zbek lotinidagi belgilar ko'p baytli: oddiy `s[:n]`
// oxirgi belgini ikkiga bo'lib, yaroqsiz UTF-8 hosil qiladi va JSON
// kodlashda `�` bo'lib chiqadi.
func truncateAtRune(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !isRuneStart(s[n]) {
		n--
	}
	return s[:n]
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }
