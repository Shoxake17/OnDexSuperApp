package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"chustapp/internal/orders"
)

// ─── Muassasa turi ─────────────────────────────────────────────────────

// RestaurantKind — muassasa turi ("Kafe", "Choyxona"...). Faqat OnDex
// administratori belgilaydi: u mijoz ilovasidagi toifalash va shartnoma
// shartlariga bog'liq, restoran o'zi o'zgartira olmaydi.
//
// Yopiq ro'yxat: erkin matn bo'lsa "Kafe", "kafe", "Kafeteriy" uch xil
// toifa bo'lib ketardi. Bo'sh qiymat — hali belgilanmagan.
type RestaurantKind string

const (
	KindRestaurant RestaurantKind = "restaurant"
	KindCafe       RestaurantKind = "cafe"
	KindCanteen    RestaurantKind = "canteen"
	KindTeahouse   RestaurantKind = "teahouse"
	KindFastFood   RestaurantKind = "fast_food"
	KindCoffeeShop RestaurantKind = "coffee_shop"
	KindBakery     RestaurantKind = "bakery"
)

type RestaurantKindInfo struct {
	Kind  RestaurantKind `json:"kind"`
	Title string         `json:"title"`
}

var restaurantKinds = []RestaurantKindInfo{
	{KindRestaurant, "Restoran"},
	{KindCafe, "Kafe"},
	{KindCanteen, "Oshxona"},
	{KindTeahouse, "Choyxona"},
	{KindFastFood, "Fast food"},
	{KindCoffeeShop, "Qahvaxona"},
	{KindBakery, "Qandolatxona"},
}

var ErrUnknownRestaurantKind = errors.New("muassasa turi noto'g'ri")

// RestaurantKinds — nusxa: chaqiruvchi umumiy ro'yxatni o'zgartira olmasin.
func RestaurantKinds() []RestaurantKindInfo { return slices.Clone(restaurantKinds) }

// ParseRestaurantKind — bo'sh qiymat "belgilanmagan".
func ParseRestaurantKind(s string) (RestaurantKind, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "", nil
	}
	for _, k := range restaurantKinds {
		if string(k.Kind) == s {
			return k.Kind, nil
		}
	}
	return "", ErrUnknownRestaurantKind
}

// Title — o'zbekcha nom; belgilanmagan yoki noma'lum tur uchun bo'sh.
func (k RestaurantKind) Title() string {
	for _, info := range restaurantKinds {
		if info.Kind == k {
			return info.Title
		}
	}
	return ""
}

// ─── Tavsif ────────────────────────────────────────────────────────────

// MaxDescriptionLen — tavsif uzunligi (belgi, bayt emas).
const MaxDescriptionLen = 500

var (
	ErrDescriptionTooLong = fmt.Errorf("tavsif %d belgidan oshmasligi kerak", MaxDescriptionLen)
	ErrDescriptionChars   = errors.New("tavsifda ko'rinmas boshqaruv belgilari bo'lishi mumkin emas")
)

// NormalizeDescription — mijozga ko'rinadigan tavsif.
//
// Yangi qator va tab ruxsat; qolgan boshqaruv belgilari va matn
// yo'nalishini o'zgartiruvchi belgilar (U+202E...) RAD ETILADI: ular bilan
// ilovadagi matnni teskari ko'rsatib, aldovchi yozuv yasash mumkin.
// Emoji ichidagi "zero width joiner" (U+200D) istisno — u 👨‍🍳 kabi
// belgilarning bir qismi.
func NormalizeDescription(s string) (string, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
	for _, r := range s {
		if r == '\n' || r == '\t' || r == '‍' {
			continue
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return "", ErrDescriptionChars
		}
	}
	if utf8.RuneCountInString(s) > MaxDescriptionLen {
		return "", ErrDescriptionTooLong
	}
	return s, nil
}

// ─── To'lov usullari ───────────────────────────────────────────────────

// PaymentMethods — restoran qabul qiladigan to'lov usullari.
//
//   - Cash — naqd pul (kuryerga yoki ofitsiantga);
//   - CardTerminal — joyida karta orqali, terminalda (buyurtma tizimda
//     "naqd" kabi — to'lov qabul qilishda — yaratiladi, bu belgi mijozga
//     "terminal bor" degan ma'lumot);
//   - CardOnline — oldindan onlayn karta to'lovi (Octo).
//
// Click/Payme hozircha tizimga ulanmagan (mijoz ilovasida "Tez orada"),
// shuning uchun bu yerda ular uchun maydon ATAYLAB yo'q: yoqib qo'yiladigan,
// lekin ishlamaydigan sozlama yolg'on bo'lardi.
//
// ┌─ OnDex Wallet ────────────────────────────────────────────────────────┐
// Platforma hamyoni (keyinchalik keshbek bilan to'lash uchun) HAR BIR
// restoranda DOIM yoqilgan — bu restoran sozlamasi emas, platforma qoidasi.
// Shuning uchun u bazaga YOZILMAYDI (`bson:"-"`) va JSON'da har doim
// `true` chiqadi (`MarshalJSON`): bazadagi hech qanday qiymat uni o'chira
// olmaydi. PATCH'da `false` yuborilsa rad etiladi.
//
// `Validate` uni "kamida bitta usul" hisobiga QO'SHMAYDI: hamyon orqali
// to'lov hali ishga tushmagan, aks holda restoran barcha haqiqiy usullarni
// o'chirib, buyurtma qabul qilolmay qolardi.
// └───────────────────────────────────────────────────────────────────────┘
type PaymentMethods struct {
	Cash         bool `json:"cash" bson:"cash"`
	CardTerminal bool `json:"card_terminal" bson:"card_terminal"`
	CardOnline   bool `json:"card_online" bson:"card_online"`
	OnDexWallet  bool `json:"ondex_wallet" bson:"-"`
}

var ErrWalletAlwaysOn = errors.New("OnDex Wallet doim yoqilgan — uni o'chirib bo'lmaydi")

// MarshalJSON — `ondex_wallet` har doim true.
func (p PaymentMethods) MarshalJSON() ([]byte, error) {
	type plain PaymentMethods
	p.OnDexWallet = true
	return json.Marshal(plain(p))
}

var (
	ErrNoPaymentMethod    = errors.New("kamida bitta to'lov usuli yoqilgan bo'lishi kerak")
	ErrPaymentNotAccepted = errors.New("bu restoran tanlangan to'lov usulini qabul qilmaydi")
)

// DefaultPaymentMethods — to'lov usullarini sozlamagan restoran uchun.
// Aynan sozlamalar paydo bo'lishidan oldingi xatti-harakat: naqd va onlayn
// karta. Mavjud restoranlarda hech narsa o'zgarmaydi.
func DefaultPaymentMethods() PaymentMethods {
	return PaymentMethods{Cash: true, CardOnline: true}
}

func (p PaymentMethods) Validate() error {
	if !p.Cash && !p.CardTerminal && !p.CardOnline {
		return ErrNoPaymentMethod
	}
	return nil
}

// EffectivePaymentMethods — saqlangan yoki standart qiymat (hamyon doim yoqiq).
func (r *Restaurant) EffectivePaymentMethods() PaymentMethods {
	p := DefaultPaymentMethods()
	if r.PaymentMethods != nil {
		p = *r.PaymentMethods
	}
	p.OnDexWallet = true
	return p
}

// AcceptsPayment — buyurtmaning to'lov usuli shu restoranda qabul qilinadimi.
// "Naqd" buyurtmasi — to'lov QABUL QILISHDA: naqd yoki terminal karta.
func (r *Restaurant) AcceptsPayment(m orders.PaymentMethod) bool {
	p := r.EffectivePaymentMethods()
	switch m {
	case orders.PaymentCard:
		return p.CardOnline
	case orders.PaymentWallet:
		// OnDexWallet HAR DOIM true (`EffectivePaymentMethods`) — bu
		// restoran sozlamasi emas, platforma qoidasi.
		return p.OnDexWallet
	case orders.PaymentCash, "":
		return p.Cash || p.CardTerminal
	}
	return false
}
