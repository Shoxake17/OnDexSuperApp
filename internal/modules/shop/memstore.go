package shop

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

// MemRepo — xotiradagi ombor (dev/e2e). `Tx` bitta qulf ostida; ichidagi
// amallar qulfni qayta olmaydi (`locked` nusxa orqali).
type MemRepo struct {
	mu     *sync.Mutex
	locked bool
	d      *memData
}

type memData struct {
	products   map[string]map[string]Product
	base       map[string]BaseProduct
	storeCats  map[string]map[string]bool
	typeCats   map[string]map[string]bool
	sales      map[string]map[string]memSale
	lines      map[string][]SaleLine
	debts      map[string]map[string]Debt
	pharm      map[string]map[string]PharmProduct
	pharmBase  map[string]PharmBase
	pharmSales map[string][]PharmSale
}

type memSale struct {
	Sale
	day time.Time
}

// NewMemRepo — bo'sh ombor; `base` — global shtrix-kod bazasi (ixtiyoriy).
func NewMemRepo(base ...BaseProduct) *MemRepo {
	d := &memData{products: map[string]map[string]Product{}, base: map[string]BaseProduct{},
		storeCats: map[string]map[string]bool{}, typeCats: map[string]map[string]bool{},
		sales: map[string]map[string]memSale{}, lines: map[string][]SaleLine{}, debts: map[string]map[string]Debt{},
		pharm: map[string]map[string]PharmProduct{}, pharmBase: map[string]PharmBase{}, pharmSales: map[string][]PharmSale{}}
	for _, b := range base {
		d.base[b.Barcode] = b
	}
	return &MemRepo{mu: &sync.Mutex{}, d: d}
}

func (m *MemRepo) lock() func() {
	if m.locked {
		return func() {}
	}
	m.mu.Lock()
	return m.mu.Unlock
}

// Tx — bitta qulf ostida, ma'lumot NUSXASI ustida ishlaydi va faqat muvaffaqiyatda
// almashtiradi: xato bo'lsa hech narsa o'zgarmaydi (Postgres tranzaksiyasi kabi).
// Apteka sotuvida qoldiq yetmasa butun chek bekor bo'lishi shunga tayanadi.
func (m *MemRepo) Tx(ctx context.Context, fn func(r Repo) error) error {
	unlock := m.lock()
	defer unlock()
	cp := m.d.clone()
	inner := &MemRepo{mu: m.mu, locked: true, d: cp}
	if err := fn(inner); err != nil {
		return err
	}
	*m.d = *cp
	return nil
}

func (d *memData) clone() *memData {
	c := &memData{products: map[string]map[string]Product{}, base: d.base,
		storeCats: map[string]map[string]bool{}, typeCats: map[string]map[string]bool{},
		sales: map[string]map[string]memSale{}, lines: map[string][]SaleLine{}, debts: map[string]map[string]Debt{},
		pharm: map[string]map[string]PharmProduct{}, pharmBase: map[string]PharmBase{}, pharmSales: map[string][]PharmSale{}}
	for k, v := range d.products {
		c.products[k] = map[string]Product{}
		for kk, vv := range v {
			c.products[k][kk] = vv
		}
	}
	for k, v := range d.storeCats {
		c.storeCats[k] = map[string]bool{}
		for kk := range v {
			c.storeCats[k][kk] = true
		}
	}
	for k, v := range d.typeCats {
		c.typeCats[k] = map[string]bool{}
		for kk := range v {
			c.typeCats[k][kk] = true
		}
	}
	for k, v := range d.sales {
		c.sales[k] = map[string]memSale{}
		for kk, vv := range v {
			c.sales[k][kk] = vv
		}
	}
	for k, v := range d.lines {
		c.lines[k] = append([]SaleLine(nil), v...)
	}
	for k, v := range d.debts {
		c.debts[k] = map[string]Debt{}
		for kk, vv := range v {
			c.debts[k][kk] = vv
		}
	}
	for k, v := range d.pharm {
		c.pharm[k] = map[string]PharmProduct{}
		for kk, vv := range v {
			c.pharm[k][kk] = vv
		}
	}
	for k, v := range d.pharmBase {
		c.pharmBase[k] = v
	}
	for k, v := range d.pharmSales {
		c.pharmSales[k] = append([]PharmSale(nil), v...)
	}
	return c
}

func match(q string, fields ...string) bool {
	if q == "" {
		return true
	}
	q = strings.ToLower(q)
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), q) {
			return true
		}
	}
	return false
}

// ── Mahsulotlar ──

func (m *MemRepo) GetProduct(_ context.Context, storeID, barcode string) (*Product, error) {
	defer m.lock()()
	p, ok := m.d.products[storeID][barcode]
	if !ok {
		return nil, ErrNotFound
	}
	return &p, nil
}

func (m *MemRepo) FindProductByName(_ context.Context, storeID, name string) (*Product, error) {
	defer m.lock()()
	for _, p := range sortedProducts(m.d.products[storeID]) {
		if match(name, p.Name) {
			return &p, nil
		}
	}
	return nil, ErrNotFound
}

func sortedProducts(in map[string]Product) []Product {
	out := make([]Product, 0, len(in))
	for _, p := range in {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (m *MemRepo) ListProducts(_ context.Context, storeID string) ([]Product, error) {
	defer m.lock()()
	return sortedProducts(m.d.products[storeID]), nil
}

func (m *MemRepo) SearchProducts(_ context.Context, storeID, q, category string, limit int) ([]Product, error) {
	defer m.lock()()
	var out []Product
	for _, p := range sortedProducts(m.d.products[storeID]) {
		if (category == "" || p.Category == category) && match(q, p.Name, p.Barcode) {
			out = append(out, p)
			if len(out) == limit {
				break
			}
		}
	}
	return out, nil
}

func (m *MemRepo) SaveProduct(_ context.Context, storeID string, p *Product) error {
	defer m.lock()()
	if m.d.products[storeID] == nil {
		m.d.products[storeID] = map[string]Product{}
	}
	m.d.products[storeID][p.Barcode] = *p
	return nil
}

func (m *MemRepo) DeleteProduct(_ context.Context, storeID, barcode string) (bool, error) {
	defer m.lock()()
	if _, ok := m.d.products[storeID][barcode]; !ok {
		return false, nil
	}
	delete(m.d.products[storeID], barcode)
	return true, nil
}

// ── Global baza ──

func sortedBase(in map[string]BaseProduct) []BaseProduct {
	out := make([]BaseProduct, 0, len(in))
	for _, b := range in {
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (m *MemRepo) GetBase(_ context.Context, barcode string) (*BaseProduct, error) {
	defer m.lock()()
	b, ok := m.d.base[barcode]
	if !ok {
		return nil, ErrNotFound
	}
	return &b, nil
}

func (m *MemRepo) FindBaseByName(_ context.Context, name string) (*BaseProduct, error) {
	defer m.lock()()
	for _, b := range sortedBase(m.d.base) {
		if match(name, b.Name) {
			return &b, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MemRepo) SearchBase(_ context.Context, q, category string, limit int) ([]BaseProduct, error) {
	defer m.lock()()
	var out []BaseProduct
	for _, b := range sortedBase(m.d.base) {
		if (category == "" || b.Category == category) && match(q, b.Name, b.Barcode) {
			out = append(out, b)
			if len(out) == limit {
				break
			}
		}
	}
	return out, nil
}

func (m *MemRepo) CountBase(context.Context) (int, error) {
	defer m.lock()()
	return len(m.d.base), nil
}

func (m *MemRepo) ListBase(_ context.Context, offset, limit int) ([]BaseProduct, error) {
	defer m.lock()()
	all := sortedBase(m.d.base)
	if offset >= len(all) {
		return nil, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], nil
}

// ── Kategoriyalar ──

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (m *MemRepo) StoreCategories(_ context.Context, storeID string) ([]string, error) {
	defer m.lock()()
	return keys(m.d.storeCats[storeID]), nil
}

func (m *MemRepo) TypeCategories(_ context.Context, shopType string) ([]string, error) {
	defer m.lock()()
	return keys(m.d.typeCats[shopType]), nil
}

func (m *MemRepo) AddCategory(_ context.Context, storeID, shopType, name string) error {
	defer m.lock()()
	if m.d.storeCats[storeID] == nil {
		m.d.storeCats[storeID] = map[string]bool{}
	}
	m.d.storeCats[storeID][name] = true
	if shopType != "" {
		if m.d.typeCats[shopType] == nil {
			m.d.typeCats[shopType] = map[string]bool{}
		}
		m.d.typeCats[shopType][name] = true
	}
	return nil
}

// ── Sotuvlar ──

func saleKey(barcode, payment string, day time.Time) string {
	return barcode + "|" + payment + "|" + day.Format("2006-01-02")
}

func (m *MemRepo) GetDailySale(_ context.Context, storeID, barcode, payment string, day time.Time) (*Sale, error) {
	defer m.lock()()
	s, ok := m.d.sales[storeID][saleKey(barcode, payment, day)]
	if !ok {
		return nil, ErrNotFound
	}
	return &s.Sale, nil
}

func (m *MemRepo) SaveSale(_ context.Context, storeID string, s *Sale, day time.Time) error {
	defer m.lock()()
	if m.d.sales[storeID] == nil {
		m.d.sales[storeID] = map[string]memSale{}
	}
	m.d.sales[storeID][saleKey(s.Barcode, s.PaymentMethod, day)] = memSale{Sale: *s, day: day}
	return nil
}

func (m *MemRepo) AddSaleLine(_ context.Context, storeID string, l SaleLine) error {
	defer m.lock()()
	m.d.lines[storeID] = append(m.d.lines[storeID], l)
	return nil
}

func within(t time.Time, from, to *time.Time) bool {
	return (from == nil || !t.Before(*from)) && (to == nil || !t.After(*to))
}

func (m *MemRepo) ListSales(_ context.Context, storeID string, from, to *time.Time) ([]Sale, error) {
	defer m.lock()()
	var out []Sale
	for _, s := range m.d.sales[storeID] {
		if within(s.SoldAt, from, to) {
			out = append(out, s.Sale)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SoldAt.After(out[j].SoldAt) })
	return out, nil
}

func (m *MemRepo) DeleteSale(_ context.Context, storeID, id string) (bool, error) {
	defer m.lock()()
	for k, s := range m.d.sales[storeID] {
		if s.ID == id {
			delete(m.d.sales[storeID], k)
			return true, nil
		}
	}
	return false, nil
}

func (m *MemRepo) ListSaleLines(_ context.Context, storeID string, from, to *time.Time) ([]SaleLine, error) {
	defer m.lock()()
	var out []SaleLine
	for _, l := range m.d.lines[storeID] {
		if within(l.SoldAt, from, to) {
			out = append(out, l)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].SoldAt.After(out[j].SoldAt) })
	return out, nil
}

// ── Qarzlar ──

func (m *MemRepo) ListDebts(_ context.Context, storeID string) ([]Debt, error) {
	defer m.lock()()
	var out []Debt
	for _, d := range m.d.debts[storeID] {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastPayment.After(out[j].LastPayment) })
	return out, nil
}

func (m *MemRepo) GetDebt(_ context.Context, storeID, id string) (*Debt, error) {
	defer m.lock()()
	d, ok := m.d.debts[storeID][id]
	if !ok {
		return nil, ErrNotFound
	}
	return &d, nil
}

func (m *MemRepo) SaveDebt(_ context.Context, storeID string, d *Debt) error {
	defer m.lock()()
	if m.d.debts[storeID] == nil {
		m.d.debts[storeID] = map[string]Debt{}
	}
	m.d.debts[storeID][d.ID] = *d
	return nil
}

func (m *MemRepo) DeleteDebt(_ context.Context, storeID, id string) error {
	defer m.lock()()
	delete(m.d.debts[storeID], id)
	return nil
}

// ── Apteka ──

func sortedPharm(in map[string]PharmProduct) []PharmProduct {
	out := make([]PharmProduct, 0, len(in))
	for _, p := range in {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (m *MemRepo) GetPharm(_ context.Context, storeID, barcode string) (*PharmProduct, error) {
	defer m.lock()()
	p, ok := m.d.pharm[storeID][barcode]
	if !ok {
		return nil, ErrNotFound
	}
	return &p, nil
}

func (m *MemRepo) ListPharm(_ context.Context, storeID string) ([]PharmProduct, error) {
	defer m.lock()()
	return sortedPharm(m.d.pharm[storeID]), nil
}

func (m *MemRepo) SearchPharm(_ context.Context, storeID, q, category string, limit int) ([]PharmProduct, error) {
	defer m.lock()()
	var out []PharmProduct
	for _, p := range sortedPharm(m.d.pharm[storeID]) {
		if (category == "" || p.Category == category) && match(q, p.Name, p.Barcode) {
			out = append(out, p)
			if len(out) == limit {
				break
			}
		}
	}
	return out, nil
}

func (m *MemRepo) FindPharmInfo(_ context.Context, storeID, q string) (*PharmProduct, error) {
	defer m.lock()()
	for _, p := range sortedPharm(m.d.pharm[storeID]) {
		if match(q, p.Barcode, p.Name, p.BatchNumber) {
			return &p, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MemRepo) SavePharm(_ context.Context, storeID string, p *PharmProduct) error {
	defer m.lock()()
	if m.d.pharm[storeID] == nil {
		m.d.pharm[storeID] = map[string]PharmProduct{}
	}
	m.d.pharm[storeID][p.Barcode] = *p
	return nil
}

func (m *MemRepo) DeletePharm(_ context.Context, storeID, barcode string) (bool, error) {
	defer m.lock()()
	if _, ok := m.d.pharm[storeID][barcode]; !ok {
		return false, nil
	}
	delete(m.d.pharm[storeID], barcode)
	return true, nil
}

func (m *MemRepo) SearchPharmBase(_ context.Context, q, category string, limit int) ([]PharmBase, error) {
	defer m.lock()()
	all := make([]PharmBase, 0, len(m.d.pharmBase))
	for _, b := range m.d.pharmBase {
		all = append(all, b)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })
	var out []PharmBase
	for _, b := range all {
		if (category == "" || b.Category == category) && match(q, b.Name, b.Barcode) {
			out = append(out, b)
			if len(out) == limit {
				break
			}
		}
	}
	return out, nil
}

func (m *MemRepo) CountPharmBase(context.Context) (int, error) {
	defer m.lock()()
	return len(m.d.pharmBase), nil
}

func (m *MemRepo) ListPharmBase(ctx context.Context) ([]PharmBase, error) {
	return m.SearchPharmBase(ctx, "", "", 1<<30)
}

func (m *MemRepo) AddPharmBase(_ context.Context, b PharmBase, _ string) error {
	defer m.lock()()
	if _, ok := m.d.pharmBase[b.Barcode]; ok {
		return ErrConflict
	}
	m.d.pharmBase[b.Barcode] = b
	return nil
}

func (m *MemRepo) SavePharmSale(_ context.Context, storeID string, s *PharmSale) error {
	defer m.lock()()
	m.d.pharmSales[storeID] = append(m.d.pharmSales[storeID], *s)
	return nil
}

func (m *MemRepo) ListPharmSales(_ context.Context, storeID string, limit int) ([]PharmSale, error) {
	defer m.lock()()
	out := append([]PharmSale(nil), m.d.pharmSales[storeID]...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].SoldAt.After(out[j].SoldAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *MemRepo) PharmSaleTotals(_ context.Context, storeID string) (int, int64, error) {
	defer m.lock()()
	var sum int64
	for _, s := range m.d.pharmSales[storeID] {
		sum += s.TotalTiyin
	}
	return len(m.d.pharmSales[storeID]), sum, nil
}
