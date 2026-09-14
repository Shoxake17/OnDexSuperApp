package books

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// PageImage — kitobning bitta sahifasi rasm ko'rinishida.
type PageImage struct {
	PageNr int
	// Ext — pdfcpu aniqlagan format ("jpg", "png", "tif"...).
	Ext  string
	Data []byte
}

const (
	// MaxPageImages — nechta sahifa ajratiladi.
	//
	// Har sahifa alohida R2 yuklashi: 400 sahifa allaqachon 400 ta
	// tarmoq amali. Bundan kattasi bitta HTTP so'rov ichida
	// bajarilmasligi kerak - u fon vazifasiga aylanishi lozim.
	MaxPageImages = 400

	// MaxPageImagesBytes — ajratilgan rasmlarning umumiy hajmi.
	// PDF chegarasi 25 MB, lekin ichidagi rasmlar SIQILMAGAN holda
	// undan kattaroq bo'lishi mumkin.
	MaxPageImagesBytes = 200 << 20

	// minPageImagePixels — bundan kichigi sahifa emas.
	//
	// Skanerlangan sahifada bitta katta rasm bo'ladi. Kichkinalari -
	// logotip, chiziq, bezak. Ular sahifa sifatida qo'shilsa kitob
	// o'rtasida tushunarsiz mayda rasmlar paydo bo'lardi.
	minPageImagePixels = 200 * 200
)

// ErrNoPageImages — PDF ichida sahifa rasmlari topilmadi.
var ErrNoPageImages = errors.New("PDF ichida sahifa rasmlari topilmadi")

// ExtractPageImages — skanerlangan PDF dan sahifa rasmlarini ajratadi.
//
// ┌─ NEGA RASTERLASH EMAS, AJRATISH ───────────────────────────────────┐
// PDF ni rasmga "chizish" (rasterlash) uchun to'liq render dvigateli
// kerak (pdfium/mupdf) - u CGO talab qiladi va Windows'da qurilmaydi.
//
// Skanerlangan kitobda bunga hojat yo'q: har sahifa ALLAQACHON rasm
// bo'lib PDF ichida yotibdi. Uni chizish emas, shunchaki OLIB CHIQISH
// kerak - bu esa sof Go bilan bajariladi.
//
// Shu sabab bu funksiya faqat skanerlangan kitob uchun ishlaydi va
// bu yetarli: matnli kitobda matn ajratiladi.
// └────────────────────────────────────────────────────────────────────┘
func ExtractPageImages(rs io.ReadSeeker) (pages []PageImage, err error) {
	// PDF tahlilchilari buzilgan faylda panic qilishga moyil -
	// `ExtractText` dagi bilan bir xil sabab.
	defer func() {
		if rec := recover(); rec != nil {
			pages = nil
			err = fmt.Errorf("PDF o'qib bo'lmadi (fayl buzilgan): %v", rec)
		}
	}()

	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	conf := model.NewDefaultConfiguration()
	// Faqat o'qiymiz - hech narsa yozilmaydi va tekshiruv qattiqroq
	// bo'lmasligi kerak: real kitoblarda kichik nomuvofiqliklar ko'p.
	conf.ValidationMode = model.ValidationRelaxed

	// Har sahifadan ENG KATTA rasm olinadi. Skanerlangan sahifada
	// odatda bitta rasm bo'ladi, lekin ba'zan ustiga mayda bezak
	// qo'shilgan bo'ladi - o'lcham bo'yicha tanlash shuni ajratadi.
	best := map[int]PageImage{}
	total := 0

	digest := func(img model.Image, _ bool, _ int) error {
		if img.PageNr <= 0 || img.PageNr > MaxPageImages {
			return nil
		}
		if img.Width*img.Height < minPageImagePixels {
			return nil
		}
		// Chegaradan ORTIQ o'qilmaydi: PDF ichidagi siqilgan oqim ochilganda
		// fayl hajmidan ko'p marta katta bo'lishi mumkin, tekshiruv esa
		// o'qishdan KEYIN bo'lsa xotira allaqachon sarflangan bo'lardi.
		data, rerr := io.ReadAll(io.LimitReader(img, int64(MaxPageImagesBytes-total)+1))
		if rerr != nil {
			// Bitta rasm o'qilmasa butun kitob yo'qotilmaydi.
			return nil //nolint:nilerr // ataylab: buzuq rasm o'tkazib yuboriladi, kitob davom etadi
		}
		total += len(data)
		if total > MaxPageImagesBytes {
			return errors.New("PDF ichidagi rasmlar juda katta")
		}
		prev, ok := best[img.PageNr]
		if ok && len(prev.Data) >= len(data) {
			return nil
		}
		best[img.PageNr] = PageImage{
			PageNr: img.PageNr,
			Ext:    img.FileType,
			Data:   data,
		}
		return nil
	}

	if err := api.ExtractImages(rs, nil, digest, conf); err != nil {
		return nil, fmt.Errorf("sahifa rasmlari ajratilmadi: %w", err)
	}
	if len(best) == 0 {
		return nil, ErrNoPageImages
	}

	pages = make([]PageImage, 0, len(best))
	for _, p := range best {
		pages = append(pages, p)
	}
	// Xarita tartibsiz - sahifalar RAQAM bo'yicha tartiblanishi shart,
	// aks holda kitob aralashib ketadi.
	sort.Slice(pages, func(i, j int) bool { return pages[i].PageNr < pages[j].PageNr })
	return pages, nil
}

// ReaderAt — `ExtractPageImages` uchun baytlardan `io.ReadSeeker`.
func ReaderFor(raw []byte) io.ReadSeeker { return bytes.NewReader(raw) }
