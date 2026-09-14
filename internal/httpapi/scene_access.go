package httpapi

import (
	"net/http"
	"strings"

	"chustapp/internal/catalog"
	"chustapp/internal/scenes"
)

// ┌─ MAKET HAVOLASI: KIM KO'RADI VA QANDAY ────────────────────────────┐
// `GET /restaurants` va `GET /restaurants/{id}` OCHIQ endpointlar —
// mijoz ilovasining bosh sahifasi ular orqali to'ladi va kirmagan
// odam ham restoranlar ro'yxatini ko'rishi kerak.
//
// Lekin maket manzili ochiq javobda TURMASLIGI kerak (sabab:
// `catalog.Restaurant.PublicView`). Shuning uchun bu yerda MAJBURIY
// emas, IXTIYORIY autentifikatsiya qo'llanadi:
//
//	token yo'q / yaroqsiz  -> maket maydonlari olib tashlanadi
//	token yaroqli          -> imzolangan, muddatli havola qo'yiladi
//
// Endpoint hech qachon 401 qaytarmaydi — ochiqligi saqlanadi.
// └────────────────────────────────────────────────────────────────────┘

// hasValidSession — so'rovda ISHLAYDIGAN sessiya bormi.
//
// `auth` middleware'idan farqi: bu javob YOZMAYDI va yo'lni
// to'xtatmaydi, faqat "ha/yo'q" qaytaradi. Bekor qilingan sessiya
// (chiqish, akkaunt o'chirilishi) ham `false` beradi — aks holda
// chiqib ketgan qurilma maket havolasini olishda davom etardi.
func (s *Server) hasValidSession(r *http.Request) bool {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") || s.Tokens == nil {
		return false
	}
	claims, err := s.Tokens.Parse(strings.TrimPrefix(h, "Bearer "))
	if err != nil {
		return false
	}
	if s.Revoked != nil && claims.IssuedAt != nil &&
		s.Revoked.IsRevoked(claims.Subject, claims.IssuedAt.Time) {
		return false
	}
	return true
}

// sceneViewOne — bitta restoranni javobga tayyorlaydi.
func (s *Server) sceneViewOne(r *http.Request, rest *catalog.Restaurant) catalog.Restaurant {
	if rest == nil {
		return catalog.Restaurant{}
	}
	if !s.hasValidSession(r) {
		return withOpenNow(rest.PublicView())
	}
	return withOpenNow(s.signScene(r, *rest))
}

// sceneViewMany — ro'yxat uchun.
//
// ┌─ NEGA KESHDAN KEYIN ───────────────────────────────────────────────┐
// `GET /restaurants` javobi Redis'da 30 soniyaga keshlanadi. Agar
// imzolangan havola kesh ichiga tushsa, BITTA foydalanuvchi uchun
// yaratilgan havola keyingi 30 soniyada HAMMAGA berilardi — ochiq
// tarqatishdan farqi qolmasdi.
//
// Shuning uchun kesh XOM ma'lumotni saqlaydi, imzo esa har javobda
// alohida qo'yiladi.
// └────────────────────────────────────────────────────────────────────┘
func (s *Server) sceneViewMany(r *http.Request, list []*catalog.Restaurant) []catalog.Restaurant {
	out := make([]catalog.Restaurant, 0, len(list))
	signed := s.hasValidSession(r)
	for _, rest := range list {
		if rest == nil {
			continue
		}
		if !signed {
			out = append(out, withOpenNow(rest.PublicView()))
			continue
		}
		out = append(out, withOpenNow(s.signScene(r, *rest)))
	}
	return out
}

// signScene — saqlangan qiymatni muddatli havolaga almashtiradi.
//
// Imzolovchi sozlanmagan bo'lsa (`R2_SCENES_BUCKET` yo'q) maydon
// SHUNDAYLIGICHA qoladi: bu eski, ommaviy bucket bilan ishlaydigan
// holat va u hech bo'lmasa kirgan foydalanuvchi bilan cheklangan.
// Imzolash yiqilsa maydonlar TOZALANADI — buzuq havola berishdan
// ko'ra taklifni umuman ko'rsatmaslik to'g'ri.
func (s *Server) signScene(r *http.Request, rest catalog.Restaurant) catalog.Restaurant {
	if rest.Scene3DURL == "" {
		return rest
	}
	// Yopiq bucket berilgan-u, unga kirib bo'lmagan holat: taklifni
	// ko'rsatmaymiz (`ScenesUnavailable` izohiga qarang).
	if s.ScenesUnavailable {
		return rest.PublicView()
	}
	if s.Scenes == nil {
		return rest
	}
	key := scenes.ObjectKey(rest.Scene3DURL, s.MediaPublicBaseURL)
	if key == "" {
		return rest.PublicView()
	}
	url, err := s.Scenes.SignedURL(r.Context(), key)
	if err != nil {
		return rest.PublicView()
	}
	rest.Scene3DURL = url
	return rest
}
