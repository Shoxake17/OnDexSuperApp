package main

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ── ShoxPos (Mongo) hujjatlari — faqat ko'chiriladigan maydonlar ──

type mUser struct {
	Login          string   `bson:"login"`
	EntityID       string   `bson:"ondexEntityId"`
	StoreCategory  string   `bson:"ondexStoreCategory"`
	PartnerRole    string   `bson:"ondexPartnerRole"`
	ShopType       string   `bson:"shopType"`
	ShopCategories []string `bson:"shopCategories"`
	IsDeleted      bool     `bson:"isDeleted"`
}

type mProduct struct {
	Login            string    `bson:"login"`
	Barcode          string    `bson:"barcode"`
	Name             string    `bson:"name"`
	Category         string    `bson:"category"`
	Price            float64   `bson:"price"`
	BasePrice        float64   `bson:"basePrice"`
	Quantity         float64   `bson:"quantity"`
	IncomingQuantity float64   `bson:"incomingQuantity"`
	Sold             float64   `bson:"sold"`
	AddedAt          time.Time `bson:"addedAt"`
	UpdatedAt        time.Time `bson:"updatedAt"`
	ImageURL         string    `bson:"imageUrl"`
	Description      string    `bson:"description"`
	Composition      string    `bson:"composition"`
	WeightGrams      float64   `bson:"weightGrams"`
}

type mBase struct {
	Barcode   string  `bson:"barcode"`
	Name      string  `bson:"name"`
	Category  string  `bson:"category"`
	Price     float64 `bson:"price"`
	BasePrice float64 `bson:"basePrice"`
}

type mSale struct {
	ID            string    `bson:"-"`
	Login         string    `bson:"login"`
	Barcode       string    `bson:"barcode"`
	Name          string    `bson:"name"`
	Quantity      float64   `bson:"quantity"`
	Price         float64   `bson:"price"`
	BasePrice     float64   `bson:"basePrice"`
	Total         float64   `bson:"total"`
	Category      string    `bson:"category"`
	PaymentMethod string    `bson:"paymentMethod"`
	SoldAt        time.Time `bson:"soldAt"`
}

// bucketLine — SalesBucket.data (gzip JSON) ichidagi qator.
type bucketLine struct {
	Barcode       string    `json:"barcode"`
	Name          string    `json:"name"`
	Quantity      float64   `json:"quantity"`
	Price         float64   `json:"price"`
	BasePrice     float64   `json:"basePrice"`
	Category      string    `json:"category"`
	Total         float64   `json:"total"`
	PaymentMethod string    `json:"paymentMethod"`
	SoldAt        time.Time `json:"soldAt"`
}

type mDebt struct {
	ID          string    `bson:"-"`
	Login       string    `bson:"login"`
	Name        string    `bson:"name"`
	Phone       string    `bson:"phone"`
	Amount      float64   `bson:"amount"`
	LastPayment time.Time `bson:"lastPayment"`
}

type mPharm struct {
	Login                  string     `bson:"login"`
	Barcode                string     `bson:"barcode"`
	Name                   string     `bson:"name"`
	Category               string     `bson:"category"`
	Price                  float64    `bson:"price"`
	BasePrice              float64    `bson:"basePrice"`
	Quantity               float64    `bson:"quantity"`
	IncomingQuantity       float64    `bson:"incomingQuantity"`
	Sold                   float64    `bson:"sold"`
	AddedAt                time.Time  `bson:"addedAt"`
	UpdatedAt              time.Time  `bson:"updatedAt"`
	Dosage                 string     `bson:"dosage"`
	Manufacturer           string     `bson:"manufacturer"`
	ExpiryDate             *time.Time `bson:"expiryDate"`
	BatchNumber            string     `bson:"batchNumber"`
	IsPrescriptionRequired bool       `bson:"isPrescriptionRequired"`
	StorageCondition       string     `bson:"storageCondition"`
	Description            string     `bson:"description"`
	DosageForm             string     `bson:"dosageForm"`
	ShelfNumber            string     `bson:"shelfNumber"`
	RowNumber              string     `bson:"rowNumber"`
}

type mPharmBase struct {
	Barcode                string  `bson:"barcode"`
	Name                   string  `bson:"name"`
	Category               string  `bson:"category"`
	BasePrice              float64 `bson:"basePrice"`
	Dosage                 string  `bson:"dosage"`
	Manufacturer           string  `bson:"manufacturer"`
	StorageCondition       string  `bson:"storageCondition"`
	SideEffects            string  `bson:"sideEffects"`
	Description            string  `bson:"description"`
	Composition            string  `bson:"composition"`
	DosageForm             string  `bson:"dosageForm"`
	IsPrescriptionRequired bool    `bson:"isPrescriptionRequired"`
	AddedBy                string  `bson:"addedBy"`
}

type mPharmSaleItem struct {
	Barcode      string  `bson:"barcode"`
	Name         string  `bson:"name"`
	Quantity     float64 `bson:"quantity"`
	Price        float64 `bson:"price"`
	BasePrice    float64 `bson:"basePrice"`
	Total        float64 `bson:"total"`
	Dosage       string  `bson:"dosage"`
	Manufacturer string  `bson:"manufacturer"`
}

type mPharmSale struct {
	ID            string           `bson:"-"`
	Login         string           `bson:"login"`
	Items         []mPharmSaleItem `bson:"items"`
	TotalAmount   float64          `bson:"totalAmount"`
	PaymentMethod string           `bson:"paymentMethod"`
	SoldAt        time.Time        `bson:"soldAt"`
}

type mLedger struct {
	OrderID string `bson:"orderId"`
	Login   string `bson:"login"`
	Items   []struct {
		Barcode string `bson:"barcode"`
		Qty     int    `bson:"qty"`
	} `bson:"items"`
	State string `bson:"state"`
}

// ── Konvertatsiya ──

var barcodeRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// tiyin — so'm (ehtimol kasr) -> tiyin; manfiy/NaN -> 0.
func tiyin(som float64) int64 {
	if math.IsNaN(som) || math.IsInf(som, 0) || som <= 0 {
		return 0
	}
	return int64(math.Round(som * 100))
}

// qty — miqdor 3 xonagacha; manfiy -> 0.
func qty(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return 0
	}
	return math.Round(v*1000) / 1000
}

func cutRunes(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

func orNow(t, now time.Time) time.Time {
	if t.IsZero() {
		return now
	}
	return t
}

// storeOf — ShoxPos login -> OnDex do'kon (entity_id, kategoriya). Faqat OnDex'ga
// bog'langan, o'chirilmagan do'kon akkauntlari ko'chiriladi.
type storeOf struct {
	ID       string
	Category string
	ShopType string
}

func buildStores(users []mUser) (map[string]storeOf, []string) {
	out := map[string]storeOf{}
	var skipped []string
	for _, u := range users {
		login := strings.ToLower(strings.TrimSpace(u.Login))
		switch {
		case login == "":
			continue
		case u.IsDeleted:
			skipped = append(skipped, login+": o'chirilgan")
		case u.EntityID == "":
			skipped = append(skipped, login+": OnDex'ga bog'lanmagan (ondexEntityId yo'q)")
		case u.PartnerRole != "" && u.PartnerRole != "store":
			skipped = append(skipped, login+": do'kon emas ("+u.PartnerRole+")")
		default:
			cat := u.StoreCategory
			if cat == "" {
				cat = "store"
			}
			st := map[string]string{"qurt": "qurt", "store": "dokon", "pharmacy": "dorixona"}[cat]
			out[login] = storeOf{ID: u.EntityID, Category: cat, ShopType: st}
		}
	}
	sort.Strings(skipped)
	return out, skipped
}

// saleKey — kunlik yig'indi kaliti (Postgres UNIQUE bilan bir xil).
type saleKey struct {
	Store, Barcode, Payment string
	Day                     string // YYYY-MM-DD, Toshkent
}

type saleAgg struct {
	ID         string
	Name       string
	Category   string
	Quantity   float64
	Price      int64
	Base       int64
	Total      int64
	LastSoldAt time.Time
}

// aggregateSales — ShoxPos'da bir kun+to'lov turi uchun bir nechta yozuv bo'lishi
// mumkin (oflayn sinxron). Postgres'da bitta yig'indi: miqdor va jami QO'SHILADI,
// narx va nom — eng oxirgi sotuvdan (ShoxPos ham shunday yangilardi). Natija
// deterministik: takroriy ko'chirish bir xil qiymat yozadi.
func aggregateSales(sales []mSale, stores map[string]storeOf, loc *time.Location) (map[saleKey]*saleAgg, int) {
	out := map[saleKey]*saleAgg{}
	skipped := 0
	sort.SliceStable(sales, func(i, j int) bool { return sales[i].SoldAt.Before(sales[j].SoldAt) })
	for _, s := range sales {
		st, ok := stores[strings.ToLower(s.Login)]
		if !ok || !barcodeRe.MatchString(s.Barcode) || s.SoldAt.IsZero() {
			skipped++
			continue
		}
		pay := cutRunes(s.PaymentMethod, 32)
		if pay == "" {
			pay = "cash"
		}
		k := saleKey{Store: st.ID, Barcode: s.Barcode, Payment: pay, Day: s.SoldAt.In(loc).Format("2006-01-02")}
		a := out[k]
		if a == nil {
			a = &saleAgg{ID: s.ID}
			out[k] = a
		}
		a.Quantity = math.Round((a.Quantity+qty(s.Quantity))*1000) / 1000
		total := tiyin(s.Total)
		if total == 0 {
			total = int64(math.Round(float64(tiyin(s.Price)) * qty(s.Quantity)))
		}
		a.Total += total
		// Nom va kategoriya — oxirgi BO'SH BO'LMAGAN qiymat.
		if n := cutRunes(s.Name, 300); n != "" {
			a.Name = n
		}
		if c := cutRunes(s.Category, 64); c != "" {
			a.Category = c
		}
		a.Price = tiyin(s.Price)
		if b := tiyin(s.BasePrice); b > 0 {
			a.Base = b
		}
		a.LastSoldAt = s.SoldAt
	}
	for k, a := range out {
		if a.Name == "" {
			a.Name = k.Barcode
		}
	}
	return out, skipped
}

// pharmPayment — ShoxPos enum (naqd|plastik|qarz); boshqasi naqd.
func pharmPayment(p string) string {
	switch p {
	case "naqd", "plastik", "qarz":
		return p
	}
	return "naqd"
}

// report — bitta to'plam bo'yicha hisob.
type report struct {
	Name     string
	Read     int
	Written  int
	Skipped  int
	Examples []string
}

func (r *report) skip(why string) {
	r.Skipped++
	if len(r.Examples) < 5 {
		r.Examples = append(r.Examples, why)
	}
}

func (r report) String() string {
	s := fmt.Sprintf("  %-22s o'qildi %7d  yoziladi %7d  o'tkazildi %5d", r.Name, r.Read, r.Written, r.Skipped)
	for _, e := range r.Examples {
		s += "\n      - " + e
	}
	return s
}
