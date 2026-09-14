// Stollar — restorandagi QR kodli joylar (dine_in buyurtmalar uchun):
// stol, kabina, VIP xona, topchan va boshqalar (`Kind`).
//
// ┌─ BU PAKET NIMA UCHUN ─────────────────────────────────────────────┐
// Mijoz joydagi QR kodni skanerlaydi va Telegram Mini App ochiladi.
// QR ichida joyning SIRLI tokeni bor; server shu token orqali qaysi
// restoran va qaysi joy ekanini aniqlaydi.
//
// Nega alohida paket: `catalog` menyu va restoran ma'lumotlari bilan
// band, `orders` esa buyurtma hayot sikli bilan. Joylar ikkalasidan
// ham mustaqil — ular restoranning FIZIK jihozi.
// └───────────────────────────────────────────────────────────────────┘
package tables

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var (
	ErrNotFound     = errors.New("stol topilmadi")
	ErrInactive     = errors.New("stol vaqtincha faol emas")
	ErrDuplicate    = errors.New("bu nomli joy shu zal va turda allaqachon mavjud")
	ErrEmptyLabel   = errors.New("joy raqami/nomi bo'sh bo'lishi mumkin emas")
	ErrLabelTooLong = errors.New("joy nomi juda uzun")
	ErrEmptyZone    = errors.New("zona nomi bo'sh bo'lishi mumkin emas")
	ErrZoneTooLong  = errors.New("zona nomi juda uzun")
	ErrControlChars = errors.New("nomda ko'rinmas boshqaruv belgilari bo'lishi mumkin emas")
	ErrBadCapacity  = fmt.Errorf("sig'im 1 dan %d kishigacha bo'lishi kerak", MaxCapacity)
	ErrBadBatch     = fmt.Errorf("bir martada 1 dan %d tagacha joy qo'shish mumkin, boshlang'ich raqam 0-%d", MaxBatch, maxBatchStart)
	ErrLimitReached = fmt.Errorf("bitta restoranda %d tadan ortiq joy bo'lishi mumkin emas", MaxTablesPerRestaurant)
)

// DefaultZone — yangi restoran uchun birinchi zal. Panelda "Asosiy zal"
// har doim ro'yxatda turadi; foydalanuvchi o'zi boshqa zonalar qo'shadi.
const DefaultZone = "Asosiy zal"

const (
	// MaxCapacity — bitta joy sig'imi chegarasi (banket zali uchun ham
	// yetarli, lekin "1000000 kishilik" kabi ekranni buzadigan qiymat yo'q).
	MaxCapacity = 1000
	// MaxTablesPerRestaurant — suiiste'molga qarshi chegara: har bir joy
	// panel ro'yxatiga va QR chop etishga tushadi.
	MaxTablesPerRestaurant = 1000
	// MaxBatch — bitta so'rovda yaratiladigan joylar.
	MaxBatch      = 100
	maxBatchStart = 99999

	// scanThrottle — "so'nggi skanerlash" vaqti shundan tez-tez
	// yozilmaydi: Mini App bir necha marta ochilsa ham bazaga har safar
	// yozish shart emas.
	scanThrottle = time.Minute
)

// maxLabelLen — joy nomi uzunligi chegarasi.
//
// Bu shunchaki "ehtiyot chorasi" emas: nom buyurtmaga NUSXA sifatida
// yoziladi va affitsiant ilovasida, restoran panelida, push
// bildirishnomasida ko'rsatiladi. Cheklovsiz uzun nom o'sha
// ekranlarni buzardi.
const maxLabelLen = 40

type Table struct {
	ID           string `json:"id"`
	RestaurantID string `json:"restaurant_id"`
	// Zone — zal/zona nomi ("Asosiy zal", "Ayvon", "2-qavat").
	Zone  string `json:"zone"`
	Kind  Kind   `json:"kind"`
	Label string `json:"label"`
	// Capacity — sig'im (kishi). `nil` — kiritilmagan: tur qo'shilishidan
	// oldingi joylar uchun "4 kishilik" deb TAXMIN ko'rsatilmaydi.
	Capacity *int `json:"capacity"`
	// QRToken — SIR. `json:"-"` ATAYLAB: token QR kodga chop etish
	// uchun ALOHIDA endpoint orqali beriladi (faqat restoran egasiga).
	// Oddiy ro'yxat javobida chiqsa, uni ko'rgan har kim istalgan stol
	// nomidan buyurtma bera olardi.
	QRToken string `json:"-"`
	Active  bool   `json:"active"`
	// CleaningSince — xodim "tozalanmoqda" deb belgilagan payt. Shundan
	// KEYIN yangi buyurtma kelsa belgi o'z-o'zidan eskiradi (`StatusOf`).
	CleaningSince *time.Time `json:"cleaning_since"`
	// LastScannedAt — QR kod oxirgi marta skanerlangan payt (kim
	// skanerlagani SAQLANMAYDI).
	LastScannedAt *time.Time `json:"last_scanned_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

type Repository interface {
	Create(ctx context.Context, t *Table) error
	// CreateMany — hammasi yoki hech biri (bir nechta joyni birdaniga
	// qo'shish yarim yo'lda to'xtab qolmasin).
	CreateMany(ctx context.Context, list []*Table) error
	GetByID(ctx context.Context, id string) (*Table, error)
	// GetByToken — QR koddan kelgan token bo'yicha. Restoran ID'si
	// oldindan ma'lum emas, shuning uchun token butun tizim bo'ylab
	// unikal (migration 0032).
	GetByToken(ctx context.Context, token string) (*Table, error)
	ListByRestaurant(ctx context.Context, restaurantID string) ([]*Table, error)
	// Update — zona, tur, nom, sig'im, faollik va tozalash holatini
	// yozadi. `qr_token`, `last_scanned_at`, `created_at` ga TEGMAYDI.
	Update(ctx context.Context, t *Table) error
	// TouchScanned — skanerlash vaqtini yozadi, agar oldingisi
	// `minInterval` dan eski bo'lsa.
	TouchScanned(ctx context.Context, id string, at time.Time, minInterval time.Duration) error
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

// hasControl — nom va zonada ko'rinmas belgilar (yangi qator, nol
// bayt, yo'nalish o'zgartirgich) bo'lmasin: ular chop etilgan QR
// varaqasini, bildirishnoma matnini va log qatorlarini buzardi.
func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return true
		}
	}
	return false
}

func normalizeLabel(label string) (string, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return "", ErrEmptyLabel
	}
	if hasControl(label) {
		return "", ErrControlChars
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
	if hasControl(zone) {
		return "", ErrControlChars
	}
	if len([]rune(zone)) > maxLabelLen {
		return "", ErrZoneTooLong
	}
	return zone, nil
}

func validateCapacity(c *int) (*int, error) {
	if c == nil {
		return nil, nil
	}
	if *c < 1 || *c > MaxCapacity {
		return nil, ErrBadCapacity
	}
	v := *c
	return &v, nil
}

func zoneKey(z string) string {
	z = strings.TrimSpace(z)
	if z == "" {
		z = DefaultZone
	}
	return strings.ToLower(z)
}

// sameSlot — ikki joy bir xil "manzil"dami (restoran + zal + tur + nom,
// harf registrisiz). Bir zalda "Kabina 1" va "Stol 1" birga yashay
// oladi, lekin ikkita "Stol 1" — yo'q: affitsiant ularni ajrata olmasdi.
func sameSlot(a, b *Table) bool {
	return a.RestaurantID == b.RestaurantID &&
		zoneKey(a.Zone) == zoneKey(b.Zone) &&
		a.Kind.Normalized() == b.Kind.Normalized() &&
		strings.EqualFold(a.Label, b.Label)
}

// DisplayLabel — affitsiant va buyurtma kartochkasida ko'rinadigan nom.
// Raqamning o'zi yetarli emas: ikki xil zonada "5" bo'lishi mumkin.
//
// Stol uchun eski shakl saqlanadi ("Asosiy zal · 5") — mavjud
// buyurtmalar va affitsiant ilovasi aynan shuni kutadi. Boshqa turlarda
// tur nomi qo'shiladi ("Asosiy zal · Kabina 3"), nom allaqachon u bilan
// boshlansa takrorlanmaydi.
func (t *Table) DisplayLabel() string {
	if t == nil {
		return ""
	}
	z := strings.TrimSpace(t.Zone)
	if z == "" {
		z = DefaultZone
	}
	name := t.Label
	if k := t.Kind.Normalized(); k != KindTable {
		title := k.Title()
		if !strings.HasPrefix(strings.ToLower(name), strings.ToLower(title)) {
			name = title + " " + name
		}
	}
	return z + " · " + name
}

// Spec — yangi joy.
type Spec struct {
	Zone     string
	Kind     string
	Label    string
	Capacity *int
}

// Create — "Asosiy zal"ga oddiy stol qo'shadi.
func (s *Service) Create(ctx context.Context, restaurantID, label string) (*Table, error) {
	return s.CreateTable(ctx, restaurantID, Spec{Zone: DefaultZone, Label: label})
}

// CreateInZone — belgilangan zonaga oddiy stol qo'shadi.
func (s *Service) CreateInZone(ctx context.Context, restaurantID, zone, label string) (*Table, error) {
	return s.CreateTable(ctx, restaurantID, Spec{Zone: zone, Label: label})
}

// build — tekshirilgan, tokenli yangi joy (hali saqlanmagan).
//
// QR token SHU YERDA, BIR MARTA yaratiladi va keyin hech qachon
// o'zgarmaydi (regenerate metodi yo'q). Shuning uchun ertasi kuni QR
// kodi boshqacha chiqmaydi — bu chizma, token emas.
func (s *Service) build(restaurantID string, spec Spec) (*Table, error) {
	if strings.TrimSpace(restaurantID) == "" {
		return nil, errors.New("restaurant_id bo'sh")
	}
	zone, err := normalizeZone(spec.Zone)
	if err != nil {
		return nil, err
	}
	kind, err := ParseKind(spec.Kind)
	if err != nil {
		return nil, err
	}
	label, err := normalizeLabel(spec.Label)
	if err != nil {
		return nil, err
	}
	capacity, err := validateCapacity(spec.Capacity)
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
	return &Table{
		ID:           id,
		RestaurantID: restaurantID,
		Zone:         zone,
		Kind:         kind,
		Label:        label,
		Capacity:     capacity,
		QRToken:      token,
		Active:       true,
		CreatedAt:    s.now(),
	}, nil
}

// CreateTable — yangi joy qo'shadi.
func (s *Service) CreateTable(ctx context.Context, restaurantID string, spec Spec) (*Table, error) {
	t, err := s.build(restaurantID, spec)
	if err != nil {
		return nil, err
	}
	existing, err := s.repo.ListByRestaurant(ctx, restaurantID)
	if err != nil {
		return nil, err
	}
	// Chegara "yumshoq": ikki parallel so'rov uni bittaga oshirishi
	// mumkin. Maqsad — suiiste'molni to'xtatish, aniq hisob emas.
	if len(existing) >= MaxTablesPerRestaurant {
		return nil, ErrLimitReached
	}
	// Takror tekshiruvi SHU YERDA ham (bazadagi unikal indeksdan
	// tashqari): indeks harf registrini farqlaydi, ya'ni "vip" va "VIP"
	// ikki xil joy bo'lib qolardi.
	for _, x := range existing {
		if sameSlot(x, t) {
			return nil, ErrDuplicate
		}
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// BatchSpec — bir nechta ketma-ket raqamli joy: Prefix + From..From+Count-1.
type BatchSpec struct {
	Zone     string
	Kind     string
	Prefix   string
	Capacity *int
	From     int
	Count    int
}

// batchLabel — "A-" + 5 = "A-5", "VIP" + 5 = "VIP 5", "" + 5 = "5".
func batchLabel(prefix string, n int) string {
	prefix = strings.TrimSpace(prefix)
	num := strconv.Itoa(n)
	if prefix == "" {
		return num
	}
	last := []rune(prefix)[len([]rune(prefix))-1]
	if unicode.IsLetter(last) || unicode.IsDigit(last) {
		return prefix + " " + num
	}
	return prefix + num
}

// CreateBatch — bir nechta joyni BIRDANIGA qo'shadi: hammasi yoki hech
// biri. Takror nom bo'lsa hech narsa yaratilmaydi va xatoda aynan
// qaysi nom ekani aytiladi.
func (s *Service) CreateBatch(ctx context.Context, restaurantID string, b BatchSpec) ([]*Table, error) {
	if b.Count < 1 || b.Count > MaxBatch || b.From < 0 || b.From > maxBatchStart {
		return nil, ErrBadBatch
	}
	if hasControl(b.Prefix) {
		return nil, ErrControlChars
	}
	existing, err := s.repo.ListByRestaurant(ctx, restaurantID)
	if err != nil {
		return nil, err
	}
	if len(existing)+b.Count > MaxTablesPerRestaurant {
		return nil, ErrLimitReached
	}
	list := make([]*Table, 0, b.Count)
	for i := 0; i < b.Count; i++ {
		t, err := s.build(restaurantID, Spec{
			Zone: b.Zone, Kind: b.Kind, Capacity: b.Capacity,
			Label: batchLabel(b.Prefix, b.From+i),
		})
		if err != nil {
			return nil, err
		}
		for _, x := range existing {
			if sameSlot(x, t) {
				return nil, fmt.Errorf("%w: %s", ErrDuplicate, t.DisplayLabel())
			}
		}
		list = append(list, t)
	}
	if err := s.repo.CreateMany(ctx, list); err != nil {
		return nil, err
	}
	return list, nil
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

// MarkScanned — QR skanerlangan paytni yozadi ("So'nggi skanerlangan
// QR kodlar" uchun). Kim skanerlagani saqlanmaydi.
func (s *Service) MarkScanned(ctx context.Context, id string) error {
	return s.repo.TouchScanned(ctx, id, s.now(), scanThrottle)
}

func (s *Service) List(ctx context.Context, restaurantID string) ([]*Table, error) {
	return s.repo.ListByRestaurant(ctx, restaurantID)
}

func (s *Service) Get(ctx context.Context, id string) (*Table, error) {
	return s.repo.GetByID(ctx, id)
}

// Patch — tahrirlash; `nil` maydon o'zgarmaydi.
type Patch struct {
	Label *string
	Zone  *string
	Kind  *string
	// Capacity — yangi sig'im; ClearCapacity — sig'imni olib tashlash.
	Capacity      *int
	ClearCapacity bool
	Active        *bool
	// Cleaning — true: "tozalanmoqda" (vaqt shu payt yoziladi, allaqachon
	// belgilangan bo'lsa saqlanadi); false: belgini olib tashlash.
	Cleaning *bool
}

// Edit — bir nechta maydonni BITTA yozuvda o'zgartiradi (qisman
// saqlanib qolgan tahrir bo'lmaydi). QR token o'zgarmaydi.
func (s *Service) Edit(ctx context.Context, id string, p Patch) (*Table, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	slotChanged := false
	if p.Label != nil {
		label, err := normalizeLabel(*p.Label)
		if err != nil {
			return nil, err
		}
		slotChanged = slotChanged || label != t.Label
		t.Label = label
	}
	if p.Zone != nil {
		zone, err := normalizeZone(*p.Zone)
		if err != nil {
			return nil, err
		}
		slotChanged = slotChanged || zone != t.Zone
		t.Zone = zone
	}
	if p.Kind != nil {
		kind, err := ParseKind(*p.Kind)
		if err != nil {
			return nil, err
		}
		slotChanged = slotChanged || kind != t.Kind.Normalized()
		t.Kind = kind
	}
	if p.ClearCapacity {
		t.Capacity = nil
	} else if p.Capacity != nil {
		c, err := validateCapacity(p.Capacity)
		if err != nil {
			return nil, err
		}
		t.Capacity = c
	}
	if p.Active != nil {
		t.Active = *p.Active
	}
	if p.Cleaning != nil {
		switch {
		case !*p.Cleaning:
			t.CleaningSince = nil
		case t.CleaningSince == nil:
			now := s.now()
			t.CleaningSince = &now
		}
	}
	if slotChanged {
		siblings, err := s.repo.ListByRestaurant(ctx, t.RestaurantID)
		if err != nil {
			return nil, err
		}
		for _, x := range siblings {
			if x.ID != t.ID && sameSlot(x, t) {
				return nil, ErrDuplicate
			}
		}
	}
	if err := s.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// Rename — stol nomini o'zgartiradi.
func (s *Service) Rename(ctx context.Context, id, label string) (*Table, error) {
	return s.Edit(ctx, id, Patch{Label: &label})
}

// SetZone — stolni boshqa zalga ko'chiradi. QR token o'zgarmaydi.
func (s *Service) SetZone(ctx context.Context, id, zone string) (*Table, error) {
	return s.Edit(ctx, id, Patch{Zone: &zone})
}

// SetActive — stolni vaqtincha yoqadi/o'chiradi.
func (s *Service) SetActive(ctx context.Context, id string, active bool) (*Table, error) {
	return s.Edit(ctx, id, Patch{Active: &active})
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
