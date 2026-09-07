// WebSocket obuna kalitlari.
//
// Sxema `internal/notify/topic.go` da BIR JOYDA belgilangan — bu yerda
// faqat qisqa yorliqlar. Avval kalitlar ikki xil tizimda aralash edi
// (xom ID va "restaurant:<id>" mavzular), o'sha yerdagi izohga qarang.
package httpapi

import "chustapp/internal/notify"

// restaurantTopic — restoran/kuryer XODIM kanali: yangi buyurtma,
// holat o'zgarishi, dispatch, kuryer taklifi. Bu MAXFIY oqim —
// unga faqat o'sha tashkilotning tokeni bilan obuna bo'linadi
// (`routes_ws.go`).
func restaurantTopic(id string) string { return notify.Entity(notify.ModuleFood, id) }

// restaurantPublicTopic — shu restoran sahifasini ochib turgan HAMMA
// mijozlar uchun ochiq kanal: menyu/aksiya/3D model yangilandi.
// `?restaurant_id=` AYNAN shu kanalga obuna qiladi; sabab
// `notify/topic.go` dagi `PublicEntity` izohida.
func restaurantPublicTopic(id string) string { return notify.PublicEntity(notify.ModuleFood, id) }

// sendPublicRestaurantEvent — ochiq signalni IKKALA kanalga yuboradi:
// mijozlarga (ochiq kanal) va restoran panelining o'ziga (xodim
// kanali). Ochiq signallar shu funksiya orqali o'tishi kerak.
func (s *Server) sendPublicRestaurantEvent(restaurantID string, event map[string]any) {
	if s.Hub == nil || restaurantID == "" {
		return
	}
	s.Hub.Send(restaurantPublicTopic(restaurantID), event)
	s.Hub.Send(restaurantTopic(restaurantID), event)
}

// orderTopic — kuryerning JONLI GPS joylashuvini shu buyurtmani kuzatib
// turgan mijozga yetkazish uchun.
func orderTopic(id string) string { return notify.Order(id) }

// userTopic — foydalanuvchining SHAXSIY kanali (bildirishnomalar).
func userTopic(id string) string { return notify.User(id) }

// adminTopic — superadmin panelining jonli kanali. Unga faqat `admin`
// roli obuna bo'ladi (`routes_ws.go`), sabab `notify/topic.go` da.
func adminTopic() string { return notify.Admin() }
