package shop

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGRepo — Postgres ombori (`shop` sxemasi, 0081).
//
// Tranzaksiya ichida (`Tx`) mahsulot/dori/qarz `FOR UPDATE` bilan o'qiladi:
// ikki kassa bir vaqtda sotsa ham qoldiq va "sotilgan" yo'qolmaydi, apteka
// chekidagi qoldiq tekshiruvi esa haqiqatan atomik.
type PGRepo struct {
	pool *pgxpool.Pool
	db   querier
	inTx bool
}

type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func NewPGRepo(pool *pgxpool.Pool) *PGRepo { return &PGRepo{pool: pool, db: pool} }

func (p *PGRepo) Tx(ctx context.Context, fn func(r Repo) error) error {
	if p.inTx {
		return fn(p)
	}
	return pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		return fn(&PGRepo{pool: p.pool, db: tx, inTx: true})
	})
}

func (p *PGRepo) lockSuffix() string {
	if p.inTx {
		return " FOR UPDATE"
	}
	return ""
}

// likeArg — `%q%` (LIKE maxsus belgilari ekranlangan): trigram GIN indeksi
// ishlatiladi, `strpos` esa butun jadvalni ko'rib chiqardi.
func likeArg(q string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(q) + "%"
}

func noRows(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// ── Mahsulotlar ──

const productCols = `barcode, name, category, price_tiyin, base_price_tiyin, quantity::float8, incoming_quantity::float8,
	sold::float8, image_url, description, composition, weight_grams, added_at, updated_at`

func scanProduct(row pgx.Row) (*Product, error) {
	var x Product
	err := row.Scan(&x.Barcode, &x.Name, &x.Category, &x.PriceTiyin, &x.BasePriceTiyin, &x.Quantity, &x.IncomingQuantity,
		&x.Sold, &x.ImageURL, &x.Description, &x.Composition, &x.WeightGrams, &x.AddedAt, &x.UpdatedAt)
	if err != nil {
		return nil, noRows(err)
	}
	return &x, nil
}

func collectProducts(rows pgx.Rows, err error) ([]Product, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Product
	for rows.Next() {
		x, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *x)
	}
	return out, rows.Err()
}

func (p *PGRepo) GetProduct(ctx context.Context, storeID, barcode string) (*Product, error) {
	return scanProduct(p.db.QueryRow(ctx, `SELECT `+productCols+` FROM shop.products
		WHERE store_id = $1 AND barcode = $2`+p.lockSuffix(), storeID, barcode))
}

func (p *PGRepo) FindProductByName(ctx context.Context, storeID, name string) (*Product, error) {
	return scanProduct(p.db.QueryRow(ctx, `SELECT `+productCols+` FROM shop.products
		WHERE store_id = $1 AND strpos(lower(name), lower($2)) > 0 ORDER BY name LIMIT 1`, storeID, name))
}

func (p *PGRepo) ListProducts(ctx context.Context, storeID string) ([]Product, error) {
	return collectProducts(p.db.Query(ctx, `SELECT `+productCols+` FROM shop.products
		WHERE store_id = $1 ORDER BY name LIMIT 20000`, storeID))
}

func (p *PGRepo) SearchProducts(ctx context.Context, storeID, q, category string, limit int) ([]Product, error) {
	return collectProducts(p.db.Query(ctx, `SELECT `+productCols+` FROM shop.products
		WHERE store_id = $1 AND ($2 = '' OR strpos(lower(name), lower($2)) > 0 OR strpos(lower(barcode), lower($2)) > 0)
		  AND ($3 = '' OR category = $3)
		ORDER BY name LIMIT $4`, storeID, q, category, limit))
}

func (p *PGRepo) SaveProduct(ctx context.Context, storeID string, x *Product) error {
	_, err := p.db.Exec(ctx, `INSERT INTO shop.products (store_id, `+productInsertCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (store_id, barcode) DO UPDATE SET name=EXCLUDED.name, category=EXCLUDED.category,
		  price_tiyin=EXCLUDED.price_tiyin, base_price_tiyin=EXCLUDED.base_price_tiyin, quantity=EXCLUDED.quantity,
		  incoming_quantity=EXCLUDED.incoming_quantity, sold=EXCLUDED.sold, image_url=EXCLUDED.image_url,
		  description=EXCLUDED.description, composition=EXCLUDED.composition, weight_grams=EXCLUDED.weight_grams,
		  updated_at=EXCLUDED.updated_at`,
		storeID, x.Barcode, x.Name, x.Category, x.PriceTiyin, x.BasePriceTiyin, x.Quantity, x.IncomingQuantity, x.Sold,
		x.ImageURL, x.Description, x.Composition, x.WeightGrams, x.AddedAt, x.UpdatedAt)
	return err
}

const productInsertCols = `barcode, name, category, price_tiyin, base_price_tiyin, quantity, incoming_quantity, sold,
	image_url, description, composition, weight_grams, added_at, updated_at`

func (p *PGRepo) DeleteProduct(ctx context.Context, storeID, barcode string) (bool, error) {
	tag, err := p.db.Exec(ctx, `DELETE FROM shop.products WHERE store_id = $1 AND barcode = $2`, storeID, barcode)
	return tag.RowsAffected() > 0, err
}

// ── Global baza ──

const baseCols = `barcode, name, category, price_tiyin, base_price_tiyin`

func collectBase(rows pgx.Rows, err error) ([]BaseProduct, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BaseProduct
	for rows.Next() {
		var b BaseProduct
		if err := rows.Scan(&b.Barcode, &b.Name, &b.Category, &b.PriceTiyin, &b.BasePriceTiyin); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (p *PGRepo) GetBase(ctx context.Context, barcode string) (*BaseProduct, error) {
	var b BaseProduct
	err := p.db.QueryRow(ctx, `SELECT `+baseCols+` FROM shop.product_base WHERE barcode = $1`, barcode).
		Scan(&b.Barcode, &b.Name, &b.Category, &b.PriceTiyin, &b.BasePriceTiyin)
	if err != nil {
		return nil, noRows(err)
	}
	return &b, nil
}

func (p *PGRepo) FindBaseByName(ctx context.Context, name string) (*BaseProduct, error) {
	list, err := collectBase(p.db.Query(ctx, `SELECT `+baseCols+` FROM shop.product_base
		WHERE lower(name) LIKE lower($1) ORDER BY name LIMIT 1`, likeArg(name)))
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return &list[0], nil
}

func (p *PGRepo) SearchBase(ctx context.Context, q, category string, limit int) ([]BaseProduct, error) {
	return collectBase(p.db.Query(ctx, `SELECT `+baseCols+` FROM shop.product_base
		WHERE ($1 = '%%' OR lower(name) LIKE lower($1) OR barcode LIKE $1)
		  AND ($2 = '' OR category = $2)
		ORDER BY name LIMIT $3`, likeArg(q), category, limit))
}

func (p *PGRepo) CountBase(ctx context.Context) (int, error) {
	var n int
	err := p.db.QueryRow(ctx, `SELECT count(*) FROM shop.product_base`).Scan(&n)
	return n, err
}

func (p *PGRepo) ListBase(ctx context.Context, offset, limit int) ([]BaseProduct, error) {
	return collectBase(p.db.Query(ctx, `SELECT `+baseCols+` FROM shop.product_base ORDER BY name OFFSET $1 LIMIT $2`, offset, limit))
}

// ── Kategoriyalar ──

func collectStrings(rows pgx.Rows, err error) ([]string, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func (p *PGRepo) StoreCategories(ctx context.Context, storeID string) ([]string, error) {
	return collectStrings(p.db.Query(ctx, `SELECT name FROM shop.store_categories WHERE store_id = $1 ORDER BY name`, storeID))
}

func (p *PGRepo) TypeCategories(ctx context.Context, shopType string) ([]string, error) {
	return collectStrings(p.db.Query(ctx, `SELECT name FROM shop.type_categories WHERE shop_type = $1 ORDER BY name`, shopType))
}

func (p *PGRepo) AddCategory(ctx context.Context, storeID, shopType, name string) error {
	if _, err := p.db.Exec(ctx, `INSERT INTO shop.store_categories (store_id, name) VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, storeID, name); err != nil {
		return err
	}
	if shopType == "" {
		return nil
	}
	_, err := p.db.Exec(ctx, `INSERT INTO shop.type_categories (shop_type, name) VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, shopType, name)
	return err
}

// ── Sotuvlar ──

const saleCols = `id, barcode, name, category, quantity::float8, price_tiyin, base_price_tiyin, total_tiyin, payment_method, sold_at`

func scanSale(row pgx.Row) (*Sale, error) {
	var s Sale
	err := row.Scan(&s.ID, &s.Barcode, &s.Name, &s.Category, &s.Quantity, &s.PriceTiyin, &s.BasePriceTiyin,
		&s.TotalTiyin, &s.PaymentMethod, &s.SoldAt)
	if err != nil {
		return nil, noRows(err)
	}
	return &s, nil
}

func (p *PGRepo) GetDailySale(ctx context.Context, storeID, barcode, payment string, day time.Time) (*Sale, error) {
	return scanSale(p.db.QueryRow(ctx, `SELECT `+saleCols+` FROM shop.sales
		WHERE store_id = $1 AND barcode = $2 AND payment_method = $3 AND sale_day = $4`+p.lockSuffix(),
		storeID, barcode, payment, day))
}

func (p *PGRepo) SaveSale(ctx context.Context, storeID string, s *Sale, day time.Time) error {
	_, err := p.db.Exec(ctx, `INSERT INTO shop.sales (id, store_id, barcode, name, category, quantity, price_tiyin,
		  base_price_tiyin, total_tiyin, payment_method, sale_day, sold_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (id) DO UPDATE SET quantity=EXCLUDED.quantity, total_tiyin=EXCLUDED.total_tiyin, sold_at=EXCLUDED.sold_at`,
		s.ID, storeID, s.Barcode, s.Name, s.Category, s.Quantity, s.PriceTiyin, s.BasePriceTiyin, s.TotalTiyin,
		s.PaymentMethod, day, s.SoldAt)
	return err
}

func (p *PGRepo) AddSaleLine(ctx context.Context, storeID string, l SaleLine) error {
	_, err := p.db.Exec(ctx, `INSERT INTO shop.sale_lines (store_id, barcode, name, category, quantity, price_tiyin,
		  base_price_tiyin, total_tiyin, payment_method, sold_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		storeID, l.Barcode, l.Name, l.Category, l.Quantity, l.PriceTiyin, l.BasePriceTiyin, l.TotalTiyin, l.PaymentMethod, l.SoldAt)
	return err
}

func (p *PGRepo) ListSales(ctx context.Context, storeID string, from, to *time.Time) ([]Sale, error) {
	rows, err := p.db.Query(ctx, `SELECT `+saleCols+` FROM shop.sales
		WHERE store_id = $1 AND ($2::timestamptz IS NULL OR sold_at >= $2) AND ($3::timestamptz IS NULL OR sold_at <= $3)
		ORDER BY sold_at DESC LIMIT 20000`, storeID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Sale
	for rows.Next() {
		s, err := scanSale(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

func (p *PGRepo) DeleteSale(ctx context.Context, storeID, id string) (bool, error) {
	tag, err := p.db.Exec(ctx, `DELETE FROM shop.sales WHERE store_id = $1 AND id = $2`, storeID, id)
	return tag.RowsAffected() > 0, err
}

func (p *PGRepo) ListSaleLines(ctx context.Context, storeID string, from, to *time.Time) ([]SaleLine, error) {
	rows, err := p.db.Query(ctx, `SELECT barcode, name, category, quantity::float8, price_tiyin, base_price_tiyin,
		  total_tiyin, payment_method, sold_at FROM shop.sale_lines
		WHERE store_id = $1 AND ($2::timestamptz IS NULL OR sold_at >= $2) AND ($3::timestamptz IS NULL OR sold_at <= $3)
		ORDER BY sold_at DESC, id DESC LIMIT 50000`, storeID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SaleLine
	for rows.Next() {
		var l SaleLine
		if err := rows.Scan(&l.Barcode, &l.Name, &l.Category, &l.Quantity, &l.PriceTiyin, &l.BasePriceTiyin,
			&l.TotalTiyin, &l.PaymentMethod, &l.SoldAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// PurgeSaleLines — saqlash muddatidan eski qatorlar (fon vazifasi chaqiradi).
func (p *PGRepo) PurgeSaleLines(ctx context.Context, before time.Time) (int64, error) {
	tag, err := p.db.Exec(ctx, `DELETE FROM shop.sale_lines WHERE sold_at < $1`, before)
	return tag.RowsAffected(), err
}

// ── Qarzlar ──

func (p *PGRepo) ListDebts(ctx context.Context, storeID string) ([]Debt, error) {
	rows, err := p.db.Query(ctx, `SELECT id, name, phone, amount_tiyin, last_payment FROM shop.debts
		WHERE store_id = $1 ORDER BY last_payment DESC LIMIT 20000`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Debt
	for rows.Next() {
		var d Debt
		if err := rows.Scan(&d.ID, &d.Name, &d.Phone, &d.AmountTiyin, &d.LastPayment); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (p *PGRepo) GetDebt(ctx context.Context, storeID, id string) (*Debt, error) {
	var d Debt
	err := p.db.QueryRow(ctx, `SELECT id, name, phone, amount_tiyin, last_payment FROM shop.debts
		WHERE store_id = $1 AND id = $2`+p.lockSuffix(), storeID, id).Scan(&d.ID, &d.Name, &d.Phone, &d.AmountTiyin, &d.LastPayment)
	if err != nil {
		return nil, noRows(err)
	}
	return &d, nil
}

func (p *PGRepo) SaveDebt(ctx context.Context, storeID string, d *Debt) error {
	_, err := p.db.Exec(ctx, `INSERT INTO shop.debts (id, store_id, name, phone, amount_tiyin, last_payment)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (id) DO UPDATE SET amount_tiyin=EXCLUDED.amount_tiyin, last_payment=EXCLUDED.last_payment
		WHERE shop.debts.store_id = EXCLUDED.store_id`,
		d.ID, storeID, d.Name, d.Phone, d.AmountTiyin, d.LastPayment)
	return err
}

func (p *PGRepo) DeleteDebt(ctx context.Context, storeID, id string) error {
	_, err := p.db.Exec(ctx, `DELETE FROM shop.debts WHERE store_id = $1 AND id = $2`, storeID, id)
	return err
}

// ── Apteka ──

const pharmCols = `barcode, name, category, price_tiyin, base_price_tiyin, quantity::float8, incoming_quantity::float8,
	sold::float8, dosage, manufacturer, expiry_date, batch_number, prescription_required, storage_condition, description,
	dosage_form, shelf_number, row_number, added_at, updated_at`

func scanPharm(row pgx.Row) (*PharmProduct, error) {
	var x PharmProduct
	err := row.Scan(&x.Barcode, &x.Name, &x.Category, &x.PriceTiyin, &x.BasePriceTiyin, &x.Quantity, &x.IncomingQuantity,
		&x.Sold, &x.Dosage, &x.Manufacturer, &x.ExpiryDate, &x.BatchNumber, &x.PrescriptionRequired, &x.StorageCondition,
		&x.Description, &x.DosageForm, &x.ShelfNumber, &x.RowNumber, &x.AddedAt, &x.UpdatedAt)
	if err != nil {
		return nil, noRows(err)
	}
	return &x, nil
}

func collectPharm(rows pgx.Rows, err error) ([]PharmProduct, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PharmProduct
	for rows.Next() {
		x, err := scanPharm(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *x)
	}
	return out, rows.Err()
}

func (p *PGRepo) GetPharm(ctx context.Context, storeID, barcode string) (*PharmProduct, error) {
	return scanPharm(p.db.QueryRow(ctx, `SELECT `+pharmCols+` FROM shop.pharm_products
		WHERE store_id = $1 AND barcode = $2`+p.lockSuffix(), storeID, barcode))
}

func (p *PGRepo) ListPharm(ctx context.Context, storeID string) ([]PharmProduct, error) {
	return collectPharm(p.db.Query(ctx, `SELECT `+pharmCols+` FROM shop.pharm_products
		WHERE store_id = $1 ORDER BY name LIMIT 20000`, storeID))
}

func (p *PGRepo) SearchPharm(ctx context.Context, storeID, q, category string, limit int) ([]PharmProduct, error) {
	return collectPharm(p.db.Query(ctx, `SELECT `+pharmCols+` FROM shop.pharm_products
		WHERE store_id = $1 AND ($2 = '' OR strpos(lower(name), lower($2)) > 0 OR strpos(lower(barcode), lower($2)) > 0)
		  AND ($3 = '' OR category = $3)
		ORDER BY name LIMIT $4`, storeID, q, category, limit))
}

func (p *PGRepo) FindPharmInfo(ctx context.Context, storeID, q string) (*PharmProduct, error) {
	return scanPharm(p.db.QueryRow(ctx, `SELECT `+pharmCols+` FROM shop.pharm_products
		WHERE store_id = $1 AND (strpos(lower(barcode), lower($2)) > 0 OR strpos(lower(name), lower($2)) > 0
		  OR strpos(lower(batch_number), lower($2)) > 0)
		ORDER BY name LIMIT 1`, storeID, q))
}

func (p *PGRepo) SavePharm(ctx context.Context, storeID string, x *PharmProduct) error {
	_, err := p.db.Exec(ctx, `INSERT INTO shop.pharm_products (store_id, barcode, name, category, price_tiyin,
		  base_price_tiyin, quantity, incoming_quantity, sold, dosage, manufacturer, expiry_date, batch_number,
		  prescription_required, storage_condition, description, dosage_form, shelf_number, row_number, added_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)
		ON CONFLICT (store_id, barcode) DO UPDATE SET name=EXCLUDED.name, category=EXCLUDED.category,
		  price_tiyin=EXCLUDED.price_tiyin, base_price_tiyin=EXCLUDED.base_price_tiyin, quantity=EXCLUDED.quantity,
		  incoming_quantity=EXCLUDED.incoming_quantity, sold=EXCLUDED.sold, dosage=EXCLUDED.dosage,
		  manufacturer=EXCLUDED.manufacturer, expiry_date=EXCLUDED.expiry_date, batch_number=EXCLUDED.batch_number,
		  prescription_required=EXCLUDED.prescription_required, storage_condition=EXCLUDED.storage_condition,
		  description=EXCLUDED.description, dosage_form=EXCLUDED.dosage_form, shelf_number=EXCLUDED.shelf_number,
		  row_number=EXCLUDED.row_number, updated_at=EXCLUDED.updated_at`,
		storeID, x.Barcode, x.Name, x.Category, x.PriceTiyin, x.BasePriceTiyin, x.Quantity, x.IncomingQuantity, x.Sold,
		x.Dosage, x.Manufacturer, x.ExpiryDate, x.BatchNumber, x.PrescriptionRequired, x.StorageCondition, x.Description,
		x.DosageForm, x.ShelfNumber, x.RowNumber, x.AddedAt, x.UpdatedAt)
	return err
}

func (p *PGRepo) DeletePharm(ctx context.Context, storeID, barcode string) (bool, error) {
	tag, err := p.db.Exec(ctx, `DELETE FROM shop.pharm_products WHERE store_id = $1 AND barcode = $2`, storeID, barcode)
	return tag.RowsAffected() > 0, err
}

const pharmBaseCols = `barcode, name, category, base_price_tiyin, dosage, manufacturer, storage_condition, side_effects,
	description, composition, dosage_form, prescription_required`

func collectPharmBase(rows pgx.Rows, err error) ([]PharmBase, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PharmBase
	for rows.Next() {
		var b PharmBase
		if err := rows.Scan(&b.Barcode, &b.Name, &b.Category, &b.BasePriceTiyin, &b.Dosage, &b.Manufacturer,
			&b.StorageCondition, &b.SideEffects, &b.Description, &b.Composition, &b.DosageForm, &b.PrescriptionRequired); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (p *PGRepo) SearchPharmBase(ctx context.Context, q, category string, limit int) ([]PharmBase, error) {
	return collectPharmBase(p.db.Query(ctx, `SELECT `+pharmBaseCols+` FROM shop.pharm_base
		WHERE ($1 = '%%' OR lower(name) LIKE lower($1) OR barcode LIKE $1)
		  AND ($2 = '' OR category = $2)
		ORDER BY name LIMIT $3`, likeArg(q), category, limit))
}

func (p *PGRepo) CountPharmBase(ctx context.Context) (int, error) {
	var n int
	err := p.db.QueryRow(ctx, `SELECT count(*) FROM shop.pharm_base`).Scan(&n)
	return n, err
}

func (p *PGRepo) ListPharmBase(ctx context.Context) ([]PharmBase, error) {
	return collectPharmBase(p.db.Query(ctx, `SELECT `+pharmBaseCols+` FROM shop.pharm_base ORDER BY name LIMIT 50000`))
}

func (p *PGRepo) AddPharmBase(ctx context.Context, b PharmBase, storeID string) error {
	_, err := p.db.Exec(ctx, `INSERT INTO shop.pharm_base (`+pharmBaseCols+`, added_by_store)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		b.Barcode, b.Name, b.Category, b.BasePriceTiyin, b.Dosage, b.Manufacturer, b.StorageCondition, b.SideEffects,
		b.Description, b.Composition, b.DosageForm, b.PrescriptionRequired, storeID)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return ErrConflict
	}
	return err
}

func (p *PGRepo) SavePharmSale(ctx context.Context, storeID string, s *PharmSale) error {
	items, err := json.Marshal(s.Items)
	if err != nil {
		return err
	}
	_, err = p.db.Exec(ctx, `INSERT INTO shop.pharm_sales (id, store_id, items, total_tiyin, payment_method, sold_at)
		VALUES ($1,$2,$3,$4,$5,$6)`, s.ID, storeID, items, s.TotalTiyin, s.PaymentMethod, s.SoldAt)
	return err
}

func (p *PGRepo) ListPharmSales(ctx context.Context, storeID string, limit int) ([]PharmSale, error) {
	rows, err := p.db.Query(ctx, `SELECT id, items, total_tiyin, payment_method, sold_at FROM shop.pharm_sales
		WHERE store_id = $1 ORDER BY sold_at DESC LIMIT $2`, storeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PharmSale
	for rows.Next() {
		var s PharmSale
		var raw []byte
		if err := rows.Scan(&s.ID, &raw, &s.TotalTiyin, &s.PaymentMethod, &s.SoldAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &s.Items); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (p *PGRepo) PharmSaleTotals(ctx context.Context, storeID string) (int, int64, error) {
	var n int
	var sum int64
	err := p.db.QueryRow(ctx, `SELECT count(*), COALESCE(sum(total_tiyin), 0) FROM shop.pharm_sales WHERE store_id = $1`,
		storeID).Scan(&n, &sum)
	return n, sum, err
}

// ── Qurt buyurtmasi ledger ──

func (p *PGRepo) GetOrderLedger(ctx context.Context, orderID string) (*OrderLedger, error) {
	var (
		l   OrderLedger
		raw []byte
	)
	err := p.db.QueryRow(ctx, `SELECT order_id, store_id, items, state FROM shop.qurt_order_ledger
		WHERE order_id = $1`+p.lockSuffix(), orderID).Scan(&l.OrderID, &l.StoreID, &raw, &l.State)
	if err != nil {
		return nil, noRows(err)
	}
	if err := json.Unmarshal(raw, &l.Items); err != nil {
		return nil, err
	}
	return &l, nil
}

func (p *PGRepo) SaveOrderLedger(ctx context.Context, l *OrderLedger) error {
	items, err := json.Marshal(l.Items)
	if err != nil {
		return err
	}
	_, err = p.db.Exec(ctx, `INSERT INTO shop.qurt_order_ledger (order_id, store_id, items, state)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (order_id) DO UPDATE SET items = EXCLUDED.items, state = EXCLUDED.state, updated_at = now()`,
		l.OrderID, l.StoreID, items, l.State)
	return err
}

func (p *PGRepo) DeleteOrderLedger(ctx context.Context, orderID string) error {
	_, err := p.db.Exec(ctx, `DELETE FROM shop.qurt_order_ledger WHERE order_id = $1`, orderID)
	return err
}
