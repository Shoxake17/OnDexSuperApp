package orders

import "time"

// Status — buyurtma holati. Holatlar orasidagi o'tishlar statemachine.go da qat'iy belgilangan.
type Status string

const (
	StatusCreated   Status = "created"   // mijoz yaratdi, restoran hali ko'rmadi
	StatusAccepted  Status = "accepted"  // restoran qabul qildi
	StatusPreparing Status = "preparing" // tayyorlanmoqda
	StatusReady     Status = "ready"     // tayyor (yetkazishda — kuryer olishi mumkin; stolda — affitsiant olib borishi kerak)
	StatusPickedUp  Status = "picked_up" // kuryer oldi, yo'lda
	StatusDelivered Status = "delivered" // yetkazildi (terminal)
	StatusServed    Status = "served"    // affitsiant stolga olib bordi (terminal, faqat dine_in)
	StatusRejected  Status = "rejected"  // restoran rad etdi (terminal)
	StatusCancelled Status = "cancelled" // bekor qilindi (terminal)
)

// Type — buyurtma turi.
//
// ┌─ NEGA MODELDA, ALOHIDA JADVALDA EMAS ─────────────────────────────┐
// Ikkala tur ham bir xil hayot siklining KATTA qismini bo'lishadi:
// yaratildi → qabul qilindi → tayyorlanmoqda → tayyor. Faqat OXIRGI
// qadam farq qiladi (kuryer olib ketadi / affitsiant stolga qo'yadi).
//
// Alohida jadval qilinsa, restoran paneli, narxlash, aksiyalar,
// tarix, bildirishnomalar — hammasi IKKI MARTA yozilardi. Bitta
// maydon esa farqni aynan kerakli uch joyda ushlaydi: dispatch,
// manzil tekshiruvi va holat mashinasi.
// └───────────────────────────────────────────────────────────────────┘
type Type string

const (
	// TypeDelivery — kuryer yetkazadi. BO'SH QIYMAT ham shu deb
	// hisoblanadi: bazadagi eski buyurtmalarda ustun yo'q edi va
	// migratsiya ularni `delivery` bilan to'ldiradi, lekin xotira
	// omborida va testlarda nol qiymat uchrashi mumkin.
	TypeDelivery Type = "delivery"
	// TypeDineIn — mijoz restoranda, stol QR kodi orqali buyurtma berdi.
	TypeDineIn Type = "dine_in"
)

// Normalized — bo'sh turni `delivery` ga keltiradi.
//
// Bu funksiya SHART: nol qiymatni har bir chaqiruv joyida qo'lda
// tekshirish unutiladi va o'shanda eski buyurtma "na delivery, na
// dine_in" bo'lib qolib, holat mashinasidan o'ta olmasdi.
func (t Type) Normalized() Type {
	if t == "" {
		return TypeDelivery
	}
	return t
}

// Actor — holatni kim o'zgartirmoqchi. Har bir o'tish faqat ruxsat etilgan aktorlarga ochiq.
type Actor string

const (
	ActorCustomer   Actor = "customer"
	ActorRestaurant Actor = "restaurant"
	ActorCourier    Actor = "courier"
	ActorWaiter     Actor = "waiter"
	ActorAdmin      Actor = "admin"
	ActorSystem     Actor = "system"
)

type Item struct {
	ProductID  string `json:"product_id"`
	Name       string `json:"name"`
	Qty        int    `json:"qty"`
	PriceTiyin int64  `json:"price_tiyin"` // pul har doim tiyinda, float ishlatilmaydi
	// DiscountPriceTiyin — restoran shu mahsulotga belgilagan chegirma
	// narxi (0 = chegirma yo'q). `PriceTiyin` dan ALOHIDA saqlanadi.
	//
	// MUHIM: bu narxlash uchun XOM ma'lumot. Mahsulot chegirmasi va
	// aksiya chegirmasi HECH QACHON qo'shilmaydi — narxlash qatlami
	// (`priceCart`) ikkalasidan mijozga foydaliroq BITTASINI tanlaydi.
	// Avval ular stack bo'lib, jami nolga tushib ketardi.
	DiscountPriceTiyin int64 `json:"discount_price_tiyin,omitempty"`
	// ImageURL — buyurtma vaqtidagi taom rasmi, Name/PriceTiyin kabi
	// "suratga olinadi" (snapshot): taom keyinchalik o'chirilsa/rasmi
	// almashtirilsa ham, eski buyurtma tarixida asl rasm saqlanib qoladi.
	ImageURL string `json:"image_url,omitempty"`
	// Category — buyurtma vaqtidagi mahsulot turkumi, aksiyalarni
	// (promotions.TargetCategories) qo'llash uchun kerak. Boshqa
	// maydonlar kabi suratga olinadi (snapshot).
	Category string `json:"category,omitempty"`
}

type StatusChange struct {
	From Status    `json:"from"`
	To   Status    `json:"to"`
	By   Actor     `json:"by"`
	At   time.Time `json:"at"`
}

// Address — buyurtmaga biriktiriladigan yetkazish manzili tafsilotlari.
// `users.AddressDetails` bilan bir xil maydonlar, lekin ATAYLAB alohida
// tur: buyurtma paketi foydalanuvchilar paketiga bog'lanib qolmasligi
// uchun (va bu — profil manzili emas, buyurtma vaqtidagi NUSXA).
type Address struct {
	Text      string `json:"text,omitempty"`
	Entrance  string `json:"entrance,omitempty"`  // podъezd
	Floor     string `json:"floor,omitempty"`     // qavat
	Apartment string `json:"apartment,omitempty"` // kvartira
	Intercom  string `json:"intercom,omitempty"`  // domofon
	Comment   string `json:"comment,omitempty"`   // kuryer uchun izoh
}

type Order struct {
	ID string `json:"id"`
	// OrderNumber — mijoz/restoran/kuryerga ko'rsatiladigan raqam, "DDMMYY-N"
	// shaklida (masalan "300726-0000123", Yandex uslubida) — ichki `ID`
	// (tasodifiy hex) hech qachon foydalanuvchiga ko'rsatilmaydi, faqat shu
	// raqam. Buyurtmani RESTORANDA olib ketishda kuryer shu raqamning
	// OXIRGI 4 xonasini xodimga og'zaki aytadi — alohida tasdiqlash kodi
	// endi YO'Q (statemachine.go'dagi StatusReady qoidasiga qarang).
	OrderNumber  string         `json:"order_number"`
	CustomerID   string         `json:"customer_id"`
	RestaurantID string         `json:"restaurant_id"`
	CourierID    string         `json:"courier_id,omitempty"` // dispatch muvaffaqiyatli bo'lganda to'ladi
	Items        []Item         `json:"items"`
	// SubtotalTiyin — aksiya qo'llanilishidan OLDINGI summa (Σ price*qty).
	// DiscountTiyin — shu summadan qancha ayirilgani (0 = aksiya
	// qo'llanilmagan). TotalTiyin har doim SubtotalTiyin-DiscountTiyin'ga
	// teng — HAQIQIY, promotions.ApplyBest orqali hisoblangan qiymatlar,
	// checkout'da ko'rsatiladigan raqamlar bilan bir xil manba.
	SubtotalTiyin   int64  `json:"subtotal_tiyin"`
	DiscountTiyin   int64  `json:"discount_tiyin,omitempty"`
	PromotionID     string `json:"promotion_id,omitempty"`
	// PromotionName — qo'llanilgan aksiya nomi, buyurtma vaqtida
	// suratga olinadi (aksiya keyin o'chirilsa/o'zgartirilsa ham eski
	// buyurtma tarixida asl nom saqlanadi).
	PromotionName string `json:"promotion_name,omitempty"`
	TotalTiyin    int64  `json:"total_tiyin"`
	Status        Status `json:"status"`
	History      []StatusChange `json:"history"`
	DeliveryLat  float64        `json:"delivery_lat"`
	DeliveryLng  float64        `json:"delivery_lng"`
	// DeliveryAddress — buyurtma berilgan PAYTDAGI manzil tafsilotlari
	// SURATI (podyezd/qavat/kvartira/domofon/izoh). Foydalanuvchining
	// profilidagi manzilga HAVOLA emas, nusxa — mijoz keyin manzilini
	// o'zgartirsa ham, kuryer aynan shu buyurtma uchun kiritilgan
	// ma'lumotni ko'radi (Item'dagi nom/narx nusxalanishi bilan bir xil
	// mantiq). Busiz kuryer faqat koordinatani ko'rardi va ko'p qavatli
	// uyga yetkazishda kvartira raqamini bilmasdi.
	DeliveryAddress Address   `json:"delivery_address"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	// ── Stolda ovqatlanish (dine_in) ──
	//
	// Type bo'sh bo'lsa `delivery` demak (Type.Normalized()ga qarang) —
	// eski buyurtmalar va eski klientlar shu sababli buzilmaydi.
	Type Type `json:"type,omitempty"`
	// TableID — `restaurant_tables.id`. QR token EMAS: token — sir
	// (uni bilgan odam shu stolga buyurtma bera oladi), shuning uchun
	// u buyurtmada saqlanmaydi va API javoblarida hech qachon
	// ko'rinmaydi.
	TableID string `json:"table_id,omitempty"`
	// TableLabel — stolning ko'rinadigan nomi ("5", "VIP-2") buyurtma
	// vaqtidagi NUSXASI. Items/DeliveryAddress bilan bir xil mantiq:
	// stol keyin qayta nomlansa ham, eski buyurtma tarixida asl nom
	// qoladi — affitsiant va restoran bir xil narsani ko'radi.
	TableLabel string `json:"table_label,omitempty"`
	// PartySize — nechta kishi (mijoz o'zi kiritadi). Faqat
	// ma'lumot uchun: idish-tovoq, non, joy tayyorlash. Narxga
	// TA'SIR QILMAYDI.
	PartySize int `json:"party_size,omitempty"`

	// PreparationMinutes/ReadyAt — restoran "Qabul qilindi" bosgan payt
	// kiritadigan taxminiy tayyorlash vaqti. ReadyAt = qabul qilingan payt +
	// PreparationMinutes — dispatch matching engine kuryerning restoranga
	// ETA'sini AYNAN shu vaqtga moslashtirish uchun ishlatadi (na juda erta,
	// na juda kech kelsin).
	PreparationMinutes int        `json:"preparation_minutes,omitempty"`
	ReadyAt            *time.Time `json:"ready_at,omitempty"`

	// IdempotencyKey — mijoz ilovasi buyurtma yaratishda yuboradigan
	// ixtiyoriy, bir martalik tasodifiy kalit (masalan
	// customer_app/lib/api.dart'dagi newIdempotencyKey()). Tarmoq uzilib,
	// javob kelmay qolgan holatda foydalanuvchi qayta bossa, mijoz XUDDI
	// SHU kalitni qayta yuboradi — server buni ko'rib, YANGI buyurtma
	// yaratish o'rniga ESKI (birinchi urinishda yaratilgan) buyurtmani
	// qaytaradi (Service.Create()ga qarang). Bo'sh bo'lishi mumkin (eski
	// klientlar/boshqa rollar buyurtma yaratmaydi) — shu holda tekshiruv
	// o'tkazib yuboriladi. Noyoblik (bitta mijoz doirasida) DB darajasida
	// ham majburlanadi (migration 0016, `customer_id`+`idempotency_key`
	// unique indeks) — check-then-insert orasidagi tabiiy race'ga qarshi
	// oxirgi himoya sifatida.
	IdempotencyKey string `json:"idempotency_key,omitempty"`

	// Version — optimistik parallel boshqaruv (optimistic concurrency)
	// uchun. Har bir Save() muvaffaqiyatli yozganda +1 oshadi. Repository
	// implementatsiyalari Save()da "faqat o'qilgan Version hali ham DB'dagi
	// bilan bir xil bo'lsagina yoz" shartini qo'llashi SHART — aks holda
	// ikkita parallel so'rov (masalan kuryer "picked_up" bosayotganda admin
	// "cancel" bossa) bir xil eski holatni o'qib, biri ikkinchisining
	// yozuvini jimgina bosib yuborishi mumkin edi (Repository.Save izohiga
	// qarang).
	Version int `json:"version"`
}

func (o *Order) IsTerminal() bool {
	switch o.Status {
	case StatusDelivered, StatusServed, StatusRejected, StatusCancelled:
		return true
	}
	return false
}

// IsDineIn — stolda ovqatlanish buyurtmasimi.
func (o *Order) IsDineIn() bool { return o.Type.Normalized() == TypeDineIn }
