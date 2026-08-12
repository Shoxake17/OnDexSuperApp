package httpapi

import (
	"embed"
	"net/http"
)

//go:embed assets/map_picker.html
var mapPickerFS embed.FS

// registerMapPickerRoutes — Windows desktop paneli uchun xarita sahifasi.
//
// ┌─ NEGA BU SAHIFA UMUMAN BOR ────────────────────────────────────────┐
// `google_maps_flutter` faqat android/ios/web platformalarini e'lon
// qiladi; Windows uchun implementatsiya MAVJUD EMAS. Desktop panelda
// `GoogleMap` vidjeti "TargetPlatform.windows is not yet supported by
// the maps plugin" xatosini chizadi. Shuning uchun desktopda xarita
// WebView2 ichida, shu sahifa orqali ko'rsatiladi.
// └────────────────────────────────────────────────────────────────────┘
//
// ┌─ NEGA AUTH TALAB QILINMAYDI ───────────────────────────────────────┐
// Sahifada SIR YO'Q: Maps kaliti ichiga yozilmagan. Kalitni sahifa
// keyinroq `/config/maps` dan oladi va u endpoint auth talab qiladi.
// Ya'ni himoya joyida qoladi, faqat statik HTML ochiq bo'ladi.
//
// Agar bu sahifa auth talab qilsa, WebView navigatsiyasiga
// `Authorization` sarlavhasini qo'shish kerak bo'lardi yoki token
// URL'ga yozilardi — ikkinchisi tokenni brauzer tarixi va server
// loglariga tushirardi.
// └────────────────────────────────────────────────────────────────────┘
func (s *Server) registerMapPickerRoutes(mux *http.ServeMux) {
	page, err := mapPickerFS.ReadFile("assets/map_picker.html")
	if err != nil {
		// `go:embed` fayli build vaqtida tekshiriladi — bu yerga
		// tushish deyarli imkonsiz, lekin jimgina o'tkazib yuborish
		// "xarita ochilmayapti" degan tushunarsiz nosozlikka olib
		// kelardi.
		panic("map_picker.html embed qilinmadi: " + err.Error())
	}

	mux.HandleFunc("GET /map-picker", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Sahifa o'zgarmas (embed qilingan), lekin keshlanmasin —
		// yangilanish darhol yetib borsin.
		w.Header().Set("Cache-Control", "no-store")
		// Bu sahifa faqat WebView ichida ochilishi kerak. Boshqa
		// saytga freym qilib qo'yilishining ma'nosi yo'q.
		w.Header().Set("X-Frame-Options", "DENY")
		_, _ = w.Write(page)
	})
}
