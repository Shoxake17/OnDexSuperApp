package catalog

import (
	"context"
	"errors"
	"strings"
	"time"
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
	Tags     string  `json:"tags"`

	// ┌─ 0 = "MA'LUMOT YO'Q", "yomon" EMAS ────────────────────────────┐
	// Mijoz tomonida 0 bo'lganda tegishli chip UMUMAN chizilmaydi —
	// "0.0 ★" yoki "0 daqiqa" ko'rsatilmaydi. Shu sabab bu maydonlar
	// ko'rsatkich BO'LMAGAN restoranni ham buzmaydi.
	// └────────────────────────────────────────────────────────────────┘
	//
	// Reyting HOZIRCHA qo'lda kiritiladi (admin panel). Haqiqiy
	// baholash tizimi qurilganda (ROADMAP 3-band) shu maydonlar
	// buyurtmalardan hisoblanadi va qo'lda kiritish olib tashlanadi.
	Rating      float64 `json:"rating"`
	RatingCount int     `json:"rating_count"`

	// Yetkazish vaqti oralig'i, daqiqada. Mahsulotdagi PrepTimeText'dan
	// FARQLI: u bitta taomni tayyorlash vaqti, bu esa mijozgacha
	// yetkazishning umumiy taxminiy oralig'i.
	ETAMinMinutes int `json:"eta_min_minutes"`
	ETAMaxMinutes int `json:"eta_max_minutes"`

	// ┌─ 3D MAKET ─────────────────────────────────────────────────────┐
	// Ba'zi restoranlarning ichki maketi 3D da chizilgan: mijoz ilova
	// ichida kirib, aylanib, stolga o'tirib buyurtma bera oladi.
	//
	// Bog'lanish SHU YERDA, ilovada EMAS. Aks holda har yangi kafe
	// qo'shilganda ilovaning yangi versiyasini chiqarish kerak
	// bo'lardi. Endi admin panelda manzilni yozish kifoya.
	//
	// Bo'sh bo'lsa — bu restoranda 3D yo'q va mijozga taklif
	// ko'rsatilmaydi. Ulanmagan restoranlarga begona maket
	// bog'lanmasligi shu bilan ta'minlanadi.
	// └────────────────────────────────────────────────────────────────┘
	//
	// Maket fayli (.pck) manzili. Faqat R2 domeni qabul qilinadi.
	Scene3DURL string `json:"scene_3d_url,omitempty"`
	// Faylning SHA-256 yig'indisi. Ilova yuklab olgach TEKSHIRADI:
	// maket ichida bajariladigan kod bor, tekshiruvsiz ishga
	// tushirish mumkin emas.
	Scene3DSHA256 string `json:"scene_3d_sha256,omitempty"`
	// Fayl hajmi (bayt) — mijozga "~104 MB yuklanadi" deb oldindan
	// aytish uchun.
	Scene3DBytes int64 `json:"scene_3d_bytes,omitempty"`
}

// PublicView — restoranning AUTENTIFIKATSIYASIZ ko'rsatiladigan nusxasi.
//
// ┌─ NEGA MAKET MAYDONLARI OLIB TASHLANADI ────────────────────────────┐
// `GET /restaurants` ochiq endpoint va u maket manzilini ham
// qaytarardi. Ya'ni javobni bir marta o'qigan har kim (bot ham)
// 100-220 MB lik faylni cheksiz yuklab olardi.
//
// Maket rasm emas: u restoranning ichki maketi va ichida
// BAJARILADIGAN kod bor. Shuning uchun u kirgan foydalanuvchiga,
// muddatli imzolangan havola bilan beriladi (`internal/scenes`).
//
// Uchala maydon BIRGA olib tashlanadi: faqat manzilni yashirib,
// xesh va hajmni qoldirish ma'nosiz va chalkash bo'lardi.
// └────────────────────────────────────────────────────────────────────┘
func (r Restaurant) PublicView() Restaurant {
	r.Scene3DURL = ""
	r.Scene3DSHA256 = ""
	r.Scene3DBytes = 0
	return r
}

type Product struct {
	ID           string `json:"id"`
	RestaurantID string `json:"restaurant_id"`
	Name         string `json:"name"`
	Category     string `json:"category"`
	PriceTiyin   int64  `json:"price_tiyin"`
	// DiscountPriceTiyin — ixtiyoriy chegirma narxi, 0 = chegirma yo'q.
	// Agar 0 dan katta bo'lsa, PriceTiyin'dan KICHIK bo'lishi shart
	// (tekshiruv POST /restaurants/{id}/products handler'ida).
	DiscountPriceTiyin int64 `json:"discount_price_tiyin"`
	// WholesalePriceTiyin — ixtiyoriy ULGURJI narx (0 = kiritilmagan).
	//
	// ┌─ MIJOZGA HECH QACHON KO'RSATILMAYDI ──────────────────────────┐
	// Bu — restoranning ICHKI biznes ma'lumoti (tannarx/ulgurji
	// hisob-kitob uchun). Ochiq endpoint'lar (`GET /restaurants/{id}/
	// menu`, `GET /products/search`) uni `PublicView()` orqali OLIB
	// TASHLAB yuboradi; restoran paneli esa avtorizatsiya talab
	// qiladigan `GET /restaurants/{id}/products` dan oladi.
	//
	// Narxlashga UMUMAN ta'sir qilmaydi — mijoz to'laydigan summa faqat
	// PriceTiyin, DiscountPriceTiyin va aksiyalardan kelib chiqadi.
	// └───────────────────────────────────────────────────────────────┘
	WholesalePriceTiyin int64   `json:"wholesale_price_tiyin"`
	Stock               int     `json:"stock"`       // 0 = cheksiz (nazorat qilinmaydi); restoran paneli UI'sida endi ko'rsatilmaydi
	Weight              float64 `json:"weight"`      // 0 = ko'rsatilmaydi (ixtiyoriy); birligi WeightUnit'da
	WeightUnit          string  `json:"weight_unit"` // "g","kg","ml","l","dona","porsiya" — faqat Weight > 0 bo'lsa ma'noli
	Description         string  `json:"description"` // ixtiyoriy, mijoz ilovasida taom tafsilotlar oynasida ko'rsatiladi
	// PrepTimeText — ixtiyoriy, erkin matnli taxminiy tayyorlash vaqti
	// (masalan "20-30 daqiqa"), mijozga ko'rsatish uchun. Bu — buyurtma
	// qabul qilinganda restoran kiritadigan HAQIQIY, aniq
	// Order.PreparationMinutes'dan FARQLI: bu yerdagi qiymat taomning
	// o'zida statik saqlanadigan, taxminiy ko'rsatkich, xolos.
	PrepTimeText string `json:"prep_time_text"`
	ImageURL     string `json:"image_url"`
	Available    bool   `json:"available"`

	// ┌─ 3D MODEL (AI orqali rasmdan generatsiya) ────────────────────┐
	// Model3DURL — R2 dagi tayyor GLB fayl. Mijozga FAQAT shu maydon
	// ko'rsatiladi (`PublicView`), qolgan ikkitasi restoran panelining
	// ichki holati.
	//
	// Generatsiya UZOQ (bir necha daqiqa) va TASHQI xizmatga bog'liq,
	// shuning uchun holat mahsulotning O'ZIDA saqlanadi: server qayta
	// ishga tushsa ham tugallanmagan vazifa yo'qolmaydi
	// (`Model3DTaskID` bo'yicha davom ettiriladi).
	// └───────────────────────────────────────────────────────────────┘
	Model3DURL    string `json:"model_3d_url,omitempty"`
	Model3DStatus string `json:"model_3d_status,omitempty"`
	Model3DTaskID string `json:"model_3d_task_id,omitempty"`
}

// 3D model holatlari. Bo'sh satr — hech qachon generatsiya qilinmagan.
const (
	Model3DPending = "pending" // Tripo'da navbatda/ishlanmoqda
	Model3DReady   = "ready"   // GLB R2 ga yuklandi, Model3DURL to'ldi
	Model3DFailed  = "failed"  // generatsiya yiqildi (qayta urinsa bo'ladi)
)

// PublicView — mahsulotning MIJOZGA ko'rsatiladigan nusxasi: restoranning
// ichki biznes maydonlari (hozircha ulgurji narx) olib tashlanadi.
//
// Ochiq endpoint'lar `catalog.Product` ni to'g'ridan-to'g'ri JSON qilib
// yozadi, ya'ni struct'ga qo'shilgan HAR QANDAY yangi maydon avtomatik
// ravishda ommaviy bo'lib qoladi. Shuning uchun ichki maydonlar shu
// yagona joyda kesiladi — yangi maydon qo'shganda ham shu yerga qarash
// kifoya.
func (p Product) PublicView() Product {
	p.WholesalePriceTiyin = 0
	// 3D generatsiyasining ICHKI holati mijozga kerak emas: unga faqat
	// tayyor model havolasi (`Model3DURL`) ko'rsatiladi. Task ID esa
	// tashqi xizmatdagi ichki identifikator — uni tarqatishning hech
	// qanday sababi yo'q ("eng kam ma'lumot" prinsipi).
	p.Model3DStatus = ""
	p.Model3DTaskID = ""
	return p
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
// Book — kafe kutubxonasidagi kitob.
//
// ┌─ NEGA MAHSULOT EMAS ───────────────────────────────────────────────┐
// Kitob sotilmaydi va savatga tushmaydi: u faqat 3D maketda javondan
// olib o'qish uchun. Mahsulot modeliga qo'shilsa, narx, ombor va
// buyurtma mantig'i unga ham tegishli bo'lib qolardi - va kitobni
// tasodifan sotib olish mumkin bo'lardi.
//
// Alohida tur bu ehtimolni butunlay yo'q qiladi: kitobda narx maydoni
// UMUMAN yo'q.
// └────────────────────────────────────────────────────────────────────┘
type Book struct {
	ID           string `json:"id"`
	RestaurantID string `json:"restaurant_id"`
	Title        string `json:"title"`
	Author       string `json:"author"`

	// CoverURL — muqova rasmi (ixtiyoriy).
	CoverURL string `json:"cover_url"`

	// Text — kitob matni. 3D maketdagi o'quvchi uni sahifalarga
	// bo'lib chiqaradi: sahifa o'lchami ekranga bog'liq, shuning
	// uchun bo'linish SERVERDA emas, ko'rsatish paytida bo'ladi.
	Text string `json:"text"`

	// Pages — sahifa rasmlari (ixtiyoriy). Matnli kitob uchun bo'sh.
	// Ikkalasi ham bo'lsa, rasmlar ustun turadi.
	Pages []string `json:"pages"`

	// PDFURL — yuklangan PDF ning R2 dagi manzili.
	//
	// ┌─ NEGA MATN BILAN BIRGA SAQLANADI ──────────────────────────────┐
	// `Text` PDF dan SERVERDA ajratiladi va o'sha zahoti saqlanadi.
	// Ya'ni maketdagi o'quvchi PDF ni umuman yuklab olmaydi — u
	// tayyor matnni oladi. Godot PDF ni ocha olmaydi, ochsa ham 20 MB
	// faylni telefonda tahlil qilish o'rinsiz bo'lardi.
	//
	// PDF ning o'zi asl nusxa sifatida qoladi: matn qaytadan
	// ajratilishi kerak bo'lsa (masalan ajratuvchi yaxshilansa) manba
	// yo'qolmagan bo'ladi.
	// └────────────────────────────────────────────────────────────────┘
	PDFURL string `json:"pdf_url"`

	// Active — o'chirilgan kitob maketda ko'rinmaydi, lekin
	// yozuvi saqlanib qoladi.
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

// MaxBookTextBytes — kitob matnining eng katta hajmi.
//
// Chegara ataylab: matn Mongo yozuviga to'liq sig'ishi va bitta
// so'rovda uzatilishi kerak. 400 KB ~ 200 sahifalik kitob.
const MaxBookTextBytes = 400 * 1024

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

// BookRepository — kafe kutubxonasi.
//
// ┌─ NEGA `Repository` GA QO'SHILMADI ─────────────────────────────────┐
// Uni umumiy katalog interfeysiga qo'shish barcha amalga oshirishlarni
// (jumladan sinovlar uchun xotira omborini) o'zgartirishni talab
// qilardi - kitoblar esa ularga umuman kerak emas.
//
// Alohida interfeys bo'lgani uchun server uni IXTIYORIY deb qaraydi:
// ombor ulanmagan bo'lsa, kitob endpointlari xizmat yo'qligini
// aytadi va qolgan hamma narsa ishlayveradi.
// └────────────────────────────────────────────────────────────────────┘
type BookRepository interface {
	// ListBooks — restoran kitoblari. `withText=false` bo'lsa matn
	// qaytmaydi: ro'yxatda u kerak emas va javobni shishirardi.
	ListBooks(ctx context.Context, restaurantID string, withText bool) ([]*Book, error)
	GetBook(ctx context.Context, id string) (*Book, error)
	SaveBook(ctx context.Context, b *Book) error
	DeleteBook(ctx context.Context, id string) error
}
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
