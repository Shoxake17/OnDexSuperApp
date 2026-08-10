// Package promotions — restoran aksiyalarini (chegirma kampaniyalarini)
// boshqarish. HOZIRCHA FAQAT boshqaruv (CRUD + ro'yxat) — checkout/buyurtma
// narxlashiga ULANMAGAN (orders.PriceOrder bu paketni bilmaydi). Shuning
// uchun "Foydalanish soni"/"Savdo summasi" har doim HAQIQIY (soxta emas),
// lekin hozircha 0 bo'ladi — real qiymatlar faqat checkout-integratsiyasi
// (alohida, keyingi bosqichdagi ish) qo'shilgach paydo bo'ladi.
package promotions

import (
	"context"
	"errors"
	"time"
)

// Type — aksiyaning marketing mexanikasi (kategoriyasi). Barcha turlar
// bir xil Chegirma/Faollik davri maydonlaridan foydalanadi — Type faqat
// tavsiflovchi yorliq, alohida maydon to'plamini talab qilmaydi
// (image/aksiyaqosh.png namunasiga mos).
type Type string

const (
	TypePercent      Type = "percent"       // Foiz orqali chegirma
	TypeFixedAmount  Type = "fixed_amount"  // Summa orqali chegirma
	TypeBOGO         Type = "bogo"          // 1+1 aksiyasi
	TypeBundle       Type = "bundle"        // To'plam aksiyasi
	TypeFreeDelivery Type = "free_delivery" // Yetkazib berish chegirmasi
	TypeLoyalty      Type = "loyalty"       // Sodiqlik bonusi
)

// AllTypes — tanlov ro'yxatlari uchun barcha ruxsat etilgan turlar.
var AllTypes = []Type{TypePercent, TypeFixedAmount, TypeBOGO, TypeBundle, TypeFreeDelivery, TypeLoyalty}

func IsValidType(t Type) bool {
	for _, x := range AllTypes {
		if x == t {
			return true
		}
	}
	return false
}

// DiscountUnit — Chegirma qiymati qanday birlikda kiritilganini bildiradi.
type DiscountUnit string

const (
	DiscountUnitPercent DiscountUnit = "percent"
	DiscountUnitAmount  DiscountUnit = "amount"
)

// Status — HISOBLANGAN holat (bazada saqlanmaydi), Active bayrog'i va
// StartAt/EndAt/Indefinite'ga qarab joriy vaqt asosida aniqlanadi.
type Status string

const (
	StatusPaused    Status = "paused" // restoran o'zi to'xtatgan (Active=false)
	StatusScheduled Status = "scheduled"
	StatusActive    Status = "active"
	StatusExpired   Status = "expired"
)

type Promotion struct {
	ID           string `json:"id"`
	RestaurantID string `json:"restaurant_id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Type         Type   `json:"type"`

	// Chegirma sozlamalari — barcha turlar uchun umumiy.
	DiscountUnit  DiscountUnit `json:"discount_unit"`
	DiscountValue int64        `json:"discount_value"` // percent: 1-100; amount: tiyin
	// MinOrderAmountTiyin — 0 = cheklovsiz.
	MinOrderAmountTiyin int64 `json:"min_order_amount_tiyin"`
	// MaxDiscountAmountTiyin — 0 = cheklovsiz (foiz asosidagi chegirmani
	// yuqoridan cheklash uchun, masalan "30% lekin ko'pi bilan 50 000 so'm").
	MaxDiscountAmountTiyin int64 `json:"max_discount_amount_tiyin"`

	// Faollik davri — to'liq sana+vaqt (avvalgi versiyada faqat sana edi).
	StartAt time.Time `json:"start_at"`
	// EndAt — Indefinite=true bo'lsa e'tiborga olinmaydi.
	EndAt      time.Time `json:"end_at"`
	Indefinite bool      `json:"indefinite"`
	// Active — restoran aksiyani qo'lda to'xtatgan/faollashtirgan holati,
	// sana oralig'idan MUSTAQIL (masalan muddati hali tugamagan aksiyani
	// ham vaqtincha to'xtatib qo'yish mumkin).
	Active bool `json:"active"`

	// Aksiya qo'llaniladigan joy — bir nechtasi birga tanlanishi mumkin.
	AppliesToProducts   bool `json:"applies_to_products"`
	AppliesToOrders     bool `json:"applies_to_orders"`
	AppliesToCategories bool `json:"applies_to_categories"`
	// TargetProductIDs/TargetCategories — AppliesToProducts/Categories
	// bo'lsa ma'noli, shu restoranning HAQIQIY mahsulot/turkumlaridan
	// tanlanadi (soxta/erkin matn emas).
	TargetProductIDs []string `json:"target_product_ids"`
	TargetCategories []string `json:"target_categories"`

	// MinPreviousOrders — FAQAT TypeLoyalty uchun ma'noli: mijoz shu
	// restorandan kamida shuncha marta oldin buyurtma bergan bo'lishi
	// kerak (0 = shart yo'q). Boshqa turlar buni e'tiborga olmaydi.
	MinPreviousOrders int64 `json:"min_previous_orders"`

	// UsageCount/SalesTotalTiyin — HAQIQIY qo'llanilish statistikasi,
	// har safar checkout'da shu aksiya tanlanganda ApplyPromotionUsage
	// orqali oshiriladi (promotions_page.dart'dagi "Foydalanish"/"Savdo"
	// ustunlari shundan o'qiydi — soxta 0 emas).
	UsageCount      int64 `json:"usage_count"`
	SalesTotalTiyin int64 `json:"sales_total_tiyin"`

	ImageURL  string    `json:"image_url"`
	CreatedAt time.Time `json:"created_at"`
}

// ComputeStatus — joriy vaqtga va Active bayrog'iga nisbatan holatni
// hisoblaydi.
func (p *Promotion) ComputeStatus(now time.Time) Status {
	if !p.Active {
		return StatusPaused
	}
	if now.Before(p.StartAt) {
		return StatusScheduled
	}
	if !p.Indefinite && now.After(p.EndAt) {
		return StatusExpired
	}
	return StatusActive
}

const MaxNameLength = 100
const MaxDescriptionLength = 200

var (
	ErrNotFound = errors.New("aksiya topilmadi")
)

type Repository interface {
	ListByRestaurant(ctx context.Context, restaurantID string) ([]*Promotion, error)
	GetByID(ctx context.Context, id string) (*Promotion, error)
	Save(ctx context.Context, p *Promotion) error
	// Delete — o'chirilayotgan aksiya berilgan restoranga tegishli
	// bo'lishini chaqiruvchi (HTTP handler) tekshiradi, repo shunchaki o'chiradi.
	Delete(ctx context.Context, id string) error
	// DeleteByRestaurant — restoran o'chirilayotganda uning BARCHA
	// aksiyalarini o'chiradi va nechtasi o'chirilganini qaytaradi.
	//
	// MUHIM: Postgres'da `promotions.restaurant_id` restoranga FOREIGN KEY
	// bilan bog'langan. Busiz restoranni o'chirish urinishi FK
	// buzilishiga tushib, superadminga tushunarsiz 500 xatosi qaytarardi
	// (aksiyasi bor restoranni umuman o'chirib bo'lmasdi). Mongo/memory
	// backend'larida FK yo'q, lekin u yerda ham aksiyalar mavjud
	// bo'lmagan restoranga ishora qilib "yetim" bo'lib qolardi.
	DeleteByRestaurant(ctx context.Context, restaurantID string) (int, error)
	// IncrementUsage — checkout'da shu aksiya haqiqatan qo'llanilganda
	// bitta marta chaqiriladi (UsageCount+1, SalesTotalTiyin+amountTiyin).
	IncrementUsage(ctx context.Context, id string, amountTiyin int64) error
}
