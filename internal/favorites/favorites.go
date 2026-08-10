// Package favorites — mijozning "Istaklarim" ro'yxati: qaysi mahsulotlarni
// yurak belgisi bilan saqlab qo'yganini eslab qoladi. Ataylab MINIMAL —
// faqat (customer_id, product_id) juftligini saqlaydi; mahsulotning o'zi
// (nomi, narxi, rasmi, restorani) HAR DOIM katalogdan (Mongo/Postgres) real
// vaqtda o'qiladi, shuning uchun mahsulot narxi/rasmi o'zgarsa, "Istaklarim"
// sahifasi ham avtomatik yangi ma'lumotni ko'rsatadi — eskirgan nusxa
// saqlanmaydi.
package favorites

import "context"

type Repository interface {
	// Add — idempotent: mahsulot allaqachon saqlangan bo'lsa xato
	// qaytarmaydi (qayta bosilganda ham xavfsiz).
	Add(ctx context.Context, customerID, productID string) error
	// Remove — mahsulot ro'yxatda bo'lmasa ham xato qaytarmaydi.
	Remove(ctx context.Context, customerID, productID string) error
	// ListProductIDs — mijozning saqlangan mahsulot ID'lari, ENG YANGISI
	// birinchi (qo'shilgan tartibi bo'yicha teskari).
	ListProductIDs(ctx context.Context, customerID string) ([]string, error)
}
