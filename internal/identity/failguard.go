package identity

import (
	"sync"
	"time"
)

// failGuard — akkaunt bo'yicha XATO urinishlar hisoblagichi (TOTP, kod, parol).
//
// Oddiy token-bucket muvaffaqiyatli urinishlarni ham sanardi va halol egasi
// (ulash + bir necha kirish) o'zini bloklab qo'yardi. Bu yerda faqat
// xatolar sanaladi: `window` ichida `max` ta xato — akkaunt `window`
// davomida bloklanadi; muvaffaqiyat hisobni tozalaydi.
//
// AUD-14: kod taxmin qilinadigan yo'llar `Begin` dan foydalanadi — tekshiruv
// va "urinish boshlandi" belgisi BITTA qulf ostida. Avvalgi "Blocked ->
// (tekshirish) -> Fail" ketma-ketligida N ta parallel so'rov hammasi
// "bloklanmagan" deb o'tib, `max` chegarasini chetlab o'tardi (TOCTOU).
// Endi bajarilayotgan urinishlar ham chegaraga kiradi: bir vaqtda
// `max - xatolar` dan ortiq urinish bo'lmaydi.
//
// Xotirada (bitta nusxa): bir nechta API nusxasida hujumchi har nusxada
// `max` urinish oladi — 10^6 kod maydonida baribir ahamiyatsiz.
type failGuard struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	now    func() time.Time
	byKey  map[string]*failEntry
}

type failEntry struct {
	n        int // xatolar
	inflight int // tugallanmagan urinishlar (Begin)
	first    time.Time
}

// attemptResult — `Begin` urinishining natijasi.
type attemptResult int

const (
	attemptAbort attemptResult = iota // tekshiruvgacha yetmadi (hisobga olinmaydi)
	attemptFail                       // noto'g'ri kod/parol
	attemptOK                         // muvaffaqiyat — hisob tozalanadi
)

func newFailGuard(max int, window time.Duration, now func() time.Time) *failGuard {
	return &failGuard{max: max, window: window, now: now, byKey: map[string]*failEntry{}}
}

// entry — qulf ostida: eskirgan yozuv yangilanadi.
func (g *failGuard) entry(key string, create bool) *failEntry {
	now := g.now()
	e, ok := g.byKey[key]
	if ok && e.inflight == 0 && now.Sub(e.first) >= g.window {
		delete(g.byKey, key)
		e, ok = nil, false
	}
	if !ok && create {
		if len(g.byKey) > 100_000 { // xotirani cheklash: eskirganlarni tozalash
			for k, x := range g.byKey {
				if x.inflight == 0 && now.Sub(x.first) >= g.window {
					delete(g.byKey, k)
				}
			}
		}
		e = &failEntry{first: now}
		g.byKey[key] = e
	}
	return e
}

// Blocked — kalit hozir bloklanganmi (faqat o'qish; taxmin qilib bo'lmaydigan
// dalillar — passkey imzosi — va oldindan tekshiruv uchun).
func (g *failGuard) Blocked(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	e := g.entry(key, false)
	return e != nil && e.n+e.inflight >= g.max
}

// Begin — urinishni atomik boshlaydi. ok=false — bloklangan. Aks holda
// `end` ANIQ bir marta chaqirilishi kerak (odatda defer bilan).
func (g *failGuard) Begin(key string) (end func(attemptResult), ok bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	e := g.entry(key, true)
	if e.n+e.inflight >= g.max {
		if e.n == 0 && e.inflight == 0 {
			delete(g.byKey, key)
		}
		return nil, false
	}
	e.inflight++
	var once sync.Once
	return func(r attemptResult) { once.Do(func() { g.finish(key, r) }) }, true
}

func (g *failGuard) finish(key string, r attemptResult) {
	g.mu.Lock()
	defer g.mu.Unlock()
	e := g.byKey[key]
	if e != nil && e.inflight > 0 {
		e.inflight--
	}
	switch r {
	case attemptOK:
		if e != nil && e.inflight == 0 {
			delete(g.byKey, key)
		} else if e != nil {
			e.n = 0 // parallel urinishlar hali tugamagan — faqat xatolar tozalanadi
		}
	case attemptFail:
		g.failLocked(key, e)
	default:
		if e != nil && e.n == 0 && e.inflight == 0 {
			delete(g.byKey, key)
		}
	}
}

func (g *failGuard) failLocked(key string, e *failEntry) {
	now := g.now()
	if e == nil {
		g.byKey[key] = &failEntry{n: 1, first: now}
		return
	}
	if e.n == 0 || now.Sub(e.first) >= g.window {
		e.n, e.first = 1, now
		return
	}
	e.n++
}

// Fail — xato urinishni qayd qiladi (`Begin` ishlatilmagan yo'llar uchun).
func (g *failGuard) Fail(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.failLocked(key, g.entry(key, false))
}

// Reset — muvaffaqiyatdan keyin (`Begin` ishlatilmagan yo'llar uchun).
func (g *failGuard) Reset(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if e := g.byKey[key]; e != nil && e.inflight > 0 {
		e.n = 0
		return
	}
	delete(g.byKey, key)
}
