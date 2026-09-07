// Package safego — fon goroutine'larini panikadan himoyalab ishga
// tushirish.
//
// ┌─ NEGA BU PAKET BOR (bug.md 34 va 44-bandlar) ──────────────────────┐
// Go'da goroutine ichidagi TUTILMAGAN panic butun JARAYONNI
// o'ldiradi. `net/http` ning handler recover'i bu yerda yordam
// bermaydi: u faqat SO'ROV goroutine'ini qamrab oladi, qo'lda
// ochilgan `go ...` esa undan tashqarida.
//
// Ya'ni bitta fon vazifasidagi nil-pointer butun API'ni yiqitadi va
// HAMMA foydalanuvchi uziladi.
//
// Bu nazariy emas: 34-band aynan shunday panic topgan
// (`send on closed channel` — WebSocket hub'ida), va u
// `notify` ning recover'siz goroutine'i orqali jarayonni
// o'ldirardi.
//
// To'g'ri naqsh loyihada allaqachon bor edi
// (`internal/httpapi/dispatch.go: safeGo`), lekin faqat BITTA joyda
// ishlatilgandi. Endi u shu paketda va hamma joy shundan oladi —
// nusxalar ajralib ketmasin.
// └────────────────────────────────────────────────────────────────────┘
package safego

import (
	"log/slog"
	"runtime/debug"
)

// Go — fon goroutine'ini `recover()` bilan o'rab ishga tushiradi.
//
// `name` — logda ko'rinadigan vazifa nomi. U aniq bo'lishi kerak:
// panic sodir bo'lganda birinchi savol "qaysi vazifa?" bo'ladi.
//
// Panic YUTILMAYDI — u `slog.Error` bilan stack'i bilan birga
// yoziladi. Server ishlashda davom etadi, lekin xato ko'rinadi.
func Go(name string, fn func()) {
	go Run(name, fn)
}

// Run — `Go` ning sinxron varianti: joriy goroutine'da bajaradi.
//
// Kerak bo'ladigan joy: goroutine allaqachon boshqa sabab bilan
// ochilgan (masalan `for` sikli ichida) va uni o'rash kerak.
func Run(name string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("fon vazifasida panic (server ishlashda davom etadi)",
				"vazifa", name, "panic", r, "stack", string(debug.Stack()))
		}
	}()
	fn()
}
