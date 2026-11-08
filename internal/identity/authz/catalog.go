// Package authz — merchant realmi ruxsatlari (AUTH-PHASE-A.md 6-bo'lim).
//
// ┌─ ASOSIY QOIDALAR ─────────────────────────────────────────────────┐
//   - Ruxsatlar katalogi FAQAT shu faylda (kodda). Bazadagi rol faqat
//     katalogdagi satrlarni saqlaydi; noma'lum ruxsat yozishda RAD etiladi,
//     o'qishda e'tiborsiz qoldiriladi (eski/yaroqsiz satr huquq bermaydi).
//   - Rol va ruxsat tokenga YOZILMAYDI — har so'rovda `Authorizer` bazadan
//     (30 s kesh) tekshiradi: bo'shatilgan xodim ko'pi bilan 30 s ishlaydi.
//   - Begona biznes = 404 (borligi oshkor bo'lmaydi), o'z biznesida ruxsat
//     yo'q = 403.
//   - Xavfli ruxsatlarni (Dangerous) faqat EGASI beradi.
//
// └───────────────────────────────────────────────────────────────────┘
package authz

import (
	"slices"
	"sort"
)

// Permission — `resource.action` ko'rinishidagi ruxsat.
type Permission string

// BusinessType — `businesses.type` va `roles.business_type` CHECK bilan bir xil.
type BusinessType string

const (
	TypeRestaurant BusinessType = "restaurant"
	TypeStore      BusinessType = "store"
	TypeClub       BusinessType = "club"
	TypeTaxi       BusinessType = "taxi"
	TypeHotel      BusinessType = "hotel"
)

// BusinessTypes — barcha turlar (tartib: interfeys ro'yxati).
func BusinessTypes() []BusinessType {
	return []BusinessType{TypeRestaurant, TypeStore, TypeClub, TypeTaxi, TypeHotel}
}

func (t BusinessType) Valid() bool { return slices.Contains(BusinessTypes(), t) }

// Har biznes turida bo'ladigan umumiy ruxsatlar.
const (
	PermBusinessView     Permission = "business.view"
	PermBusinessSettings Permission = "business.settings"
	PermStaffView        Permission = "staff.view"
	PermStaffManage      Permission = "staff.manage"
	PermReportsView      Permission = "reports.view"
	PermAuditView        Permission = "audit.view"
	PermPayoutsManage    Permission = "payouts.manage"
	PermRefundsCreate    Permission = "refunds.create"
	PermBillingManage    Permission = "billing.manage"
)

// Biznes turiga xos ruxsatlar.
const (
	PermMenuView       Permission = "menu.view"
	PermMenuManage     Permission = "menu.manage"
	PermOrdersView     Permission = "orders.view"
	PermOrdersManage   Permission = "orders.manage"
	PermTablesView     Permission = "tables.view"
	PermTablesManage   Permission = "tables.manage"
	PermDineInServe    Permission = "dinein.serve"
	PermKitchenView    Permission = "kitchen.view"
	PermCouriersManage Permission = "couriers.manage"
	PermDeliveryRun    Permission = "delivery.run"
	PermPOSSell        Permission = "pos.sell"

	PermProductsView    Permission = "products.view"
	PermProductsManage  Permission = "products.manage"
	PermInventoryView   Permission = "inventory.view"
	PermInventoryManage Permission = "inventory.manage"
	PermSalesView       Permission = "sales.view"
	PermDebtsView       Permission = "debts.view"
	PermDebtsManage     Permission = "debts.manage"
	PermLogisticsManage Permission = "logistics.manage"

	PermDevicesView   Permission = "devices.view"
	PermDevicesManage Permission = "devices.manage"

	PermRidesDispatch Permission = "rides.dispatch"
	PermRidesDrive    Permission = "rides.drive"

	PermRoomsView       Permission = "rooms.view"
	PermRoomsManage     Permission = "rooms.manage"
	PermBookingsView    Permission = "bookings.view"
	PermBookingsManage  Permission = "bookings.manage"
	PermHousekeepingRun Permission = "housekeeping.run"
	PermFolioManage     Permission = "folio.manage"
)

// PermInfo — katalog yozuvi (interfeys: "Rol yaratish" oynasidagi belgilar).
type PermInfo struct {
	Key       Permission `json:"key"`
	Title     string     `json:"title"`
	Dangerous bool       `json:"dangerous"`
}

var common = []PermInfo{
	{PermBusinessView, "Biznesni ko'rish", false},
	{PermBusinessSettings, "Biznes sozlamalari", false},
	{PermStaffView, "Xodimlar ro'yxatini ko'rish", false},
	{PermStaffManage, "Xodimlarni boshqarish (qo'shish, rol berish, bo'shatish)", true},
	{PermReportsView, "Hisobotlar", false},
	{PermAuditView, "Harakatlar jurnali", false},
	{PermPayoutsManage, "Pul yechish va to'lov rekvizitlari", true},
	{PermRefundsCreate, "Pulni qaytarish", true},
	{PermBillingManage, "Obuna va hisob-kitob", true},
}

var byType = map[BusinessType][]PermInfo{
	TypeRestaurant: {
		{PermMenuView, "Menyuni ko'rish", false},
		{PermMenuManage, "Menyuni tahrirlash", false},
		{PermOrdersView, "Buyurtmalarni ko'rish", false},
		{PermOrdersManage, "Buyurtmalarni boshqarish", false},
		{PermTablesView, "Stollarni ko'rish", false},
		{PermTablesManage, "Stollar va QR kodlar", false},
		{PermDineInServe, "Stolga xizmat (ofitsiant)", false},
		{PermKitchenView, "Oshxona ekrani", false},
		{PermCouriersManage, "Kuryerlarni boshqarish", false},
		{PermDeliveryRun, "Buyurtma yetkazish (kuryer)", false},
		{PermPOSSell, "Kassa", false},
	},
	TypeStore: {
		{PermProductsView, "Mahsulotlarni ko'rish", false},
		{PermProductsManage, "Mahsulotlarni tahrirlash", false},
		{PermInventoryView, "Qoldiqni ko'rish", false},
		{PermInventoryManage, "Kirim va qoldiq", false},
		{PermPOSSell, "Kassa (sotish)", false},
		{PermSalesView, "Sotuvlar tarixi", false},
		{PermDebtsView, "Qarzlarni ko'rish", false},
		{PermDebtsManage, "Qarzlarni boshqarish", false},
		{PermOrdersView, "Onlayn buyurtmalarni ko'rish", false},
		{PermOrdersManage, "Onlayn buyurtmalarni boshqarish", false},
		{PermLogisticsManage, "Logistika", false},
		{PermDeliveryRun, "Buyurtma yetkazish (kuryer)", false},
	},
	TypeClub: {
		{PermDevicesView, "Qurilmalar holati", false},
		{PermDevicesManage, "Qurilmalar va seanslar", false},
		{PermPOSSell, "Kassa", false},
	},
	TypeTaxi: {
		{PermRidesDispatch, "Dispetcherlik", false},
		{PermRidesDrive, "Haydovchi", false},
	},
	TypeHotel: {
		{PermRoomsView, "Xonalarni ko'rish", false},
		{PermRoomsManage, "Xonalar va narxlar", false},
		{PermBookingsView, "Bronlarni ko'rish", false},
		{PermBookingsManage, "Bronlarni boshqarish", false},
		{PermHousekeepingRun, "Tozalash vazifalari", false},
		{PermFolioManage, "Hisob (folio) va to'lovlar", false},
	},
}

// Catalog — tur uchun barcha ruxsatlar (nusxa).
func Catalog(t BusinessType) []PermInfo {
	if !t.Valid() {
		return nil
	}
	return slices.Concat(common, byType[t])
}

func lookup(t BusinessType, p Permission) (PermInfo, bool) {
	for _, x := range Catalog(t) {
		if x.Key == p {
			return x, true
		}
	}
	return PermInfo{}, false
}

// Known — ruxsat shu tur katalogida bormi.
func Known(t BusinessType, p Permission) bool { _, ok := lookup(t, p); return ok }

// IsDangerous — faqat egasi bera oladigan ruxsat.
func IsDangerous(t BusinessType, p Permission) bool {
	x, ok := lookup(t, p)
	return ok && x.Dangerous
}

// Template — tizim rol shabloni (`roles.org_id IS NULL`).
type Template struct {
	Type        BusinessType
	Key         string
	Name        string
	Permissions []Permission
	Apps        []string // shu rol odatda qaysi ilovalarga kiradi (taklif oynasida standart)
}

// TemplateID — barqaror ID: qayta yoqilganda yangilanadi, ko'paymaydi.
func (t Template) ID() string { return "tpl_" + string(t.Type) + "_" + t.Key }

var templates = []Template{
	{TypeRestaurant, "manager", "Menejer", []Permission{PermBusinessView, PermBusinessSettings, PermStaffView,
		PermReportsView, PermMenuView, PermMenuManage, PermOrdersView, PermOrdersManage, PermTablesView,
		PermTablesManage, PermKitchenView, PermCouriersManage, PermPOSSell}, []string{AppPanel}},
	{TypeRestaurant, "cashier", "Kassir", []Permission{PermBusinessView, PermOrdersView, PermOrdersManage,
		PermTablesView, PermPOSSell}, []string{AppPanel}},
	{TypeRestaurant, "waiter", "Ofitsiant", []Permission{PermMenuView, PermOrdersView, PermTablesView,
		PermDineInServe}, []string{AppPro}},
	{TypeRestaurant, "cook", "Oshpaz", []Permission{PermMenuView, PermOrdersView, PermKitchenView}, []string{AppPro}},
	{TypeRestaurant, "courier", "Kuryer", []Permission{PermDeliveryRun}, []string{AppGo}},

	{TypeStore, "cashier", "Kassir", []Permission{PermBusinessView, PermProductsView, PermInventoryView,
		PermPOSSell, PermSalesView, PermDebtsView, PermDebtsManage, PermOrdersView, PermOrdersManage}, []string{AppPanel}},
	{TypeStore, "stockkeeper", "Omborchi", []Permission{PermBusinessView, PermProductsView, PermProductsManage,
		PermInventoryView, PermInventoryManage}, []string{AppPanel}},
	{TypeStore, "courier", "Kuryer", []Permission{PermDeliveryRun}, []string{AppGo}},

	{TypeClub, "operator", "Operator", []Permission{PermBusinessView, PermDevicesView, PermDevicesManage,
		PermPOSSell}, []string{AppPanel}},

	{TypeTaxi, "dispatcher", "Dispetcher", []Permission{PermBusinessView, PermRidesDispatch}, []string{AppPanel}},
	{TypeTaxi, "driver", "Haydovchi", []Permission{PermRidesDrive}, []string{AppGo}},

	{TypeHotel, "reception", "Resepshn", []Permission{PermBusinessView, PermRoomsView, PermBookingsView,
		PermBookingsManage, PermFolioManage}, []string{AppPanel}},
	{TypeHotel, "housekeeping", "Housekeeping", []Permission{PermRoomsView, PermHousekeepingRun}, []string{AppPro}},
	{TypeHotel, "accountant", "Buxgalter", []Permission{PermBusinessView, PermBookingsView, PermFolioManage,
		PermReportsView}, []string{AppPanel}},
}

// Templates — barcha shablonlar (nusxa).
func Templates() []Template {
	out := make([]Template, len(templates))
	for i, t := range templates {
		t.Permissions = slices.Clone(t.Permissions)
		t.Apps = slices.Clone(t.Apps)
		out[i] = t
	}
	return out
}

// Ilovalar (`memberships.apps` CHECK bilan bir xil).
const (
	AppPanel = "panel" // Merchant panel (web + desktop)
	AppPro   = "pro"   // OnDexPro
	AppGo    = "go"    // OnDexGO
)

// ValidApp — ilova nomi ma'lummi.
func ValidApp(a string) bool { return a == AppPanel || a == AppPro || a == AppGo }

// AppForClient — OIDC klientidan ilova. Noma'lum klient — "" (hech qaysi
// a'zolik uni ruxsat etmaydi).
func AppForClient(clientID string) string {
	switch clientID {
	// ondex-sso — OnDex kirish sahifasi: undan faqat panel klientlariga
	// (merchant-web) kod beriladi, shuning uchun panel ruxsati talab qilinadi.
	case "merchant-web", "merchant-desktop", "ondex-sso":
		return AppPanel
	case "ondex-pro":
		return AppPro
	case "ondex-go":
		return AppGo
	}
	return ""
}

func sortPerms(p []Permission) []Permission {
	sort.Slice(p, func(i, j int) bool { return p[i] < p[j] })
	return p
}
