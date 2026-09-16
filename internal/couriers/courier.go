package couriers

import (
	"context"
	"time"
)

// VehicleType — kuryerning transport turi. Dispatch vaqtida Google Distance
// Matrix so'roviga qaysi harakatlanish rejimi (mode) bilan murojaat
// qilinishini belgilaydi (piyoda/velosiped/moped-mashina uchun ETA butunlay
// boshqacha hisoblanadi).
type VehicleType string

const (
	VehicleFoot  VehicleType = "foot"  // piyoda — Google "walking"
	VehicleBike  VehicleType = "bike"  // velosiped — Google "bicycling"
	VehicleMoped VehicleType = "moped" // mototsikl/skuter — Google "driving"
	VehicleCar   VehicleType = "car"   // mashina — Google "driving"
)

func (v VehicleType) Valid() bool {
	switch v {
	case VehicleFoot, VehicleBike, VehicleMoped, VehicleCar:
		return true
	}
	return false
}

// PlatformPool — OnDex platforma kuryerlari havuzi (`Courier.RestaurantID`
// bo'sh). Restoran havuzi — restoran ID'sining o'zi.
const PlatformPool = ""

type Courier struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// RestaurantID — restoranning O'Z kuryeri bo'lsa o'sha restoran ID'si
	// ("Xodimlar" bo'limi, `staff.UserAccounts`); bo'sh — OnDex platforma
	// kuryeri. Dispatch havuzi shu maydon bo'yicha QAT'IY ajratiladi.
	RestaurantID string  `json:"restaurant_id"`
	Lat          float64 `json:"lat"`
	Lng          float64 `json:"lng"`
	Available    bool    `json:"available"` // online va bo'sh
	// Approved — superadmin tasdiqlagach true. Tasdiqlanmagan kuryer
	// online bo'la olmaydi va unga taklif yuborilmaydi.
	Approved bool `json:"approved"`
	// VehicleType — ro'yxatdan o'tishda kuryer o'zi tanlaydi. Bo'sh bo'lsa
	// (eski, migratsiyadan oldingi yozuvlar) dispatch VehicleMoped
	// ("driving") deb hisoblaydi — bu xavfsiz standart taxmin.
	VehicleType VehicleType `json:"vehicle_type"`
	// Rating — 1.0..5.0, boshlang'ich qiymat 5.0 (neytral, hali hech kim
	// pastga tushirmagan holat). HOZIRCHA buni o'zgartiradigan haqiqiy
	// "mijoz kuryerni baholaydi" funksiyasi qurilmagan — bu maydon kelajakda
	// shu funksiya uchun tayyor infratuzilma sifatida qo'shilgan, hozircha
	// har doim boshlang'ich qiymatida qoladi (soxta/o'ylab topilgan raqam
	// EMAS — bu ataylab neytral standart, hali "haqiqiy" reyting yo'q).
	Rating float64 `json:"rating"`
	// CompletedOrders — muvaffaqiyatli yetkazilgan (StatusDelivered)
	// buyurtmalar soni. TO'LIQ HAQIQIY va obyektiv — har safar buyurtma
	// "delivered" holatiga o'tganda +1 qilinadi (cmd/api/main.go).
	CompletedOrders int `json:"completed_orders"`
}

// Repository — hozir in-memory, keyin PostgreSQL.
type Repository interface {
	GetByID(ctx context.Context, id string) (*Courier, error)
	Create(ctx context.Context, c *Courier) error
	ListAll(ctx context.Context) ([]*Courier, error)
	// ListAvailable — barcha tasdiqlangan VA hozir onlayn (bo'sh) kuryerlar —
	// bu FAQAT nomzodlar HAVUZI. Kimga BIRINCHI navbatda taklif yuborilishi
	// ETA/reyting/tajriba asosida `ScoreCandidates` orqali hisoblanadi
	// (qarang: scoring.go, dispatch.go).
	ListAvailable(ctx context.Context) ([]*Courier, error)
	// ListAvailableNear — `ListAvailable` ning YAQINLIK bo'yicha
	// filtrlangan varianti. Nomzodlar DB darajasida tanlanadi
	// (PostGIS `ST_DWithin` + GiST indeks), eng yaqinidan boshlab.
	//
	// ┌─ NEGA KERAK ──────────────────────────────────────────────────┐
	// `ListAvailable` MASOFADAN QAT'I NAZAR hamma onlayn kuryerni
	// qaytaradi. Keyin Go tomonda HAR BIRI uchun Google Distance
	// Matrix'ga so'rov ketadi — bu PULLIK. 20 km naridagi kuryer
	// uchun ham ETA hisoblanardi va natija baribir tashlab
	// yuborilardi.
	//
	// Jonli o'lchov (50 000 kuryer): eski usul 50 002 qator,
	// yangisi 20 qator (13 ms, GiST indeks bilan).
	// └───────────────────────────────────────────────────────────────┘
	//
	// `maxAge` — joylashuv shundan eski bo'lsa kuryer TASHLAB
	// YUBORILADI (ilovasi qotib qolgan yoki tarmoqdan uzilgan).
	// Nol bo'lsa eskilik tekshirilmaydi.
	//
	// `pool` — QAT'IY havuz: restoran ID'si bo'lsa FAQAT shu restoranning
	// o'z kuryerlari, `PlatformPool` bo'lsa FAQAT platforma kuryerlari.
	// Aralashmaydi: restoran kuryeri boshqa restoran buyurtmasini ko'rmaydi,
	// platforma qidiruvi ham restoranlarning xodimlarini olib ketmaydi.
	ListAvailableNear(ctx context.Context, pool string, lat, lng float64,
		radiusMeters float64, maxAge time.Duration, limit int) ([]*Courier, error)
	// SetName — kuryer ismi (restoran xodim yozuvini tahrirlaganda).
	SetName(ctx context.Context, id, name string) error
	SetAvailable(ctx context.Context, id string, available bool) error
	// ClaimIfAvailable — kuryerni ATOMIK ravishda band qiladi: faqat u
	// HOZIR bo'sh bo'lsa `available=false` qiladi va `true` qaytaradi;
	// allaqachon band bo'lsa hech narsa o'zgartirmay `false` qaytaradi.
	//
	// MUHIM: busiz ikki parallel buyurtma bitta bo'sh kuryerni ko'rib,
	// ikkalasi ham taklif yuborardi va kuryer ikkalasini ham qabul qila
	// olardi (`SetAvailable(false)` shartsiz edi) — natijada bitta
	// kuryerga IKKI faol yetkazma biriktirilardi.
	ClaimIfAvailable(ctx context.Context, id string) (bool, error)
	SetApproved(ctx context.Context, id string, approved bool) error
	// SoftDelete — kuryer yozuvini ro'yxatlardan olib tashlaydi va
	// undagi SHAXSIY ma'lumotni (ism) o'chiradi.
	//
	// ┌─ NEGA HAQIQIY `DELETE` EMAS ──────────────────────────────────┐
	// `orders.courier_id` shu jadvalga FOREIGN KEY. Bir marta ham
	// yetkazgan kuryerni haqiqatan o'chirish ikki yomon variantdan
	// birini tanlashga majbur qilardi: yo baza cheklovi buzilib
	// o'chirish umuman ishlamaydi, yo o'nlab tarixiy buyurtma
	// birgalikda o'chib ketadi (moliyaviy hisobot yo'qoladi).
	//
	// Shu sabab yozuv qoladi, LEKIN: ismi tozalanadi, `approved` va
	// `available` o'chiriladi va `deleted_at` qo'yiladi — ya'ni u
	// superadmin ro'yxatida ham, dispatch nomzodlari orasida ham
	// BOSHQA KO'RINMAYDI. Foydalanuvchi akkaunti (`users`) esa
	// haqiqatan o'chiriladi — shaxsiy ma'lumot aynan o'sha yerda.
	// └───────────────────────────────────────────────────────────────┘
	SoftDelete(ctx context.Context, id string) error
	// UpdateLocation — kuryer ilovasi davriy yuboradigan joriy koordinata
	// (dispatch endi ETA hisoblash uchun AYNAN shu koordinatadan foydalanadi).
	UpdateLocation(ctx context.Context, id string, lat, lng float64) error
	// IncrementCompletedOrders — buyurtma "delivered" holatiga o'tganda
	// chaqiriladi (cmd/api/main.go). Haqiqiy, obyektiv tajriba hisoblagichi.
	IncrementCompletedOrders(ctx context.Context, id string) error
}
