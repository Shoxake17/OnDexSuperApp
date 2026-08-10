package catalog

import (
	"context"
	"errors"
	"strings"
	"unicode"
)

type Restaurant struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Address  string  `json:"address"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	Open     bool    `json:"open"` 
	LogoURL  string  `json:"logo_url"`  
	CoverURL string  `json:"cover_url"` 
	Tags string `json:"tags"`
}

type Product struct {
	ID           string  `json:"id"`
	RestaurantID string  `json:"restaurant_id"`
	Name         string  `json:"name"`
	Category     string  `json:"category"`
	PriceTiyin   int64   `json:"price_tiyin"`
	// DiscountPriceTiyin — ixtiyoriy chegirma narxi, 0 = chegirma yo'q.
	// Agar 0 dan katta bo'lsa, PriceTiyin'dan KICHIK bo'lishi shart
	// (tekshiruv POST /restaurants/{id}/products handler'ida).
	DiscountPriceTiyin int64   `json:"discount_price_tiyin"`
	Stock              int     `json:"stock"`       // 0 = cheksiz (nazorat qilinmaydi); restoran paneli UI'sida endi ko'rsatilmaydi
	Weight             float64 `json:"weight"`      // 0 = ko'rsatilmaydi (ixtiyoriy); birligi WeightUnit'da
	WeightUnit         string  `json:"weight_unit"` // "g","kg","ml","l","dona","porsiya" — faqat Weight > 0 bo'lsa ma'noli
	Description        string  `json:"description"` // ixtiyoriy, mijoz ilovasida taom tafsilotlar oynasida ko'rsatiladi
	// PrepTimeText — ixtiyoriy, erkin matnli taxminiy tayyorlash vaqti
	// (masalan "20-30 daqiqa"), mijozga ko'rsatish uchun. Bu — buyurtma
	// qabul qilinganda restoran kiritadigan HAQIQIY, aniq
	// Order.PreparationMinutes'dan FARQLI: bu yerdagi qiymat taomning
	// o'zida statik saqlanadigan, taxminiy ko'rsatkich, xolos.
	PrepTimeText string `json:"prep_time_text"`
	ImageURL     string `json:"image_url"`
	Available    bool   `json:"available"`
}

// MaxPrepTimeTextLength — PrepTimeText uchun maksimal uzunlik (erkin matn,
// lekin cheksiz uzun bo'lmasligi kerak).
const MaxPrepTimeTextLength = 50

// AllowedWeightUnits — Product.WeightUnit uchun ruxsat etilgan qiymatlar
// (whitelist) — foydalanuvchi kiritgan ixtiyoriy matn emas, aniq ro'yxatdan
// tanlanadi, shu bilan xavfsizlik uchun ham qat'iy tekshiriladi.
var AllowedWeightUnits = []string{"g", "kg", "ml", "l", "dona", "porsiya"}

func IsAllowedWeightUnit(u string) bool {
	for _, x := range AllowedWeightUnits {
		if x == u {
			return true
		}
	}
	return false
}

// MaxDescriptionLength — tasvif maydoni uchun maksimal uzunlik (abuse/DoS
// oldini olish — cheksiz uzun matn saqlanmasin).
const MaxDescriptionLength = 1000

// ProductSearchResult — SearchProducts natijasi: taom + qaysi restorandan
// ekanligini ko'rsatish uchun kerakli minimal restoran ma'lumoti (mijoz
// ilovasi turkum bo'yicha barcha restoranlardagi taomlarni bitta ro'yxatda
// ko'rsatishi uchun).
type ProductSearchResult struct {
	Product
	RestaurantName    string `json:"restaurant_name"`
	RestaurantLogoURL string `json:"restaurant_logo_url"`
	RestaurantOpen    bool   `json:"restaurant_open"`
}

// PredefinedCategories — BARCHA restoranlar uchun umumiy, standart taom
// turkumlari ro'yxati. Bu YAGONA manba (single source of truth): restoran
// paneli taom qo'shishda shu ro'yxatdan tanlaydi, mijoz ilovasi esa
// restoranlar sahifasidagi turkum qatorini shu RO'YXAT asosida (har doim
// to'liq, restoranlarda haqiqatda ishlatilgan-ishlatilmaganidan qat'i
// nazar) chizadi — ikkala tomon alohida-alohida ro'yxat saqlasa,
// yozilishi biroz farq qilib ketishi ("Fastfood" / "Fast Food" kabi)
// muqarrar, shuning uchun backend'da bitta joyda saqlanadi.
var PredefinedCategories = []string{
	"Fast Food",
	"Ichimliklar",
	"Shirinliklar",
	"Milliy taomlar",
	"Yevropa taomlar",
	"Steyklar",
	"Pizza",
	"Burgerlar",
	"KFC",
	"Norin",
	"Suyuq ovqatlar",
	"Quyuq ovqatlar",
	"Salatlar",
	"Gazaklar",
	"Desertlar",
}

// NormalizeForSearch — harf/raqamdan boshqa hammasini (bo'shliq, tire,
// apostrof, ...) olib tashlab, kichik harfga o'tkazadi. Shu tufayli
// "Fastfood" (restoran turkumi) va "Fast Food" (taom turkumi) kabi faqat
// bo'shliq/registr bilan farq qiladigan matnlar bir xil deb topiladi —
// SearchProducts turkum va nom bo'yicha moslikni shu funksiya orqali
// tekshiradi.
func NormalizeForSearch(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var (
	ErrNotFound         = errors.New("topilmadi")
	ErrRestaurantClosed = errors.New("restoran hozir yopiq")
	ErrMixedRestaurants = errors.New("bitta buyurtmada faqat bitta restoran taomlari bo'lishi mumkin")
	ErrUnavailable      = errors.New("taom hozir mavjud emas")
)

type Repository interface {
	ListRestaurants(ctx context.Context) ([]*Restaurant, error)
	GetRestaurant(ctx context.Context, id string) (*Restaurant, error)
	SaveRestaurant(ctx context.Context, r *Restaurant) error // yangi yoki yangilash
	// DeleteRestaurant — restoran va uning barcha taomlarini o'chiradi.
	DeleteRestaurant(ctx context.Context, id string) error
	ListProducts(ctx context.Context, restaurantID string) ([]*Product, error)
	GetProductsByIDs(ctx context.Context, ids []string) ([]*Product, error)
	// SearchProducts — restoranga bog'liq bo'lmagan holda, nomi yoki
	// turkumi so'rovga mos (case-insensitive, qisman mos ham) barcha
	// mavjud taomlarni qaytaradi. Masalan "Lavash" so'rovi "Lavash mini"
	// nomli taomni ham topadi.
	SearchProducts(ctx context.Context, query string) ([]*ProductSearchResult, error)
	SaveProduct(ctx context.Context, p *Product) error
	// DeleteProduct — taomni menyudan butunlay o'chiradi. Eski buyurtmalarga
	// ta'sir qilmaydi: order.Item nomi/narxi buyurtma vaqtida nusxalanib
	// saqlanadi, mahsulotga jonli havola emas.
	DeleteProduct(ctx context.Context, id string) error
}
