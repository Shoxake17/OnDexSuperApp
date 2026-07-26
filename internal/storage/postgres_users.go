package storage

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/users"
)

type PgUserRepo struct{ pool *pgxpool.Pool }

func NewPgUserRepo(pool *pgxpool.Pool) *PgUserRepo { return &PgUserRepo{pool: pool} }

func (r *PgUserRepo) GetByPhone(ctx context.Context, phone string) (*users.User, error) {
	return r.getBy(ctx, "phone", phone)
}

func (r *PgUserRepo) GetByID(ctx context.Context, id string) (*users.User, error) {
	return r.getBy(ctx, "id", id)
}

func (r *PgUserRepo) getBy(ctx context.Context, col, val string) (*users.User, error) {
	var u users.User
	err := r.pool.QueryRow(ctx,
		`SELECT id, phone, name, role, entity_id, created_at FROM users WHERE `+col+` = $1`, val,
	).Scan(&u.ID, &u.Phone, &u.Name, &u.Role, &u.EntityID, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, users.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *PgUserRepo) Create(ctx context.Context, u *users.User) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO users (id, phone, name, role, entity_id, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		u.ID, u.Phone, u.Name, u.Role, u.EntityID, u.CreatedAt)
	return err
}

func (r *PgUserRepo) UpdateRole(ctx context.Context, id string, role users.Role, entityID string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET role = $2, entity_id = $3 WHERE id = $1`, id, role, entityID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return users.ErrUserNotFound
	}
	return nil
}

func (r *PgUserRepo) DeleteByRoleEntity(ctx context.Context, role users.Role, entityID string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM users WHERE role = $1 AND entity_id = $2`, role, entityID)
	return err
}

func (r *PgUserRepo) ListByRole(ctx context.Context, role users.Role) ([]*users.User, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, phone, name, role, entity_id, created_at FROM users WHERE role = $1 ORDER BY created_at`, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*users.User
	for rows.Next() {
		var u users.User
		if err := rows.Scan(&u.ID, &u.Phone, &u.Name, &u.Role, &u.EntityID, &u.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, &u)
	}
	return list, rows.Err()
}

type PgCodeStore struct{ pool *pgxpool.Pool }

func NewPgCodeStore(pool *pgxpool.Pool) *PgCodeStore { return &PgCodeStore{pool: pool} }

func (s *PgCodeStore) Save(ctx context.Context, c *users.Code) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO phone_codes (phone, code_hash, expires_at, created_at, attempts)
		 VALUES ($1,$2,$3,$4,0)
		 ON CONFLICT (phone) DO UPDATE SET
			code_hash = EXCLUDED.code_hash,
			expires_at = EXCLUDED.expires_at,
			created_at = EXCLUDED.created_at,
			attempts = 0`,
		c.Phone, c.CodeHash, c.ExpiresAt, c.CreatedAt)
	return err
}

func (s *PgCodeStore) Get(ctx context.Context, phone string) (*users.Code, error) {
	var c users.Code
	err := s.pool.QueryRow(ctx,
		`SELECT phone, code_hash, expires_at, created_at, attempts FROM phone_codes WHERE phone = $1`, phone,
	).Scan(&c.Phone, &c.CodeHash, &c.ExpiresAt, &c.CreatedAt, &c.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, users.ErrInvalidCode
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *PgCodeStore) IncrementAttempts(ctx context.Context, phone string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE phone_codes SET attempts = attempts + 1 WHERE phone = $1`, phone)
	return err
}

func (s *PgCodeStore) Delete(ctx context.Context, phone string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM phone_codes WHERE phone = $1`, phone)
	return err
}

// SeedDemoUsers — dev muhit uchun tayyor rollar: 3 kuryer, 1 restoran, 1 admin.
// Mijozlar seed qilinmaydi — ular telefon orqali o'zi ro'yxatdan o'tadi.
func SeedDemoUsers(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, phone, name, role, entity_id) VALUES
			('u_c1',    '+998900000001', 'Aziz (kuryer)',     'courier',    'c1'),
			('u_c2',    '+998900000002', 'Bekzod (kuryer)',   'courier',    'c2'),
			('u_c3',    '+998900000003', 'Doniyor (kuryer)',  'courier',    'c3'),
			('u_rest1', '+998900000010', 'Chust Osh Markazi', 'restaurant', 'r1'),
			('u_admin', '+998900000099', 'Admin',             'admin',      '')
		ON CONFLICT (id) DO NOTHING`)
	return err
}

// DemoUsers — in-memory rejim uchun xuddi shu seed ro'yxati.
func DemoUsers() []users.User {
	return []users.User{
		{ID: "u_c1", Phone: "+998900000001", Name: "Aziz (kuryer)", Role: users.RoleCourier, EntityID: "c1"},
		{ID: "u_c2", Phone: "+998900000002", Name: "Bekzod (kuryer)", Role: users.RoleCourier, EntityID: "c2"},
		{ID: "u_c3", Phone: "+998900000003", Name: "Doniyor (kuryer)", Role: users.RoleCourier, EntityID: "c3"},
		{ID: "u_rest1", Phone: "+998900000010", Name: "Chust Osh Markazi", Role: users.RoleRestaurant, EntityID: "r1"},
		{ID: "u_admin", Phone: "+998900000099", Name: "Admin", Role: users.RoleAdmin},
	}
}
