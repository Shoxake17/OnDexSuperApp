package users

import (
	"context"
	"errors"
	"time"
)

type Role string

const (
	RoleCustomer   Role = "customer"
	RoleCourier    Role = "courier"
	RoleRestaurant Role = "restaurant"
	RoleAdmin      Role = "admin"
)

type User struct {
	ID    string `json:"id"`
	Phone string `json:"phone"`
	Name  string `json:"name"`
	Role  Role   `json:"role"`
	// EntityID — rol bog'langan obyekt: kuryer uchun kuryer IDsi (c1),
	// restoran xodimi uchun restoran IDsi (r1). Mijoz/admin uchun bo'sh.
	EntityID  string    `json:"entity_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

var (
	ErrUserNotFound    = errors.New("foydalanuvchi topilmadi")
	ErrInvalidPhone    = errors.New("telefon raqam formati noto'g'ri (+998XXXXXXXXX bo'lishi kerak)")
	ErrInvalidCode     = errors.New("kod noto'g'ri yoki muddati tugagan")
	ErrTooManyAttempts = errors.New("juda ko'p noto'g'ri urinish — yangi kod so'rang")
	ErrTooSoon         = errors.New("yangi kod so'rash uchun biroz kuting")
)

type Repository interface {
	GetByPhone(ctx context.Context, phone string) (*User, error)
	GetByID(ctx context.Context, id string) (*User, error)
	Create(ctx context.Context, u *User) error
	// UpdateRole — mijoz kuryer bo'lganda (yoki admin rol berganda) ishlatiladi.
	UpdateRole(ctx context.Context, id string, role Role, entityID string) error
	ListByRole(ctx context.Context, role Role) ([]*User, error)
	// DeleteByRoleEntity — obyekt (restoran/kuryer) o'chirilganda unga
	// bog'langan akkauntlarni ham o'chirish uchun.
	DeleteByRoleEntity(ctx context.Context, role Role, entityID string) error
}

// Code — telefonga yuborilgan tasdiqlash kodi. Kod ochiq saqlanmaydi, faqat hash.
type Code struct {
	Phone     string
	CodeHash  string
	ExpiresAt time.Time
	CreatedAt time.Time
	Attempts  int
}

type CodeStore interface {
	Save(ctx context.Context, c *Code) error // bor bo'lsa ustidan yozadi
	Get(ctx context.Context, phone string) (*Code, error)
	IncrementAttempts(ctx context.Context, phone string) error
	Delete(ctx context.Context, phone string) error
}

// SmsSender — SMS yuborish qatlami. Dev'da log, production'da Eskiz.uz bo'ladi.
type SmsSender interface {
	Send(phone, text string) error
}
