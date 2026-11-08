package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"chustapp/internal/images"
)

// Akkaunt rasmi (avatar) — markazlashgan (Plane ONDEXECOSY-205).
//
// Rasm OnDex akkauntiga tegishli: R2'da (ImageStore) saqlanadi va barcha
// ilovalarga OIDC userinfo `picture` orqali beriladi. Ilovalar (ShoxPos)
// o'z media xizmatini (Cloudinary) yuritmaydi — rasmni foydalanuvchining
// O'Z markaziy tokeni bilan shu yerga yuboradi (BFF).
//
//	PUT    /v2/me/avatar   multipart `file` (jpg/png/webp, <= 2 MB) -> {picture}
//	DELETE /v2/me/avatar   -> 204
//
// Rasm serverda qayta kodlanadi (kvadrat, WebP): EXIF/joylashuv ma'lumoti
// va rasm ichiga yashirilgan yuk o'chadi; asl fayl saqlanmaydi.
const maxAvatarSize = 2 << 20

// AvatarStore — `storage.PgAvatarStore` / `MemoryAvatarStore` (httpapi
// storage paketini import qilmaydi). Bo'sh satr — rasm yo'q.
type AvatarStore interface {
	Get(ctx context.Context, userID string) (string, error)
	Set(ctx context.Context, userID, url string) error
}

func (s *Server) registerAvatarRoutes(mux *http.ServeMux) {
	ready := func(w http.ResponseWriter) bool {
		if s.Avatars == nil || s.ImageStore == nil {
			httpError(w, http.StatusServiceUnavailable, errors.New("profil rasmi xizmati yoqilmagan"))
			return false
		}
		return true
	}

	mux.HandleFunc("PUT /v2/me/avatar", s.authV2(nil, func(w http.ResponseWriter, r *http.Request) {
		if !ready(w) {
			return
		}
		c := claimsV2From(r)
		if s.UploadLimiter != nil && !s.UploadLimiter.Allow("avatar:"+c.Subject) {
			httpError(w, http.StatusTooManyRequests, errors.New("juda ko'p yuklash — biroz kuting"))
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxAvatarSize+64<<10)
		if err := r.ParseMultipartForm(maxAvatarSize); err != nil {
			httpError(w, http.StatusBadRequest, errors.New("rasm juda katta (maks 2 MB) yoki formati noto'g'ri"))
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			httpError(w, http.StatusBadRequest, errors.New("fayl topilmadi (multipart 'file' maydoni kerak)"))
			return
		}
		defer file.Close()
		switch strings.ToLower(filepath.Ext(header.Filename)) {
		case ".jpg", ".jpeg", ".png", ".webp":
		default:
			httpError(w, http.StatusBadRequest, errors.New("faqat jpg, png, webp rasm qabul qilinadi"))
			return
		}
		webp, err := images.ProcessLogoImage(file)
		if err != nil {
			httpError(w, http.StatusBadRequest, errors.New("fayl haqiqiy rasm emas yoki buzilgan"))
			return
		}
		url, err := uploadImmutable(r.Context(), s.ImageStore, "avatars/"+NewID()+".webp", webp, "image/webp")
		if err != nil {
			slog.Error("avatar saqlanmadi", "err", err)
			httpError(w, http.StatusInternalServerError, errors.New("rasm saqlashda xato"))
			return
		}
		if err := s.Avatars.Set(r.Context(), c.Subject, url); err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		noStore(w)
		writeJSON(w, http.StatusOK, map[string]string{"picture": url})
	}))

	mux.HandleFunc("DELETE /v2/me/avatar", s.authV2(nil, func(w http.ResponseWriter, r *http.Request) {
		if !ready(w) {
			return
		}
		if err := s.Avatars.Set(r.Context(), claimsV2From(r).Subject, ""); err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}
