package httpapi

import (
	"context"
	"net/http"
	"strings"

	"chustapp/internal/catalog"
	"chustapp/internal/orders"
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
		return s.withETA(r.Context(), s.withRating(r.Context(), s.withDeliveryFee(r.Context(), withOpenState(rest.PublicView()))))
	}
	return s.withETA(r.Context(), s.withRating(r.Context(), s.withDeliveryFee(r.Context(), withOpenState(s.signScene(r, *rest)))))
}

// withDeliveryFee — ochiq javobga yetkazish narxini qo'shadi
// (`staff.Service.DeliveryFee` — yagona haqiqiy manba, `catalog.go`
// dagi "YETKAZIB BERISH NARXI BU YERDA EMAS" izohiga qarang). Xodimlar
// xizmati ulanmagan yoki so'rov xato bo'lsa maydon shunchaki bo'sh
// qoladi — narx faqat QO'SHIMCHA ma'lumot, restoranlar ro'yxatining
// o'zini bunga bog'lab qo'yish noto'g'ri bo'lardi.
func (s *Server) withDeliveryFee(ctx context.Context, v catalog.Restaurant) catalog.Restaurant {
	if s.StaffSvc == nil {
		return v
	}
	fee, err := s.StaffSvc.DeliveryFee(ctx, v.ID)
	if err != nil {
		return v
	}
	v.DeliveryFeeTiyin = &fee
	return v
}

// withETA — `ETAMinMinutes`/`ETAMaxMinutes` ni HAQIQIY buyurtma
// tarixidan qayta hisoblaydi (`orders.ComputeETARange`) — admin/
// restoran qo'lda kiritgan eski qiymatni E'TIBORGA OLMAY. Reyting
// bilan bir xil naqsh (`withRating`): bu maydonlar endi
// tahrirlanadigan ro'yxatda yo'q, shu funksiya ularning YAGONA
// haqiqiy manbai.
func (s *Server) withETA(ctx context.Context, v catalog.Restaurant) catalog.Restaurant {
	v.ETAMinMinutes = 0
	v.ETAMaxMinutes = 0
	if s.OrderRepo == nil {
		return v
	}
	list, err := s.OrderRepo.ListByRestaurant(ctx, v.ID, orders.ETASampleSize)
	if err != nil {
		return v
	}
	lo, hi := orders.ComputeETARange(list)
	v.ETAMinMinutes = lo
	v.ETAMaxMinutes = hi
	return v
}

// withRating — `Rating`/`RatingCount` ni HAQIQIY buyurtma bahosidan
// qayta hisoblaydi (`internal/ratings`), admin/restoran qo'lda
// kiritgan eski qiymatni E'TIBORGA OLMAY. Bu maydonlar endi
// tahrirlanadigan ro'yxatda yo'q (`restaurantEditableFields`) — shu
// funksiya ularning YAGONA haqiqiy manbai.
//
// Xizmat ulanmagan yoki xato bo'lsa maydonlar 0 qoladi — "ma'lumot
// yo'q" holati, mijoz tomonida chip chizilmaydi (`catalog.go`dagi 0
// qoidasi), soxta eski qiymat ko'rsatilmaydi.
func (s *Server) withRating(ctx context.Context, v catalog.Restaurant) catalog.Restaurant {
	v.Rating = 0
	v.RatingCount = 0
	if s.RatingsRepo == nil {
		return v
	}
	avg, count, err := s.RatingsRepo.Summary(ctx, v.ID)
	if err != nil {
		return v
	}
	v.Rating = avg
	v.RatingCount = count
	return v
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
			out = append(out, s.withETA(r.Context(), s.withRating(r.Context(), s.withDeliveryFee(r.Context(), withOpenState(rest.PublicView())))))
			continue
		}
		out = append(out, s.withETA(r.Context(), s.withRating(r.Context(), s.withDeliveryFee(r.Context(), withOpenState(s.signScene(r, *rest))))))
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
	key := scenes.ObjectKey(rest.Scene3DURL, append([]string{s.MediaPublicBaseURL}, legacyMediaBaseURLs()...)...)
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
