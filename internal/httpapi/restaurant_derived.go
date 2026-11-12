package httpapi

import (
	"context"
	"time"

	"chustapp/internal/orders"
	"chustapp/internal/ttlcache"
)

// ┌─ RESTORANNING HISOBLANADIGAN KO'RSATKICHLARI KESHI (2026-11-12) ────────┐
// `GET /restaurants` va `GET /restaurants/{id}` har bir restoran uchun HAR
// SO'ROVDA reytingni (`restaurant_ratings` AVG/COUNT) va yetkazish vaqti
// oralig'ini (oxirgi `orders.ETASampleSize` ta buyurtmani o'qib) qayta
// hisoblardi. Prod yuklama testida (scripts/loadtest-prod) aynan shu yo'l
// eng sekini edi: ~1000 rps da GET /restaurants p95 4.3 s, buyurtmalar
// ko'paygani sari og'irlashadi.
//
// ZIDDIYATSIZLIK QOIDALARI:
//   - Bu faqat KO'RSATISH uchun ma'lumot. Pul (narx, yetkazish haqi) va
//     ochiq/yopiq holat bu yerdan OLINMAYDI — checkout ularni o'zi qayta
//     hisoblaydi; `open_now` har javobda alohida hisoblanadi.
//   - TTL 60 s: yetkazish vaqti statistik oraliq (oxirgi N buyurtma) — bir
//     daqiqalik eskirish mijoz uchun sezilmaydi.
//   - Yangi baho qo'yilganda shu restoran kaliti DARHOL bekor qilinadi
//     (`routes_ratings.go`) — mijoz o'z bahosini darhol ko'radi.
//   - Bir vaqtda kelgan so'rovlar uchun bazaga BITTA so'rov (singleflight,
//     `ttlcache`); xato keshlanmaydi; bekor qilish epoch bilan poygasiz.
//   - Bir nechta API nusxasida: har nusxaning o'z keshi, eng ko'p 60 s farq
//     (Redis pub/sub bekor qilish — IN-2.2 bilan).
// └─────────────────────────────────────────────────────────────────────────┘

const restaurantDerivedTTL = 60 * time.Second

type restaurantDerived struct {
	rating      float64
	ratingCount int
	etaMin      int
	etaMax      int
}

func newRestaurantDerivedCache() *ttlcache.Cache[restaurantDerived] {
	return ttlcache.New[restaurantDerived](restaurantDerivedTTL, 8192)
}

// derivedFor — keshdan yoki (bo'lmasa) bazadan. Manba ulanmagan yoki xato
// bo'lsa nol qiymatlar ("ma'lumot yo'q") qaytadi va KESHLANMAYDI.
func (s *Server) derivedFor(reqCtx context.Context, restaurantID string) restaurantDerived {
	load := func() (restaurantDerived, error) {
		// Yuklash so'rov kontekstidan AJRATILGAN: uni kutayotgan boshqa
		// so'rovlar ham bor (singleflight) — birinchi mijoz sahifani yopsa,
		// qolganlari xato olmasin. Cheksiz kutish ham yo'q (5 s).
		ctx, cancel := context.WithTimeout(context.WithoutCancel(reqCtx), 5*time.Second)
		defer cancel()
		var d restaurantDerived
		if s.RatingsRepo != nil {
			avg, n, err := s.RatingsRepo.Summary(ctx, restaurantID)
			if err != nil {
				return restaurantDerived{}, err
			}
			d.rating, d.ratingCount = avg, n
		}
		if s.OrderRepo != nil {
			list, err := s.OrderRepo.ListByRestaurant(ctx, restaurantID, orders.ETASampleSize)
			if err != nil {
				return restaurantDerived{}, err
			}
			d.etaMin, d.etaMax = orders.ComputeETARange(list)
		}
		return d, nil
	}
	if s.restDerived == nil {
		d, _ := load()
		return d
	}
	d, err := s.restDerived.Get(restaurantID, load)
	if err != nil {
		return restaurantDerived{}
	}
	return d
}

// invalidateRestaurantDerived — restoran ko'rsatkichlari o'zgardi (yangi baho).
func (s *Server) invalidateRestaurantDerived(restaurantID string) {
	if s.restDerived != nil {
		s.restDerived.Invalidate(restaurantID)
	}
}
