package tablebill

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGStore — Postgres ombori (`tablebill` sxemasi, 0078).
type PGStore struct{ pool *pgxpool.Pool }

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

const billCols = `id, restaurant_id, table_id, table_label, status, deposit_reservation_id, deposit_tiyin,
	created_at, updated_at, closed_at, version`

func scanBill(row pgx.Row) (*Bill, error) {
	var b Bill
	err := row.Scan(&b.ID, &b.RestaurantID, &b.TableID, &b.TableLabel, &b.Status, &b.DepositReservationID,
		&b.DepositTiyin, &b.CreatedAt, &b.UpdatedAt, &b.ClosedAt, &b.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &b, err
}

func (p *PGStore) GetOpen(ctx context.Context, tableID string) (*Bill, error) {
	return scanBill(p.pool.QueryRow(ctx, `SELECT `+billCols+` FROM tablebill.bills WHERE table_id = $1 AND status = 'open'`, tableID))
}

func (p *PGStore) CreateOpen(ctx context.Context, b *Bill) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO tablebill.bills (`+billCols+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		b.ID, b.RestaurantID, b.TableID, b.TableLabel, b.Status, b.DepositReservationID, b.DepositTiyin,
		b.CreatedAt, b.UpdatedAt, b.ClosedAt, b.Version)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return ErrConflict
	}
	return err
}

func (p *PGStore) Get(ctx context.Context, id string) (*Bill, error) {
	return scanBill(p.pool.QueryRow(ctx, `SELECT `+billCols+` FROM tablebill.bills WHERE id = $1`, id))
}

func (p *PGStore) ListOpen(ctx context.Context, restaurantID string) ([]*Bill, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+billCols+` FROM tablebill.bills
		WHERE restaurant_id = $1 AND status = 'open' ORDER BY created_at LIMIT 500`, restaurantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Bill
	for rows.Next() {
		b, err := scanBill(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (p *PGStore) Attach(ctx context.Context, billID string, orderIDs []string) error {
	if len(orderIDs) == 0 {
		return nil
	}
	_, err := p.pool.Exec(ctx, `INSERT INTO tablebill.bill_orders (bill_id, order_id)
		SELECT $1, unnest($2::text[]) ON CONFLICT (order_id) DO NOTHING`, billID, orderIDs)
	return err
}

func (p *PGStore) OrderIDs(ctx context.Context, billID string) ([]string, error) {
	rows, err := p.pool.Query(ctx, `SELECT order_id FROM tablebill.bill_orders WHERE bill_id = $1`, billID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func (p *PGStore) Attached(ctx context.Context, orderIDs []string) (map[string]bool, error) {
	out := map[string]bool{}
	if len(orderIDs) == 0 {
		return out, nil
	}
	rows, err := p.pool.Query(ctx, `SELECT order_id FROM tablebill.bill_orders WHERE order_id = ANY($1)`, orderIDs)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	for _, id := range ids {
		out[id] = true
	}
	return out, err
}

func (p *PGStore) SetDeposit(ctx context.Context, billID, reservationID string, amount int64) error {
	tag, err := p.pool.Exec(ctx, `UPDATE tablebill.bills SET deposit_reservation_id = $2, deposit_tiyin = $3,
		version = version + 1 WHERE id = $1 AND deposit_reservation_id = ''`, billID, reservationID, amount)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrConflict
	}
	return err
}

const shareCols = `id, bill_id, payer_id, kind, method, amount_tiyin, tip_tiyin, items, waiter_id, waiter_name,
	status, created_at, paid_at`

func scanShare(row pgx.Row) (*Share, error) {
	var s Share
	var items []byte
	err := row.Scan(&s.ID, &s.BillID, &s.PayerID, &s.Kind, &s.Method, &s.AmountTiyin, &s.TipTiyin, &items,
		&s.WaiterID, &s.WaiterName, &s.Status, &s.CreatedAt, &s.PaidAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if len(items) > 0 {
		if err := json.Unmarshal(items, &s.Items); err != nil {
			return nil, err
		}
	}
	return &s, nil
}

func (p *PGStore) Shares(ctx context.Context, billID string) ([]*Share, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+shareCols+` FROM tablebill.shares WHERE bill_id = $1 ORDER BY created_at`, billID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Share
	for rows.Next() {
		s, err := scanShare(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (p *PGStore) GetShare(ctx context.Context, id string) (*Share, error) {
	return scanShare(p.pool.QueryRow(ctx, `SELECT `+shareCols+` FROM tablebill.shares WHERE id = $1`, id))
}

// AddShare — bitta tranzaksiyada: hisob versiyasi tekshiriladi va oshiriladi,
// keyin ulush yoziladi (parallel ulushlar bir qatorni ikki marta band qilmasin).
func (p *PGStore) AddShare(ctx context.Context, s *Share, version int64) error {
	items, err := json.Marshal(s.Items)
	if err != nil {
		return err
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `UPDATE tablebill.bills SET version = version + 1, updated_at = $3
		WHERE id = $1 AND version = $2 AND status = 'open'`, s.BillID, version, s.CreatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	if _, err := tx.Exec(ctx, `INSERT INTO tablebill.shares (`+shareCols+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		s.ID, s.BillID, s.PayerID, s.Kind, s.Method, s.AmountTiyin, s.TipTiyin, items, s.WaiterID, s.WaiterName,
		s.Status, s.CreatedAt, s.PaidAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *PGStore) SetShareStatus(ctx context.Context, id, from, to string, paidAt *time.Time) error {
	tag, err := p.pool.Exec(ctx, `UPDATE tablebill.shares SET status = $3, paid_at = $4 WHERE id = $1 AND status = $2`,
		id, from, to, paidAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	_, err = p.pool.Exec(ctx, `UPDATE tablebill.bills SET version = version + 1
		WHERE id = (SELECT bill_id FROM tablebill.shares WHERE id = $1)`, id)
	return err
}

func (p *PGStore) Close(ctx context.Context, billID string, at time.Time) error {
	_, err := p.pool.Exec(ctx, `UPDATE tablebill.bills SET status = 'paid', closed_at = $2, updated_at = $2,
		version = version + 1 WHERE id = $1 AND status = 'open'`, billID, at)
	return err
}

func (p *PGStore) Tips(ctx context.Context, restaurantID string, from, to time.Time) ([]WaiterTips, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT s.waiter_name, SUM(s.tip_tiyin), COUNT(*)
		FROM tablebill.shares s JOIN tablebill.bills b ON b.id = s.bill_id
		WHERE b.restaurant_id = $1 AND s.status = 'paid' AND s.tip_tiyin > 0
		  AND s.paid_at >= $2 AND s.paid_at < $3
		GROUP BY s.waiter_name ORDER BY 2 DESC LIMIT 200`, restaurantID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WaiterTips
	for rows.Next() {
		var w WaiterTips
		if err := rows.Scan(&w.WaiterName, &w.TipsTiyin, &w.Count); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}