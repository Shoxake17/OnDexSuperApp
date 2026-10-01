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
//   - CardOnline — oldindan onlayn "Bank kartasi" (shlyuzning umumiy
//     to'lov sahifasi: Uzcard, Humo, Visa, Mastercard);
//   - Payme, Click, Uzum — mijoz AYNAN shu ilova orqali oldindan to'laydi.
//
// ┌─ HAR BIR ONLAYN USUL ALOHIDA YOQILADI ────────────────────────────────┐
// Restoran qaysi usulni yoqsa, FAQAT o'sha mijozga ko'rinadi: Uzum
// o'chirilgan bo'lsa mijoz ilovasida Uzum tugmasi umuman chiqmaydi va
// server ham unga to'lov ochishni rad etadi (`AcceptsChannel`).
//
// Payme/Click/Uzum ham buyurtma bo'yicha baribir `card` (onlayn to'lov):
// ular alohida to'lov USULI emas, to'lov KANALI. Shuning uchun buyurtma
// holat mashinasi, statistika va hisobotlar o'zgarmaydi.
//
// Yangi restoranda ular O'CHIQ (`DefaultPaymentMethods`): restoran
// ongli ravishda yoqadi, aks holda u shartnomasi bo'lmagan usulni
// mijozga taklif qilib qo'yardi.
// └───────────────────────────────────────────────────────────────────────┘
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
	Payme        bool `json:"payme" bson:"payme"`
	Click        bool `json:"click" bson:"click"`
	Uzum         bool `json:"uzum" bson:"uzum"`
	OnDexWallet  bool `json:"ondex_wallet" bson:"-"`
}

// AnyOnline — oldindan onlayn to'lashning kamida bitta yo'li yoqilganmi.
func (p PaymentMethods) AnyOnline() bool {
	return p.CardOnline || p.Payme || p.Click || p.Uzum
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
	if !p.Cash && !p.CardTerminal && !p.AnyOnline() {
		return ErrNoPaymentMethod
	}
	return nil
}

// ─── To'lov shlyuzi do'koni ────────────────────────────────────────────

// MaxStoreIDLen — do'kon identifikatorining eng katta uzunligi.
// Multicard `id` (son) yoki `uuid` qabul qiladi; 64 belgi ikkalasiga
// ham yetadi.
const MaxStoreIDLen = 64

var ErrStoreIDBad = errors.New(
	"do'kon identifikatori noto'g'ri — faqat raqam, harf, chiziqcha va pastki chiziq")

// ValidateStoreID — bo'sh (ulanmagan) yoki xavfsiz belgilardan iborat.
//
// ┌─ NEGA FILTR KERAK ────────────────────────────────────────────────┐
// Bu qiymat Multicard so'rovining YO'L qismiga tushadi
// (`/payment/store/{id}/history`). Klient uni `url.PathEscape` bilan
// o'raydi, lekin ikkinchi qatlam bo'lgani ma'qul: kutilmagan belgi
// bazaga ham tushmasin va logda ham chalkashlik yaratmasin.
// └───────────────────────────────────────────────────────────────────┘
func ValidateStoreID(id string) error {
	if id == "" {
		return nil // ulanmagan holat
	}
	if len(id) > MaxStoreIDLen {
		return ErrStoreIDBad
	}
	for _, r := range id {
		ok := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') || r == '-' || r == '_'
		if !ok {
			return ErrStoreIDBad
		}
	}
	return nil
}

// MaxAppIDLen / MaxSecretLen — restoranning O'Z shlyuz kalitlari uchun
// chegaralar.
//
// Chegara MA'LUMOTGA qarab emas, HIMOYAGA qarab qo'yilgan: kalitlar
// bazaga yoziladi va shifrlanadi, ya'ni cheksiz uzun qiymat xotira va
// hujjat hajmini behuda o'stirardi. Sinov kaliti 20 belgi, `rhmt_test`
// 9 belgi — 256 hamma real holatga yetadi.
const (
	MaxAppIDLen  = 128
	MaxSecretLen = 256
)

var ErrAppIDBad = errors.New(
	"application_id noto'g'ri — faqat raqam, harf, chiziqcha va pastki chiziq")

// ValidateAppID — restoranning `application_id` si.
//
// `ValidateStoreID` bilan bir xil belgilar to'plami, lekin BO'SH
// qiymat qabul QILINMAYDI: bu yerda u "ulanmagan" degani emas, chunki
// ulash so'rovida kalit majburiy.
func ValidateAppID(id string) error {
	if id == "" || len(id) > MaxAppIDLen {
		return ErrAppIDBad
	}
	for _, r := range id {
		ok := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') || r == '-' || r == '_'
		if !ok {
			return ErrAppIDBad
		}
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

// Onlayn to'lov kanallari (`/orders/{id}/pay?ps=`). Bo'sh kanal —
// shlyuzning umumiy sahifasi ("Bank kartasi").
const (
	ChannelPayme = "payme"
	ChannelClick = "click"
	ChannelUzum  = "uzum"
)

// AcceptsChannel — restoran shu onlayn kanal orqali to'lovni qabul
// qiladimi.
//
// ┌─ SOZLAMA SERVERDA TEKSHIRILADI ───────────────────────────────────┐
// Panelda o'chirilgan usul faqat mijoz ilovasida yashirilsa, u bezak
// bo'lib qolardi: eski ilova yoki to'g'ridan-to'g'ri so'rov o'chirilgan
// kanal bilan to'lov ocha olardi. Shuning uchun `/pay` shu yerdan o'tadi.
//
// Noma'lum kanal HECH QACHON qabul qilinmaydi (jimgina umumiy sahifaga
// aylantirilmaydi).
// └───────────────────────────────────────────────────────────────────┘
func (r *Restaurant) AcceptsChannel(channel string) bool {
	p := r.EffectivePaymentMethods()
	switch channel {
	case "":
		return p.CardOnline
	case ChannelPayme:
		return p.Payme
	case ChannelClick:
		return p.Click
	case ChannelUzum:
		return p.Uzum
	}
	return false
}

// PaymentOptions — MIJOZ ILOVASIGA ko'rinadigan to'lov usullari.
//
// ┌─ NEGA `PaymentMethods` DAN ALOHIDA ───────────────────────────────┐
// `PaymentMethods` — restoran SOZLAMASI ("men nimani yoqdim").
// `PaymentOptions` — mijoz HOZIR nimani tanlay oladi: sozlama va
// to'lov shlyuziga ULANGANLIK birgalikda. Restoran Uzum'ni yoqib
// qo'ygan, lekin Rahmat hisobini hali ulamagan bo'lsa, mijozga Uzum
// ko'rsatilmaydi — aks holda u tanlab, buyurtma bergach xatoga uchrardi.
//
// Hisoblanadigan maydon: bazaga YOZILMAYDI (`computedFields` testi).
//
// ┌─ KESH BILAN ──────────────────────────────────────────────────────┐
// `Restaurant.MulticardStoreID` `json:"-"` (sir emas, lekin mijozga
// chiqmasin), shuning uchun Redis keshidan o'qilgan nusxada u BO'SH.
// "Ulangan"lik keshdan keyin hisoblansa, hamma restoran ulanmagan
// ko'rinardi va mijozga onlayn usullar chiqmasdi. Shu sababli ro'yxat
// keshga yozilishidan OLDIN `PaymentOptions` hisoblanadi
// (`routes_catalog.go`), `WithPaymentOptions` esa tayyor qiymatni
// o'zgartirmaydi.
// └───────────────────────────────────────────────────────────────────┘
type PaymentOptions struct {
	Cash  bool `json:"cash"`
	Card  bool `json:"card"`
	Payme bool `json:"payme"`
	Click bool `json:"click"`
	Uzum  bool `json:"uzum"`
}

// Connected — restoran to'lov shlyuzida kassaga ulanganmi.
func (r *Restaurant) Connected() bool { return r != nil && r.MulticardStoreID != "" }

// WithPaymentOptions — mijozga beriladigan nusxaga `PaymentOptions` ni
// qo'shadi.
func (r Restaurant) WithPaymentOptions() Restaurant {
	if r.PaymentOptions != nil {
		return r // keshga yozishdan oldin hisoblangan (`PaymentOptions` izohi)
	}
	p := r.EffectivePaymentMethods()
	online := r.Connected()
	r.PaymentOptions = &PaymentOptions{
		Cash:  p.Cash || p.CardTerminal,
		Card:  p.CardOnline && online,
		Payme: p.Payme && online,
		Click: p.Click && online,
		Uzum:  p.Uzum && online,
	}
	return r
}

// AcceptsPayment — buyurtmaning to'lov usuli shu restoranda qabul qilinadimi.
// "Naqd" buyurtmasi — to'lov QABUL QILISHDA: naqd yoki terminal karta.
func (r *Restaurant) AcceptsPayment(m orders.PaymentMethod) bool {
	p := r.EffectivePaymentMethods()
	switch m {
	case orders.PaymentCard:
		// "Karta" buyurtmasi — oldindan onlayn to'lov. Qaysi kanal
		// (umumiy sahifa, Payme, Click, Uzum) bilan to'lanishi
		// to'lov boshlanganda tekshiriladi (`AcceptsChannel`).
		return p.AnyOnline()
	case orders.PaymentWallet:
		// OnDexWallet HAR DOIM true (`EffectivePaymentMethods`) — bu
		// restoran sozlamasi emas, platforma qoidasi.
		return p.OnDexWallet
	case orders.PaymentCash, "":
		return p.Cash || p.CardTerminal
	}
	return false
}
