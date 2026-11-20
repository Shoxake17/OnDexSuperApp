package main

import (
	"context"
	"net/http"
	"strings"
)

// publicCORSWorks — ommaviy media domeni brauzerga 3D model (GLB) berishga
// ruxsat beradimi: haqiqiy CORS preflight so'rovi yuboriladi.
//
// R2 tokeni ataylab eng kam huquqli (bucket sozlamasini o'zgartira olmaydi),
// shuning uchun qoida Cloudflare panelida qo'lda turadi va server uni
// API orqali o'qiy olmaydi. Natijani CDN'ning o'zi aytadi.
func publicCORSWorks(ctx context.Context, publicURL string) bool {
	base := strings.TrimRight(strings.TrimSpace(publicURL), "/")
	if base == "" {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodOptions, base+"/__cors_probe.glb", nil) //nolint:gosec // URL operator sozlamasidan (R2_PUBLIC_URL), foydalanuvchidan emas
	if err != nil {
		return false
	}
	req.Header.Set("Origin", "https://ondex.uz")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	res, err := http.DefaultClient.Do(req) //nolint:gosec // yuqoridagi izoh: manzil faqat env'dan
	if err != nil {
		return false
	}
	_ = res.Body.Close()
	allow := res.Header.Get("Access-Control-Allow-Origin")
	return allow == "*" || allow == "https://ondex.uz"
}
