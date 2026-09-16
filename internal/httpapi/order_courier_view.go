// Buyurtmadagi kuryer haqida mijoz/restoran ko'radigan ma'lumot.
package httpapi

import (
	"context"
	"strings"

	"chustapp/internal/orders"
	"chustapp/internal/users"
)

// courierPublicName — mijozga ko'rsatiladigan kuryer ismi: faqat ISM
// (birinchi so'z), familiyasiz.
//
// Kuryer — restoran xodimi va uning to'liq ismi xodim yozuvidan keladi.
// Mijozga "kim kelyapti" bilish uchun ism yetarli; familiya shaxsiy
// ma'lumot va yetkazish uchun kerak emas (Yandex/Uber ham shunday).
func courierPublicName(full string) string {
	fields := strings.Fields(full)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// courierLocationShared — kuryerning jonli joylashuvi mijozga ochiqmi.
//
// Faqat yetkazish buyurtmasida, kuryer biriktirilgan va buyurtma
// YAKUNLANMAGAN paytda. Yetkazilgach (yoki bekor qilingach) kuryer
// boshqa buyurtmaga ketadi — uning joylashuvi endi bu mijozga tegishli
// emas.
func courierLocationShared(o *orders.Order) bool {
	return o != nil && o.CourierID != "" && !o.IsDineIn() && !o.IsTerminal()
}

// withCourierInfo — `GET /orders/{id}` javobiga kuryer ismi va (faqat
// mijozga) joriy joylashuvini qo'shadi.
//
// ┌─ NEGA ─────────────────────────────────────────────────────────────┐
// Avval javobda faqat `courier_id` bor edi va mijoz ilovasi
// "Kuryer: staff-5f1c…" deb ichki identifikatorni ko'rsatardi
// (restoran kuryerlarining ID'si xodim yozuvidan yasaladi). Joylashuv
// esa faqat WebSocket orqali kelardi — ekran ochilganda xarita kuryerni
// keyingi yangilanishgacha (10-20 s) ko'rsata olmasdi.
// └────────────────────────────────────────────────────────────────────┘
//
// Chaqiruvchi `canSeeOrder` ni allaqachon tekshirgan. Kuryerning o'zi
// va affitsiant uchun hech narsa qo'shilmaydi.
func (s *Server) withCourierInfo(ctx context.Context, c *users.Claims, o *orders.Order, out any) any {
	if o == nil || o.CourierID == "" || o.IsDineIn() || s.CourierRepo == nil {
		return out
	}
	switch c.Role {
	case users.RoleCustomer, users.RoleRestaurant, users.RoleAdmin:
	default:
		return out
	}
	cr, err := s.CourierRepo.GetByID(ctx, o.CourierID)
	if err != nil {
		return out
	}
	name := strings.TrimSpace(cr.Name)
	if c.Role == users.RoleCustomer {
		name = courierPublicName(name)
	}
	m := withExtraField(out, "courier_name", name)
	if c.Role == users.RoleCustomer && courierLocationShared(o) && (cr.Lat != 0 || cr.Lng != 0) {
		m["courier_location"] = map[string]float64{"lat": cr.Lat, "lng": cr.Lng}
	}
	return m
}
