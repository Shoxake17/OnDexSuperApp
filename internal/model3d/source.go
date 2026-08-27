package model3d

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"chustapp/internal/catalog"
	"chustapp/internal/images"
)

// maxSourceImageBytes — generatsiyaga kiruvchi rasm uchun chegara.
// Taom rasmlari ~100-300KB (WebP), 8MB — buzilgan/aldamchi javobga
// qarshi keng zaxira.
const maxSourceImageBytes = 8 << 20

// sourceImageURL — taom rasmidan tashqi xizmat QABUL QILADIGAN
// havolani tayyorlaydi.
//
// ┌─ NEGA QAYTA KODLASH KERAK ────────────────────────────────────────┐
// Taom rasmlari WebP formatida saqlanadi, Tripo esa rasm HAVOLASI
// orqali faqat JPEG/PNG qabul qiladi. Shuning uchun bir marta JPEG
// nusxa tayyorlanadi va `model-src/` prefiksi ostida saqlanadi.
//
// Nusxa QAYTA ISHLATILADI: bir xil mahsulot uchun kalit ham bir xil
// (`model-src/<productID>.jpg`), ya'ni qayta generatsiyada yangi fayl
// yaratilmaydi — omborda axlat to'planmaydi.
//
// Kodlashning O'ZI `images.ToJPEG` da: dekodlash, xavfsizlik tekshiruvi
// va format aniqlash mantig'i shu paketda takrorlanmaydi.
// └───────────────────────────────────────────────────────────────────┘
func (s *Service) sourceImageURL(ctx context.Context, p *catalog.Product) (string, error) {
	raw, err := s.fetchOwnImage(ctx, p.ImageURL)
	if err != nil {
		return "", err
	}

	jpegBytes, err := images.ToJPEG(bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("rasmni JPEG ga o'girishda xato: %w", err)
	}

	key := "model-src/" + p.ID + ".jpg"
	url, err := s.store.Upload(ctx, key, bytes.NewReader(jpegBytes), int64(len(jpegBytes)), "image/jpeg")
	if err != nil {
		return "", err
	}
	return url, nil
}

// fetchOwnImage — mahsulot rasmini o'z omborimizdan o'qiydi.
//
// Havola SSRF tekshiruvidan o'tadi: `ImageURL` bazadan keladi va
// bazaga admin/restoran yozadi — ya'ni ishonchli, lekin "ishonchli"
// degani "tekshirilmagan" degani emas. Bir xil qoida hamma tashqi
// so'rovga qo'llanadi (`safeHTTPSURL`).
func (s *Service) fetchOwnImage(ctx context.Context, imageURL string) ([]byte, error) {
	u, err := safeHTTPSURL(imageURL)
	if err != nil {
		return nil, err
	}
	req, err := newRequest(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("taom rasmi o'qilmadi: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSourceImageBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxSourceImageBytes {
		return nil, ErrTooLarge
	}
	return data, nil
}
