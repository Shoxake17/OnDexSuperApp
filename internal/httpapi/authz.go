// Avtorizatsiya qoidalari — "kim nimani ko'ra oladi va o'zgartira oladi".
//
// ATAYLAB alohida fayl: bu qoidalar HTTP dan mustaqil (sof funksiyalar),
// shuning uchun ularni handler kodidan qidirib topish shart emas va
// testda to'g'ridan-to'g'ri chaqirish mumkin.
package httpapi

import (
	"context"
	"encoding/json"

	"chustapp/internal/orders"
	"chustapp/internal/users"
)

func roleToActor(role users.Role) orders.Actor {
	switch role {
	case users.RoleCustomer:
		return orders.ActorCustomer
	case users.RoleRestaurant:
		return orders.ActorRestaurant
	case users.RoleCourier:
		return orders.ActorCourier
	case users.RoleWaiter:
		return orders.ActorWaiter
	case users.RoleAdmin:
		return orders.ActorAdmin
	}
	return ""
}

// ownsOrderAction — holatni o'zgartirish huquqi: mijoz o'z buyurtmasi, restoran
// o'z restorani buyurtmasi, kuryer o'ziga biriktirilgan buyurtma; admin hammasi.
func ownsOrderAction(c *users.Claims, o *orders.Order) bool {
	switch c.Role {
	case users.RoleCustomer:
		return o.CustomerID == c.Subject
	case users.RoleRestaurant:
		return o.RestaurantID == c.EntityID
	case users.RoleCourier:
		return o.CourierID == c.EntityID
	case users.RoleWaiter:
		// ┌─ AFFITSIANT — IKKI SHART ─────────────────────────────────┐
		// 1. O'Z restorani (EntityID — restoran ID'si, xuddi
		//    RoleRestaurant kabi);
		// 2. FAQAT stol buyurtmasi.
		//
		// Ikkinchi shart SHART: usiz affitsiant o'z restoranidagi
		// YETKAZISH buyurtmalarini ham o'zgartira olardi. Holat
		// mashinasi buni baribir to'xtatadi (`delivery` jadvalida
		// `ActorWaiter` umuman yo'q), lekin himoya bitta qatlamga
		// tayanmasligi kerak — bu yerdagi tekshiruv buyurtmani
		// KO'RISHNI ham cheklaydi (canSeeOrder shu funksiyani
		// chaqiradi), holat mashinasi esa faqat yozishni cheklaydi.
		// └───────────────────────────────────────────────────────────┘
		return o.RestaurantID == c.EntityID && o.IsDineIn()
	case users.RoleAdmin:
		return true
	}
	return false
}

func canSeeOrder(c *users.Claims, o *orders.Order) bool {
	// Ko'rish huquqi hozircha o'zgartirish huquqi bilan bir xil, faqat kuryer
	// hali biriktirilmagan bo'lsa ham taklif bosqichida ko'rishi kerak bo'ladi —
	// bu WebSocket bosqichida qayta ko'riladi.
	return ownsOrderAction(c, o)
}

// redactForCourierBeforePickup — kuryer buyurtmani QABUL qilgach, taomni
// hali RESTORANDAN OLMAGUNCHA (pickup-kod tasdiqlanmaguncha) taomlar
// ro'yxati va mijozning yetkazib berish koordinatasini KO'RMASLIGI kerak —
// faqat restoran manzili (qayerga borishi) ma'lum bo'lishi kifoya. Bu
// mijoz maxfiyligini ham, "kuryer haqiqatan bormasdan ma'lumotni bilib
// olishi"ni ham cheklaydi. Boshqa har qanday tomon (mijoz, restoran, admin)
// yoki picked_up/delivered holatidagi buyurtma — to'liq holicha qaytadi.
func redactForCourierBeforePickup(c *users.Claims, o *orders.Order) any {
	if c.Role != users.RoleCourier || c.EntityID != o.CourierID {
		return o
	}
	switch o.Status {
	case orders.StatusAccepted, orders.StatusPreparing, orders.StatusReady:
		redacted := *o
		redacted.Items = nil
		redacted.DeliveryLat = 0
		redacted.DeliveryLng = 0
		// DeliveryAddress ham YASHIRILADI — u koordinatadan ham
		// oshkoraroq (ko'cha nomi, kvartira, domofon kodi). Kuryer
		// restoranga yetib borib buyurtmani olmaguncha mijozning uy
		// manzilini bilishi kerak emas — koordinatani yashirib, matnli
		// manzilni ochiq qoldirish redaksiyani ma'nosiz qilardi.
		redacted.DeliveryAddress = orders.Address{}
		return &redacted
	default:
		return o
	}
}

// restaurantPhoneFor — restoran akkaunti (`users`, role=restaurant,
// entity_id=restaurantID) telefon raqamini topadi. Alohida DB so'rovi
// o'rniga `ListByRole` + filtr ishlatiladi — bitta shahar uchun restoranlar
// soni kam (o'nlab), qo'shimcha repository metodi/migratsiya shart emas.
// Topilmasa bo'sh satr qaytadi (chaqiruvchi shunchaki maydonni qo'shmaydi).
func restaurantPhoneFor(ctx context.Context, userRepo users.Repository, restaurantID string) string {
	// Bitta indeksli so'rov (`idx_users_role_entity`). Avval barcha restoran
	// akkauntlari o'qilib, Go'da filtrlanardi — `role` indeksi yo'qligi
	// sababli bu butun `users` jadvalini (mijozlar bilan) ko'rib chiqardi.
	u, err := userRepo.GetByRoleEntity(ctx, users.RoleRestaurant, restaurantID)
	if err != nil {
		return ""
	}
	return u.Phone
}

// customerPhoneFor — mijoz akkauntining telefon raqami. Restoran(lar)dan
// farqli, mijoz `users.User.ID`si bevosita `orders.Order.CustomerID`
// sifatida saqlanadi (`EntityID` orqali emas) — shuning uchun to'g'ridan-
// to'g'ri `GetByID` bilan topiladi, ro'yxatni skanerlash shart emas.
func customerPhoneFor(ctx context.Context, userRepo users.Repository, customerID string) string {
	u, err := userRepo.GetByID(ctx, customerID)
	if err != nil {
		return ""
	}
	return u.Phone
}

// withExtraField — `v` (odatda *orders.Order yoki uning redaksiyalangan
// nusxasi) JSON shaklini BUZMASDAN (hamon "yassi" obyekt sifatida) bitta
// qo'shimcha maydon bilan boyitadi. Domain modelini (`orders.Order`) bunday
// so'rovga xos maydon (`restaurant_phone`) bilan ifloslantirmaslik uchun —
// bu faqat shu endpoint javobiga xos, buyurtmaning o'zi bilan saqlanmaydi.
func withExtraField(v any, key string, value any) map[string]any {
	b, err := json.Marshal(v)
	if err != nil {
		return map[string]any{key: value}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]any{key: value}
	}
	m[key] = value
	return m
}
