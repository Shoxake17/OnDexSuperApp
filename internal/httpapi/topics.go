// WebSocket obuna kalitlari.
//
// Sxema `internal/notify/topic.go` da BIR JOYDA belgilangan — bu yerda
// faqat qisqa yorliqlar. Avval kalitlar ikki xil tizimda aralash edi
// (xom ID va "restaurant:<id>" mavzular), o'sha yerdagi izohga qarang.
package httpapi

import "chustapp/internal/notify"

// restaurantTopic — menyu/aksiya jonli yangilanishlari uchun umumiy
// kalit (foydalanuvchiga emas, RESTORANga bog'liq) — shu restoran
// menyusini ochib turgan HAMMA mijozlarga bir vaqtda yetkaziladi.
func restaurantTopic(id string) string { return notify.Entity(notify.ModuleFood, id) }

// orderTopic — kuryerning JONLI GPS joylashuvini shu buyurtmani kuzatib
// turgan mijozga yetkazish uchun.
func orderTopic(id string) string { return notify.Order(id) }

// userTopic — foydalanuvchining SHAXSIY kanali (bildirishnomalar).
func userTopic(id string) string { return notify.User(id) }

// adminTopic — superadmin panelining jonli kanali. Unga faqat `admin`
// roli obuna bo'ladi (`routes_ws.go`), sabab `notify/topic.go` da.
func adminTopic() string { return notify.Admin() }
