package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/ttlcache"
)

// ┌─ OCHIQ KATALOG UCHUN JARAYON ICHIDAGI L1 KESH (2026-11-13) ─────────────┐
// Prod yuklama testida CPU profili (pprof, 1500 rps): `cache.GetJSON` jami
// protsessorning ~18% ini oldi — restoranlar ro'yxati va menyu Redis'da
// keshlangan, lekin HAR SO'ROVDA Redis'ga tarmoq orqali borilib (~10%),
// JSON qayta dekodlanardi (~12%).
//
// Yechim — Redis OLDIDA qisqa (3 s) jarayon ichidagi kesh:
//   - menyu hamma uchun bir xil -> TAYYOR JSON baytlari saqlanadi (dekodlash
//     ham, qayta kodlash ham yo'q);
//   - restoranlar ro'yxatiga har javobda vaqtga bog'liq maydonlar (open_now,
//     reyting, maket imzosi) qo'shiladi -> DEKODLANGAN ro'yxat saqlanadi.
//
// ZIDDIYATSIZLIK QOIDALARI:
//   - Kesh tozalanadigan HAMMA joy `dropRestaurantsCache` / `dropMenuCache`
//     dan o'tadi: Redis va L1 BIRGA tozalanadi (bittasi unutilmaydi).
//   - L1 ro'yxati FAQAT O'QISH uchun: handlerlar uni o'zgartirmaydi
//     (`visibleRestaurants` yangi slice yaratadi, `sceneViewMany` qiymat
//     nusxasini oladi). `TestCatalogL1ListNotMutated` buni qulflaydi.
//   - Bir nechta API nusxasida: boshqa nusxa L1'i eng ko'p 3 s eskiradi
//     (Redis bekor qilish pub/sub — IN-2.2).
//   - Pul va mavjudlik checkout'da baribir qayta tekshiriladi.
// └─────────────────────────────────────────────────────────────────────────┘

const catalogL1TTL = 3 * time.Second

const allRestaurantsL1Key = "all"

type catalogL1 struct {
	rests *ttlcache.Cache[[]*catalog.Restaurant]
	menus *ttlcache.Cache[[]byte]
}

func newCatalogL1() catalogL1 {
	return catalogL1{
		rests: ttlcache.New[[]*catalog.Restaurant](catalogL1TTL, 4),
		menus: ttlcache.New[[]byte](catalogL1TTL, 4096),
	}
}

// dropRestaurantsCache — restoranlar ro'yxati o'zgardi (ochildi/yopildi,
// sozlama, admin tahriri, to'lov ulanishi): Redis + L1.
func (s *Server) dropRestaurantsCache(ctx context.Context) {
	s.Cache.Del(ctx, restaurantsCacheKey)
	if s.catL1.rests != nil {
		s.catL1.rests.Invalidate(allRestaurantsL1Key)
	}
}

// dropMenuCache — restoran menyusi o'zgardi (taom qo'shildi/tahrirlandi/
// o'chirildi, stop-list, 3D model holati): Redis + L1.
func (s *Server) dropMenuCache(ctx context.Context, restaurantID string) {
	s.Cache.Del(ctx, menuCacheKey(restaurantID))
	if s.catL1.menus != nil {
		s.catL1.menus.Invalidate(restaurantID)
	}
}

// restaurantsList — L1 -> Redis -> baza. Qaytgan ro'yxat FAQAT O'QISH uchun.
func (s *Server) restaurantsList(ctx context.Context) ([]*catalog.Restaurant, error) {
	load := func() ([]*catalog.Restaurant, error) {
		var list []*catalog.Restaurant
		if s.Cache.GetJSON(ctx, restaurantsCacheKey, &list) {
			return list, nil
		}
		list, err := s.CatalogRepo.ListRestaurants(ctx)
		if err != nil {
			return nil, err
		}
		annotatePaymentOptions(list)
		s.Cache.SetJSON(ctx, restaurantsCacheKey, list, 30*time.Second)
		return list, nil
	}
	if s.catL1.rests == nil {
		return load()
	}
	return s.catL1.rests.Get(allRestaurantsL1Key, load)
}

// menuJSON — menyuning TAYYOR JSON javobi (writeJSON bilan bir xil: oxirida
// yangi qator). L1 -> Redis -> baza.
func (s *Server) menuJSON(ctx context.Context, restaurantID string) ([]byte, error) {
	load := func() ([]byte, error) {
		var list []*catalog.Product
		if !s.Cache.GetJSON(ctx, menuCacheKey(restaurantID), &list) {
			fresh, err := s.CatalogRepo.ListProducts(ctx, restaurantID)
			if err != nil {
				return nil, err
			}
			// Ichki maydonlar (ulgurji narx) KESHGA YOZILISHDAN OLDIN olib
			// tashlanadi — ommaviy kesh ularni umuman saqlamaydi.
			list = publicProducts(fresh)
			s.Cache.SetJSON(ctx, menuCacheKey(restaurantID), list, 30*time.Second)
		}
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(list); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	if s.catL1.menus == nil {
		return load()
	}
	return s.catL1.menus.Get(restaurantID, load)
}

// publicProducts — OCHIQ javob uchun nusxa: restoranning ichki maydonlari
// (`catalog.Product.PublicView`) kesiladi. Ro'yxat nusxalanadi — repo
// qaytargan obyektlar o'zgartirilmaydi (ular kesh/xotira ombori bilan
// bo'lishilgan bo'lishi mumkin). (Avval registerCatalogRoutes ichidagi
// yopiq funksiya edi; L1 ham ishlatgani uchun paket darajasiga chiqarildi.)
func publicProducts(list []*catalog.Product) []*catalog.Product {
	out := make([]*catalog.Product, 0, len(list))
	for _, p := range list {
		if p == nil {
			continue
		}
		pub := p.PublicView()
		out = append(out, &pub)
	}
	return out
}

// writeRawJSON — oldindan kodlangan JSON (writeJSON bilan bir xil sarlavhalar).
func writeRawJSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
