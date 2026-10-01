// Paket ttlcache — bitta jarayon ichidagi QISQA muddatli, chegaralangan kesh.
package ttlcache

import (
	"sync"
	"time"
)

// Cache — bitta jarayon ichidagi QISQA muddatli kesh (o'qish ko'p,
// o'zgarish kam bo'lgan kichik ma'lumotlar uchun: restoran, taom, aksiya,
// xodimlar ro'yxati).
//
// ┌─ NEGA KERAK ──────────────────────────────────────────────────────┐
// Narxlash (`/quote`, buyurtma yaratish) har so'rovda bir xil restoran,
// taom, aksiya va xodimlar ro'yxatini bazadan qayta o'qirdi. Yuklama
// sinovida (lokal, 2 000+ so'rov/s) aynan shu o'qishlar to'yinishga olib
// keldi: quote p50 ~240 ms, qolgan yo'llar ~1–6 ms.
//
// ┌─ TO'G'RILIK KAFOLATLARI ──────────────────────────────────────────┐
//  1. YOZISHDA BEKOR QILINADI: dekoratorlar saqlash/o'chirishdan keyin
//     kalitni tashlaydi — shu jarayonda o'zgarish DARHOL ko'rinadi, TTL
//     faqat boshqa jarayon yozganda (admin vositalari) eskirish chegarasi.
//  2. POYGA: o'qish boshlangandan KEYIN bekor qilish bo'lsa (epoch
//     o'zgargan), o'sha o'qish natijasi keshga YOZILMAYDI — eskirgan qiymat
//     yangi qiymatni bosib ketolmaydi.
//  3. XATO KESHLANMAYDI (jumladan "topilmadi": yangi yaratilgan yozuv
//     darhol ko'rinishi uchun).
//  4. BIR VAQTDAGI bir xil kalit uchun bazaga BITTA so'rov (singleflight):
//     kesh tugagan paytda minglab so'rov bazani bosib ketmasin.
//  5. Hajm chegaralangan: to'lib ketsa butunlay tozalanadi (oddiy va
//     xotira oshib ketmaydi).
//
// └───────────────────────────────────────────────────────────────────┘
//
// Qaytarilgan qiymat kesh bilan BO'LISHILADI — chaqiruvchiga berishdan oldin
// dekorator uni NUSXALASHI SHART (aks holda bitta so'rov qiymatni o'zgartirsa
// boshqalar ko'radi).
type Cache[V any] struct {
	mu    sync.Mutex
	ttl   time.Duration
	max   int
	now   func() time.Time
	epoch uint64
	data  map[string]entry[V]
	fly   map[string]*flight[V]
}

type entry[V any] struct {
	v       V
	expires time.Time
}

type flight[V any] struct {
	done chan struct{}
	v    V
	err  error
}

func New[V any](ttl time.Duration, max int) *Cache[V] {
	return &Cache[V]{
		ttl: ttl, max: max, now: time.Now,
		data: make(map[string]entry[V]), fly: make(map[string]*flight[V]),
	}
}

// Get — keshdan oladi, yo'q bo'lsa `load` ni BITTA marta chaqiradi.
func (c *Cache[V]) Get(key string, load func() (V, error)) (V, error) {
	c.mu.Lock()
	if e, ok := c.data[key]; ok && c.now().Before(e.expires) {
		c.mu.Unlock()
		return e.v, nil
	}
	if f, ok := c.fly[key]; ok {
		c.mu.Unlock()
		<-f.done
		return f.v, f.err
	}
	f := &flight[V]{done: make(chan struct{})}
	c.fly[key] = f
	startEpoch := c.epoch
	c.mu.Unlock()

	v, err := load()

	c.mu.Lock()
	f.v, f.err = v, err
	if c.fly[key] == f { // bekor qilish yangi uchishni boshlagan bo'lishi mumkin
		delete(c.fly, key)
	}
	if err == nil && c.epoch == startEpoch {
		c.putLocked(key, v)
	}
	c.mu.Unlock()
	close(f.done)
	return v, err
}

// Peek — faqat keshdan (yuklamasdan) va hozirgi epoch'ni qaytaradi.
func (c *Cache[V]) Peek(key string) (v V, ok bool, epoch uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, found := c.data[key]; found && c.now().Before(e.expires) {
		return e.v, true, c.epoch
	}
	var zero V
	return zero, false, c.epoch
}

// Put — `Peek` bergan epoch o'zgarmagan bo'lsagina yozadi.
func (c *Cache[V]) Put(key string, v V, epoch uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.epoch == epoch {
		c.putLocked(key, v)
	}
}

func (c *Cache[V]) putLocked(key string, v V) {
	if len(c.data) >= c.max {
		c.data = make(map[string]entry[V])
	}
	c.data[key] = entry[V]{v: v, expires: c.now().Add(c.ttl)}
}

// Invalidate — kalitni tashlaydi va o'qilayotgan (eskirgan bo'lishi mumkin)
// natijalar keshga yozilmasligi uchun epoch'ni oshiradi.
func (c *Cache[V]) Invalidate(key string) {
	c.mu.Lock()
	delete(c.data, key)
	delete(c.fly, key)
	c.epoch++
	c.mu.Unlock()
}

// Clear — hammasini tashlaydi.
func (c *Cache[V]) Clear() {
	c.mu.Lock()
	c.data = make(map[string]entry[V])
	c.fly = make(map[string]*flight[V])
	c.epoch++
	c.mu.Unlock()
}
