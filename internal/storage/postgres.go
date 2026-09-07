// PostgreSQL implementatsiyalari. DATABASE_URL berilganda main shularni ishlatadi.
package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/couriers"
	"chustapp/internal/orders"
)

// Postgres unique indeks nomlari — migration 0016/0017'ga qarang. Save()
// ularni pgconn.PgError.ConstraintName orqali aniqlab, mos xatoga
// (orders.ErrDuplicateIdempotencyKey) aylantiradi yoki (order_number
// kolliziyasida) qayta uradi.
const (
	idxOrdersIdempotency  = "idx_orders_idempotency"
	idxOrdersNumberUnique = "idx_orders_number_unique"
)

type PgOrderRepo struct{ pool *pgxpool.Pool }

func NewPgOrderRepo(pool *pgxpool.Pool) *PgOrderRepo { return &PgOrderRepo{pool: pool} }

func (r *PgOrderRepo) GetByID(ctx context.Context, id string) (*orders.Order, error) {
	o, err := scanOrderRow(r.pool.QueryRow(ctx,
		`SELECT `+orderColumns+` FROM orders WHERE id = $1`, id))
	if err != nil {
		return nil, err
	}
	return o, nil
}

// unmarshalAddress — `delivery_address` JSONB'ni o'qiydi. Eski (0025
// migratsiyasidan oldingi) buyurtmalarda bo'sh/`{}` bo'lishi normal,
// shuning uchun bo'sh qiymat xato emas.
func unmarshalAddress(raw []byte, dst *orders.Address) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dst)
}

// FindByIdempotencyKey — Service.Create()dagi "shu mijoz avval xuddi shu
// kalit bilan buyurtma yaratganmi" tekshiruvi uchun. Topilmasa
// orders.ErrNotFound (Service shuni kutadi).
func (r *PgOrderRepo) FindByIdempotencyKey(ctx context.Context, customerID, key string) (*orders.Order, error) {
	if key == "" {
		return nil, orders.ErrNotFound
	}
	o, err := scanOrderRow(r.pool.QueryRow(ctx,
		`SELECT `+orderColumns+` FROM orders
		 WHERE customer_id = $1 AND idempotency_key = $2`, customerID, key))
	if err != nil {
		return nil, err
	}
	o.IdempotencyKey = key
	return o, nil
}

// Save — yangi buyurtma bo'lsa qo'shadi, mavjud bo'lsa yangilaydi.
// Optimistik parallel boshqaruv (orders.ErrConflict): UPDATE FAQAT o.Version
// hali ham bazadagi bilan bir xil bo'lsa qo'llanadi (`WHERE orders.version =
// EXCLUDED.version`) — mos kelmasa (chaqiruvchi eskirgan holatni o'qib,
// o'sha oraliqda boshqa so'rov allaqachon yozib ulgurgan bo'lsa) Postgres
// DO UPDATE'ni o'tkazib yuboradi, RETURNING hech narsa qaytarmaydi (0 qator)
// — buni pgx.ErrNoRows sifatida ushlaymiz va orders.ErrConflict'ga
// aylantiramiz. Bu — klassik "compare-and-swap orqali WHERE" naqshi,
// SELECT FOR UPDATE'dan farqli, qulf USHLAMAYDI (parallel o'qishlarga
// to'sqinlik qilmaydi, faqat ziddiyatli yozuvni aniqlaydi).
func (r *PgOrderRepo) Save(ctx context.Context, o *orders.Order) error {
	itemsJSON, err := json.Marshal(o.Items)
	if err != nil {
		return err
	}
	historyJSON, err := json.Marshal(o.History)
	if err != nil {
		return err
	}
	addressJSON, err := json.Marshal(o.DeliveryAddress)
	if err != nil {
		return err
	}
	var courierID *string
	if o.CourierID != "" {
		courierID = &o.CourierID
	}
	// Stol maydonlari — yetkazish buyurtmasida NULL bo'lib qolsin
	// (bo'sh satr emas): "stol yo'q" va "stol nomi bo'sh" farqli
	// holatlar va indeks ham NULL'larni saqlamaydi.
	var tableID, tableLabel *string
	var partySize *int
	if o.TableID != "" {
		tableID = &o.TableID
	}
	if o.TableLabel != "" {
		tableLabel = &o.TableLabel
	}
	if o.PartySize > 0 {
		partySize = &o.PartySize
	}
	// order_number ustunga umuman yozilmaydi — DEFAULT ifoda orqali faqat
	// INSERT'da avtomatik hisoblanadi ("DDMMYY-0000001" formatida, ichki
	// ketma-ketlikka asoslanib — 0013-migratsiyaga qarang), keyingi
	// UPDATE'larda o'zgarmaydi. RETURNING orqali haqiqiy qiymatini Go
	// strukturasiga o'qib olamiz. idempotency_key ham xuddi shunday —
	// faqat INSERT'da yoziladi, UPDATE'larda o'zgarmaydi.
	//
	// Qayta urinish sikli FAQAT order_number kolliziyasi (idxOrdersNumberUnique,
	// amalda deyarli imkonsiz — 7 xonali tasodifiy son) uchun: DEFAULT
	// har safar YANGI qiymat hisoblaydi, shuning uchun oddiy qayta so'rov
	// yetarli. idempotency-key kolliziyasi va versiya ziddiyati qayta
	// urinilmaydi — ular chaqiruvchiga (Service) aniq xato sifatida
	// qaytariladi (u alohida qaror qabul qiladi: eski buyurtmani
	// qaytarish yoki 409 berish).
	const maxOrderNumberRetries = 3
	for attempt := 0; attempt < maxOrderNumberRetries; attempt++ {
		err = r.pool.QueryRow(ctx, `
			INSERT INTO orders (id, customer_id, restaurant_id, courier_id, status, total_tiyin,
			                    delivery_lat, delivery_lng, items, history, created_at, updated_at,
			                    preparation_minutes, ready_at, version, idempotency_key,
			                    subtotal_tiyin, discount_tiyin, promotion_id, promotion_name,
			                    promotion_discount_tiyin,
			                    payment_method, payment_state,
			                    delivery_address,
			                    order_type, table_id, table_label, party_size)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,
			        $22,$23,$24,$25,$26,$27,$28)
			ON CONFLICT (id) DO UPDATE SET
				courier_id           = EXCLUDED.courier_id,
				status               = EXCLUDED.status,
				history              = EXCLUDED.history,
				updated_at           = EXCLUDED.updated_at,
				preparation_minutes  = EXCLUDED.preparation_minutes,
				ready_at             = EXCLUDED.ready_at,
				-- To'lov holati buyurtma hayoti davomida O'ZGARADI
				-- (kutilmoqda -> bloklandi -> yechildi), shuning uchun u
				-- yangilanadigan ustunlar ro'yxatida bo'lishi SHART.
				-- payment_method esa o'zgarmaydi (faqat INSERT'da).
				payment_state        = EXCLUDED.payment_state,
				version              = orders.version + 1
			WHERE orders.version = EXCLUDED.version
			RETURNING order_number, version`,
			o.ID, o.CustomerID, o.RestaurantID, courierID, o.Status, o.TotalTiyin,
			o.DeliveryLat, o.DeliveryLng, itemsJSON, historyJSON, o.CreatedAt, o.UpdatedAt,
			o.PreparationMinutes, o.ReadyAt, o.Version, o.IdempotencyKey,
			o.SubtotalTiyin, o.DiscountTiyin, o.PromotionID, o.PromotionName,
			o.PromotionDiscountTiyin,
			o.PaymentMethod, o.PaymentState,
			addressJSON,
			// Stol maydonlari FAQAT INSERT'da yoziladi (order_number va
			// idempotency_key kabi): buyurtma qaysi stolga tegishli
			// ekani keyin O'ZGARMAYDI. DO UPDATE ro'yxatiga qo'shilsa,
			// har bir holat o'zgarishida ular qayta yozilardi va
			// eskirgan nusxa bilan kelgan so'rov stolni almashtirib
			// yuborishi mumkin bo'lardi.
			o.Type.Normalized(), tableID, tableLabel, partySize,
		).Scan(&o.OrderNumber, &o.Version)

		if err == nil {
			return nil
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return orders.ErrConflict
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			switch pgErr.ConstraintName {
			case idxOrdersIdempotency:
				return orders.ErrDuplicateIdempotencyKey
			case idxOrdersNumberUnique:
				continue // DEFAULT keyingi urinishda yangi tasodifiy raqam beradi
			}
		}
		return err
	}
	return fmt.Errorf("order_number generatsiyasida qayta-qayta kolliziya (juda kamdan-kam holat)")
}

func (r *PgOrderRepo) HasActiveByRestaurant(ctx context.Context, restaurantID string) (bool, error) {
	var exists bool
	// Terminal holatlar ro'yxati Go'dan keladi — `orders` paketidagi
	// yagona manba (bug.md 45-band: SQL literalida `served` yo'q edi va
	// yakunlangan stol buyurtmalari abadiy "faol" bo'lib qolardi).
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM orders
			WHERE restaurant_id = $1
			  AND status <> ALL($2)
		)`, restaurantID, orders.TerminalStatusStrings()).Scan(&exists)
	return exists, err
}

// HasActiveByCustomer — mijozning yakunlanmagan buyurtmasi bormi
// (bug.md 27-band).
//
// BUTUN tarix bo'yicha `EXISTS` — avval bu tekshiruv `ListByCustomer`
// ning oxirgi 20 tasi bilan chegaralangan edi va faol mijozda
// yakunlanmagan buyurtma o'sha 20 tadan pastda qolib ketishi mumkin
// edi.
func (r *PgOrderRepo) HasActiveByCustomer(ctx context.Context, customerID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM orders
			WHERE customer_id = $1
			  AND status <> ALL($2)
		)`, customerID, orders.TerminalStatusStrings()).Scan(&exists)
	return exists, err
}

// GetActiveByCourier — kuryerning hozir yetkazib berayotgan buyurtmasi
// (bo'lsa, eng so'nggisi). Kuryer GPS joylashuvini yangilaganda mijozga
// jonli yuborish uchun (cmd/api/main.go, POST /couriers/{id}/location).
func (r *PgOrderRepo) GetActiveByCourier(ctx context.Context, courierID string) (*orders.Order, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		SELECT id FROM orders
		WHERE courier_id = $1 AND status <> ALL($2)
		ORDER BY created_at DESC LIMIT 1`, courierID, orders.TerminalStatusStrings(),
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, orders.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

// orderColumns — buyurtma ustunlarining YAGONA ro'yxati.
//
// Avval bu ro'yxat GetByID va FindByIdempotencyKey ichida ham
// qo'lda takrorlangan edi (jami uch nusxa) va har biri o'zining
// Scan chaqiruvi bilan yurardi. Yangi ustun qo'shilganda ularning
// birortasi unutilsa, xato KOMPILYATSIYADA emas, ISHLASH paytida
// "number of field descriptions must equal number of destinations"
// bo'lib chiqardi. Endi ro'yxat ham, Scan ham bitta joyda.
const orderColumns = `id, order_number, customer_id, restaurant_id, courier_id, status, total_tiyin,
		       delivery_lat, delivery_lng, items, history, created_at, updated_at,
		       preparation_minutes, ready_at, version,
		       subtotal_tiyin, discount_tiyin, promotion_id, promotion_name,
		       promotion_discount_tiyin, payment_method, payment_state, delivery_address,
		       order_type, table_id, table_label, party_size`

// rowScanner — pgx.Row va pgx.Rows ning umumiy qismi. Ikkalasi ham
// `Scan(...any) error` beradi, shuning uchun bitta scan funksiyasi
// ikkalasiga ham yetadi.
type rowScanner interface{ Scan(dest ...any) error }

// scanOrder — bitta qatorni `orders.Order` ga o'qiydi.
//
// NULL bo'lishi mumkin bo'lgan ustunlar ko'rsatkich orqali o'qiladi:
// `courier_id` (kuryer hali biriktirilmagan), `table_id`/`table_label`/
// `party_size` (yetkazish buyurtmalarida har doim NULL).
func scanOrder(row rowScanner) (*orders.Order, error) {
	var o orders.Order
	var courierID, tableID, tableLabel *string
	var partySize *int
	var itemsJSON, historyJSON, addressJSON []byte
	err := row.Scan(&o.ID, &o.OrderNumber, &o.CustomerID, &o.RestaurantID, &courierID,
		&o.Status, &o.TotalTiyin,
		&o.DeliveryLat, &o.DeliveryLng, &itemsJSON, &historyJSON, &o.CreatedAt, &o.UpdatedAt,
		&o.PreparationMinutes, &o.ReadyAt, &o.Version,
		&o.SubtotalTiyin, &o.DiscountTiyin, &o.PromotionID, &o.PromotionName,
		&o.PromotionDiscountTiyin, &o.PaymentMethod, &o.PaymentState, &addressJSON,
		&o.Type, &tableID, &tableLabel, &partySize)
	if err != nil {
		return nil, err
	}
	if courierID != nil {
		o.CourierID = *courierID
	}
	if tableID != nil {
		o.TableID = *tableID
	}
	if tableLabel != nil {
		o.TableLabel = *tableLabel
	}
	if partySize != nil {
		o.PartySize = *partySize
	}
	if err := json.Unmarshal(itemsJSON, &o.Items); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(historyJSON, &o.History); err != nil {
		return nil, err
	}
	if err := unmarshalAddress(addressJSON, &o.DeliveryAddress); err != nil {
		return nil, err
	}
	return &o, nil
}

// scanOrderRow — bitta qator kutilganda; topilmasa orders.ErrNotFound.
func scanOrderRow(row pgx.Row) (*orders.Order, error) {
	o, err := scanOrder(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, orders.ErrNotFound
	}
	return o, err
}

func (r *PgOrderRepo) ListRecent(ctx context.Context, limit int) ([]*orders.Order, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+orderColumns+` FROM orders ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOrderRows(rows)
}

// ListByRestaurant — restoran paneli va affitsiant ilovasi shu
// ro'yxatni ko'radi.
//
// ┌─ TO'LANMAGAN KARTA BUYURTMASI RO'YXATDA KO'RINMAYDI ──────────────┐
// Mijoz kartani tanlab, to'lov sahifasini yopib yuborishi mumkin.
// Bunday buyurtma restoranga ko'rinsa, xodim uni "Qabul qilish"ga
// urinib xato olardi (o'tish `ChangeStatus` da to'siladi) va zalda
// chalkashlik bo'lardi.
//
// Filtr AYNAN SHU YERDA — so'rov qatlamida: yangi handler qo'shilganda
// ham uni yozishni unutib bo'lmaydi.
// └───────────────────────────────────────────────────────────────────┘
func (r *PgOrderRepo) ListByRestaurant(ctx context.Context, restaurantID string, limit int) ([]*orders.Order, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+orderColumns+` FROM orders
		 WHERE restaurant_id = $1
		   AND NOT (payment_method = 'card' AND payment_state NOT IN ('held','paid'))
		 ORDER BY created_at DESC LIMIT $2`, restaurantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOrderRows(rows)
}

func (r *PgOrderRepo) ListByCustomer(ctx context.Context, customerID string, limit int) ([]*orders.Order, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+orderColumns+` FROM orders WHERE customer_id = $1
		 ORDER BY created_at DESC LIMIT $2`, customerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOrderRows(rows)
}

// CountByCustomerAndRestaurant — bekor qilingan/rad etilganlarni
// HISOBGA OLMASDAN sanaydi (promotions.TypeLoyalty uchun "haqiqiy
// buyurtma bergan" degani, urinib bekor qilinganini emas).
func (r *PgOrderRepo) CountByCustomerAndRestaurant(ctx context.Context, customerID, restaurantID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM orders
		 WHERE customer_id = $1 AND restaurant_id = $2 AND status NOT IN ('cancelled', 'rejected')`,
		customerID, restaurantID).Scan(&count)
	return count, err
}

func scanOrderRows(rows pgx.Rows) ([]*orders.Order, error) {
	var list []*orders.Order
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, o)
	}
	return list, rows.Err()
}

type PgCourierRepo struct{ pool *pgxpool.Pool }

func NewPgCourierRepo(pool *pgxpool.Pool) *PgCourierRepo { return &PgCourierRepo{pool: pool} }

const courierColumns = `id, name, lat, lng, available, approved, vehicle_type, rating, completed_orders`

func (r *PgCourierRepo) GetByID(ctx context.Context, id string) (*couriers.Courier, error) {
	var c couriers.Courier
	err := r.pool.QueryRow(ctx,
		`SELECT `+courierColumns+` FROM couriers WHERE id = $1`, id,
	).Scan(&c.ID, &c.Name, &c.Lat, &c.Lng, &c.Available, &c.Approved,
		&c.VehicleType, &c.Rating, &c.CompletedOrders)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, couriers.ErrNoCourier
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *PgCourierRepo) Create(ctx context.Context, c *couriers.Courier) error {
	if c.VehicleType == "" {
		c.VehicleType = couriers.VehicleMoped
	}
	if c.Rating == 0 {
		c.Rating = 5.0
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO couriers (id, name, lat, lng, available, approved, vehicle_type, rating, completed_orders)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		c.ID, c.Name, c.Lat, c.Lng, c.Available, c.Approved, c.VehicleType, c.Rating, c.CompletedOrders)
	return err
}

func (r *PgCourierRepo) ListAll(ctx context.Context) ([]*couriers.Courier, error) {
	// `deleted_at IS NULL` — o'chirilgan kuryer superadmin ro'yxatida
	// ham ko'rinmaydi (yozuv faqat buyurtma tarixi uchun qoladi,
	// qarang: `SoftDelete`).
	rows, err := r.pool.Query(ctx,
		`SELECT `+courierColumns+` FROM couriers WHERE deleted_at IS NULL ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCouriers(rows)
}

// IncrementCompletedOrders — buyurtma "delivered" bo'lganda +1. Haqiqiy,
// obyektiv tajriba hisoblagichi — ScoreCandidates shundan foydalanadi.
func (r *PgCourierRepo) IncrementCompletedOrders(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE couriers SET completed_orders = completed_orders + 1, updated_at = now() WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return couriers.ErrNoCourier
	}
	return nil
}

// SoftDelete — interfeys izohiga qarang (`couriers.Repository`).
//
// Ism ATAYLAB bo'sh satrga emas, aniq belgiga almashtiriladi: buyurtma
// tarixida "kuryer: —" o'rniga "o'chirilgan akkaunt" ko'rinishi kerak,
// aks holda ma'lumot yo'qolgandek tuyulardi.
func (r *PgCourierRepo) SoftDelete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE couriers
		    SET deleted_at = now(), name = 'O''chirilgan kuryer',
		        available = FALSE, approved = FALSE, updated_at = now()
		  WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return couriers.ErrNoCourier
	}
	return nil
}

func (r *PgCourierRepo) SetApproved(ctx context.Context, id string, approved bool) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE couriers SET approved = $2, updated_at = now() WHERE id = $1`, id, approved)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return couriers.ErrNoCourier
	}
	return nil
}

// ListAvailable — barcha tasdiqlangan va onlayn kuryerlar — bu FAQAT
// nomzodlar havuzi. Ularning qaysi biriga birinchi navbatda taklif
// yuborilishi ETA/reyting/tajriba asosida ScoreCandidates orqali
// hisoblanadi (dispatch.go).
func (r *PgCourierRepo) ListAvailable(ctx context.Context) ([]*couriers.Courier, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+courierColumns+`
		FROM couriers
		WHERE available AND approved AND deleted_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCouriers(rows)
}

// ListAvailableNear — nomzodlarni YAQINLIK bo'yicha DB darajasida
// tanlaydi (migration 0029).
//
// ┌─ SO'ROV NEGA AYNAN SHUNDAY ───────────────────────────────────────┐
//
//	ST_MakePoint($2, $1)  — argumentlar (LNG, LAT), ya'ni X, Y.
//	                        Almashtirilsa kod xatosiz ishlaydi, lekin
//	                        kuryerlar butunlay boshqa joyda bo'ladi.
//	                        Test bilan qoplangan.
//	::geography           — metrlarda hisoblash uchun (geometry bo'lsa
//	                        natija GRADUSDA chiqardi va radius ma'nosiz
//	                        bo'lardi).
//	ST_DWithin            — GiST indeksdan foydalanadi (`&&` +
//	                        `_st_expand`). `ST_Distance(...) < r`
//	                        yozilsa indeks ISHLATILMASDI va har so'rov
//	                        to'liq jadval skanerlashga aylanardi.
//	ORDER BY <->          — eng yaqinidan boshlab.
//	LIMIT                 — Google Distance Matrix pullik, nomzodlar
//	                        soni CHEKLANISHI shart.
//
// └───────────────────────────────────────────────────────────────────┘
func (r *PgCourierRepo) ListAvailableNear(ctx context.Context, lat, lng float64,
	radiusMeters float64, maxAge time.Duration, limit int) ([]*couriers.Courier, error) {

	// Chegaralar — chaqiruvchi xato qiymat bersa ham so'rov xavfsiz
	// qolsin (LIMIT 0 hech narsa qaytarmasdi, manfiy radius esa
	// PostGIS'da xatoga olib kelardi).
	if limit <= 0 {
		limit = 20
	}
	if radiusMeters <= 0 {
		radiusMeters = 5000
	}

	// maxAge = 0 -> eskilik tekshirilmaydi. `$5::interval` NULL
	// bo'lganda shart o'z-o'zidan TRUE bo'ladi.
	var age any
	if maxAge > 0 {
		age = fmt.Sprintf("%d seconds", int(maxAge.Seconds()))
	}

	rows, err := r.pool.Query(ctx, `
		SELECT `+courierColumns+`
		FROM couriers
		WHERE available AND approved AND deleted_at IS NULL
		  AND ($5::interval IS NULL
		       OR location_updated_at IS NULL
		       OR location_updated_at > now() - $5::interval)
		  AND ST_DWithin(
		        location,
		        ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography,
		        $3)
		ORDER BY location <-> ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography
		LIMIT $4`,
		lat, lng, radiusMeters, limit, age)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCouriers(rows)
}

func scanCouriers(rows pgx.Rows) ([]*couriers.Courier, error) {
	var list []*couriers.Courier
	for rows.Next() {
		var c couriers.Courier
		if err := rows.Scan(&c.ID, &c.Name, &c.Lat, &c.Lng, &c.Available, &c.Approved,
			&c.VehicleType, &c.Rating, &c.CompletedOrders); err != nil {
			return nil, err
		}
		list = append(list, &c)
	}
	return list, rows.Err()
}

func (r *PgCourierRepo) SetAvailable(ctx context.Context, id string, available bool) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE couriers SET available = $2, updated_at = now() WHERE id = $1`, id, available)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return couriers.ErrNoCourier
	}
	return nil
}

// ClaimIfAvailable — shartli UPDATE (`WHERE available = true`). Postgres
// qatorni yangilash paytida qulflaydi, shuning uchun bir vaqtda kelgan
// ikki chaqiruvdan FAQAT bittasi 1 qator yangilaydi — ikkinchisi 0
// oladi va `false` qaytaradi.
//
// `approved = true` sharti ham SHU YERDA: taklif yuborilgandan keyin,
// lekin kuryer "Qabul qilaman" bosgunga qadar superadmin uni bloklashi
// mumkin (tor, lekin haqiqiy oyna) — busiz bloklangan kuryer buyurtmani
// baribir olib ketardi. Tekshiruvni aynan shu atomik UPDATE ichiga
// qo'yish alohida so'rovdan ko'ra ishonchli (yana race qolmaydi).
func (r *PgCourierRepo) ClaimIfAvailable(ctx context.Context, id string) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE couriers SET available = false, updated_at = now()
		 WHERE id = $1 AND available = true AND approved = true`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// UpdateLocation — `location` ustuni GENERATED, ya'ni `lat`/`lng`
// yozilishi bilan O'ZI yangilanadi (migration 0029). Alohida yozish
// SHART EMAS va mumkin ham emas.
//
// `location_updated_at` esa ALOHIDA yangilanadi: `updated_at` boshqa
// amallarda ham o'zgaradi va "joylashuv yangimi?" savoliga javob bera
// olmaydi (0029 dagi izoh).
func (r *PgCourierRepo) UpdateLocation(ctx context.Context, id string, lat, lng float64) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE couriers
		    SET lat = $2, lng = $3,
		        location_updated_at = now(),
		        updated_at = now()
		  WHERE id = $1`, id, lat, lng)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return couriers.ErrNoCourier
	}
	return nil
}

// SeedDemoCouriers — demo kuryerlar (faqat dev muhit uchun; bor bo'lsa tegmaydi).
// Turli transport turlarida — dispatch matching engine'ni haqiqiy sharoitda
// (turli ETA rejimlari bilan) sinash uchun.
func SeedDemoCouriers(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO couriers (id, name, lat, lng, available, approved, vehicle_type, rating, completed_orders) VALUES
			('c1', 'Aziz',    41.0056, 71.2378, TRUE, TRUE, 'moped', 5.0, 0),
			('c2', 'Bekzod',  41.0010, 71.2400, TRUE, TRUE, 'bike',  5.0, 0),
			('c3', 'Doniyor', 40.9980, 71.2330, TRUE, TRUE, 'foot',  5.0, 0)
		ON CONFLICT (id) DO NOTHING`)
	if err != nil {
		return err
	}
	// Demo kuryerlarning joylashuvini "yangi", tasdig'ini esa qayta
	// TRUE qilib belgilaymiz.
	//
	// JOYLASHUV — dispatch endi joylashuvi `locationMaxAge` dan eski
	// bo'lgan kuryerni nomzod qilmaydi (migration 0029). Seed bir
	// marta bajarilgani uchun demo kuryerlar bir necha kundan keyin
	// "eskirgan" bo'lib qolardi va DEV muhitda dispatch hech kimni
	// topa olmasdi — sabab esa umuman ko'rinmasdi.
	//
	// ┌─ TASDIQ NEGA SHU YERDA (bug.md 98-band) ──────────────────────┐
	// Avval demo kuryerlarni `0004_courier_approval.sql` migratsiyasi
	// tasdiqlardi — ya'ni PRODUCTION'da ham. Bu xavfsizlik qarorini
	// migratsiyaga topshirish noto'g'ri edi (tasdiqlangan kuryer
	// mijoz manzilini va telefonini ko'radi).
	//
	// Endi u o'sha migratsiyadan olib tashlandi va
	// `0041_revoke_demo_courier_approval.sql` mavjud bazalarda
	// orqaga qaytaradi. Demo yozuvlarning holati esa AYNAN shu
	// yerga — DEV-ONLY seed'ga ko'chdi (`SeedDemoCouriers` faqat
	// `APP_ENV=development` da chaqiriladi).
	//
	// `INSERT ... DO NOTHING` yetarli emas: mavjud dev bazasida
	// yozuvlar allaqachon bor va 0041 ularni `FALSE` qilib qo'yadi.
	// Shuning uchun tasdiq har seed'da QAYTA qo'yiladi.
	// └───────────────────────────────────────────────────────────────┘
	//
	// Faqat DEMO ID'lar (c1..c3) — haqiqiy kuryerlarga tegmaydi.
	_, err = pool.Exec(ctx, `
		UPDATE couriers
		   SET location_updated_at = now(),
		       approved = TRUE
		 WHERE id IN ('c1','c2','c3')`)
	return err
}
