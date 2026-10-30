package users

import (
	"errors"
	"strings"
)

// StoreCategory — do'kon akkaunti (`RoleStore`) kategoriyasi. Yopiq ro'yxat:
// POS (ShoxPos) paneli shu qiymat bo'yicha qaysi sahifa va API ochiq
// ekanligini hal qiladi, shuning uchun ixtiyoriy matn QABUL QILINMAYDI.
type StoreCategory string

const (
	// StoreCategoryQurt — qurt sotuvchisi: cheklangan panel (mahsulot
	// qo'shish/bazasi, sotuvlar, kirim, statistika, qarzlar, sozlamalar).
	StoreCategoryQurt StoreCategory = "qurt"
	// StoreCategoryStore — oddiy do'kon: to'liq panel.
	StoreCategoryStore StoreCategory = "store"
	// StoreCategoryPharmacy — dorixona: dorixona paneli.
	StoreCategoryPharmacy StoreCategory = "pharmacy"
)

// ErrUnknownStoreCategory — ro'yxatda yo'q kategoriya.
var ErrUnknownStoreCategory = errors.New("do'kon kategoriyasi noto'g'ri: qurt, store yoki pharmacy")

// StoreCategories — admin panelida ko'rsatiladigan tartib.
func StoreCategories() []StoreCategory {
	return []StoreCategory{StoreCategoryQurt, StoreCategoryStore, StoreCategoryPharmacy}
}

// ParseStoreCategory — matnni tekshiradi. Bo'sh qiymat XATO: do'kon yaratishda
// kategoriya majburiy (jimgina "to'liq panel" berib yuborilmasin).
func ParseStoreCategory(s string) (StoreCategory, error) {
	c := StoreCategory(strings.ToLower(strings.TrimSpace(s)))
	for _, k := range StoreCategories() {
		if c == k {
			return c, nil
		}
	}
	return "", ErrUnknownStoreCategory
}

// Title — o'zbekcha nom.
func (c StoreCategory) Title() string {
	switch c {
	case StoreCategoryQurt:
		return "Qurt"
	case StoreCategoryStore:
		return "Do'kon"
	case StoreCategoryPharmacy:
		return "Dorixona"
	}
	return string(c)
}
