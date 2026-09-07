// Kesh kalitlari va handler'larga xos javob shakllari.
package httpapi

import "chustapp/internal/promotions"

// keyin tegishli kalit(lar) darhol tozalanadi — TTL faqat zaxira sifatida.
const restaurantsCacheKey = "cache:restaurants:list"

func menuCacheKey(restaurantID string) string { return "cache:menu:" + restaurantID }

// searchCacheKey — ochiq qidiruv natijasi uchun (bug.md 12-band).
//
// `q` NORMALLASHTIRILGAN shaklda beriladi (`NormalizeForSearch`), ya'ni
// faqat harf va raqamlardan iborat — Redis kalitiga xavfsiz.
func searchCacheKey(normalizedQuery string) string {
	return "cache:search:" + normalizedQuery
}

// maxSearchQueryLength — `GET /products/search` dagi `q` chegarasi.
// Juda uzun so'rov foydali natija bermaydi, lekin normalizatsiya va
// solishtirish ishini oshiradi.
const maxSearchQueryLength = 100

// savePromotionResponse — POST /restaurants/{id}/promotions javobi:
// saqlangan aksiyaning o'zi (Promotion o'zining json teglari bilan tekis
// chiqadi) + shu bilan bir vaqtda avtomatik moslashtirilgan (bir xil
// mahsulot/turkumga to'qnashgani uchun) eski aksiyalar nomlari — restoran
// paneli buni foydalanuvchiga ko'rsatishi uchun
// (resolvePromotionConflicts()ga qarang). StoppedPromotionNames — butunlay
// to'xtatilganlar; AdjustedPromotionNames — faol qolgan, lekin
// to'qnashgan mahsulot/turkum ro'yxatidan olib tashlanganlar.
type savePromotionResponse struct {
	*promotions.Promotion
	StoppedPromotionNames  []string `json:"stopped_promotion_names,omitempty"`
	AdjustedPromotionNames []string `json:"adjusted_promotion_names,omitempty"`
}
