package storage

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"chustapp/internal/tables"
)

// MemoryTableRepo — DATABASE_URL berilmaganda (mahalliy ishlab chiqish
// va testlar) ishlatiladigan xotira ombori.
//
// MUHIM: qiymatlar CHUQUR NUSXA sifatida saqlanadi va qaytariladi
// (`cloneTable` — ko'rsatkichli maydonlar ham). Aks holda chaqiruvchi
// qaytgan `*Capacity` ni o'zgartirib omborni qulfsiz o'zgartira olardi —
// bu aynan `internal/telegram` da topilgan poyga bilan bir xil xato.
type MemoryTableRepo struct {
	mu sync.RWMutex
	// byID — asosiy saqlash joyi.
	byID map[string]tables.Table
	// byToken — token → ID indeksi. Har safar butun xaritani
	// skanerlash mumkin edi, lekin bu qidiruv HAR BIR QR
	// skanerlashda bajariladi.
	byToken map[string]string
}

func NewMemoryTableRepo() *MemoryTableRepo {
	return &MemoryTableRepo{
		byID:    make(map[string]tables.Table),
		byToken: make(map[string]string),
	}
}

func cloneTable(t tables.Table) tables.Table {
	if t.Capacity != nil {
		v := *t.Capacity
		t.Capacity = &v
	}
	if t.CleaningSince != nil {
		v := *t.CleaningSince
		t.CleaningSince = &v
	}
	if t.LastScannedAt != nil {
		v := *t.LastScannedAt
		t.LastScannedAt = &v
	}
	if strings.TrimSpace(t.Zone) == "" {
		t.Zone = tables.DefaultZone
	}
	t.Kind = t.Kind.Normalized()
	return t
}

// sameSlotMem — Postgres'dagi unikal indeks (restaurant_id, zone, kind,
// label) bilan bir xil qoida; ikkala ombor BIR XIL xatoni qaytarishi
// shart, aks holda xotirada o'tgan test Postgres'da yiqilardi.
func sameSlotMem(a, b tables.Table) bool {
	return a.RestaurantID == b.RestaurantID &&
		strings.EqualFold(zoneOf(a), zoneOf(b)) &&
		a.Kind.Normalized() == b.Kind.Normalized() &&
		strings.EqualFold(a.Label, b.Label)
}

func (r *MemoryTableRepo) conflictLocked(t tables.Table, skipID string) bool {
	for id, x := range r.byID {
		if id != skipID && sameSlotMem(x, t) {
			return true
		}
	}
	return false
}

func (r *MemoryTableRepo) Create(_ context.Context, t *tables.Table) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conflictLocked(*t, "") {
		return tables.ErrDuplicate
	}
	r.byID[t.ID] = cloneTable(*t)
	r.byToken[t.QRToken] = t.ID
	return nil
}

// CreateMany — avval HAMMASI tekshiriladi (bazadagilar bilan ham, o'zaro
// ham), keyin yoziladi: Postgres tranzaksiyasi bilan bir xil natija.
func (r *MemoryTableRepo) CreateMany(_ context.Context, list []*tables.Table) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, t := range list {
		if r.conflictLocked(*t, "") {
			return tables.ErrDuplicate
		}
		for _, other := range list[:i] {
			if sameSlotMem(*other, *t) {
				return tables.ErrDuplicate
			}
		}
	}
	for _, t := range list {
		r.byID[t.ID] = cloneTable(*t)
		r.byToken[t.QRToken] = t.ID
	}
	return nil
}

func (r *MemoryTableRepo) GetByID(_ context.Context, id string) (*tables.Table, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	x, ok := r.byID[id]
	if !ok {
		return nil, tables.ErrNotFound
	}
	cp := cloneTable(x)
	return &cp, nil
}

func (r *MemoryTableRepo) GetByToken(_ context.Context, token string) (*tables.Table, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byToken[token]
	if !ok {
		return nil, tables.ErrNotFound
	}
	x, ok := r.byID[id]
	if !ok {
		return nil, tables.ErrNotFound
	}
	cp := cloneTable(x)
	return &cp, nil
}

func (r *MemoryTableRepo) ListByRestaurant(_ context.Context, restaurantID string) ([]*tables.Table, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []*tables.Table
	for _, x := range r.byID {
		if x.RestaurantID == restaurantID {
			cp := cloneTable(x)
			list = append(list, &cp)
		}
	}
	// Barqaror tartib: xarita bo'ylab yurish Go'da TASODIFIY va
	// busiz restoran panelida stollar har yangilashda sakrab turardi.
	sort.Slice(list, func(i, j int) bool {
		zi, zj := zoneOf(*list[i]), zoneOf(*list[j])
		if zi != zj {
			return zi < zj
		}
		if list[i].Kind != list[j].Kind {
			return list[i].Kind < list[j].Kind
		}
		return list[i].Label < list[j].Label
	})
	return list, nil
}

func (r *MemoryTableRepo) Update(_ context.Context, t *tables.Table) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	old, ok := r.byID[t.ID]
	if !ok {
		return tables.ErrNotFound
	}
	if r.conflictLocked(*t, t.ID) {
		return tables.ErrDuplicate
	}
	// ┌─ TOKEN O'ZGARMAYDI ──────────────────────────────────────────┐
	// QR kod menyu varaqasiga chop etilgan va stolda abadiy turadi.
	// Postgres implementatsiyasi `qr_token` ni UPDATE ro'yxatiga
	// umuman qo'shmaydi — xotira ombori ham AYNAN shunday
	// ishlashi shart, aks holda testlar bir joyda o'tib, ishlab
	// chiqarishda boshqacha natija berardi.
	//
	// Shuning uchun kiruvchi qiymat emas, ESKI token saqlanadi
	// (skanerlash va yaratilish vaqti ham).
	// └───────────────────────────────────────────────────────────────┘
	updated := cloneTable(*t)
	updated.RestaurantID = old.RestaurantID
	updated.QRToken = old.QRToken
	updated.LastScannedAt = old.LastScannedAt
	updated.CreatedAt = old.CreatedAt
	r.byID[t.ID] = updated
	return nil
}

func (r *MemoryTableRepo) TouchScanned(_ context.Context, id string, at time.Time, minInterval time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	x, ok := r.byID[id]
	if !ok {
		return nil // Postgres'dagi kabi: yo'q qator — xato emas
	}
	if x.LastScannedAt != nil && x.LastScannedAt.After(at.Add(-minInterval)) {
		return nil
	}
	v := at
	x.LastScannedAt = &v
	r.byID[id] = x
	return nil
}

func zoneOf(t tables.Table) string {
	z := strings.TrimSpace(t.Zone)
	if z == "" {
		return tables.DefaultZone
	}
	return z
}

func (r *MemoryTableRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	x, ok := r.byID[id]
	if !ok {
		return tables.ErrNotFound
	}
	delete(r.byToken, x.QRToken)
	delete(r.byID, id)
	return nil
}
