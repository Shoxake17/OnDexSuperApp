package ratelimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Backend — taqsimlangan token-bucket (Redis: `storage.RedisRateBackend`).
// `key` allaqachon nom bilan ajratilgan va xeshlangan (IP/telefon ochiq
// holda Redis'ga yozilmaydi).
type Backend interface {
	Take(ctx context.Context, key string, perSecond, capacity float64) (ok bool, wait time.Duration, err error)
}

// backendTimeout — Redis javob bermasa so'rov KUTIB QOLMASIN: lokal
// cheklovga o'tiladi.
const backendTimeout = 150 * time.Millisecond

var (
	backend atomic.Pointer[backendBox]

	regMu    sync.Mutex
	registry = map[string]*Limiter{}
	all      []*Limiter

	lastWarn atomic.Int64
)

type backendBox struct{ b Backend }

// UseBackend — nomlangan (`Shared`) limiterlar uchun taqsimlangan backend.
// nil — faqat xotira.
func UseBackend(b Backend) {
	if b == nil {
		backend.Store(nil)
		return
	}
	backend.Store(&backendBox{b: b})
}

// Shared — limiterga BARQAROR nom beradi: backend berilganda hisob barcha
// API nusxalarida umumiy bo'ladi. Bir xil nom = bitta mantiqiy cheklov
// (masalan har `httpapi.NewServer` dagi `statsLimiter`) — hisob umumiy.
// Nom — Go o'zgaruvchisi nomi (kod bazasida yagona).
func (l *Limiter) Shared(name string) *Limiter {
	if name == "" {
		panic("ratelimit: Shared nomi bo'sh")
	}
	regMu.Lock()
	defer regMu.Unlock()
	if prev, ok := registry[name]; ok && (prev.rate != l.rate || prev.capacity != l.capacity) {
		// Bir nom — ikki xil chegara: taqsimlangan hisob noaniq bo'lardi.
		panic(fmt.Sprintf("ratelimit: %q nomi turli chegaralar bilan ishlatilgan", name))
	}
	l.name = name
	registry[name] = l
	all = append(all, l)
	return l
}

// Name — `Shared` nomi ("" — faqat lokal).
func (l *Limiter) Name() string { return l.name }

// distributed — backend orqali; done=false bo'lsa lokal yo'lga o'tiladi.
func (l *Limiter) distributed(key string) (ok bool, wait time.Duration, done bool) {
	if l.name == "" {
		return false, 0, false
	}
	bb := backend.Load()
	if bb == nil {
		return false, 0, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), backendTimeout)
	defer cancel()
	sum := sha256.Sum256([]byte(key))
	ok, wait, err := bb.b.Take(ctx, l.name+":"+hex.EncodeToString(sum[:16]), l.rate, l.capacity)
	if err != nil {
		// Log shovqini: ko'pi bilan 30 soniyada bir.
		now := time.Now().Unix()
		if prev := lastWarn.Load(); now-prev >= 30 && lastWarn.CompareAndSwap(prev, now) {
			slog.Warn("ratelimit: taqsimlangan backend xatosi — lokal cheklov ishlatilmoqda", "limiter", l.name, "err", err)
		}
		return false, 0, false
	}
	if !ok && wait < time.Second {
		wait = time.Second
	}
	return ok, wait.Round(time.Second), true
}

// ResetAll — barcha `Shared` limiterlarning XOTIRADAGI chelaklarini
// tozalaydi (testlar: paket darajasidagi limiterlar testlar orasida
// bo'linmasin). Production kodida chaqirilmaydi.
func ResetAll() {
	regMu.Lock()
	list := append([]*Limiter(nil), all...)
	regMu.Unlock()
	for _, l := range list {
		l.Reset()
	}
}

// Reset — limiterning xotiradagi chelaklarini tozalaydi.
func (l *Limiter) Reset() {
	l.mu.Lock()
	l.buckets = make(map[string]*bucket)
	l.mu.Unlock()
}
