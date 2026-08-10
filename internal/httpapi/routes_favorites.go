package httpapi

import (
	"fmt"
	"net/http"
	"slices"

	"chustapp/internal/catalog"
)

func (s *Server) registerFavoriteRoutes(mux *http.ServeMux) {
	// ---------- Istaklarim (favorites) ----------
	// Istalgan login qilgan foydalanuvchi o'zining yurak belgisi bilan
	// saqlagan mahsulotlarini boshqaradi. Har doim FAQAT o'z ro'yxati —
	// customer_id JWT claims'dan olinadi, so'rovdan emas (soxtalashtirib
	// bo'lmaydi).

	// POST /favorites/{productId} — mahsulot HAQIQATDA katalogda mavjudligi
	// tekshiriladi (o'chirilgan/soxta ID bilan chalkash yozuv qolmasin).
	// Idempotent: allaqachon saqlangan bo'lsa ham xato qaytarmaydi.
	mux.HandleFunc("POST /favorites/{productId}", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			productID := r.PathValue("productId")
			products, err := s.CatalogRepo.GetProductsByIDs(r.Context(), []string{productID})
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			if len(products) == 0 {
				httpError(w, http.StatusNotFound, catalog.ErrNotFound)
				return
			}
			// Yuqori chegara — avval umuman yo'q edi: bitta akkaunt
			// sikl bilan cheksiz yozuv qo'shib, bazani shishirishi
			// mumkin edi. 500 ta — haqiqiy foydalanuvchi uchun mo'l.
			// Tekshiruv IDEMPOTENTLIKNI buzmasligi kerak, shuning
			// uchun allaqachon saqlangan mahsulot chegaradan qat'i
			// nazar qayta qo'shilaveradi (aks holda ro'yxat to'lganda
			// yurakni qayta bosish xato berardi).
			const maxFavorites = 500
			existing, err := s.FavoritesRepo.ListProductIDs(r.Context(), claimsFrom(r).Subject)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			if len(existing) >= maxFavorites && !slices.Contains(existing, productID) {
				httpError(w, http.StatusBadRequest,
					fmt.Errorf("istaklar ro'yxati to'lgan (maksimal %d)", maxFavorites))
				return
			}
			if err := s.FavoritesRepo.Add(r.Context(), claimsFrom(r).Subject, productID); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]bool{"favorited": true})
		}))

	// DELETE /favorites/{productId} — ro'yxatda bo'lmasa ham xato
	// qaytarmaydi (mijoz ikki marta bossa ham xavfsiz).
	mux.HandleFunc("DELETE /favorites/{productId}", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			productID := r.PathValue("productId")
			if err := s.FavoritesRepo.Remove(r.Context(), claimsFrom(r).Subject, productID); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]bool{"favorited": false})
		}))

	// GET /favorites/ids — YENGIL versiya: faqat ID'lar ro'yxati, hech
	// qanday katalog/restoran boyitishisiz (GET /favorites'dan farqli).
	// Mijoz ilovasi menyu/turkum ekranlarida yurak belgisining boshlang'ich
	// holatini bilish uchun FAQAT ID kerak — to'liq (nomi/narxi/rasmi/
	// restorani bilan) ro'yxatni so'rash har safar keraksiz katalog+
	// restoran DB so'rovlariga olib kelardi (ayniqsa mijoz ko'p restoran
	// menyusini ochib-yopib yursa).
	mux.HandleFunc("GET /favorites/ids", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			ids, err := s.FavoritesRepo.ListProductIDs(r.Context(), claimsFrom(r).Subject)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, ids)
		}))

	// GET /favorites — mijozning to'liq "Istaklarim" ro'yxati (mahsulot +
	// restoran ma'lumoti bilan, /products/search bilan bir xil
	// ProductSearchResult shaklida — mijoz ilovasi bir xil kartochka
	// vidjetini qayta ishlata oladi).
	mux.HandleFunc("GET /favorites", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			ids, err := s.FavoritesRepo.ListProductIDs(r.Context(), claimsFrom(r).Subject)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			if len(ids) == 0 {
				writeJSON(w, http.StatusOK, []any{})
				return
			}
			products, err := s.CatalogRepo.GetProductsByIDs(r.Context(), ids)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			byID := make(map[string]*catalog.Product, len(products))
			for _, p := range products {
				byID[p.ID] = p
			}
			restaurantCache := map[string]*catalog.Restaurant{}
			// ids tartibi (ListProductIDs — eng yangisi birinchi) saqlanadi;
			// GetProductsByIDs buni kafolatlamaydi.
			out := make([]*catalog.ProductSearchResult, 0, len(ids))
			for _, id := range ids {
				p, ok := byID[id]
				if !ok {
					continue // mahsulot keyinchalik restoran tomonidan o'chirilgan
				}
				rest, ok := restaurantCache[p.RestaurantID]
				if !ok {
					rest, _ = s.CatalogRepo.GetRestaurant(r.Context(), p.RestaurantID)
					restaurantCache[p.RestaurantID] = rest
				}
				res := &catalog.ProductSearchResult{Product: *p}
				if rest != nil {
					res.RestaurantName = rest.Name
					res.RestaurantLogoURL = rest.LogoURL
					res.RestaurantOpen = rest.Open
				}
				out = append(out, res)
			}
			writeJSON(w, http.StatusOK, out)
		}))
}
