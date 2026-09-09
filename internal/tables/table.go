// Stollar — restorandagi QR kodli stollar (dine_in buyurtmalar uchun).
//
// ┌─ BU PAKET NIMA UCHUN ─────────────────────────────────────────────┐
// Mijoz stoldagi QR kodni skanerlaydi va Telegram Mini App ochiladi.
// QR ichida stolning SIRLI tokeni bor; server shu token orqali qaysi
// restoran va qaysi stol ekanini aniqlaydi.
//
// Nega alohida paket: `catalog` menyu va restoran ma'lumotlari bilan
// band, `orders` esa buyurtma hayot sikli bilan. Stollar ikkalasidan
// ham mustaqil — ular restoranning FIZIK jihozi.
// └───────────────────────────────────────────────────────────────────┘
package tables

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrNotFound     = errors.New("stol topilmadi")
	ErrInactive     = errors.New("stol vaqtincha faol emas")
	ErrDuplicate    = errors.New("bu nomli stol allaqachon mavjud")
	ErrEmptyLabel   = errors.New("stol raqami bo'sh bo'lishi mumkin emas")
	ErrLabelTooLong = errors.New("stol raqami juda uzun")
	ErrEmptyZone    = errors.New("zona nomi bo'sh bo'lishi mumkin emas")
	ErrZoneTooLong  = errors.New("zona nomi juda uzun")
)

// DefaultZone — yangi restoran uchun birinchi zal. Panelda "Asosiy zal"
// har doim ro'yxatda turadi; foydalanuvchi o'zi boshqa zonalar qo'shadi.
const DefaultZone = "Asosiy zal"

// maxLabelLen — stol nomi uzunligi chegarasi.
//
// Bu shunchaki "ehtiyot chorasi" emas: nom buyurtmaga NUSXA sifatida
// yoziladi va affitsiant ilovasida, restoran panelida, push
// bildirishnomasida ko'rsatiladi. Cheklovsiz uzun nom o'sha
// ekranlarni buzardi.
const maxLabelLen = 40

type Table struct {
	ID           string `json:"id"`
	RestaurantID string `json:"restaurant_id"`
	// Zone — zal/zona nomi ("Asosiy zal", "Ayvon", "VIP").
	Zone  string `json:"zone"`
	Label string `json:"label"`
	// QRToken — SIR. `json:"-"` ATAYLAB: token QR kodga chop etish
	// uchun ALOHIDA endpoint orqali beriladi (faqat restoran egasiga).
	// Oddiy ro'yxat javobida chiqsa, uni ko'rgan har kim istalgan stol
	// nomidan buyurtma bera olardi.
	QRToken   string    `json:"-"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

type Repository interface {
	Create(ctx context.Context, t *Table) error
	GetByID(ctx context.Context, id string) (*Table, error)
	// GetByToken — QR koddan kelgan token bo'yicha. Restoran ID'si
	// oldindan ma'lum emas, shuning uchun token butun tizim bo'ylab
	// unikal (migration 0032).
	GetByToken(ctx context.Context, token string) (*Table, error)
	ListByRestaurant(ctx context.Context, restaurantID string) ([]*Table, error)
	Update(ctx context.Context, t *Table) error
	Delete(ctx context.Context, id string) error
}

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// newToken — 32 tasodifiy bayt, hex.
//
// `crypto/rand` — `math/rand` EMAS. Bu farq bu yerda hal qiluvchi:
// `math/rand` urug'i (seed) taxmin qilinsa, hujumchi barcha stollar
// tokenini hisoblab chiqarib, istalgan restoranda istalgan stol
// nomidan buyurtma bera olardi.
func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("tasodifiy token yaratib bo'lmadi: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("ID yaratib bo'lmadi: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func normalizeLabel(label string) (string, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return "", ErrEmptyLabel
	}
	if len([]rune(label)) > maxLabelLen {
		return "", ErrLabelTooLong
	}
	return label, nil
}

func normalizeZone(zone string) (string, error) {
	zone = strings.TrimSpace(zone)
	if zone == "" {
		return DefaultZone, nil
	}
	if len([]rune(zone)) > maxLabelLen {
		return "", ErrZoneTooLong
	}
	return zone, nil
}

// DisplayLabel — affitsiant va buyurtma kartochkasida ko'rinadigan nom.
// Raqamning o'zi yetarli emas: ikki xil zonada "5" bo'lishi mumkin.
func (t *Table) DisplayLabel() string {
	if t == nil {
		return ""
	}
	z := strings.TrimSpace(t.Zone)
	if z == "" {
		z = DefaultZone
	}
	return z + " · " + t.Label
}

// Create — yangi stol qo'shadi va unga QR token yaratadi.
// Zona berilmasa `DefaultZone` ("Asosiy zal") ishlatiladi.
func (s *Service) Create(ctx context.Context, restaurantID, label string) (*Table, error) {
	return s.CreateInZone(ctx, restaurantID, DefaultZone, label)
}

// CreateInZone — belgilangan zonaga stol qo'shadi.
//
// QR token BIR MARTA yaratiladi va keyin hech qachon o'zgarmaydi
// (regenerate metodi yo'q). Shuning uchun ertasi kuni QR kodi
// boshqacha chiqmaydi — bu chizma, token emas.
func (s *Service) CreateInZone(ctx context.Context, restaurantID, zone, label string) (*Table, error) {
	if strings.TrimSpace(restaurantID) == "" {
		return nil, errors.New("restaurant_id bo'sh")
	}
	zone, err := normalizeZone(zone)
	if err != nil {
		return nil, err
	}
	label, err = normalizeLabel(label)
	if err != nil {
		return nil, err
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}
	token, err := newToken()
	if err != nil {
		return nil, err
	}
	t := &Table{
		ID:           id,
		RestaurantID: restaurantID,
		Zone:         zone,
		Label:        label,
		QRToken:      token,
		Active:       true,
		CreatedAt:    s.now(),
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// Resolve — QR tokendan stolni topadi.
//
// Faol bo'lmagan stol RAD ETILADI: restoran stolni vaqtincha
// o'chirganda (ta'mir, mavsumiy ayvon) o'sha stolning eski QR kodi
// bilan buyurtma kelib qolmasligi kerak.
func (s *Service) Resolve(ctx context.Context, token string) (*Table, error) {
	token = strings.TrimSpace(token)
	// Uzunlik tekshiruvi — bazaga bemaqsad so'rov yubormaslik uchun.
	// Token har doim aynan 64 belgi (32 bayt hex).
	if len(token) != 64 {
		return nil, ErrNotFound
	}
	t, err := s.repo.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if !t.Active {
		return nil, ErrInactive
	}
	return t, nil
}

func (s *Service) List(ctx context.Context, restaurantID string) ([]*Table, error) {
	return s.repo.ListByRestaurant(ctx, restaurantID)
}

func (s *Service) Get(ctx context.Context, id string) (*Table, error) {
	return s.repo.GetByID(ctx, id)
}

// Rename — stol nomini o'zgartiradi.
func (s *Service) Rename(ctx context.Context, id, label string) (*Table, error) {
	label, err := normalizeLabel(label)
	if err != nil {
		return nil, err
	}
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Label = label
	if err := s.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// SetZone — stolni boshqa zalga ko'chiradi. QR token o'zgarmaydi.
func (s *Service) SetZone(ctx context.Context, id, zone string) (*Table, error) {
	zone, err := normalizeZone(zone)
	if err != nil {
		return nil, err
	}
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Zone = zone
	if err := s.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// SetActive — stolni vaqtincha yoqadi/o'chiradi.
func (s *Service) SetActive(ctx context.Context, id string, active bool) (*Table, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Active = active
	if err := s.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// ┌─ QR TOKEN ABADIY — YANGILASH FUNKSIYASI ATAYLAB YO'Q ─────────────┐
// QR kod ilovada emas, MENYU VARAQASINING PASTIDA chop etilgan va
// stolda turadi (foydalanuvchi qarori, 2026-08-12). Uni almashtirish
// = butun zaldagi barcha menyu varaqalarini qayta chop etish.
//
// Shuning uchun token bir marta yaratiladi va HECH QACHON
// o'zgarmaydi. Bu qoida uch qatlamda majburlanadi:
//   1. bu yerda — yangilash metodi umuman yo'q;
//   2. `Repository.Update` — `qr_token` ustuniga TEGMAYDI;
//   3. HTTP — `regenerate` endpointi yo'q.
//
// QR surati tarqalib ketsa nima qilinadi: stol `Active = false`
// qilinadi (`SetActive`). Bu yagona to'g'ri yechim — chunki yangi
// token baribir yangi varaqa chop etishni talab qilardi.
//
// Zarar chegarasi kichik: soxta buyurtma bo'sh stolga tushadi,
// affitsiant borib hech kimni ko'rmaydi va bekor qiladi. To'lov
// ilovada emas (stolda naqd/karta), shuning uchun moliyaviy yo'qotish
// YO'Q.
// └───────────────────────────────────────────────────────────────────┘

// Delete — stolni butunlay o'chiradi.
//
// DIQQAT: chop etilgan QR kod ABADIY ishlamay qoladi va uni qaytarib
// bo'lmaydi (token yangi stolga boshqa qiymat bilan beriladi).
// Vaqtincha yopish uchun `SetActive(false)` ishlatilishi kerak.
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
