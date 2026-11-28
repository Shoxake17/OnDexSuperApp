// migrateshop — POS-4 (Plane ONDEXECOSY-489): ShoxPos (Mongo) ma'lumotlarini
// OnDex POS (`shop` sxemasi, Postgres) ga ko'chiradi.
//
//	go run ./cmd/migrateshop            # QURUQ: faqat hisobot, bazaga yozmaydi
//	go run ./cmd/migrateshop --apply    # bitta tranzaksiyada yozadi
//
// Muhit: DATABASE_URL (OnDex Postgres), SHOXPOS_MONGO_URI va SHOXPOS_MONGO_DB
// (ShoxPos bazasi).
//
// Qoidalar:
//   - Do'kon = ShoxPos foydalanuvchisining `ondexEntityId` si (OnDex'da `store`
//     roli bilan mavjud bo'lishi SHART); bog'lanmagan/o'chirilgan — o'tkaziladi
//     va hisobotda ko'rsatiladi.
//   - Pul so'mdan tiyinga (x100, yaxlitlash); miqdor 3 xonagacha.
//   - Takroriy ishga tushirish xavfsiz (upsert); "Sotilganlar" (30 kun) ko'chirilgan
//     do'konlar uchun qaytadan yoziladi. Faqat ALMASHTIRISHDAN OLDIN ishlating:
//     keyin ShoxPos yozishdan to'xtaydi va OnDex POS'ning o'zi manba bo'ladi.
//
// Production tartibi: 1) zaxira (backup-others.sh: Postgres + ShoxPos Mongo);
// 2) quruq ishga tushirish, hisobotni egasi ko'radi; 3) ShoxPos yozishni
// to'xtatish (texnik tanaffus); 4) --apply; 5) Caddy almashtirish (POS-5).
package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	apply := flag.Bool("apply", false, "bazaga yozish (busiz faqat hisobot)")
	flag.Parse()
	if err := run(*apply); err != nil {
		slog.Error("migrateshop", "err", err)
		os.Exit(1)
	}
}

type source struct {
	users      []mUser
	products   []mProduct
	base       []mBase
	sales      []mSale
	lines      []bucketLine
	lineLogin  []string
	debts      []mDebt
	pharm      []mPharm
	pharmBase  []mPharmBase
	pharmSales []mPharmSale
	shopTypes  []struct {
		Value      string   `bson:"value"`
		Categories []string `bson:"categories"`
	}
	ledgers    []mLedger
	badBuckets int
}

func run(apply bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	dsn, muri, mdb := os.Getenv("DATABASE_URL"), os.Getenv("SHOXPOS_MONGO_URI"), os.Getenv("SHOXPOS_MONGO_DB")
	if dsn == "" || muri == "" || mdb == "" {
		return errors.New("DATABASE_URL, SHOXPOS_MONGO_URI va SHOXPOS_MONGO_DB majburiy")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	var ready *string
	if err := pool.QueryRow(ctx, `SELECT to_regclass('shop.qurt_order_ledger')::text`).Scan(&ready); err != nil || ready == nil {
		return errors.New("shop sxemasi yo'q — avval yangi API'ni ishga tushiring (0081 migratsiyasi)")
	}
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(muri).SetRegistry(lenientRegistry()))
	if err != nil {
		return err
	}
	defer client.Disconnect(context.Background()) //nolint:errcheck // chiqishda
	src, err := load(ctx, client.Database(mdb))
	if err != nil {
		return fmt.Errorf("ShoxPos o'qish: %w", err)
	}

	stores, skippedUsers := buildStores(src.users)
	// OnDex'da shu ID bilan faol do'kon akkaunti borligini tekshirish.
	for login, st := range stores {
		var ok bool
		err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE entity_id = $1 AND role = 'store'
			AND deleted_at IS NULL)`, st.ID).Scan(&ok)
		if err != nil {
			return err
		}
		if !ok {
			skippedUsers = append(skippedUsers, login+": OnDex'da "+st.ID+" do'kon akkaunti topilmadi")
			delete(stores, login)
		}
	}
	loc, err := time.LoadLocation("Asia/Tashkent")
	if err != nil {
		loc = time.FixedZone("UZT", 5*3600)
	}
	now := time.Now().UTC()

	fmt.Printf("ShoxPos foydalanuvchilari: %d, ko'chiriladigan do'konlar: %d\n", len(src.users), len(stores))
	logins := make([]string, 0, len(stores))
	for l := range stores {
		logins = append(logins, l)
	}
	sort.Strings(logins)
	for _, l := range logins {
		fmt.Printf("  %-24s -> %s (%s)\n", l, stores[l].ID, stores[l].Category)
	}
	if len(skippedUsers) > 0 {
		fmt.Println("O'tkazilgan akkauntlar:")
		for _, s := range skippedUsers {
			fmt.Println("  - " + s)
		}
	}

	var tx pgx.Tx
	if apply {
		if tx, err = pool.Begin(ctx); err != nil {
			return err
		}
		defer tx.Rollback(context.Background()) //nolint:errcheck // commit'dan keyin no-op
	}
	w := &writer{tx: tx, ctx: ctx}
	reps := []report{
		migrateProducts(w, src, stores, now),
		migrateBase(w, src),
		migrateCategories(w, src, stores),
		migrateSales(w, src, stores, loc),
		migrateLines(w, src, stores, now),
		migrateDebts(w, src, stores, now),
		migratePharm(w, src, stores, now),
		migratePharmBase(w, src, stores),
		migratePharmSales(w, src, stores),
		migrateLedgers(w, src, stores),
	}
	fmt.Println("\nTo'plamlar:")
	for _, r := range reps {
		fmt.Println(r.String())
	}
	if w.err != nil {
		return fmt.Errorf("yozish (hech narsa saqlanmadi): %w", w.err)
	}
	if !apply {
		fmt.Println("\nQURUQ ishga tushirish — bazaga HECH NARSA yozilmadi. Yozish uchun: --apply")
		return nil
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	fmt.Println("\nYOZILDI (bitta tranzaksiya).")
	return nil
}

// writer — quruq rejimda tx nil: hech narsa yuborilmaydi. Birinchi xatodan keyin
// qolgan yozuvlar o'tkazib yuboriladi va tranzaksiya bekor qilinadi.
type writer struct {
	tx    pgx.Tx
	ctx   context.Context
	err   error
	batch pgx.Batch
}

func (w *writer) exec(sql string, args ...any) {
	if w.tx == nil || w.err != nil {
		return
	}
	w.batch.Queue(sql, args...)
	if w.batch.Len() >= 1000 {
		w.flush()
	}
}

func (w *writer) flush() {
	if w.tx == nil || w.err != nil || w.batch.Len() == 0 {
		return
	}
	w.err = w.tx.SendBatch(w.ctx, &w.batch).Close()
	w.batch = pgx.Batch{}
}

func load(ctx context.Context, db *mongo.Database) (*source, error) {
	s := &source{}
	all := func(coll string, out any) error {
		cur, err := db.Collection(coll).Find(ctx, bson.M{})
		if err != nil {
			return fmt.Errorf("%s: %w", coll, err)
		}
		return cur.All(ctx, out)
	}
	for _, c := range []struct {
		name string
		out  any
	}{
		{"users", &s.users}, {"products", &s.products}, {"products_base", &s.base}, {"pharmproducts", &s.pharm},
		{"pharm_base", &s.pharmBase}, {"shoptypes", &s.shopTypes}, {"qurtorderledgers", &s.ledgers},
	} {
		if err := all(c.name, c.out); err != nil {
			return nil, err
		}
	}
	// _id kerak bo'lgan to'plamlar (deterministik ID — takroriy ko'chirish xavfsiz).
	type withID struct {
		ID primitive.ObjectID `bson:"_id"`
	}
	read := func(coll string, each func(raw bson.Raw, id string) error) error {
		cur, err := db.Collection(coll).Find(ctx, bson.M{})
		if err != nil {
			return fmt.Errorf("%s: %w", coll, err)
		}
		defer cur.Close(ctx)
		for cur.Next(ctx) {
			var h withID
			_ = decodeLenient(cur.Current, &h)
			if err := each(cur.Current, "sx-"+h.ID.Hex()); err != nil {
				return fmt.Errorf("%s: %w", coll, err)
			}
		}
		return cur.Err()
	}
	if err := read("sales", func(raw bson.Raw, id string) error {
		var v mSale
		if err := decodeLenient(raw, &v); err != nil {
			return err
		}
		v.ID = id
		s.sales = append(s.sales, v)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := read("debts", func(raw bson.Raw, id string) error {
		var v mDebt
		if err := decodeLenient(raw, &v); err != nil {
			return err
		}
		v.ID = id
		s.debts = append(s.debts, v)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := read("pharmsales", func(raw bson.Raw, id string) error {
		var v mPharmSale
		if err := decodeLenient(raw, &v); err != nil {
			return err
		}
		v.ID = id
		s.pharmSales = append(s.pharmSales, v)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := read("salesbuckets", func(raw bson.Raw, _ string) error {
		var b struct {
			Login string `bson:"login"`
			Data  []byte `bson:"data"`
		}
		if err := decodeLenient(raw, &b); err != nil {
			return err
		}
		arr, ok := decodeBucket(b.Data)
		if !ok {
			s.badBuckets++ // buzilgan — ShoxPos ham o'tkazib yuborardi; hisobotda ko'rinadi
			return nil
		}
		for _, l := range arr {
			s.lines = append(s.lines, l)
			s.lineLogin = append(s.lineLogin, b.Login)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return s, nil
}

func storeFor(stores map[string]storeOf, login string) (storeOf, bool) {
	st, ok := stores[strings.ToLower(strings.TrimSpace(login))]
	return st, ok
}

func migrateProducts(w *writer, src *source, stores map[string]storeOf, now time.Time) report {
	r := report{Name: "products"}
	for _, p := range src.products {
		r.Read++
		st, ok := storeFor(stores, p.Login)
		if !ok {
			r.skip(p.Login + ": do'kon ko'chirilmaydi")
			continue
		}
		bc := strings.TrimSpace(p.Barcode)
		if !barcodeRe.MatchString(bc) {
			r.skip(fmt.Sprintf("%s: shtrix-kod formati noto'g'ri %q", p.Login, p.Barcode))
			continue
		}
		name := cutRunes(p.Name, 300)
		if name == "" {
			name = "Nomsiz mahsulot"
		}
		grams := int(p.WeightGrams)
		if grams < 0 || grams > 100000 {
			grams = 0
		}
		r.Written++
		w.exec(`INSERT INTO shop.products (store_id, barcode, name, category, price_tiyin, base_price_tiyin, quantity,
			  incoming_quantity, sold, image_url, description, composition, weight_grams, added_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
			ON CONFLICT (store_id, barcode) DO UPDATE SET name=EXCLUDED.name, category=EXCLUDED.category,
			  price_tiyin=EXCLUDED.price_tiyin, base_price_tiyin=EXCLUDED.base_price_tiyin, quantity=EXCLUDED.quantity,
			  incoming_quantity=EXCLUDED.incoming_quantity, sold=EXCLUDED.sold, image_url=EXCLUDED.image_url,
			  description=EXCLUDED.description, composition=EXCLUDED.composition, weight_grams=EXCLUDED.weight_grams,
			  updated_at=EXCLUDED.updated_at`,
			st.ID, bc, name, cutRunes(p.Category, 64), tiyin(p.Price), tiyin(p.BasePrice), qty(p.Quantity),
			qty(p.IncomingQuantity), qty(p.Sold), strings.TrimSpace(p.ImageURL), cutRunes(p.Description, 1000),
			cutRunes(p.Composition, 1000), grams, orNow(p.AddedAt, now), orNow(p.UpdatedAt, orNow(p.AddedAt, now)))
	}
	w.flush()
	return r
}

func migrateBase(w *writer, src *source) report {
	r := report{Name: "products_base"}
	for _, b := range src.base {
		r.Read++
		bc := strings.TrimSpace(b.Barcode)
		name := cutRunes(b.Name, 500)
		if bc == "" || name == "" || len(bc) > 64 {
			r.skip(fmt.Sprintf("bo'sh yoki uzun: %q", b.Barcode))
			continue
		}
		r.Written++
		w.exec(`INSERT INTO shop.product_base (barcode, name, category, price_tiyin, base_price_tiyin)
			VALUES ($1,$2,$3,$4,$5) ON CONFLICT (barcode) DO UPDATE SET name=EXCLUDED.name, category=EXCLUDED.category,
			  price_tiyin=EXCLUDED.price_tiyin, base_price_tiyin=EXCLUDED.base_price_tiyin`,
			bc, name, cutRunes(b.Category, 64), tiyin(b.Price), tiyin(b.BasePrice))
	}
	w.flush()
	return r
}

func migrateCategories(w *writer, src *source, stores map[string]storeOf) report {
	r := report{Name: "kategoriyalar"}
	for _, u := range src.users {
		st, ok := storeFor(stores, u.Login)
		if !ok {
			continue
		}
		for _, c := range u.ShopCategories {
			r.Read++
			c = cutRunes(c, 64)
			if c == "" {
				r.skip("bo'sh kategoriya")
				continue
			}
			r.Written++
			w.exec(`INSERT INTO shop.store_categories (store_id, name) VALUES ($1,$2) ON CONFLICT DO NOTHING`, st.ID, c)
		}
	}
	for _, t := range src.shopTypes {
		for _, c := range t.Categories {
			r.Read++
			c = cutRunes(c, 64)
			if c == "" || t.Value == "" {
				r.skip("bo'sh tur kategoriyasi")
				continue
			}
			r.Written++
			w.exec(`INSERT INTO shop.type_categories (shop_type, name) VALUES ($1,$2) ON CONFLICT DO NOTHING`, t.Value, c)
		}
	}
	w.flush()
	return r
}

func migrateSales(w *writer, src *source, stores map[string]storeOf, loc *time.Location) report {
	r := report{Name: "sales (kunlik)", Read: len(src.sales)}
	agg, skipped := aggregateSales(src.sales, stores, loc)
	r.Skipped = skipped
	for k, a := range agg {
		r.Written++
		w.exec(`INSERT INTO shop.sales (id, store_id, barcode, name, category, quantity, price_tiyin, base_price_tiyin,
			  total_tiyin, payment_method, sale_day, sold_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::date,$12)
			ON CONFLICT (store_id, barcode, payment_method, sale_day) DO UPDATE SET name=EXCLUDED.name,
			  category=EXCLUDED.category, quantity=EXCLUDED.quantity, price_tiyin=EXCLUDED.price_tiyin,
			  base_price_tiyin=EXCLUDED.base_price_tiyin, total_tiyin=EXCLUDED.total_tiyin, sold_at=EXCLUDED.sold_at`,
			a.ID, k.Store, k.Barcode, a.Name, a.Category, a.Quantity, a.Price, a.Base, a.Total, k.Payment, k.Day, a.LastSoldAt)
	}
	w.flush()
	return r
}

func migrateLines(w *writer, src *source, stores map[string]storeOf, now time.Time) report {
	r := report{Name: "sotilganlar (30 kun)"}
	if src.badBuckets > 0 {
		r.skip(fmt.Sprintf("%d ta buzilgan bucket o'qilmadi", src.badBuckets))
	}
	cutoff := now.Add(-30 * 24 * time.Hour)
	ids := map[string]bool{}
	for _, st := range stores {
		ids[st.ID] = true
	}
	for id := range ids {
		w.exec(`DELETE FROM shop.sale_lines WHERE store_id = $1`, id)
	}
	for i, l := range src.lines {
		r.Read++
		st, ok := storeFor(stores, src.lineLogin[i])
		if !ok || l.SoldAt.Before(cutoff) || !barcodeRe.MatchString(l.Barcode) {
			r.Skipped++
			continue
		}
		pay := cutRunes(l.PaymentMethod, 32)
		if pay == "" {
			pay = "cash"
		}
		total := tiyin(l.Total)
		if total == 0 {
			total = int64(float64(tiyin(l.Price)) * qty(l.Quantity))
		}
		r.Written++
		w.exec(`INSERT INTO shop.sale_lines (store_id, barcode, name, category, quantity, price_tiyin, base_price_tiyin,
			  total_tiyin, payment_method, sold_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			st.ID, l.Barcode, cutRunes(l.Name, 300), cutRunes(l.Category, 64), qty(l.Quantity), tiyin(l.Price),
			tiyin(l.BasePrice), total, pay, l.SoldAt)
	}
	w.flush()
	return r
}

func migrateDebts(w *writer, src *source, stores map[string]storeOf, now time.Time) report {
	r := report{Name: "debts"}
	for _, d := range src.debts {
		r.Read++
		st, ok := storeFor(stores, d.Login)
		name, phone, amount := cutRunes(d.Name, 200), cutRunes(d.Phone, 32), tiyin(d.Amount)
		switch {
		case !ok:
			r.skip(d.Login + ": do'kon ko'chirilmaydi")
			continue
		case name == "" || phone == "" || amount <= 0:
			r.skip(fmt.Sprintf("%s: to'liq emas yoki 0 qarz (%s)", d.Login, name))
			continue
		}
		r.Written++
		w.exec(`INSERT INTO shop.debts (id, store_id, name, phone, amount_tiyin, last_payment) VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (id) DO UPDATE SET amount_tiyin=EXCLUDED.amount_tiyin, last_payment=EXCLUDED.last_payment`,
			d.ID, st.ID, name, phone, amount, orNow(d.LastPayment, now))
	}
	w.flush()
	return r
}

func migratePharm(w *writer, src *source, stores map[string]storeOf, now time.Time) report {
	r := report{Name: "pharmproducts"}
	for _, p := range src.pharm {
		r.Read++
		st, ok := storeFor(stores, p.Login)
		bc := strings.TrimSpace(p.Barcode)
		if !ok || !barcodeRe.MatchString(bc) {
			r.skip(fmt.Sprintf("%s: do'kon yo'q yoki shtrix-kod %q", p.Login, p.Barcode))
			continue
		}
		name := cutRunes(p.Name, 500)
		if name == "" {
			name = bc
		}
		var exp *string
		if p.ExpiryDate != nil && !p.ExpiryDate.IsZero() {
			s := p.ExpiryDate.UTC().Format("2006-01-02")
			exp = &s
		}
		r.Written++
		w.exec(`INSERT INTO shop.pharm_products (store_id, barcode, name, category, price_tiyin, base_price_tiyin, quantity,
			  incoming_quantity, sold, dosage, manufacturer, expiry_date, batch_number, prescription_required,
			  storage_condition, description, dosage_form, shelf_number, row_number, added_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::date,$13,$14,$15,$16,$17,$18,$19,$20,$21)
			ON CONFLICT (store_id, barcode) DO UPDATE SET name=EXCLUDED.name, category=EXCLUDED.category,
			  price_tiyin=EXCLUDED.price_tiyin, base_price_tiyin=EXCLUDED.base_price_tiyin, quantity=EXCLUDED.quantity,
			  incoming_quantity=EXCLUDED.incoming_quantity, sold=EXCLUDED.sold, dosage=EXCLUDED.dosage,
			  manufacturer=EXCLUDED.manufacturer, expiry_date=EXCLUDED.expiry_date, batch_number=EXCLUDED.batch_number,
			  prescription_required=EXCLUDED.prescription_required, storage_condition=EXCLUDED.storage_condition,
			  description=EXCLUDED.description, dosage_form=EXCLUDED.dosage_form, shelf_number=EXCLUDED.shelf_number,
			  row_number=EXCLUDED.row_number, updated_at=EXCLUDED.updated_at`,
			st.ID, bc, name, cutRunes(p.Category, 64), tiyin(p.Price), tiyin(p.BasePrice), qty(p.Quantity),
			qty(p.IncomingQuantity), qty(p.Sold), cutRunes(p.Dosage, 500), cutRunes(p.Manufacturer, 500), exp,
			cutRunes(p.BatchNumber, 100), p.IsPrescriptionRequired, cutRunes(p.StorageCondition, 500),
			cutRunes(p.Description, 2000), cutRunes(p.DosageForm, 100), cutRunes(p.ShelfNumber, 500),
			cutRunes(p.RowNumber, 500), orNow(p.AddedAt, now), orNow(p.UpdatedAt, orNow(p.AddedAt, now)))
	}
	w.flush()
	return r
}

func migratePharmBase(w *writer, src *source, stores map[string]storeOf) report {
	r := report{Name: "pharm_base"}
	for _, b := range src.pharmBase {
		r.Read++
		bc, name := strings.TrimSpace(b.Barcode), cutRunes(b.Name, 500)
		if bc == "" || name == "" || len(bc) > 64 {
			r.skip(fmt.Sprintf("bo'sh yoki uzun: %q", b.Barcode))
			continue
		}
		addedBy := ""
		if st, ok := storeFor(stores, b.AddedBy); ok {
			addedBy = st.ID
		}
		r.Written++
		w.exec(`INSERT INTO shop.pharm_base (barcode, name, category, base_price_tiyin, dosage, manufacturer,
			  storage_condition, side_effects, description, composition, dosage_form, prescription_required, added_by_store)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
			ON CONFLICT (barcode) DO UPDATE SET name=EXCLUDED.name, category=EXCLUDED.category,
			  base_price_tiyin=EXCLUDED.base_price_tiyin, dosage=EXCLUDED.dosage, manufacturer=EXCLUDED.manufacturer,
			  storage_condition=EXCLUDED.storage_condition, side_effects=EXCLUDED.side_effects,
			  description=EXCLUDED.description, composition=EXCLUDED.composition, dosage_form=EXCLUDED.dosage_form,
			  prescription_required=EXCLUDED.prescription_required`,
			bc, name, cutRunes(b.Category, 64), tiyin(b.BasePrice), cutRunes(b.Dosage, 500), cutRunes(b.Manufacturer, 500),
			cutRunes(b.StorageCondition, 500), cutRunes(b.SideEffects, 2000), cutRunes(b.Description, 2000),
			cutRunes(b.Composition, 2000), cutRunes(b.DosageForm, 100), b.IsPrescriptionRequired, addedBy)
	}
	w.flush()
	return r
}

type pharmItemOut struct {
	Barcode        string  `json:"barcode"`
	Name           string  `json:"name"`
	Quantity       float64 `json:"quantity"`
	PriceTiyin     int64   `json:"price_tiyin"`
	BasePriceTiyin int64   `json:"base_price_tiyin"`
	TotalTiyin     int64   `json:"total_tiyin"`
	Dosage         string  `json:"dosage"`
	Manufacturer   string  `json:"manufacturer"`
}

func migratePharmSales(w *writer, src *source, stores map[string]storeOf) report {
	r := report{Name: "pharmsales"}
	for _, s := range src.pharmSales {
		r.Read++
		st, ok := storeFor(stores, s.Login)
		if !ok || s.SoldAt.IsZero() {
			r.skip(s.Login + ": do'kon yo'q yoki sana yo'q")
			continue
		}
		items := make([]pharmItemOut, 0, len(s.Items))
		for _, it := range s.Items {
			total := tiyin(it.Total)
			if total == 0 {
				total = int64(float64(tiyin(it.Price)) * qty(it.Quantity))
			}
			items = append(items, pharmItemOut{Barcode: it.Barcode, Name: cutRunes(it.Name, 500), Quantity: qty(it.Quantity),
				PriceTiyin: tiyin(it.Price), BasePriceTiyin: tiyin(it.BasePrice), TotalTiyin: total,
				Dosage: cutRunes(it.Dosage, 500), Manufacturer: cutRunes(it.Manufacturer, 500)})
		}
		raw, _ := json.Marshal(items)
		r.Written++
		w.exec(`INSERT INTO shop.pharm_sales (id, store_id, items, total_tiyin, payment_method, sold_at)
			VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (id) DO NOTHING`,
			s.ID, st.ID, raw, tiyin(s.TotalAmount), pharmPayment(s.PaymentMethod), s.SoldAt)
	}
	w.flush()
	return r
}

func migrateLedgers(w *writer, src *source, stores map[string]storeOf) report {
	r := report{Name: "qurt ledger"}
	for _, l := range src.ledgers {
		r.Read++
		st, ok := storeFor(stores, l.Login)
		if !ok || l.OrderID == "" || (l.State != "reserved" && l.State != "restored") {
			r.skip(l.Login + ": do'kon yo'q yoki holat noto'g'ri")
			continue
		}
		type it struct {
			Barcode string `json:"barcode"`
			Qty     int    `json:"qty"`
		}
		items := make([]it, 0, len(l.Items))
		for _, x := range l.Items {
			items = append(items, it{Barcode: x.Barcode, Qty: x.Qty})
		}
		raw, _ := json.Marshal(items)
		r.Written++
		w.exec(`INSERT INTO shop.qurt_order_ledger (order_id, store_id, items, state) VALUES ($1,$2,$3,$4)
			ON CONFLICT (order_id) DO UPDATE SET items=EXCLUDED.items, state=EXCLUDED.state, updated_at=now()`,
			l.OrderID, st.ID, raw, l.State)
	}
	w.flush()
	return r
}

// decodeBucket — SalesBucket.data (gzip JSON); buzilgan bo'lsa ok=false.
func decodeBucket(data []byte) ([]bucketLine, bool) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, false
	}
	raw, err := io.ReadAll(io.LimitReader(zr, 64<<20))
	if err != nil {
		return nil, false
	}
	var arr []bucketLine
	if json.Unmarshal(raw, &arr) != nil {
		return nil, false
	}
	return arr, true
}
