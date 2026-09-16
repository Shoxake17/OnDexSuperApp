package storage

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/tracking"
)

// PgRouteRepo — yetkazish yo'llari (migration 0053, `order_routes`).
type PgRouteRepo struct{ pool *pgxpool.Pool }

func NewPgRouteRepo(pool *pgxpool.Pool) *PgRouteRepo { return &PgRouteRepo{pool: pool} }

func (r *PgRouteRepo) GetRoute(ctx context.Context, orderID string) (*tracking.Route, error) {
	var rt tracking.Route
	var raw []byte
	err := r.pool.QueryRow(ctx, `
		SELECT order_id, origin_lat, origin_lng, dest_lat, dest_lng,
		       points, distance_meters, duration_seconds, created_at
		FROM order_routes WHERE order_id = $1`, orderID,
	).Scan(&rt.OrderID, &rt.Origin.Lat, &rt.Origin.Lng, &rt.Destination.Lat, &rt.Destination.Lng,
		&raw, &rt.DistanceMeters, &rt.DurationSeconds, &rt.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, tracking.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &rt.Points); err != nil {
		return nil, err
	}
	return &rt, nil
}

func (r *PgRouteRepo) SaveRoute(ctx context.Context, rt *tracking.Route) error {
	if err := rt.Validate(); err != nil {
		return err
	}
	points := rt.Points
	if points == nil {
		points = []tracking.Point{}
	}
	raw, err := json.Marshal(points)
	if err != nil {
		return err
	}
	createdAt := rt.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO order_routes (order_id, origin_lat, origin_lng, dest_lat, dest_lng,
		                          points, distance_meters, duration_seconds, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (order_id) DO NOTHING`,
		rt.OrderID, rt.Origin.Lat, rt.Origin.Lng, rt.Destination.Lat, rt.Destination.Lng,
		raw, rt.DistanceMeters, rt.DurationSeconds, createdAt)
	return err
}
