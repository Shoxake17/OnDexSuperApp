package httpapi

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

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
