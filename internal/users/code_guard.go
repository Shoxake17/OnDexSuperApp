package users

import (
	"errors"
	"sync"
	"time"
)

// Kod maqsadi (telefon/email) bo'yicha SUTKALIK chegara (SSO auditi).
//
// ┌─ NIMAGA ─────────────────────────────────────────────────────────────┐
// Bitta kod — 5 urinish, 60 s qayta yuborish oralig'i. Lekin hujumchi har
// daqiqada YANGI kod so'rab, har biriga 5 taxmin qilsa: soatiga ~300,
// oyiga ~216 ming taxmin — 10^6 maydonda ~20% ehtimol. Email kodi shu
// yo'l bilan (begona manzilga kod so'rab) haqiqatan terib topilishi
// mumkin edi. Sutkalik chegara bilan: kuniga ≤10 kod, ≤20 xato taxmin —
// yiliga ~0.7%. Telegram kanalida kod faqat raqam EGASINING Telegram'iga
// beriladi, lekin chegara u yerda ham amal qiladi (chuqur himoya).
// └───────────────────────────────────────────────────────────────────────┘
//
// Xotirada (bitta API nusxasi). Ko'p nusxada har nusxa o'z hisobini
// yuritadi — IN-2.2 (Redis) bilan umumiy bo'ladi.
const (
	codesPerDay    = 10
	failuresPerDay = 20
	codeGuardWin   = 24 * time.Hour
)

// ErrTooManyCodes — bu raqam/email uchun bugun juda ko'p kod so'raldi.
var ErrTooManyCodes = errors.New("bu raqam yoki email uchun bugun juda ko'p kod so'raldi — ertaga qayta urinib ko'ring")

type codeGuardEntry struct {
	start    time.Time
	issued   int
	failures int
}

type codeGuard struct {
	mu sync.Mutex
	m  map[string]*codeGuardEntry
}

func newCodeGuard() *codeGuard { return &codeGuard{m: map[string]*codeGuardEntry{}} }

func (g *codeGuard) entry(target string, now time.Time) *codeGuardEntry {
	if len(g.m) > 200_000 {
		for k, e := range g.m {
			if now.Sub(e.start) >= codeGuardWin {
				delete(g.m, k)
			}
		}
	}
	e, ok := g.m[target]
	if !ok || now.Sub(e.start) >= codeGuardWin {
		e = &codeGuardEntry{start: now}
		g.m[target] = e
	}
	return e
}

// allowIssue — yangi kod berish mumkinmi (va hisoblaydi).
func (g *codeGuard) allowIssue(target string, now time.Time) bool {
	if g == nil {
		return true
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	e := g.entry(target, now)
	if e.issued >= codesPerDay {
		return false
	}
	e.issued++
	return true
}

// blocked — bugungi xato taxminlar chegarasi tugaganmi.
func (g *codeGuard) blocked(target string, now time.Time) bool {
	if g == nil {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.entry(target, now).failures >= failuresPerDay
}

func (g *codeGuard) fail(target string, now time.Time) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.entry(target, now).failures++
}
