package shop

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Service — OnDex POS biznes qoidalari (ShoxPos bilan 1:1).
type Service struct {
	repo  Repo
	newID func() string
	now   func() time.Time
	loc   *time.Location
	// OnQurtChange — qurt do'koni mahsuloti o'zgarganda (mijoz ilovasidagi qurt
	// katalogi yangilanadi). ShoxPos'da bu HTTP sinxronlash (qurtSync) edi.
	OnQurtChange func(ctx context.Context, storeID string, p *Product, deleted bool)
}

// NewService — `loc` kunlik sotuv yig'indisi kuni uchun (Asia/Tashkent).
func NewService(repo Repo, newID func() string, loc *time.Location) *Service {
	if loc == nil {
		loc = time.UTC
	}
	return &Service{repo: repo, newID: newID, now: time.Now, loc: loc}
}

// WithClock — testlar uchun.
func (s *Service) WithClock(now func() time.Time) *Service { s.now = now; return s }

var barcodeRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func cleanBarcode(v string) (string, error) {
	v = strings.TrimSpace(v)
	if !barcodeRe.MatchString(v) {
		return "", fmt.Errorf("%w: shtrix-kod formati noto'g'ri", ErrInvalid)
	}
	return v, nil
}

func cut(v string, max int) string {
	v = strings.TrimSpace(v)
	if r := []rune(v); len(r) > max {
		return string(r[:max])
	}
	return v
}

func nonNeg(v float64, field string) (float64, error) {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return 0, fmt.Errorf("%w: %s musbat son bo'lishi kerak", ErrInvalid, field)
	}
	return math.Round(v*1000) / 1000, nil
}

func nonNegTiyin(v int64, field string) (int64, error) {
	if v < 0 {
		return 0, fmt.Errorf("%w: %s musbat bo'lishi kerak", ErrInvalid, field)
	}
	return v, nil
}

// ── Kategoriya cheklovi (ShoxPos storeAccess) ───────────────────────────

// Section — POS bo'limi; qurt do'koni faqat ruxsat etilganlarini ko'radi.
type Section string

const (
	SecProducts Section = "products"
	SecSales    Section = "sales"
	SecDebts    Section = "debts"
	SecPharmacy Section = "pharmacy"
)

// Allowed — ShoxPos QURT_ALLOW bilan 1:1: qurt — mahsulot, sotuv, qarz;
// apteka bo'limi faqat apteka do'koniga.
func (st Store) Allowed(sec Section) bool {
	switch sec {
	case SecPharmacy:
		return st.Category == CategoryPharmacy
	default:
		return true
	}
}

func (s *Service) need(st Store, sec Section) error {
	if !st.Allowed(sec) {
		return ErrForbidden
	}
	return nil
}

func (s *Service) addCategory(ctx context.Context, r Repo, st Store, category string) error {
	if category == "" {
		return nil
	}
	return r.AddCategory(ctx, st.ID, ShopType(st.Category), category)
}

// ── Mahsulotlar ─────────────────────────────────────────────────────────

// Products — do'konning barcha mahsulotlari (ShoxPos /products/user-products).
func (s *Service) Products(ctx context.Context, st Store) ([]Product, error) {
	if err := s.need(st, SecProducts); err != nil {
		return nil, err
	}
	return s.repo.ListProducts(ctx, st.ID)
}

// Incoming — kirim bazasi: kirim miqdori > 0 (ShoxPos /products/incoming).
func (s *Service) Incoming(ctx context.Context, st Store) ([]Product, error) {
	all, err := s.Products(ctx, st)
	if err != nil {
		return nil, err
	}
	out := make([]Product, 0, len(all))
	for _, p := range all {
		if p.IncomingQuantity > 0 {
			out = append(out, p)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

// Search — do'kon mahsulotlari + global baza (ShoxPos /products/search): do'konda
// bor shtrix-kod globaldan takrorlanmaydi, jami 50 ta.
func (s *Service) Search(ctx context.Context, st Store, q, category string) ([]SearchHit, error) {
	if err := s.need(st, SecProducts); err != nil {
		return nil, err
	}
	q, category = cut(q, 200), cut(category, 64)
	own, err := s.repo.SearchProducts(ctx, st.ID, q, category, 50)
	if err != nil {
		return nil, err
	}
	glob, err := s.repo.SearchBase(ctx, q, category, 50)
	if err != nil {
		return nil, err
	}
	out := make([]SearchHit, 0, 50)
	seen := map[string]bool{}
	for _, p := range own {
		seen[p.Barcode] = true
		out = append(out, SearchHit{Barcode: p.Barcode, Name: p.Name, Category: p.Category, PriceTiyin: p.PriceTiyin,
			BasePriceTiyin: p.BasePriceTiyin, Quantity: p.Quantity, IncomingQuantity: p.IncomingQuantity, Sold: p.Sold})
	}
	for _, g := range glob {
		if !seen[g.Barcode] {
			out = append(out, SearchHit{Barcode: g.Barcode, Name: g.Name, Category: g.Category, PriceTiyin: g.PriceTiyin,
				BasePriceTiyin: g.BasePriceTiyin, IsGlobal: true})
		}
	}
	if len(out) > 50 {
		out = out[:50]
	}
	return out, nil
}

// ScanResult — skaner natijasi: `Source` = "user" (do'konda bor) yoki "global".
type ScanResult struct {
	Product
	Source string `json:"source"`
}

// Scan — ShoxPos scan-product 1:1: avval do'kon mahsuloti, keyin global baza;
// shtrix-kod yoki nom bo'yicha.
func (s *Service) Scan(ctx context.Context, st Store, barcode, name string) (*ScanResult, error) {
	if err := s.need(st, SecProducts); err != nil {
		return nil, err
	}
	barcode, name = strings.TrimSpace(barcode), cut(name, 200)
	if barcode == "" && name == "" {
		return nil, fmt.Errorf("%w: shtrix-kod yoki nom kerak", ErrInvalid)
	}
	var (
		p   *Product
		err error
	)
	if barcode != "" {
		if barcode, err = cleanBarcode(barcode); err != nil {
			return nil, err
		}
		p, err = s.repo.GetProduct(ctx, st.ID, barcode)
	} else {
		p, err = s.repo.FindProductByName(ctx, st.ID, name)
	}
	if err == nil {
		return &ScanResult{Product: *p, Source: "user"}, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	var b *BaseProduct
	if barcode != "" {
		b, err = s.repo.GetBase(ctx, barcode)
	} else {
		b, err = s.repo.FindBaseByName(ctx, name)
	}
	if err != nil {
		return nil, err
	}
	return &ScanResult{Source: "global", Product: Product{Barcode: b.Barcode, Name: b.Name, Category: b.Category,
		PriceTiyin: b.PriceTiyin, BasePriceTiyin: b.BasePriceTiyin}}, nil
}

// Categories — do'kon turi + do'konning o'z kategoriyalari (takrorsiz, alifbo).
func (s *Service) Categories(ctx context.Context, st Store) ([]string, error) {
	typ, err := s.repo.TypeCategories(ctx, ShopType(st.Category))
	if err != nil {
		return nil, err
	}
	own, err := s.repo.StoreCategories(ctx, st.ID)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(typ)+len(own))
	for _, c := range append(typ, own...) {
		if c != "" && !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out, nil
}

// BaseCount — global baza hajmi (ShoxPos /products/count).
func (s *Service) BaseCount(ctx context.Context) (int, error) { return s.repo.CountBase(ctx) }

// UpsertInput — mahsulot qo'shish/tahrirlash (ShoxPos update-or-add-user-product).
// Qurt maydonlari: nil — "yuborilmagan" (mavjud qiymat saqlanadi).
type UpsertInput struct {
	Barcode        string
	Name           string
	Category       string
	PriceTiyin     int64
	BasePriceTiyin int64
	Quantity       float64
	WeightGrams    *int
	Description    *string
	Composition    *string
	ImageURL       *string
}

var errQurtWeight = fmt.Errorf("%w: gramm kiritilishi shart", ErrInvalid)

// Upsert — mahsulotni qo'shadi yoki yangilaydi (ShoxPos bilan 1:1).
func (s *Service) Upsert(ctx context.Context, st Store, in UpsertInput) (*Product, error) {
	if err := s.need(st, SecProducts); err != nil {
		return nil, err
	}
	bc, err := cleanBarcode(in.Barcode)
	if err != nil {
		return nil, err
	}
	name := cut(in.Name, 128)
	if name == "" {
		name = "Nomsiz mahsulot"
	}
	price, err := nonNegTiyin(in.PriceTiyin, "narx")
	if err != nil {
		return nil, err
	}
	base, err := nonNegTiyin(in.BasePriceTiyin, "ulgurji narx")
	if err != nil {
		return nil, err
	}
	qty, err := nonNeg(in.Quantity, "miqdor")
	if err != nil {
		return nil, err
	}
	category := cut(in.Category, 64)
	var out *Product
	err = s.repo.Tx(ctx, func(r Repo) error {
		p, gerr := r.GetProduct(ctx, st.ID, bc)
		if errors.Is(gerr, ErrNotFound) {
			p = &Product{Barcode: bc, AddedAt: s.now()}
		} else if gerr != nil {
			return gerr
		}
		p.Name, p.Category, p.PriceTiyin, p.BasePriceTiyin, p.Quantity = name, category, price, base, qty
		if st.Category == CategoryQurt {
			if err := applyQurtFields(p, in); err != nil {
				return err
			}
		}
		p.UpdatedAt = s.now()
		if err := r.SaveProduct(ctx, st.ID, p); err != nil {
			return err
		}
		out = p
		return s.addCategory(ctx, r, st, category)
	})
	if err != nil {
		return nil, err
	}
	s.qurtChanged(ctx, st, out, false)
	return out, nil
}

// applyQurtFields — qurt do'koni maydonlari; yuborilmagani (nil) saqlanadi,
// gramm esa yangi mahsulotda majburiy.
func applyQurtFields(p *Product, in UpsertInput) error {
	var err error
	if in.WeightGrams != nil {
		if *in.WeightGrams < 1 || *in.WeightGrams > 100000 {
			return fmt.Errorf("%w: gramm 1 dan 100000 gacha bo'lishi kerak", ErrInvalid)
		}
		p.WeightGrams = *in.WeightGrams
	} else if p.WeightGrams == 0 {
		return errQurtWeight
	}
	if in.Description != nil {
		if p.Description, err = longText(*in.Description, "Tavsif"); err != nil {
			return err
		}
	}
	if in.Composition != nil {
		if p.Composition, err = longText(*in.Composition, "Tarkib"); err != nil {
			return err
		}
	}
	if in.ImageURL != nil {
		if p.ImageURL, err = cleanImageURL(*in.ImageURL); err != nil {
			return err
		}
	}
	return nil
}

func longText(v, field string) (string, error) {
	v = strings.TrimSpace(v)
	if len([]rune(v)) > 1000 {
		return "", fmt.Errorf("%w: %s juda uzun (maks 1000 belgi)", ErrInvalid, field)
	}
	return v, nil
}

func cleanImageURL(v string) (string, error) {
	if v == "" {
		return "", nil
	}
	if len(v) > 500 || strings.ContainsAny(v, " \t\r\n") {
		return "", fmt.Errorf("%w: rasm manzili noto'g'ri", ErrInvalid)
	}
	ok := (strings.HasPrefix(v, "https://") && len(v) > len("https://")) ||
		(strings.HasPrefix(v, "/") && !strings.HasPrefix(v, "//") && !strings.HasPrefix(v, `/\`))
	if !ok {
		return "", fmt.Errorf("%w: rasm manzili noto'g'ri", ErrInvalid)
	}
	return v, nil
}

// Field — bitta maydonni yangilash (ShoxPos update-price/base-price/quantity).
type Field string

const (
	FieldPrice     Field = "price"
	FieldBasePrice Field = "base_price"
	FieldQuantity  Field = "quantity"
)

// UpdateField — narx (tiyin), ulgurji narx (tiyin) yoki miqdor.
func (s *Service) UpdateField(ctx context.Context, st Store, barcode string, f Field, value float64) (*Product, error) {
	if err := s.need(st, SecProducts); err != nil {
		return nil, err
	}
	bc, err := cleanBarcode(barcode)
	if err != nil {
		return nil, err
	}
	v, err := nonNeg(value, string(f))
	if err != nil {
		return nil, err
	}
	var out *Product
	err = s.repo.Tx(ctx, func(r Repo) error {
		p, err := r.GetProduct(ctx, st.ID, bc)
		if err != nil {
			return err
		}
		switch f {
		case FieldPrice:
			p.PriceTiyin = int64(math.Round(v))
		case FieldBasePrice:
			p.BasePriceTiyin = int64(math.Round(v))
		case FieldQuantity:
			p.Quantity = v
		default:
			return ErrInvalid
		}
		p.UpdatedAt = s.now()
		out = p
		return r.SaveProduct(ctx, st.ID, p)
	})
	if err != nil {
		return nil, err
	}
	s.qurtChanged(ctx, st, out, false)
	return out, nil
}

// Delete — mahsulotni o'chiradi (ShoxPos delete-product).
func (s *Service) Delete(ctx context.Context, st Store, barcode string) error {
	if err := s.need(st, SecProducts); err != nil {
		return err
	}
	bc, err := cleanBarcode(barcode)
	if err != nil {
		return err
	}
	ok, err := s.repo.DeleteProduct(ctx, st.ID, bc)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	s.qurtChanged(ctx, st, &Product{Barcode: bc}, true)
	return nil
}

func (s *Service) qurtChanged(ctx context.Context, st Store, p *Product, deleted bool) {
	if st.Category == CategoryQurt && s.OnQurtChange != nil && p != nil {
		s.OnQurtChange(context.WithoutCancel(ctx), st.ID, p, deleted)
	}
}

// ── Sotuv ───────────────────────────────────────────────────────────────

// SellLine — kassadagi bitta qator.
type SellLine struct {
	Barcode        string  `json:"barcode"`
	Name           string  `json:"name"`
	Quantity       float64 `json:"quantity"`
	PriceTiyin     int64   `json:"price_tiyin"`
	BasePriceTiyin int64   `json:"base_price_tiyin"`
	TotalTiyin     int64   `json:"total_tiyin"`
	Category       string  `json:"category"`
}

// Sell — kassa sotuvi (ShoxPos sell-product, 1:1):
//   - mahsulot qoldig'i max(0, q - n), sotilgan += n (qoldiq TEKSHIRILMAYDI —
//     kassa sotishni to'xtatmaydi); mahsulot yo'q bo'lsa 0 qoldiq bilan yaratiladi;
//   - kunlik yig'indi: shu kun+to'lov turi bo'yicha bor bo'lsa miqdor qo'shiladi
//     va jami = miqdor * oxirgi narx;
//   - har qator "Sotilganlar" ro'yxatiga yoziladi.
func (s *Service) Sell(ctx context.Context, st Store, lines []SellLine, payment string) ([]Sale, error) {
	if err := s.need(st, SecSales); err != nil {
		return nil, err
	}
	if len(lines) == 0 || len(lines) > 500 {
		return nil, fmt.Errorf("%w: sotuv bo'sh yoki noto'g'ri formatda", ErrInvalid)
	}
	payment = cut(payment, 32)
	if payment == "" {
		payment = "cash"
	}
	now := s.now()
	day := dayOf(now, s.loc)
	var saved []Sale
	err := s.repo.Tx(ctx, func(r Repo) error {
		saved = saved[:0]
		for _, l := range lines {
			sale, err := s.sellOne(ctx, r, st, l, payment, now, day)
			if err != nil {
				return err
			}
			saved = append(saved, *sale)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Qurt do'konida qoldiq mijoz katalogida ham yangilanadi (ShoxPos'da model
	// hook'i har o'zgarishda qurtSync'ni chaqirardi — sotuv ham shunga kiradi).
	if st.Category == CategoryQurt && s.OnQurtChange != nil {
		done := map[string]bool{}
		for _, sl := range saved {
			if done[sl.Barcode] {
				continue
			}
			done[sl.Barcode] = true
			if p, err := s.repo.GetProduct(ctx, st.ID, sl.Barcode); err == nil {
				s.qurtChanged(ctx, st, p, false)
			}
		}
	}
	return saved, nil
}

// QurtProducts — qurt do'konining barcha mahsulotlari (katalogni to'liq qayta
// solishtirish uchun; ShoxPos markAllForResync bilan 1:1).
func (s *Service) QurtProducts(ctx context.Context, storeID string) ([]Product, error) {
	return s.repo.ListProducts(ctx, storeID)
}

// sellOne — bitta kassa qatori: qoldiq, kunlik yig'indi va "Sotilganlar".
func (s *Service) sellOne(ctx context.Context, r Repo, st Store, l SellLine, payment string, now, day time.Time) (*Sale, error) {
	bc, err := cleanBarcode(l.Barcode)
	if err != nil {
		return nil, err
	}
	name := cut(l.Name, 128)
	if name == "" || l.Quantity <= 0 || math.IsNaN(l.Quantity) || math.IsInf(l.Quantity, 0) {
		return nil, fmt.Errorf("%w: noto'g'ri ma'lumot formati", ErrInvalid)
	}
	qty := math.Round(l.Quantity*1000) / 1000
	if l.PriceTiyin < 0 || l.BasePriceTiyin < 0 || l.TotalTiyin < 0 {
		return nil, fmt.Errorf("%w: narx manfiy bo'lishi mumkin emas", ErrInvalid)
	}
	category := cut(l.Category, 64)

	p, gerr := r.GetProduct(ctx, st.ID, bc)
	switch {
	case gerr == nil:
		p.Quantity = math.Max(0, math.Round((p.Quantity-qty)*1000)/1000)
		p.Sold = math.Round((p.Sold+qty)*1000) / 1000
		p.UpdatedAt = now
	case errors.Is(gerr, ErrNotFound):
		p = &Product{Barcode: bc, Name: name, Category: category, PriceTiyin: l.PriceTiyin,
			BasePriceTiyin: l.BasePriceTiyin, Sold: qty, AddedAt: now, UpdatedAt: now}
	default:
		return nil, gerr
	}
	if err := r.SaveProduct(ctx, st.ID, p); err != nil {
		return nil, err
	}
	if err := s.addCategory(ctx, r, st, category); err != nil {
		return nil, err
	}
	basePrice := p.BasePriceTiyin
	if basePrice == 0 {
		basePrice = l.BasePriceTiyin
	}
	total := l.TotalTiyin
	if total == 0 {
		total = int64(math.Round(float64(l.PriceTiyin) * qty))
	}

	sale, gerr := r.GetDailySale(ctx, st.ID, bc, payment, day)
	switch {
	case gerr == nil:
		sale.Quantity = math.Round((sale.Quantity+qty)*1000) / 1000
		sale.TotalTiyin = int64(math.Round(sale.Quantity * float64(l.PriceTiyin)))
		sale.SoldAt = now
	case errors.Is(gerr, ErrNotFound):
		sale = &Sale{ID: s.newID(), Barcode: bc, Name: name, Category: category, Quantity: qty,
			PriceTiyin: l.PriceTiyin, BasePriceTiyin: basePrice, TotalTiyin: total, PaymentMethod: payment, SoldAt: now}
	default:
		return nil, gerr
	}
	if err := r.SaveSale(ctx, st.ID, sale, day); err != nil {
		return nil, err
	}
	if err := r.AddSaleLine(ctx, st.ID, SaleLine{Barcode: bc, Name: name, Category: category, Quantity: qty,
		PriceTiyin: l.PriceTiyin, BasePriceTiyin: basePrice, TotalTiyin: total, PaymentMethod: payment, SoldAt: now}); err != nil {
		return nil, err
	}
	return sale, nil
}

func dayOf(t time.Time, loc *time.Location) time.Time {
	l := t.In(loc)
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, time.UTC)
}

// Sales — kunlik yig'indilar (ShoxPos /sales, yangisi birinchi).
func (s *Service) Sales(ctx context.Context, st Store, from, to *time.Time) ([]Sale, error) {
	if err := s.need(st, SecSales); err != nil {
		return nil, err
	}
	return s.repo.ListSales(ctx, st.ID, from, to)
}

// SoldLines — "Sotilganlar" (ShoxPos sold-buckets): oxirgi 30 kun.
func (s *Service) SoldLines(ctx context.Context, st Store, from, to *time.Time) ([]SaleLine, error) {
	if err := s.need(st, SecSales); err != nil {
		return nil, err
	}
	floor := s.now().Add(-SaleLineRetention)
	if from == nil || from.Before(floor) {
		from = &floor
	}
	return s.repo.ListSaleLines(ctx, st.ID, from, to)
}

// DeleteSale — kunlik yig'indi yozuvini o'chiradi.
func (s *Service) DeleteSale(ctx context.Context, st Store, id string) error {
	if err := s.need(st, SecSales); err != nil {
		return err
	}
	ok, err := s.repo.DeleteSale(ctx, st.ID, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

// ── Qarzlar ─────────────────────────────────────────────────────────────

// Debts — qarzlar ro'yxati (oxirgi to'lov bo'yicha).
func (s *Service) Debts(ctx context.Context, st Store) ([]Debt, error) {
	if err := s.need(st, SecDebts); err != nil {
		return nil, err
	}
	return s.repo.ListDebts(ctx, st.ID)
}

// AddDebt — yangi qarz.
func (s *Service) AddDebt(ctx context.Context, st Store, name, phone string, amountTiyin int64) (*Debt, error) {
	if err := s.need(st, SecDebts); err != nil {
		return nil, err
	}
	name, phone = cut(name, 200), cut(phone, 32)
	if name == "" || phone == "" || amountTiyin <= 0 {
		return nil, fmt.Errorf("%w: ma'lumotlar to'liq emas yoki noto'g'ri", ErrInvalid)
	}
	d := &Debt{ID: s.newID(), Name: name, Phone: phone, AmountTiyin: amountTiyin, LastPayment: s.now()}
	if err := s.repo.SaveDebt(ctx, st.ID, d); err != nil {
		return nil, err
	}
	return d, nil
}

// PayDebt — to'lov: to'liq bo'lsa qarz o'chadi (fully=true), aks holda kamayadi.
func (s *Service) PayDebt(ctx context.Context, st Store, id string, paidTiyin int64) (fully bool, d *Debt, err error) {
	if err := s.need(st, SecDebts); err != nil {
		return false, nil, err
	}
	if paidTiyin <= 0 {
		return false, nil, fmt.Errorf("%w: to'lov ma'lumotlari noto'g'ri", ErrInvalid)
	}
	err = s.repo.Tx(ctx, func(r Repo) error {
		cur, err := r.GetDebt(ctx, st.ID, strings.TrimSpace(id))
		if err != nil {
			return err
		}
		if paidTiyin >= cur.AmountTiyin {
			fully = true
			return r.DeleteDebt(ctx, st.ID, cur.ID)
		}
		cur.AmountTiyin -= paidTiyin
		cur.LastPayment = s.now()
		d = cur
		return r.SaveDebt(ctx, st.ID, cur)
	})
	return fully, d, err
}

// ── Apteka ──────────────────────────────────────────────────────────────

// PharmProducts — aptekadagi dorilar (ShoxPos user-pharm-products / pharm-kirim).
func (s *Service) PharmProducts(ctx context.Context, st Store) ([]PharmProduct, error) {
	if err := s.need(st, SecPharmacy); err != nil {
		return nil, err
	}
	return s.repo.ListPharm(ctx, st.ID)
}

// PharmSearchHit — dori qidiruvi natijasi (do'kon + global).
type PharmSearchHit struct {
	PharmProduct
	IsGlobal bool `json:"is_global"`
}

// PharmSearch — ShoxPos /pharmacy/search (sessiya do'koni + global, 50 ta).
func (s *Service) PharmSearch(ctx context.Context, st Store, q, category string) ([]PharmSearchHit, error) {
	if err := s.need(st, SecPharmacy); err != nil {
		return nil, err
	}
	q, category = cut(q, 200), cut(category, 64)
	own, err := s.repo.SearchPharm(ctx, st.ID, q, category, 50)
	if err != nil {
		return nil, err
	}
	glob, err := s.repo.SearchPharmBase(ctx, q, category, 50)
	if err != nil {
		return nil, err
	}
	out := make([]PharmSearchHit, 0, 50)
	seen := map[string]bool{}
	for _, p := range own {
		seen[p.Barcode] = true
		out = append(out, PharmSearchHit{PharmProduct: p})
	}
	for _, g := range glob {
		if !seen[g.Barcode] {
			out = append(out, PharmSearchHit{IsGlobal: true, PharmProduct: PharmProduct{Barcode: g.Barcode, Name: g.Name,
				Category: g.Category, BasePriceTiyin: g.BasePriceTiyin, Dosage: g.Dosage, Manufacturer: g.Manufacturer,
				StorageCondition: g.StorageCondition, Description: g.Description, DosageForm: g.DosageForm,
				PrescriptionRequired: g.PrescriptionRequired}})
		}
	}
	if len(out) > 50 {
		out = out[:50]
	}
	return out, nil
}

// PharmBaseCount — global dori bazasi hajmi.
func (s *Service) PharmBaseCount(ctx context.Context, st Store) (int, error) {
	if err := s.need(st, SecPharmacy); err != nil {
		return 0, err
	}
	return s.repo.CountPharmBase(ctx)
}

// PharmAddInput — dori qo'shish (ShoxPos add-pharm).
type PharmAddInput struct {
	Barcode, Name, Category                             string
	PriceTiyin, BasePriceTiyin                          int64
	Quantity                                            float64
	Dosage, Manufacturer, BatchNumber, StorageCondition string
	Description, DosageForm, ShelfNumber, RowNumber     string
	ExpiryDate                                          *time.Time
	PrescriptionRequired                                bool
}

// AddPharm — ShoxPos 1:1: maydonlar yoziladi, qoldiq VA kirim += miqdor.
func (s *Service) AddPharm(ctx context.Context, st Store, in PharmAddInput) (*PharmProduct, error) {
	if err := s.need(st, SecPharmacy); err != nil {
		return nil, err
	}
	bc, err := cleanBarcode(in.Barcode)
	if err != nil {
		return nil, err
	}
	name, category := cut(in.Name, 128), cut(in.Category, 64)
	if name == "" || category == "" {
		return nil, fmt.Errorf("%w: majburiy maydonlar to'ldirilmagan", ErrInvalid)
	}
	if in.PriceTiyin < 0 || in.BasePriceTiyin < 0 {
		return nil, fmt.Errorf("%w: narx manfiy bo'lishi mumkin emas", ErrInvalid)
	}
	qty, err := nonNeg(in.Quantity, "miqdor")
	if err != nil {
		return nil, err
	}
	var out *PharmProduct
	err = s.repo.Tx(ctx, func(r Repo) error {
		p, gerr := r.GetPharm(ctx, st.ID, bc)
		if errors.Is(gerr, ErrNotFound) {
			p = &PharmProduct{Barcode: bc, AddedAt: s.now()}
		} else if gerr != nil {
			return gerr
		}
		p.Name, p.Category, p.PriceTiyin, p.BasePriceTiyin = name, category, in.PriceTiyin, in.BasePriceTiyin
		p.Dosage, p.Manufacturer = cut(in.Dosage, 100), cut(in.Manufacturer, 200)
		p.ExpiryDate, p.BatchNumber = in.ExpiryDate, cut(in.BatchNumber, 100)
		p.PrescriptionRequired, p.StorageCondition = in.PrescriptionRequired, cut(in.StorageCondition, 200)
		p.Description, p.DosageForm = cut(in.Description, 2000), cut(in.DosageForm, 100)
		p.ShelfNumber, p.RowNumber = cut(in.ShelfNumber, 50), cut(in.RowNumber, 50)
		p.Quantity = math.Round((p.Quantity+qty)*1000) / 1000
		p.IncomingQuantity = math.Round((p.IncomingQuantity+qty)*1000) / 1000
		p.UpdatedAt = s.now()
		out = p
		if err := r.SavePharm(ctx, st.ID, p); err != nil {
			return err
		}
		return s.addCategory(ctx, r, st, category)
	})
	return out, err
}

// PharmInfoUpdate — ShoxPos update-pharm-product-info ruxsat etilgan maydonlari
// (nil — o'zgarmaydi).
type PharmInfoUpdate struct {
	Name, Category, Dosage, Manufacturer, ShelfNumber, RowNumber, Description *string
	PriceTiyin, BasePriceTiyin                                                *int64
	Quantity                                                                  *float64
	ExpiryDate                                                                *time.Time
	ClearExpiry                                                               bool
}

// UpdatePharmInfo — dori ma'lumotlarini tahrirlash.
func (s *Service) UpdatePharmInfo(ctx context.Context, st Store, barcode string, u PharmInfoUpdate) (*PharmProduct, error) {
	if err := s.need(st, SecPharmacy); err != nil {
		return nil, err
	}
	bc, err := cleanBarcode(barcode)
	if err != nil {
		return nil, err
	}
	var out *PharmProduct
	err = s.repo.Tx(ctx, func(r Repo) error {
		p, err := r.GetPharm(ctx, st.ID, bc)
		if err != nil {
			return err
		}
		set := func(dst *string, v *string, max int) {
			if v != nil {
				*dst = cut(*v, max)
			}
		}
		set(&p.Name, u.Name, 500)
		set(&p.Category, u.Category, 64)
		set(&p.Dosage, u.Dosage, 500)
		set(&p.Manufacturer, u.Manufacturer, 500)
		set(&p.ShelfNumber, u.ShelfNumber, 500)
		set(&p.RowNumber, u.RowNumber, 500)
		set(&p.Description, u.Description, 500)
		if p.Name == "" {
			return fmt.Errorf("%w: nom bo'sh bo'lishi mumkin emas", ErrInvalid)
		}
		if u.PriceTiyin != nil {
			if *u.PriceTiyin < 0 {
				return fmt.Errorf("%w: narx manfiy", ErrInvalid)
			}
			p.PriceTiyin = *u.PriceTiyin
		}
		if u.BasePriceTiyin != nil {
			if *u.BasePriceTiyin < 0 {
				return fmt.Errorf("%w: ulgurji narx manfiy", ErrInvalid)
			}
			p.BasePriceTiyin = *u.BasePriceTiyin
		}
		if u.Quantity != nil {
			q, err := nonNeg(*u.Quantity, "miqdor")
			if err != nil {
				return err
			}
			p.Quantity = q
		}
		if u.ClearExpiry {
			p.ExpiryDate = nil
		} else if u.ExpiryDate != nil {
			p.ExpiryDate = u.ExpiryDate
		}
		p.UpdatedAt = s.now()
		out = p
		return r.SavePharm(ctx, st.ID, p)
	})
	return out, err
}

// UpdatePharmPrice — sotuv narxi (ShoxPos update-pharm-price).
func (s *Service) UpdatePharmPrice(ctx context.Context, st Store, barcode string, priceTiyin int64) (*PharmProduct, error) {
	return s.UpdatePharmInfo(ctx, st, barcode, PharmInfoUpdate{PriceTiyin: &priceTiyin})
}

// DeletePharm — dorini o'chiradi.
func (s *Service) DeletePharm(ctx context.Context, st Store, barcode string) error {
	if err := s.need(st, SecPharmacy); err != nil {
		return err
	}
	bc, err := cleanBarcode(barcode)
	if err != nil {
		return err
	}
	ok, err := s.repo.DeletePharm(ctx, st.ID, bc)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

// PharmInfo — shtrix-kod, nom yoki partiya raqami bo'yicha bitta dori.
func (s *Service) PharmInfo(ctx context.Context, st Store, q string) (*PharmProduct, error) {
	if err := s.need(st, SecPharmacy); err != nil {
		return nil, err
	}
	q = cut(q, 200)
	if q == "" {
		return nil, fmt.Errorf("%w: qidiruv so'rovi kerak", ErrInvalid)
	}
	return s.repo.FindPharmInfo(ctx, st.ID, q)
}

// PharmSellLine — apteka chekidagi qator.
type PharmSellLine struct {
	Barcode    string  `json:"barcode"`
	Name       string  `json:"name"`
	Quantity   float64 `json:"quantity"`
	PriceTiyin int64   `json:"price_tiyin"`
}

// SellPharm — apteka sotuvi: HAR qator uchun qoldiq yetarli bo'lishi SHART;
// hammasi bitta tranzaksiyada (yetmasa hech narsa o'zgarmaydi).
func (s *Service) SellPharm(ctx context.Context, st Store, lines []PharmSellLine, payment string) (*PharmSale, error) {
	if err := s.need(st, SecPharmacy); err != nil {
		return nil, err
	}
	if len(lines) == 0 || len(lines) > 500 {
		return nil, fmt.Errorf("%w: sotuv uchun dori yo'q", ErrInvalid)
	}
	if payment == "" {
		payment = "naqd"
	}
	if !PharmPaymentMethods[payment] {
		return nil, fmt.Errorf("%w: to'lov turi noto'g'ri", ErrInvalid)
	}
	sale := &PharmSale{ID: s.newID(), PaymentMethod: payment, SoldAt: s.now()}
	err := s.repo.Tx(ctx, func(r Repo) error {
		sale.Items, sale.TotalTiyin = nil, 0
		for _, l := range lines {
			bc, err := cleanBarcode(l.Barcode)
			if err != nil {
				return err
			}
			qty, err := nonNeg(l.Quantity, "miqdor")
			if err != nil || qty == 0 {
				return fmt.Errorf("%w: miqdor noto'g'ri", ErrInvalid)
			}
			if l.PriceTiyin < 0 {
				return fmt.Errorf("%w: narx noto'g'ri", ErrInvalid)
			}
			p, err := r.GetPharm(ctx, st.ID, bc)
			if errors.Is(err, ErrNotFound) {
				return fmt.Errorf("%w: %s topilmadi", ErrNotFound, cut(l.Name, 128))
			} else if err != nil {
				return err
			}
			if p.Quantity < qty {
				return fmt.Errorf("%w: %s yetarli emas", ErrInsufficient, p.Name)
			}
			total := int64(math.Round(float64(l.PriceTiyin) * qty))
			sale.TotalTiyin += total
			name := cut(l.Name, 128)
			if name == "" {
				name = p.Name
			}
			sale.Items = append(sale.Items, PharmSaleItem{Barcode: bc, Name: name, Quantity: qty, PriceTiyin: l.PriceTiyin,
				BasePriceTiyin: p.BasePriceTiyin, TotalTiyin: total, Dosage: p.Dosage, Manufacturer: p.Manufacturer})
			p.Quantity = math.Round((p.Quantity-qty)*1000) / 1000
			p.Sold = math.Round((p.Sold+qty)*1000) / 1000
			p.UpdatedAt = s.now()
			if err := r.SavePharm(ctx, st.ID, p); err != nil {
				return err
			}
		}
		return r.SavePharmSale(ctx, st.ID, sale)
	})
	if err != nil {
		return nil, err
	}
	return sale, nil
}

// PharmSales — oxirgi 100 chek.
func (s *Service) PharmSales(ctx context.Context, st Store) ([]PharmSale, error) {
	if err := s.need(st, SecPharmacy); err != nil {
		return nil, err
	}
	return s.repo.ListPharmSales(ctx, st.ID, 100)
}

// PharmStats — cheklar soni va jami tushum (ShoxPos pharm-stats).
func (s *Service) PharmStats(ctx context.Context, st Store) (count int, totalTiyin int64, err error) {
	if err := s.need(st, SecPharmacy); err != nil {
		return 0, 0, err
	}
	return s.repo.PharmSaleTotals(ctx, st.ID)
}

// PharmBaseList — global dori ma'lumotnomasi (faqat apteka).
func (s *Service) PharmBaseList(ctx context.Context, st Store) ([]PharmBase, error) {
	if err := s.need(st, SecPharmacy); err != nil {
		return nil, err
	}
	return s.repo.ListPharmBase(ctx)
}

// AddPharmBase — global ma'lumotnomaga yangi dori (bor shtrix-kod — ErrConflict;
// mavjud yozuvni hech bir do'kon ustidan yozolmaydi).
func (s *Service) AddPharmBase(ctx context.Context, st Store, b PharmBase) (*PharmBase, error) {
	if err := s.need(st, SecPharmacy); err != nil {
		return nil, err
	}
	bc, err := cleanBarcode(b.Barcode)
	if err != nil {
		return nil, err
	}
	b.Barcode, b.Name, b.Category = bc, cut(b.Name, 500), cut(b.Category, 64)
	if b.Name == "" || b.Category == "" || b.BasePriceTiyin < 0 {
		return nil, fmt.Errorf("%w: majburiy maydonlar to'ldirilmagan", ErrInvalid)
	}
	b.Dosage, b.Manufacturer, b.StorageCondition = cut(b.Dosage, 500), cut(b.Manufacturer, 500), cut(b.StorageCondition, 500)
	b.SideEffects, b.Description = cut(b.SideEffects, 2000), cut(b.Description, 2000)
	b.Composition, b.DosageForm = cut(b.Composition, 2000), cut(b.DosageForm, 100)
	if err := s.repo.AddPharmBase(ctx, b, st.ID); err != nil {
		return nil, err
	}
	return &b, nil
}

// QurtProductOf — bitta mahsulot (katalog sinxronini qayta urinish uchun).
func (s *Service) QurtProductOf(ctx context.Context, storeID, barcode string) (*Product, error) {
	return s.repo.GetProduct(ctx, storeID, barcode)
}
