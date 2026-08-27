package httpapi

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"chustapp/internal/books"
	"chustapp/internal/images"
	"chustapp/internal/users"
)

func (s *Server) registerUploadRoutes(mux *http.ServeMux) {
	// maxUploadSize / allowedImageExt — yuklash chegaralari.
	const maxUploadSize = 5 << 20 // 5MB
	allowedImageExt := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}

	mux.HandleFunc("POST /uploads", s.auth([]users.Role{users.RoleRestaurant, users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
			if err := r.ParseMultipartForm(maxUploadSize); err != nil {
				httpError(w, http.StatusBadRequest, errors.New("fayl juda katta (maks 5MB) yoki formati noto'g'ri"))
				return
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				httpError(w, http.StatusBadRequest, errors.New("fayl topilmadi (multipart 'file' maydoni kerak)"))
				return
			}
			defer file.Close()

			ext := strings.ToLower(filepath.Ext(header.Filename))
			if !allowedImageExt[ext] {
				httpError(w, http.StatusBadRequest, errors.New("faqat jpg, png, webp rasm formatlari qabul qilinadi"))
				return
			}

			// ?type=cover — restoran banneri (keng, to'ldirib kesiladi).
			// ?type=logo — restoran logosi (kvadrat, TO'LDIRIB kesiladi,
			// oq joysiz — kichik doirada "bo'sh joy" bo'lib ko'rinmasligi
			// uchun mahsulot rasmidan ATAYLAB farqli).
			// Standart — mahsulot rasmi (kvadrat, oq joy bilan).
			imgType := r.URL.Query().Get("type")
			var webpBytes []byte
			switch imgType {
			case "cover":
				webpBytes, err = images.ProcessCoverImage(file)
			case "logo":
				webpBytes, err = images.ProcessLogoImage(file)
			default:
				webpBytes, err = images.ProcessProductImage(file)
			}
			if err != nil {
				httpError(w, http.StatusBadRequest, errors.New("fayl haqiqiy rasm emas yoki buzilgan"))
				return
			}

			prefix := "products"
			switch imgType {
			case "cover":
				prefix = "covers"
			case "logo":
				prefix = "logos"
			}
			key := prefix + "/" + NewID() + ".webp"
			url, err := s.ImageStore.Upload(r.Context(), key, bytes.NewReader(webpBytes), int64(len(webpBytes)), "image/webp")
			if err != nil {
				slog.Error("rasm yuklashda xato", "err", err)
				httpError(w, http.StatusInternalServerError, errors.New("rasm saqlashda xato"))
				return
			}
			writeJSON(w, http.StatusCreated, map[string]string{"url": url})
		}))

	// POST /uploads/book-cover — kitob muqovasi.
	//
	// Alohida marshrut, `?type=` emas: kitob muqovasi restoran
	// bannerining teskarisi (tik, 2:3) va boshqa papkaga tushadi.
	mux.HandleFunc("POST /uploads/book-cover", s.auth([]users.Role{users.RoleRestaurant, users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
			if err := r.ParseMultipartForm(maxUploadSize); err != nil {
				httpError(w, http.StatusBadRequest, errors.New("fayl juda katta (maks 5MB) yoki formati noto'g'ri"))
				return
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				httpError(w, http.StatusBadRequest, errors.New("fayl topilmadi (multipart 'file' maydoni kerak)"))
				return
			}
			defer file.Close()

			if !allowedImageExt[strings.ToLower(filepath.Ext(header.Filename))] {
				httpError(w, http.StatusBadRequest, errors.New("faqat jpg, png, webp rasm formatlari qabul qilinadi"))
				return
			}
			webpBytes, err := images.ProcessBookCoverImage(file)
			if err != nil {
				httpError(w, http.StatusBadRequest, errors.New("fayl haqiqiy rasm emas yoki buzilgan"))
				return
			}
			// Kalit TASODIFIY: foydalanuvchi bergan fayl nomi kalitga
			// umuman qo'shilmaydi. Aks holda `../` yoki bir xil nom
			// bilan boshqa kitobning muqovasini bosib ketish mumkin
			// bo'lardi.
			key := "book-covers/" + NewID() + ".webp"
			url, err := s.ImageStore.Upload(r.Context(), key,
				bytes.NewReader(webpBytes), int64(len(webpBytes)), "image/webp")
			if err != nil {
				slog.Error("muqova yuklashda xato", "err", err)
				httpError(w, http.StatusInternalServerError, errors.New("rasm saqlashda xato"))
				return
			}
			writeJSON(w, http.StatusCreated, map[string]string{"url": url})
		}))

	// POST /uploads/book-pdf — kitob PDF fayli.
	//
	// ┌─ NIMA QAYTADI VA NEGA ─────────────────────────────────────────────┐
	// Javobda IKKI narsa bor: R2 dagi manzil va PDF dan AJRATILGAN matn.
	//
	// Sabab: maketdagi o'quvchi PDF ni ocha olmaydi (Godot'da PDF
	// tahlilchisi yo'q), shuning uchun matn serverda, yuklash paytida
	// bir marta ajratiladi. Admin panel ikkalasini ham kitob yozuviga
	// qo'shib yuboradi.
	// └────────────────────────────────────────────────────────────────────┘
	mux.HandleFunc("POST /uploads/book-pdf", s.auth([]users.Role{users.RoleRestaurant, users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, books.MaxPDFBytes)
			// Diskka emas, XOTIRAGA: chegara allaqachon 25MB va
			// `ParseMultipartForm` ga shu qiymat berilsa vaqtinchalik
			// fayl umuman yaratilmaydi (tozalanmay qolish xavfi yo'q).
			if err := r.ParseMultipartForm(books.MaxPDFBytes); err != nil {
				httpError(w, http.StatusBadRequest, errors.New("fayl juda katta (maks 25MB) yoki formati noto'g'ri"))
				return
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				httpError(w, http.StatusBadRequest, errors.New("fayl topilmadi (multipart 'file' maydoni kerak)"))
				return
			}
			defer file.Close()

			if strings.ToLower(filepath.Ext(header.Filename)) != ".pdf" {
				httpError(w, http.StatusBadRequest, errors.New("faqat PDF qabul qilinadi"))
				return
			}

			// Faylni to'liq o'qiymiz: PDF tahlili `io.ReaderAt` talab
			// qiladi (format oxiridagi jadvaldan boshlab o'qiladi),
			// ya'ni oqim bo'ylab bir marta o'tish yetmaydi.
			raw, err := io.ReadAll(file)
			if err != nil {
				httpError(w, http.StatusBadRequest, errors.New("fayl o'qilmadi"))
				return
			}

			// ┌─ KENGAYTMAGA ISHONILMAYDI ────────────────────────────────┐
			// `.pdf` deb nomlangan fayl ichida HTML, SVG yoki skript
			// bo'lishi mumkin. R2 ommaviy domenda turadi va brauzer
			// ba'zi holatlarda mazmunni o'zi aniqlaydi — ya'ni bu
			// saqlangan XSS ga yo'l ochardi. Imzo (`%PDF-`) HAQIQIY
			// formatni tekshiradi.
			// └───────────────────────────────────────────────────────────┘
			if len(raw) < 5 || !bytes.HasPrefix(raw, []byte("%PDF-")) {
				httpError(w, http.StatusBadRequest, errors.New("fayl haqiqiy PDF emas"))
				return
			}

			text, pageCount, extractErr := books.ExtractText(bytes.NewReader(raw), int64(len(raw)))
			if extractErr != nil && !errors.Is(extractErr, books.ErrNoText) {
				httpError(w, http.StatusBadRequest, extractErr)
				return
			}

			key := "book-pdfs/" + NewID() + ".pdf"
			// Content-Type MAJBURAN qo'yiladi — klient yuborgan qiymat
			// ishlatilmaydi. Fayl imzo bo'yicha PDF ekani tekshirilgan,
			// shuning uchun bu yolg'on emas va brauzerning mazmunni
			// o'zicha aniqlashiga yo'l qo'ymaydi.
			url, err := s.ImageStore.Upload(r.Context(), key,
				bytes.NewReader(raw), int64(len(raw)), "application/pdf")
			if err != nil {
				slog.Error("PDF yuklashda xato", "err", err)
				httpError(w, http.StatusInternalServerError, errors.New("PDF saqlashda xato"))
				return
			}

			resp := map[string]any{
				"url":   url,
				"text":  text,
				"pages": pageCount,
			}
			// Skanerlangan kitob — xato emas, lekin admin buni BILISHI
			// kerak: maketda kitob ochilganda sahifalar bo'sh bo'ladi.
			if errors.Is(extractErr, books.ErrNoText) {
				resp["warning"] = books.ErrNoText.Error()
			}
			writeJSON(w, http.StatusCreated, resp)
		}))

	// GET /uploads/* — faqat lokal disk rejimida kerak (R2'da rasmlar
	// to'g'ridan-to'g'ri R2_PUBLIC_URL orqali, bu serverni chetlab o'tib
	// ko'rsatiladi). Yozish (yuqoridagi POST) doim himoyalangan.
	if local, ok := s.ImageStore.(*images.LocalStore); ok {
		mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(local.Dir))))
	}

	// ---------- Kuryer amallari ----------

	// POST /couriers/register — mijoz kuryer bo'lishga ariza beradi.
	// Kuryer yaratiladi, lekin approved=false: superadmin tasdiqlamaguncha
	// online bo'la olmaydi va taklif olmaydi. Yangi token qaytariladi
	// (eski tokenda rol hali "customer" edi).
}
