// Package storage — in-memory implementatsiyalar. Faqat MVP skelet uchun;
// keyingi qadam: shu interface'larni PostgreSQL (+PostGIS) bilan almashtirish.
package storage

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"chustapp/internal/couriers"
	"chustapp/internal/geo"
	"chustapp/internal/orders"
)

type MemoryOrderRepo struct {
	mu   sync.RWMutex
	data map[string]orders.Order
}

func NewMemoryOrderRepo() *MemoryOrderRepo {
	return &MemoryOrderRepo{data: make(map[string]orders.Order)}
}

func (r *MemoryOrderRepo) GetByID(_ context.Context, id string) (*orders.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	o, ok := r.data[id]
	if !ok {
		return nil, orders.ErrNotFound
	}
	cp := o // nusxa qaytariladi, chaqiruvchi map ichini buzmasin
	return &cp, nil
}

// Save — Postgres implementatsiyasi (PgOrderRepo.Save) bilan bir xil
// optimistik parallel boshqaruv semantikasi: mavjud buyurtma uchun
// o.Version bazadagi bilan bir xil bo'lishi SHART, aks holda ErrConflict
// (boshqa so'rov shu oraliqda allaqachon yozib ulgurgan). Mos kelsa,
// versiya +1 oshiriladi. Bu — `r.mu.Lock()` allaqachon butun map yozuvini
// ketma-ketlashtirsa ham SHART, chunki muammo map'ning o'zida emas (u xavfsiz
// edi), balki GetByID (RLock, darhol qo'yib yuboriladi) va keyingi Save
// (Lock) ORASIDA — o'sha oraliqda boshqa goroutine eskirgan holatni
// o'qib ulgurishi mumkin edi.
func (r *MemoryOrderRepo) Save(_ context.Context, o *orders.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, exists := r.data[o.ID]
	if exists {
		if o.Version != existing.Version {
			return orders.ErrConflict
		}
		o.Version++
	} else if o.IdempotencyKey != "" {
		// Yangi buyurtma — Postgres'dagi (customer_id, idempotency_key)
		// unique indeksga mos ravishda, shu mijoz uchun kalit allaqachon
		// ishlatilganmi tekshiramiz (Service.Create()dagi tekshiruv bilan
		// parallel so'rov orasidagi race'ga qarshi oxirgi himoya).
		for _, other := range r.data {
			if other.CustomerID == o.CustomerID && other.IdempotencyKey == o.IdempotencyKey {
				return orders.ErrDuplicateIdempotencyKey
			}
		}
	}
	// order_number faqat BIRINCHI marta (yangi buyurtma) beriladi — keyingi
	// holat o'zgarishlarida (Save qayta-qayta chaqirilganda) o'zgarmaydi.
	// Format: "DDMMYY-XXXXXXX" — birinchi 6 xona sana, keyingi 7 xona
	// TASODIFIY (1000000-9999999, nol bilan boshlanmaydi) — ketma-ket
	// hisoblagich EMAS, shuning uchun "0000123" kabi ko'p nolli raqamlar
	// chiqmaydi (foydalanuvchi so'rovi, Yandex uslubiga ko'ra). Postgres'dagi
	// UNIQUE indeksga mos (migration 0017) — kolliziya (amalda deyarli
	// imkonsiz) bo'lsa qayta generatsiya qilinadi.
	if o.OrderNumber == "" {
		for {
			randPart := 1_000_000 + rand.Intn(9_000_000)
			candidate := fmt.Sprintf("%s-%07d", o.CreatedAt.Format("020106"), randPart)
			collision := false
			for _, other := range r.data {
				if other.OrderNumber == candidate {
					collision = true
					break
				}
			}
			if !collision {
				o.OrderNumber = candidate
				break
			}
		}
	}
	r.data[o.ID] = *o
	return nil
}

func (r *MemoryOrderRepo) FindByIdempotencyKey(_ context.Context, customerID, key string) (*orders.Order, error) {
	if key == "" {
		return nil, orders.ErrNotFound
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, o := range r.data {
		if o.CustomerID == customerID && o.IdempotencyKey == key {
			cp := o
			return &cp, nil
		}
	}
	return nil, orders.ErrNotFound
}

func (r *MemoryOrderRepo) HasActiveByRestaurant(_ context.Context, restaurantID string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, o := range r.data {
		if o.RestaurantID == restaurantID && !o.IsTerminal() {
			return true, nil
		}
	}
	return false, nil
}

// GetActiveByCourier — postgres.go'dagi PgOrderRepo bilan bir xil mantiq
// (eng so'nggi yakunlanmagan buyurtma).
func (r *MemoryOrderRepo) GetActiveByCourier(_ context.Context, courierID string) (*orders.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var best *orders.Order
	for _, o := range r.data {
		if o.CourierID != courierID || o.IsTerminal() {
			continue
		}
		if best == nil || o.CreatedAt.After(best.CreatedAt) {
			cp := o
			best = &cp
		}
	}
	if best == nil {
		return nil, orders.ErrNotFound
	}
	return best, nil
}

func (r *MemoryOrderRepo) ListRecent(_ context.Context, limit int) ([]*orders.Order, error) {
	return r.listFiltered(limit, func(*orders.Order) bool { return true })
}

func (r *MemoryOrderRepo) ListByRestaurant(_ context.Context, restaurantID string, limit int) ([]*orders.Order, error) {
	return r.listFiltered(limit, func(o *orders.Order) bool { return o.RestaurantID == restaurantID })
}

func (r *MemoryOrderRepo) ListByCustomer(_ context.Context, customerID string, limit int) ([]*orders.Order, error) {
	return r.listFiltered(limit, func(o *orders.Order) bool { return o.CustomerID == customerID })
}

// CountByCustomerAndRestaurant — bekor qilingan/rad etilganlarni
// HISOBGA OLMASDAN sanaydi (promotions.TypeLoyalty uchun "haqiqiy
// buyurtma bergan" degani, urinib bekor qilinganini emas).
func (r *MemoryOrderRepo) CountByCustomerAndRestaurant(_ context.Context, customerID, restaurantID string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, o := range r.data {
		if o.CustomerID == customerID && o.RestaurantID == restaurantID &&
			o.Status != orders.StatusCancelled && o.Status != orders.StatusRejected {
			count++
		}
	}
	return count, nil
}

func (r *MemoryOrderRepo) listFiltered(limit int, keep func(*orders.Order) bool) ([]*orders.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []*orders.Order
	for _, o := range r.data {
		cp := o
		if keep(&cp) {
			list = append(list, &cp)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.After(list[j].CreatedAt) })
	if len(list) > limit {
		list = list[:limit]
	}
	return list, nil
}

type MemoryCourierRepo struct {
	mu   sync.RWMutex
	data map[string]couriers.Courier
	// locSeen — joylashuv OXIRGI marta yangilangan vaqt.
	//
	// Postgres'dagi `location_updated_at` ustunining aynan o'zi
	// (migration 0029): `Courier` structiga qo'shilmadi, chunki u
	// API javobiga chiqadi va mijozga kuryerning oxirgi qachon
	// ko'ringani kerak emas.
	locSeen map[string]time.Time
}

func NewMemoryCourierRepo(seed ...couriers.Courier) *MemoryCourierRepo {
	r := &MemoryCourierRepo{
		data:    make(map[string]couriers.Courier),
		locSeen: make(map[string]time.Time),
	}
	for _, c := range seed {
		r.data[c.ID] = c
	}
	return r
}

func (r *MemoryCourierRepo) GetByID(_ context.Context, id string) (*couriers.Courier, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.data[id]
	if !ok {
		return nil, couriers.ErrNoCourier
	}
	cp := c
	return &cp, nil
}

func (r *MemoryCourierRepo) Create(_ context.Context, c *couriers.Courier) error {
	if c.VehicleType == "" {
		c.VehicleType = couriers.VehicleMoped
	}
	if c.Rating == 0 {
		c.Rating = 5.0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[c.ID] = *c
	return nil
}

func (r *MemoryCourierRepo) ListAll(_ context.Context) ([]*couriers.Courier, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []*couriers.Courier
	for _, c := range r.data {
		cp := c
		list = append(list, &cp)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list, nil
}

func (r *MemoryCourierRepo) SetApproved(_ context.Context, id string, approved bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.data[id]
	if !ok {
		return couriers.ErrNoCourier
	}
	c.Approved = approved
	r.data[id] = c
	return nil
}

// ListAvailable — barcha tasdiqlangan va onlayn kuryerlar, masofasiz.
//
// DIQQAT: dispatch bu metodni ENDI ISHLATMAYDI — u `ListAvailableNear`
// orqali radius bo'yicha tanlaydi (migration 0029). Bu metod admin
// panel/xarita kabi "hammasini ko'rsat" so'rovlari uchun qoladi.
//
// (Avvalgi izoh "dispatch hammasiga bir vaqtda taklif yuboradi" degan
// edi — bu NOTO'G'RI edi: dispatch har doim KETMA-KET, bittalab taklif
// yuboradi, `dispatch.go` dagi asosiy siklga qarang.)
func (r *MemoryCourierRepo) ListAvailable(_ context.Context) ([]*couriers.Courier, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []*couriers.Courier
	for _, c := range r.data {
		if c.Available && c.Approved {
			cp := c
			list = append(list, &cp)
		}
	}
	return list, nil
}

// ListAvailableNear — `PgCourierRepo.ListAvailableNear` bilan BIR XIL
// xatti-harakat: radius bo'yicha filtr, eng yaqinidan tartiblash,
// limit va joylashuv eskiligini tekshirish.
//
// NEGA AYNAN BIR XIL BO'LISHI SHART: testlar xotiradagi rejimda
// ishlaydi, production esa Postgres'da. Xatti-harakat farq qilsa,
// testlar production'ni ifodalamay qoladi.
//
// Masofa `geo.HaversineMeters` bilan — Postgres tomonda PostGIS
// ellipsoid bo'yicha hisoblaydi, farq ~0.5% (bitta shahar ichida bir
// necha metr, kuryer tanlashda ahamiyatsiz).
func (r *MemoryCourierRepo) ListAvailableNear(_ context.Context, lat, lng float64,
	radiusMeters float64, maxAge time.Duration, limit int) ([]*couriers.Courier, error) {

	if limit <= 0 {
		limit = 20
	}
	if radiusMeters <= 0 {
		radiusMeters = 5000
	}
	origin := geo.LatLng{Lat: lat, Lng: lng}
	now := time.Now()

	r.mu.RLock()
	type scored struct {
		c    couriers.Courier
		dist float64
	}
	var found []scored
	for _, c := range r.data {
		if !c.Available || !c.Approved {
			continue
		}
		if maxAge > 0 {
			// Joylashuv vaqti noma'lum bo'lsa O'TKAZAMIZ — Postgres
			// tomonda ham `location_updated_at IS NULL` o'tadi
			// (migratsiyadan oldin yaratilgan yozuvlar).
			if seen, ok := r.locSeen[c.ID]; ok && now.Sub(seen) > maxAge {
				continue
			}
		}
		d := geo.HaversineMeters(origin, geo.LatLng{Lat: c.Lat, Lng: c.Lng})
		if d > radiusMeters {
			continue
		}
		found = append(found, scored{c: c, dist: d})
	}
	r.mu.RUnlock()

	sort.Slice(found, func(i, j int) bool { return found[i].dist < found[j].dist })
	if len(found) > limit {
		found = found[:limit]
	}
	list := make([]*couriers.Courier, 0, len(found))
	for i := range found {
		cp := found[i].c
		list = append(list, &cp)
	}
	return list, nil
}

func (r *MemoryCourierRepo) SetAvailable(_ context.Context, id string, available bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.data[id]
	if !ok {
		return couriers.ErrNoCourier
	}
	c.Available = available
	r.data[id] = c
	return nil
}

// ClaimIfAvailable — tekshirish va yozish BITTA mutex ostida bajariladi,
// shuning uchun parallel chaqiruvlardan faqat bittasi muvaffaqiyatli
// bo'ladi (Postgres'dagi shartli UPDATE bilan bir xil semantika).
func (r *MemoryCourierRepo) ClaimIfAvailable(_ context.Context, id string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.data[id]
	if !ok {
		return false, couriers.ErrNoCourier
	}
	// `Approved` ham tekshiriladi — taklif yuborilgandan keyin kuryer
	// bloklangan bo'lishi mumkin (Postgres implementatsiyasidagi izohga
	// qarang).
	if !c.Available || !c.Approved {
		return false, nil
	}
	c.Available = false
	r.data[id] = c
	return true, nil
}

func (r *MemoryCourierRepo) UpdateLocation(_ context.Context, id string, lat, lng float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.data[id]
	if !ok {
		return couriers.ErrNoCourier
	}
	c.Lat = lat
	c.Lng = lng
	r.data[id] = c
	r.locSeen[id] = time.Now()
	return nil
}

// IncrementCompletedOrders — buyurtma "delivered" bo'lganda +1.
func (r *MemoryCourierRepo) IncrementCompletedOrders(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.data[id]
	if !ok {
		return couriers.ErrNoCourier
	}
	c.CompletedOrders++
	r.data[id] = c
	return nil
}
