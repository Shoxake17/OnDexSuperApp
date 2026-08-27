package images

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"  // kiruvchi JPEG dekod + `ToJPEG` uchun kodlash
	_ "image/png" // kiruvchi PNG fayllarni dekodlash uchun
	"io"
	"mime"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // kiruvchi WebP fayllarni dekodlash uchun (faqat decode)

	webp "github.com/mayahiro/go-webp" // chiquvchi format — WebP kodlash (pure Go, cgo kerak emas)
)

func init() {
	// Ba'zi tizimlarda .webp uchun mime turi ro'yxatdan o'tmagan bo'ladi —
	// aniq belgilab qo'yamiz (lokal disk rejimida FileServer shunga tayanadi).
	mime.AddExtensionType(".webp", "image/webp")
}

const (
	// maxDimension — saqlanadigan kvadrat tuvalning tomoni shu o'lchamdan
	// oshmaydi (mobil ekranlar uchun yetarli, fayl hajmini nazoratda ushlaydi).
	maxDimension = 1080
	webpQuality  = 82

	// Cover (banner) rasmlar uchun: bular taom/logo emas — restoran
	// sahifasi tepasidagi keng surat, shuning uchun BUTUN kenglikni
	// to'ldirishi kerak (oq joy emas, to'liq kesib-to'ldirish).
	coverAspectRatio = 2.4 // kenglik : balandlik
	maxCoverWidth    = 1600

	// Kitob muqovasi — restoran bannerining teskarisi: TIK turadi.
	// 2:3 bosma kitob muqovasining odatiy nisbati; 3D maketdagi javon
	// panelida kitoblar aynan shu nisbatda yonma-yon teriladi.
	bookCoverAspectRatio = 2.0 / 3.0
	maxBookCoverWidth    = 800

	// Sahifa rasmi telefon ekranining YARMIDA ko'rsatiladi (yoyilmaning
	// bitta tomoni). 1240 px 150 dpi da A5 sahifa eniga to'g'ri keladi
	// va eng zich ekranda ham matn silliq chiqadi.
	maxPageWidth = 1240

	// maxDecodeWidth/maxDecodeHeight/maxDecodePixels — "decompression bomb"
	// himoyasi: kichik (masalan bir necha yuz KB) fayl ichida ULKAN piksel
	// o'lchamli rasm (masalan 20000x20000, bir xil rangli fon — juda yaxshi
	// siqiladi) yashiringan bo'lishi mumkin. image.Decode() bunday faylni
	// hech qanday chegarasiz TO'LIQ xotiraga ochadi (Go standart
	// kutubxonasi o'zi piksel-son chegarasi qo'ymaydi) — bu bitta HTTP
	// so'rov bilan serverni bir necha GB xotira sarflashga majburlaydi
	// (auth talab qilingan bo'lsa ham, o'g'irlangan/haqiqiy restoran-admin
	// token bilan amalga oshiriladigan DoS). maxDimension=1080 chegarasi
	// FOYDASIZ himoya beradi, chunki u dekodlashdan KEYIN, keyingi
	// kichraytirish bosqichida qo'llanadi — zarar (xotira sarfi) aynan
	// dekodlashning O'ZIDA yetkaziladi. Yechim: to'liq dekodlashdan OLDIN
	// image.DecodeConfig() bilan (faqat sarlavhani o'qiydigan, arzon amal)
	// o'lchamni tekshiramiz (decodeImageSafely()ga qarang). 8000x8000 va
	// 40 mln piksel — haqiqiy telefon/DSLR fotosuratlari uchun katta
	// zaxira bilan yetarli (odatiy telefon kamerasi 12-50MP), lekin
	// suiiste'mol uchun xotira sarfini qat'iy chegaralaydi (40MP RGBA ~
	// 160MB — nazoratda ushlanadigan yuqori chegara).
	maxDecodeWidth  = 8000
	maxDecodeHeight = 8000
	maxDecodePixels = 40_000_000
)

// decodeImageSafely — rasmni to'liq dekodlashdan OLDIN sarlavhasini
// (image.DecodeConfig, arzon — piksel ma'lumotini o'qimaydi) tekshirib,
// o'lchami maxDecode* chegaralaridan oshsa, DARHOL (piksellarni xotiraga
// yuklamasdan) rad etadi — "decompression bomb" DoS himoyasi.
//
// io.TeeReader orqali DecodeConfig o'qigan baytlar buferga saqlanadi,
// so'ng qolgan qism ham o'sha buferga qo'shib o'qiladi — natijada to'liq
// fayl tarkibi (bayt tartibi buzilmasdan) bufer ichida tiklanadi va odatiy
// image.Decode() shundan davom etadi. Reader faqat BIR MARTA o'qilishi
// mumkinligi (masalan multipart form fayli) uchun shart.
func decodeImageSafely(r io.Reader) (image.Image, error) {
	var buf bytes.Buffer
	cfg, _, err := image.DecodeConfig(io.TeeReader(r, &buf))
	if err != nil {
		return nil, fmt.Errorf("rasm sarlavhasini o'qib bo'lmadi: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 ||
		cfg.Width > maxDecodeWidth || cfg.Height > maxDecodeHeight ||
		cfg.Width*cfg.Height > maxDecodePixels {
		return nil, fmt.Errorf("rasm o'lchami juda katta (%dx%d) — maksimal %dx%d yoki %d mln piksel",
			cfg.Width, cfg.Height, maxDecodeWidth, maxDecodeHeight, maxDecodePixels/1_000_000)
	}
	if _, err := io.Copy(&buf, r); err != nil {
		return nil, fmt.Errorf("rasmni o'qib bo'lmadi: %w", err)
	}
	img, _, err := image.Decode(&buf)
	if err != nil {
		return nil, fmt.Errorf("rasmni o'qib bo'lmadi: %w", err)
	}
	return img, nil
}

// ProcessProductImage — istalgan formatdagi (jpg/png/webp) taom rasmini
// standart shaklga keltiradi. Kvadrat (1:1) tuvalga MOSLASHUVCHAN
// to'ldiriladi: rasm allaqachon kvadrat bo'lsa (yoki unga yaqin bo'lsa)
// deyarli hech narsa kesilmaydi; cho'zinchoq bo'lsa (masalan hot-dog)
// ortiqcha tomonlari markazdan kesilib, kartochka TO'LIQ (oq
// chekkasiz) to'ldiriladi — xuddi ProcessLogoImage/mijoz ilovasi
// kartochkalaridagi BoxFit.cover bilan bir xil mantiq. Avvalgi versiya
// (oq fonli "letterbox" panel) foydalanuvchi so'rovi bilan olib
// tashlandi — kartochkalarda oq cheka umuman qolmasligi kerak edi.
func ProcessProductImage(r io.Reader) ([]byte, error) {
	src, err := decodeImageSafely(r)
	if err != nil {
		return nil, err
	}

	square := cropToAspect(src, 1)
	if square.Bounds().Dx() > maxDimension {
		square = scaleToWidth(square, maxDimension)
	}

	var buf bytes.Buffer
	if err := webp.Encode(&buf, square, &webp.Options{
		Compression: webp.CompressionLossy,
		Quality:     webpQuality,
	}); err != nil {
		return nil, fmt.Errorf("webp kodlashda xato: %w", err)
	}
	return buf.Bytes(), nil
}

// ToJPEG — istalgan qo'llab-quvvatlanadigan rasmni (webp ham) JPEG ga
// o'giradi, o'lchamini o'zgartirmasdan.
//
// ┌─ NEGA KERAK ──────────────────────────────────────────────────────┐
// Taom rasmlari WebP formatida saqlanadi (kichik hajm, brauzer va
// mobil uchun ideal). Lekin tashqi 3D generatsiya xizmati rasm HAVOLASI
// orqali faqat JPEG/PNG qabul qiladi.
//
// Dekodlash `decodeImageSafely` orqali — ya'ni "zip bomb" ga qarshi
// o'lcham tekshiruvi va format aniqlash mantig'i TAKRORLANMAYDI, bitta
// joyda qoladi.
// └───────────────────────────────────────────────────────────────────┘
func ToJPEG(r io.Reader) ([]byte, error) {
	src, err := decodeImageSafely(r)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	// Sifat 90: bu rasm foydalanuvchiga KO'RSATILMAYDI, u faqat 3D
	// generatsiyasiga kiruvchi ma'lumot. Past sifat model aniqligini
	// pasaytiradi, shuning uchun webp'dagi 82 dan yuqori olinadi.
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 90}); err != nil {
		return nil, fmt.Errorf("jpeg kodlashda xato: %w", err)
	}
	return buf.Bytes(), nil
}

// ProcessLogoImage — restoran LOGOsini qayta ishlaydi. ProcessProductImage
// bilan bir xil (markazdan kvadratga kesib to'ldirish) — logo alohida
// funksiya sifatida saqlangan, chunki ikkalasi kelajakda turlicha
// talabga ega bo'lishi mumkin (masalan doiraviy kesish).
func ProcessLogoImage(r io.Reader) ([]byte, error) {
	src, err := decodeImageSafely(r)
	if err != nil {
		return nil, err
	}

	cropped := cropToAspect(src, 1)
	if cropped.Bounds().Dx() > maxDimension {
		cropped = scaleToWidth(cropped, maxDimension)
	}

	var buf bytes.Buffer
	if err := webp.Encode(&buf, cropped, &webp.Options{
		Compression: webp.CompressionLossy,
		Quality:     webpQuality,
	}); err != nil {
		return nil, fmt.Errorf("webp kodlashda xato: %w", err)
	}
	return buf.Bytes(), nil
}

// ProcessCoverImage — restoran banner (cover) rasmini qayta ishlaydi.
// Mahsulot/logo rasmlaridan farqli o'laroq, BUTUN maydonni to'ldirishi
// kerak (professional restoran ilovalaridagi hero-banner uslubi) —
// shuning uchun oq joy qo'shish o'rniga markazdan kesib-to'ldiriladi
// (coverAspectRatio nisbatiga). So'ng WebP formatida kodlanadi.
func ProcessCoverImage(r io.Reader) ([]byte, error) {
	src, err := decodeImageSafely(r)
	if err != nil {
		return nil, err
	}

	cropped := cropToAspect(src, coverAspectRatio)
	if cropped.Bounds().Dx() > maxCoverWidth {
		cropped = scaleToWidth(cropped, maxCoverWidth)
	}

	var buf bytes.Buffer
	if err := webp.Encode(&buf, cropped, &webp.Options{
		Compression: webp.CompressionLossy,
		Quality:     webpQuality,
	}); err != nil {
		return nil, fmt.Errorf("webp kodlashda xato: %w", err)
	}
	return buf.Bytes(), nil
}

// ProcessPageImage — kitobning bitta sahifasini qayta ishlaydi.
//
// Muqovadan farqi: KESILMAYDI. Sahifa — hujjat, uning cheti kesilsa
// matn yo'qoladi. Faqat eni chegaralanadi va WebP ga o'giriladi:
// skanerlangan sahifa PNG holida bir necha barobar kattaroq bo'ladi va
// 142 sahifali kitobda bu farq sezilarli.
func ProcessPageImage(r io.Reader) ([]byte, error) {
	src, err := decodeImageSafely(r)
	if err != nil {
		return nil, err
	}
	if src.Bounds().Dx() > maxPageWidth {
		src = scaleToWidth(src, maxPageWidth)
	}

	var buf bytes.Buffer
	if err := webp.Encode(&buf, src, &webp.Options{
		Compression: webp.CompressionLossy,
		Quality:     webpQuality,
	}); err != nil {
		return nil, fmt.Errorf("webp kodlashda xato: %w", err)
	}
	return buf.Bytes(), nil
}

// ProcessBookCoverImage — kitob muqovasini qayta ishlaydi.
//
// Restoran banneridan farqi faqat nisbatda: bu TIK (2:3) rasm. Kesish
// usuli bir xil — markazdan kesib to'ldiriladi, oq joy qo'shilmaydi:
// javondagi muqovalar orasida bo'sh oq chiziqlar paydo bo'lmasligi kerak.
func ProcessBookCoverImage(r io.Reader) ([]byte, error) {
	src, err := decodeImageSafely(r)
	if err != nil {
		return nil, err
	}

	cropped := cropToAspect(src, bookCoverAspectRatio)
	if cropped.Bounds().Dx() > maxBookCoverWidth {
		cropped = scaleToWidth(cropped, maxBookCoverWidth)
	}

	var buf bytes.Buffer
	if err := webp.Encode(&buf, cropped, &webp.Options{
		Compression: webp.CompressionLossy,
		Quality:     webpQuality,
	}); err != nil {
		return nil, fmt.Errorf("webp kodlashda xato: %w", err)
	}
	return buf.Bytes(), nil
}

// cropToAspect — rasmni markazdan berilgan kenglik:balandlik nisbatiga kesadi.
func cropToAspect(src image.Image, ratio float64) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()

	targetH := int(float64(w) / ratio)
	var cropW, cropH int
	if targetH <= h {
		cropW, cropH = w, targetH // manba nisbatan balandroq — tepa/pastdan kesamiz
	} else {
		cropW, cropH = int(float64(h)*ratio), h // manba nisbatan pastroq — yon tomonlardan kesamiz
	}

	x0 := b.Min.X + (w-cropW)/2
	y0 := b.Min.Y + (h-cropH)/2
	rect := image.Rect(x0, y0, x0+cropW, y0+cropH)

	dst := image.NewRGBA(image.Rect(0, 0, cropW, cropH))
	draw.Draw(dst, dst.Bounds(), src, rect.Min, draw.Src)
	return dst
}

func scaleToWidth(src image.Image, width int) image.Image {
	b := src.Bounds()
	height := int(float64(width) * float64(b.Dy()) / float64(b.Dx()))
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}
