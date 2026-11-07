package identity

import (
	"sync"
	"time"
)

// failGuard — akkaunt bo'yicha XATO urinishlar hisoblagichi (TOTP, kod).
//
// Oddiy token-bucket muvaffaqiyatli urinishlarni ham sanardi va halol egasi
// (ulash + bir necha kirish) o'zini bloklab qo'yardi. Bu yerda faqat
// xatolar sanaladi: `window` ichida `max` ta xato — akkaunt `window`
// davomida bloklanadi; muvaffaqiyat hisobni tozalaydi.
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
	n     int
	first time.Time
}

func newFailGuard(max int, window time.Duration, now func() time.Time) *failGuard {
	return &failGuard{max: max, window: window, now: now, byKey: map[string]*failEntry{}}
}

// Blocked — kalit hozir bloklanganmi.
func (g *failGuard) Blocked(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	e, ok := g.byKey[key]
	if !ok {
		return false
	}
	if g.now().Sub(e.first) >= g.window {
		delete(g.byKey, key)
		return false
	}
	return e.n >= g.max
}

// Fail — xato urinishni qayd qiladi.
func (g *failGuard) Fail(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	if len(g.byKey) > 100_000 { // xotirani cheklash: eskirganlarni tozalash
		for k, e := range g.byKey {
			if now.Sub(e.first) >= g.window {
				delete(g.byKey, k)
			}
		}
	}
	e, ok := g.byKey[key]
	if !ok || now.Sub(e.first) >= g.window {
		g.byKey[key] = &failEntry{n: 1, first: now}
		return
	}
	e.n++
}

// Reset — muvaffaqiyatdan keyin.
func (g *failGuard) Reset(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.byKey, key)
}
