// PostgreSQL implementatsiyalari. DATABASE_URL berilganda main shularni ishlatadi.
package storage

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/couriers"
	"chustapp/internal/orders"
)

type PgOrderRepo struct{ pool *pgxpool.Pool }

func NewPgOrderRepo(pool *pgxpool.Pool) *PgOrderRepo { return &PgOrderRepo{pool: pool} }

func (r *PgOrderRepo) GetByID(ctx context.Context, id string) (*orders.Order, error) {
	var o orders.Order
	var courierID *string
	var itemsJSON, historyJSON []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, customer_id, restaurant_id, courier_id, status, total_tiyin,
		       delivery_lat, delivery_lng, items, history, created_at, updated_at
		FROM orders WHERE id = $1`, id,
	).Scan(&o.ID, &o.CustomerID, &o.RestaurantID, &courierID, &o.Status, &o.TotalTiyin,
		&o.DeliveryLat, &o.DeliveryLng, &itemsJSON, &historyJSON, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, orders.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if courierID != nil {
		o.CourierID = *courierID
	}
	if err := json.Unmarshal(itemsJSON, &o.Items); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(historyJSON, &o.History); err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *PgOrderRepo) Save(ctx context.Context, o *orders.Order) error {
	itemsJSON, err := json.Marshal(o.Items)
	if err != nil {
		return err
	}
	historyJSON, err := json.Marshal(o.History)
	if err != nil {
		return err
	}
	var courierID *string
	if o.CourierID != "" {
		courierID = &o.CourierID
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO orders (id, customer_id, restaurant_id, courier_id, status, total_tiyin,
		                    delivery_lat, delivery_lng, items, history, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (id) DO UPDATE SET
			courier_id = EXCLUDED.courier_id,
			status     = EXCLUDED.status,
			history    = EXCLUDED.history,
			updated_at = EXCLUDED.updated_at`,
		o.ID, o.CustomerID, o.RestaurantID, courierID, o.Status, o.TotalTiyin,
		o.DeliveryLat, o.DeliveryLng, itemsJSON, historyJSON, o.CreatedAt, o.UpdatedAt)
	return err
}

func (r *PgOrderRepo) HasActiveByRestaurant(ctx context.Context, restaurantID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM orders
			WHERE restaurant_id = $1
			  AND status NOT IN ('delivered', 'rejected', 'cancelled')
		)`, restaurantID).Scan(&exists)
	return exists, err
}

func (r *PgOrderRepo) ListRecent(ctx context.Context, limit int) ([]*orders.Order, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, customer_id, restaurant_id, courier_id, status, total_tiyin,
		       delivery_lat, delivery_lng, items, history, created_at, updated_at
		FROM orders ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*orders.Order
	for rows.Next() {
		var o orders.Order
		var courierID *string
		var itemsJSON, historyJSON []byte
		if err := rows.Scan(&o.ID, &o.CustomerID, &o.RestaurantID, &courierID, &o.Status, &o.TotalTiyin,
			&o.DeliveryLat, &o.DeliveryLng, &itemsJSON, &historyJSON, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		if courierID != nil {
			o.CourierID = *courierID
		}
		if err := json.Unmarshal(itemsJSON, &o.Items); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(historyJSON, &o.History); err != nil {
			return nil, err
		}
		list = append(list, &o)
	}
	return list, rows.Err()
}

type PgCourierRepo struct{ pool *pgxpool.Pool }

func NewPgCourierRepo(pool *pgxpool.Pool) *PgCourierRepo { return &PgCourierRepo{pool: pool} }

func (r *PgCourierRepo) GetByID(ctx context.Context, id string) (*couriers.Courier, error) {
	var c couriers.Courier
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, lat, lng, available, approved FROM couriers WHERE id = $1`, id,
	).Scan(&c.ID, &c.Name, &c.Lat, &c.Lng, &c.Available, &c.Approved)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, couriers.ErrNoCourier
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *PgCourierRepo) Create(ctx context.Context, c *couriers.Courier) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO couriers (id, name, lat, lng, available, approved)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		c.ID, c.Name, c.Lat, c.Lng, c.Available, c.Approved)
	return err
}

func (r *PgCourierRepo) ListAll(ctx context.Context) ([]*couriers.Courier, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, lat, lng, available, approved FROM couriers ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCouriers(rows)
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

func (r *PgCourierRepo) FindNearby(ctx context.Context, lat, lng float64, limit int) ([]*couriers.Courier, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, lat, lng, available, approved
		FROM couriers
		WHERE available AND approved
		ORDER BY ST_DistanceSphere(ST_MakePoint(lng, lat), ST_MakePoint($2, $1))
		LIMIT $3`, lat, lng, limit)
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
		if err := rows.Scan(&c.ID, &c.Name, &c.Lat, &c.Lng, &c.Available, &c.Approved); err != nil {
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

// SeedDemoCouriers — demo kuryerlar (faqat dev muhit uchun; bor bo'lsa tegmaydi).
func SeedDemoCouriers(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO couriers (id, name, lat, lng, available, approved) VALUES
			('c1', 'Aziz',    41.0056, 71.2378, TRUE, TRUE),
			('c2', 'Bekzod',  41.0010, 71.2400, TRUE, TRUE),
			('c3', 'Doniyor', 40.9980, 71.2330, TRUE, TRUE)
		ON CONFLICT (id) DO NOTHING`)
	return err
}
