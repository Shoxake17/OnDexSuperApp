package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

// `telegram_id` NULL bo'lishi mumkin (foydalanuvchilarning aksariyati
// Telegram bog'lamaydi). `COALESCE(...,0)` bilan u oddiy `int64` ga
// o'qiladi — 0 = bog'lanmagan. Busiz har bir o'qishda `sql.NullInt64`
// ishlatib, uni har joyda ochish kerak bo'lardi.
const userColumns = `id, phone, name, role, entity_id, created_at,
	address_lat, address_lng, address_text, address_entrance, address_floor,
	address_apartment, address_intercom, address_comment,
	first_name, last_name, email, password_hash, phone_verified, email_verified,
	COALESCE(telegram_id, 0)`

func scanUser(row pgx.Row, u *users.User) error {
	return row.Scan(&u.ID, &u.Phone, &u.Name, &u.Role, &u.EntityID, &u.CreatedAt,
		&u.Address.Lat, &u.Address.Lng, &u.Address.Text, &u.Address.Entrance,
		&u.Address.Floor, &u.Address.Apartment, &u.Address.Intercom, &u.Address.Comment,
		&u.FirstName, &u.LastName, &u.Email, &u.PasswordHash, &u.PhoneVerified,
		&u.EmailVerified, &u.TelegramID)
}

// GetByTelegramID — Telegram Mini App kirishi (migration 0031).
func (r *PgUserRepo) GetByTelegramID(ctx context.Context, telegramID int64) (*users.User, error) {
	if telegramID == 0 {
		return nil, users.ErrUserNotFound
	}
	var u users.User
	err := scanUser(r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE telegram_id = $1`, telegramID), &u)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, users.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// LinkTelegram — telegram_id ni foydalanuvchiga bog'laydi.
//
// ┌─ IKKI QADAM, BITTA TRANZAKSIYADA ─────────────────────────────────┐
// 1. Shu telegram_id BOSHQA foydalanuvchida bo'lsa — uzib qo'yamiz;
// 2. So'ng joriy foydalanuvchiga yozamiz.
//
// Birinchi qadamsiz `UNIQUE` indeks yozishni rad etardi va bog'lanish
// ESKI egasida qolib ketardi — ya'ni odam Mini App'da BEGONA hisobga
// tushardi. Bu telefon raqami boshqa egaga o'tganda (O'zbekistonda
// tez-tez uchraydi) real holat.
//
// Tranzaksiya: ikkala amal orasida boshqa so'rov kirsa, yarim
// bog'langan holat qolardi.
// └───────────────────────────────────────────────────────────────────┘
func (r *PgUserRepo) LinkTelegram(ctx context.Context, userID string, telegramID int64) error {
	if telegramID == 0 {
		return fmt.Errorf("LinkTelegram: telegram_id bo'sh")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Commit muvaffaqiyatli bo'lsa no-op

	if _, err := tx.Exec(ctx,
		`UPDATE users SET telegram_id = NULL WHERE telegram_id = $1 AND id <> $2`,
		telegramID, userID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx,
		`UPDATE users SET telegram_id = $2 WHERE id = $1`, userID, telegramID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return users.ErrUserNotFound
	}
	return tx.Commit(ctx)
}

// getBy — foydalanuvchini telefon yoki ID bo'yicha o'qiydi.
//
// So'rov matni ustun nomi bilan KONKATENATSIYA QILINMAYDI. Avval
// `WHERE `+col+` = $1` shaklida edi: qiymat parametrlangani uchun
// amalda ekspluatatsiya qilib bo'lmasdi (`col` har doim kodda yozilgan
// literal), lekin bu SQL-injection naqshining o'zi — kelajakda kimdir
// `col`ni foydalanuvchi kiritmasidan berib yuborsa, zaiflik jimgina
// paydo bo'lardi. Endi ruxsat etilgan har bir ustun uchun TO'LIQ
// alohida, o'zgarmas so'rov ishlatiladi.
func (r *PgUserRepo) getBy(ctx context.Context, col, val string) (*users.User, error) {
	var query string
	switch col {
	case "phone":
		query = `SELECT ` + userColumns + ` FROM users WHERE phone = $1`
	case "id":
		query = `SELECT ` + userColumns + ` FROM users WHERE id = $1`
	case "email":
		// Email REGISTRGA BOG'LIQ EMAS taqqoslanadi — "Ali@mail.uz" va
		// "ali@mail.uz" bitta akkaunt. Indeks ham `lower(email)` bo'yicha
		// (migratsiya 0026), shuning uchun so'rov indeksdan foydalanadi.
		query = `SELECT ` + userColumns + ` FROM users WHERE lower(email) = lower($1) AND email <> ''`
	default:
		return nil, fmt.Errorf("getBy: ruxsat etilmagan ustun %q", col)
	}
	var u users.User
	err := scanUser(r.pool.QueryRow(ctx, query, val), &u)
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
		`INSERT INTO users (id, phone, name, role, entity_id, created_at,
		                    first_name, last_name, email, password_hash, phone_verified,
		                    email_verified)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		u.ID, u.Phone, u.Name, u.Role, u.EntityID, u.CreatedAt,
		u.FirstName, u.LastName, u.Email, u.PasswordHash, u.PhoneVerified,
		u.EmailVerified)
	return mapUserConstraint(err)
}

// mapUserConstraint — DB unikal cheklovlarini ANIQ xatolarga aylantiradi.
//
// Busiz takroriy telefon/email `23505` xom holda chiqib, HTTP 500 va
// constraint matnini (ya'ni ichki sxemani) foydalanuvchiga ko'rsatardi.
func mapUserConstraint(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		if strings.Contains(pgErr.ConstraintName, "email") {
			return users.ErrEmailTaken
		}
		return users.ErrPhoneTaken
	}
	return err
}

func (r *PgUserRepo) GetByEmail(ctx context.Context, email string) (*users.User, error) {
	return r.getBy(ctx, "email", email)
}

func (r *PgUserRepo) UpdateProfile(ctx context.Context, id string, p users.ProfileUpdate) error {
	// COALESCE naqshi: nil uzatilgan maydon TEGILMAYDI, ya'ni "ismni
	// yangilash" parolni tasodifan o'chirib yubormaydi.
	tag, err := r.pool.Exec(ctx, `
		UPDATE users SET
			first_name    = COALESCE($2, first_name),
			last_name     = COALESCE($3, last_name),
			email         = COALESCE($4, email),
			password_hash = COALESCE($5, password_hash),
			-- name ustuni FAQAT ism yoki familiya berilganda qayta
			-- hisoblanadi. Busiz "faqat parolni yangilash" chaqiruvi
			-- ham nomni first/last dan qayta yigib, admin yaratgan
			-- (nomi bor, first/last si bosh) restoran/kuryer
			-- akkauntlarining nomini ochirib yuborardi.
			name          = CASE WHEN $2::text IS NULL AND $3::text IS NULL THEN name
			                     ELSE TRIM(COALESCE($2, first_name) || ' ' || COALESCE($3, last_name))
			                END
		WHERE id = $1`,
		id, p.FirstName, p.LastName, p.Email, p.PasswordHash)
	if err != nil {
		return mapUserConstraint(err)
	}
	if tag.RowsAffected() == 0 {
		return users.ErrUserNotFound
	}
	return nil
}

// SetPasswordHash — FAQAT `password_hash`. `UpdateProfile` dan farqli
// o'laroq `name` ni qayta hisoblamaydi (u yerdagi COALESCE naqshi
// admin yaratgan, `first_name`/`last_name` si bo'sh akkauntlarning
// nomini o'chirib yuborardi).
func (r *PgUserRepo) SetPasswordHash(ctx context.Context, id, hash string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET password_hash = $2 WHERE id = $1`, id, hash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return users.ErrUserNotFound
	}
	return nil
}

// DisabledAITools — foydalanuvchi o'chirgan yordamchi amallari.
//
// Foydalanuvchi topilmasa BO'SH ro'yxat qaytadi, xato emas: chaqiruvchi
// uchun "hech narsa o'chirilmagan" bilan "foydalanuvchi yo'q" bir xil
// natija beradi (ikkalasida ham amal ro'yxati qisqartirilmaydi), va
// yordamchi tokeni allaqachon tekshirilgan bo'ladi.
func (r *PgUserRepo) DisabledAITools(ctx context.Context, id string) ([]string, error) {
	var raw string
	err := r.pool.QueryRow(ctx,
		`SELECT ai_disabled_tools FROM users WHERE id = $1`, id).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return splitTools(raw), nil
}

func (r *PgUserRepo) SetDisabledAITools(ctx context.Context, id string, tools []string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET ai_disabled_tools = $2 WHERE id = $1`,
		id, strings.Join(tools, ","))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return users.ErrUserNotFound
	}
	return nil
}

// splitTools — bo'sh elementlarsiz ajratish. Bo'sh satr `[""]` emas,
// `nil` bo'lishi kerak, aks holda "" nomli amal o'chirilgan hisoblanardi.
func splitTools(raw string) []string {
	out := make([]string, 0, 4)
	for _, s := range strings.Split(raw, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (r *PgUserRepo) MarkEmailVerified(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET email_verified = TRUE WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return users.ErrUserNotFound
	}
	return nil
}

func (r *PgUserRepo) MarkPhoneVerified(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET phone_verified = TRUE WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return users.ErrUserNotFound
	}
	return nil
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

func (r *PgUserRepo) DeleteByRoleEntity(ctx context.Context, role users.Role, entityID string) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`DELETE FROM users WHERE role = $1 AND entity_id = $2 RETURNING id`, role, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Delete — akkauntni va unga bog'langan SHAXSIY ma'lumotlarni
// o'chiradi (interfeys izohiga qarang).
//
// TRANZAKSIYA SHART: `favorites` alohida o'chiriladi va u
// foydalanuvchi qatoridan KEYIN o'chirilsa, oradagi nosozlik
// "yetim" sevimlilarni qoldirardi — ular hech qachon ko'rinmaydi,
// lekin bazada abadiy qoladi.
func (r *PgUserRepo) Delete(ctx context.Context, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// `favorites.customer_id` da FOREIGN KEY yo'q — cascade ishlamaydi.
	if _, err := tx.Exec(ctx, `DELETE FROM favorites WHERE customer_id = $1`, id); err != nil {
		return err
	}
	// Qolganlari (notifications, device_tokens, user_devices) —
	// `ON DELETE CASCADE`.
	tag, err := tx.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return users.ErrUserNotFound
	}
	return tx.Commit(ctx)
}

func (r *PgUserRepo) UpdateAddress(ctx context.Context, id string, a users.AddressDetails) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET address_lat = $2, address_lng = $3, address_text = $4,
			address_entrance = $5, address_floor = $6, address_apartment = $7,
			address_intercom = $8, address_comment = $9
		 WHERE id = $1`,
		id, a.Lat, a.Lng, a.Text, a.Entrance, a.Floor, a.Apartment, a.Intercom, a.Comment)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return users.ErrUserNotFound
	}
	return nil
}

func (r *PgUserRepo) ListByRole(ctx context.Context, role users.Role) ([]*users.User, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+userColumns+` FROM users WHERE role = $1 ORDER BY created_at`, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*users.User
	for rows.Next() {
		var u users.User
		if err := scanUser(rows, &u); err != nil {
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
		c.Target, c.CodeHash, c.ExpiresAt, c.CreatedAt)
	return err
}

func (s *PgCodeStore) Get(ctx context.Context, phone string) (*users.Code, error) {
	var c users.Code
	err := s.pool.QueryRow(ctx,
		`SELECT phone, code_hash, expires_at, created_at, attempts FROM phone_codes WHERE phone = $1`, phone,
	).Scan(&c.Target, &c.CodeHash, &c.ExpiresAt, &c.CreatedAt, &c.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, users.ErrInvalidCode
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// IncrementAttempts — bitta atomik UPDATE ... RETURNING: oshirish va
// yangi qiymatni o'qish bir amalda bajariladi, shuning uchun parallel
// so'rovlar bir xil qiymatni ko'ra olmaydi.
func (s *PgCodeStore) IncrementAttempts(ctx context.Context, phone string) (int, error) {
	var attempts int
	err := s.pool.QueryRow(ctx,
		`UPDATE phone_codes SET attempts = attempts + 1 WHERE phone = $1
		 RETURNING attempts`, phone).Scan(&attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		// Kod yozuvi yo'q (muddati o'tgan/o'chirilgan) — chaqiruvchi
		// buni "noto'g'ri kod" deb qaraydi.
		return 0, nil
	}
	return attempts, err
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

// PromoteToAdmin — `BOOTSTRAP_ADMIN_PHONE` uchun: mavjud foydalanuvchiga
// admin rolini beradi. Foydalanuvchi topilmasa `false` qaytaradi.
//
// YANGI AKKAUNT YARATMAYDI — ataylab. Raqam egasi avval odatdagi OTP
// oqimi bilan ro'yxatdan o'tishi kerak; shunda raqamga egalik
// tasdiqlangan bo'ladi. Aks holda server sozlamasiga yozilgan istalgan
// raqam uchun tasdiqlanmagan superadmin akkaunt paydo bo'lardi.
//
// `entity_id` bo'shatiladi: admin butun tizimga tegishli, biror
// restoran/kuryerga bog'lanmaydi. Avval restoran bo'lgan foydalanuvchi
// ko'tarilsa, eski bog'lanish qolib ketmasligi kerak.
func PromoteToAdmin(ctx context.Context, pool *pgxpool.Pool, phone string) (bool, error) {
	tag, err := pool.Exec(ctx,
		`UPDATE users SET role = 'admin', entity_id = '' WHERE phone = $1`, phone)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
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
