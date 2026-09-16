// Package staff — restoran xodimlari ("Xodimlar" bo'limi).
//
// ┌─ BU PAKET NIMA UCHUN ─────────────────────────────────────────────┐
// Xodim — restoranning ICHKI yozuvi (oshpaz, kassir, tozalovchi...),
// tizim akkaunti EMAS. Ko'p xodimga ilova umuman kerak emas.
//
// Faqat ilovasi bor lavozim (hozircha ofitsiant — "OnDex Affitsiant")
// uchun restoran "ilovaga kirish" ni yoqishi mumkin. Shunda akkaunt
// holati xodim yozuviga ERGASHADI (`Service.syncAccess`): xodim
// ta'tilga chiqsa, ishdan bo'shasa yoki lavozimi o'zgarsa — kirish
// DARHOL yopiladi va sessiyalar bekor qilinadi.
//
// Universal: kafe, restoran, oshxona, choyxona, qahvaxona, fast food —
// hammasida uchraydigan lavozimlar yopiq ro'yxatda. Erkin matn bo'lsa
// "Oshpaz", "oshpaz", "Povar" uch xil lavozim bo'lib ketardi va
// taqsimot diagrammasi ma'nosiz bo'lardi.
// └───────────────────────────────────────────────────────────────────┘
package staff

import (
	"slices"
	"strings"
)

type Position string

const (
	PositionManager       Position = "manager"
	PositionAdministrator Position = "administrator"
	PositionHeadChef      Position = "head_chef"
	PositionChef          Position = "chef"
	PositionCookAssistant Position = "cook_assistant"
	PositionTandoorBaker  Position = "tandoor_baker"
	PositionBaker         Position = "baker"
	PositionWaiter        Position = "waiter"
	PositionBartender     Position = "bartender"
	PositionBarista       Position = "barista"
	PositionHost          Position = "host"
	PositionCashier       Position = "cashier"
	PositionCourier       Position = "courier"
	PositionDishwasher    Position = "dishwasher"
	PositionCleaner       Position = "cleaner"
	PositionHelper        Position = "helper"
	PositionSecurity      Position = "security"
	PositionOther         Position = "other"
)

// Group — lavozimlar guruhi (filtr va hisobot uchun).
type Group string

const (
	GroupManagement Group = "management"
	GroupKitchen    Group = "kitchen"
	GroupHall       Group = "hall"
	GroupCashier    Group = "cashier"
	GroupDelivery   Group = "delivery"
	GroupService    Group = "service"
	GroupOther      Group = "other"
)

var groupTitles = map[Group]string{
	GroupManagement: "Boshqaruv",
	GroupKitchen:    "Oshxona",
	GroupHall:       "Zal xizmati",
	GroupCashier:    "Kassa",
	GroupDelivery:   "Yetkazib berish",
	GroupService:    "Xo'jalik xizmati",
	GroupOther:      "Boshqa",
}

type PositionInfo struct {
	Key        Position `json:"key"`
	Title      string   `json:"title"`
	Group      Group    `json:"group"`
	GroupTitle string   `json:"group_title"`
	// AppAccess — shu lavozim uchun OnDex ilovasi bormi. Faqat shunda
	// "ilovaga kirish" yoqiladi; qolganlarida akkaunt yaratilmaydi.
	AppAccess bool `json:"app_access"`
}

var positions = []PositionInfo{
	{PositionManager, "Menejer", GroupManagement, "", false},
	{PositionAdministrator, "Administrator", GroupManagement, "", false},
	{PositionHeadChef, "Bosh oshpaz", GroupKitchen, "", false},
	{PositionChef, "Oshpaz", GroupKitchen, "", false},
	{PositionCookAssistant, "Oshpaz yordamchisi", GroupKitchen, "", false},
	{PositionTandoorBaker, "Tandirchi", GroupKitchen, "", false},
	{PositionBaker, "Novvoy / Qandolatchi", GroupKitchen, "", false},
	{PositionWaiter, "Ofitsiant", GroupHall, "", true},
	{PositionBartender, "Barmen", GroupHall, "", false},
	{PositionBarista, "Barista", GroupHall, "", false},
	{PositionHost, "Kutib oluvchi (xostes)", GroupHall, "", false},
	{PositionCashier, "Kassir", GroupCashier, "", false},
	// "OnDex Kuryer" ilovasi — restoranning O'Z yetkazib beruvchisi
	// (dispatch taklifni faqat o'z restoranining kuryerlariga yuboradi).
	{PositionCourier, "Yetkazib beruvchi", GroupDelivery, "", true},
	{PositionDishwasher, "Idish yuvuvchi", GroupService, "", false},
	{PositionCleaner, "Tozalovchi", GroupService, "", false},
	{PositionHelper, "Yordamchi", GroupService, "", false},
	{PositionSecurity, "Xavfsizlik xodimi", GroupService, "", false},
	{PositionOther, "Boshqa", GroupOther, "", false},
}

func init() {
	for i := range positions {
		positions[i].GroupTitle = groupTitles[positions[i].Group]
	}
}

// Positions — nusxa: chaqiruvchi umumiy ro'yxatni o'zgartira olmasin.
func Positions() []PositionInfo { return slices.Clone(positions) }

func ParsePosition(s string) (Position, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	for _, p := range positions {
		if string(p.Key) == s {
			return p.Key, nil
		}
	}
	return "", ErrUnknownPosition
}

func (p Position) info() (PositionInfo, bool) {
	for _, x := range positions {
		if x.Key == p {
			return x, true
		}
	}
	return PositionInfo{}, false
}

// Title — o'zbekcha nom; noma'lum kalit uchun kalitning o'zi.
func (p Position) Title() string {
	if x, ok := p.info(); ok {
		return x.Title
	}
	return string(p)
}

func (p Position) AllowsAppAccess() bool {
	x, ok := p.info()
	return ok && x.AppAccess
}
