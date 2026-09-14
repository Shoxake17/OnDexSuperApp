package tables

import (
	"errors"
	"slices"
	"strings"
)

// Kind — joy turi: stol, kabina, VIP xona, topchan...
//
// ┌─ NEGA ERKIN MATN EMAS, YOPIQ RO'YXAT ─────────────────────────────┐
// Bitta tizim restoran, kafe, oshxona va choyxonaga xizmat qiladi.
// Tur erkin matn bo'lsa, bir restoranda "Kabina", "kabina", "Kabinka"
// uchta alohida toifaga aylanib, filtr va hisobotlar sochilib ketardi;
// panel esa har biri uchun qanday belgi chizishni bilmasdi.
//
// Yopiq ro'yxat uch joyda bir xil turadi va test bilan qulflangan
// (`kind_test.go`): shu fayl, Postgres CHECK cheklovi (migration 0044)
// va restoran paneli (`lib/pages/tables/table_models.dart`). Yangi tur
// qo'shish = uchalasiga qo'shish; bittasi unutilsa test yiqiladi.
//
// Zal/hudud ("Ayvon", "2-qavat") bu yerda EMAS — u erkin matn (`Zone`),
// chunki har bir binoning tuzilishi o'ziga xos.
// └───────────────────────────────────────────────────────────────────┘
type Kind string

const (
	KindTable       Kind = "table"
	KindCabin       Kind = "cabin"
	KindVIPRoom     Kind = "vip_room"
	KindTapchan     Kind = "tapchan"
	KindBarCounter  Kind = "bar_counter"
	KindLounge      Kind = "lounge"
	KindBanquetHall Kind = "banquet_hall"
)

// KindInfo — panelga beriladigan tavsif.
type KindInfo struct {
	Kind  Kind   `json:"kind"`
	Title string `json:"title"`
}

var kinds = []KindInfo{
	{KindTable, "Stol"},
	{KindCabin, "Kabina"},
	{KindVIPRoom, "VIP xona"},
	{KindTapchan, "Topchan"},
	{KindBarCounter, "Bar stoyka"},
	{KindLounge, "Lounge"},
	{KindBanquetHall, "Banket zali"},
}

var ErrUnknownKind = errors.New("joy turi noto'g'ri")

// Kinds — barcha turlar, paneldagi tartibda. Nusxa qaytadi: chaqiruvchi
// umumiy ro'yxatni o'zgartira olmasin.
func Kinds() []KindInfo { return slices.Clone(kinds) }

// ParseKind — bo'sh qiymat `KindTable` (eski mijozlar turni yubormaydi).
func ParseKind(s string) (Kind, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return KindTable, nil
	}
	for _, k := range kinds {
		if string(k.Kind) == s {
			return k.Kind, nil
		}
	}
	return "", ErrUnknownKind
}

// Normalized — bazadagi bo'sh qiymat (migratsiyadan oldingi qator yoki
// xotira omboridagi nol qiymat) `KindTable` deb o'qiladi.
func (k Kind) Normalized() Kind {
	if k == "" {
		return KindTable
	}
	return k
}

// Title — o'zbekcha nom ("Kabina"). Noma'lum tur uchun "Joy".
func (k Kind) Title() string {
	k = k.Normalized()
	for _, info := range kinds {
		if info.Kind == k {
			return info.Title
		}
	}
	return "Joy"
}
