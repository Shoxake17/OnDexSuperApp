package storage

import (
	"context"
	"sort"
	"strings"
	"sync"

	"chustapp/internal/tables"
)

// MemoryTableRepo — DATABASE_URL berilmaganda (mahalliy ishlab chiqish
// va testlar) ishlatiladigan xotira ombori.
//
// MUHIM: qiymatlar NUSXA sifatida saqlanadi va NUSXA sifatida
// qaytariladi. Ko'rsatkich qaytarilsa chaqiruvchi omborni qulfsiz
// o'zgartira olardi — bu aynan `internal/telegram` da topilgan va
// tuzatilgan poyga (data race) bilan bir xil xato bo'lardi.
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

func (r *MemoryTableRepo) Create(_ context.Context, t *tables.Table) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Bir restoranda bir xil nom bo'lmasin (Postgres'dagi
	// idx_restaurant_tables_label bilan bir xil qoida — ikkala ombor
	// ham BIR XIL xatoni qaytarishi shart, aks holda xotirada
	// ishlaydigan test Postgres'da yiqilardi).
	for _, x := range r.byID {
		if x.RestaurantID == t.RestaurantID && strings.EqualFold(x.Label, t.Label) {
			return tables.ErrDuplicate
		}
	}
	r.byID[t.ID] = *t
	r.byToken[t.QRToken] = t.ID
	return nil
}

func (r *MemoryTableRepo) GetByID(_ context.Context, id string) (*tables.Table, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	x, ok := r.byID[id]
	if !ok {
		return nil, tables.ErrNotFound
	}
	cp := x
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
	cp := x
	return &cp, nil
}

func (r *MemoryTableRepo) ListByRestaurant(_ context.Context, restaurantID string) ([]*tables.Table, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []*tables.Table
	for _, x := range r.byID {
		if x.RestaurantID == restaurantID {
			cp := x
			list = append(list, &cp)
		}
	}
	// Barqaror tartib: xarita bo'ylab yurish Go'da TASODIFIY va
	// busiz restoran panelida stollar har yangilashda sakrab turardi.
	sort.Slice(list, func(i, j int) bool { return list[i].Label < list[j].Label })
	return list, nil
}

func (r *MemoryTableRepo) Update(_ context.Context, t *tables.Table) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	old, ok := r.byID[t.ID]
	if !ok {
		return tables.ErrNotFound
	}
	// Nom o'zgargan bo'lsa — takrorlanmasin.
	for id, x := range r.byID {
		if id != t.ID && x.RestaurantID == t.RestaurantID && strings.EqualFold(x.Label, t.Label) {
			return tables.ErrDuplicate
		}
	}
	// ┌─ TOKEN O'ZGARMAYDI ──────────────────────────────────────────┐
	// QR kod menyu varaqasiga chop etilgan va stolda abadiy turadi.
	// Postgres implementatsiyasi `qr_token` ni UPDATE ro'yxatiga
	// umuman qo'shmaydi — xotira ombori ham AYNAN shunday
	// ishlashi shart, aks holda testlar bir joyda o'tib, ishlab
	// chiqarishda boshqacha natija berardi.
	//
	// Shuning uchun kiruvchi qiymat emas, ESKI token saqlanadi.
	// └───────────────────────────────────────────────────────────────┘
	updated := *t
	updated.QRToken = old.QRToken
	r.byID[t.ID] = updated
	return nil
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
