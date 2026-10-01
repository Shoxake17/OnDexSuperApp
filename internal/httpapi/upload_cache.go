package httpapi

import (
	"bytes"
	"context"
	"io"

	"chustapp/internal/images"
)

// immutableCacheControl — bir yil, o'zgarmas. Faqat TASODIFIY kalitli (`NewID`)
// yuklashlarga qo'llanadi: bunday obyekt hech qachon qayta yozilmaydi, shuning
// uchun brauzer va CDN uni qayta so'ramasa ham eskirgan nusxa ko'rinmaydi.
const immutableCacheControl = "public, max-age=31536000, immutable"

// optionUploader — sarlavhalarni qo'llab-quvvatlaydigan ombor (R2). Lokal disk
// ombori (dev) buni bilmaydi va oddiy `Upload` ga tushadi.
type optionUploader interface {
	UploadWithOptions(ctx context.Context, key string, r io.Reader, size int64, opt images.UploadOptions) (string, error)
}

// uploadImmutable — tasodifiy kalitli obyektni `Cache-Control: immutable` bilan
// yuklaydi (ombor qo'llasa).
//
// MUHIM: bu funksiyani faqat noyob (qayta yozilmaydigan) kalitlar uchun
// ishlating. Kalit qayta ishlatilsa, `immutable` tufayli eski nusxa bir yilgacha
// ko'rinib qolishi mumkin. Umumiy `images.Store.Upload` ataylab o'zgarmagan:
// `cmd/r2upload` ixtiyoriy kalitga qayta yozishi mumkin.
func uploadImmutable(ctx context.Context, store images.Store, key string, body []byte, contentType string) (string, error) {
	if u, ok := store.(optionUploader); ok {
		return u.UploadWithOptions(ctx, key, bytes.NewReader(body), int64(len(body)),
			images.UploadOptions{ContentType: contentType, CacheControl: immutableCacheControl})
	}
	return store.Upload(ctx, key, bytes.NewReader(body), int64(len(body)), contentType)
}
