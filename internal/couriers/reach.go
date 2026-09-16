package couriers

import (
	"context"
	"log/slog"
	"sort"
	"time"
)

// closedAppLocationMaxAge — ilovasi YOPIQ, lekin push bilan uyg'otsa
// bo'ladigan kuryerning oxirgi joylashuvi shundan eski bo'lmasligi kerak.
//
// Ilova yopilganda joylashuv yuborish to'xtaydi (Flutter jarayoni bilan
// birga). Oddiy chegara (`locationMaxAge`, 3 daqiqa) bunday kuryerni
// darhol nomzodlardan chiqarardi — "liniyada" bo'lsa ham buyurtma
// kelmasdi. Restoranning o'z kuryeri odatda restoran yonida kutadi, shuning
// uchun eski joylashuv ham ETA uchun yaroqli.
//
// ┌─ NEGA 12 SOAT (avval 30 daqiqa edi) ──────────────────────────────┐
// Jonli sinov (2026-09-15): kuryer liniyada, ilova 21:57 da yopilgan,
// buyurtma 22:26 da keldi. Oxirgi QABUL QILINGAN joylashuv 30 daqiqadan
// eski bo'lib qolgani uchun dispatch 8 tsikl "bo'sh onlayn kuryer yo'q"
// deb aylandi va push UMUMAN yuborilmadi — push tokeni bor edi. Liniyada
// turgan kuryer vaqt o'tishi bilan JIM yo'qolmasligi kerak: chegara —
// bitta smena. Liniyadan chiqishni unutgan kuryer esa ketma-ket javobsiz
// takliflardan keyin avtomatik chiqariladi (`missed.go`).
// └───────────────────────────────────────────────────────────────────┘
const closedAppLocationMaxAge = 12 * time.Hour

// Reachability — taklif kuryerga YETIB BORADIMI.
//
// ┌─ NEGA (2026-09-15) ────────────────────────────────────────────────┐
// Taklif avval faqat WebSocket orqali yuborilardi. Ilovasi yopilgan
// kuryer bazada "liniyada" bo'lib qolar, dispatch unga taklif yuborib
// har to'lqinda 20 soniya BEHUDA kutardi, 3 daqiqadan keyin esa
// jimgina chiqarib yuborardi. Endi:
//   - ilova ochiq (WS ulangan) — oddiy tartibda;
//   - ilova yopiq, lekin push tokeni bor — push bilan uyg'otiladi,
//     joylashuvi 30 daqiqagacha qabul qilinadi, navbatda ochiqlardan KEYIN;
//   - ilova yopiq va push yo'q — taklif yuborilmaydi (yetib bormaydi).
//
// └────────────────────────────────────────────────────────────────────┘
type Reachability interface {
	// Online — ilova ochiq, jonli kanal (WebSocket) ulangan.
	Online(courierID string) bool
	// CanPush — ilova yopiq bo'lsa ham push bilan uyg'otsa bo'ladi.
	CanPush(ctx context.Context, courierID string) bool
}

// WithReachability — yetkazib bo'lishni tekshiruvchini ulaydi. Ulanmasa
// (testlar, push sozlanmagan muhit) avvalgi xatti-harakat saqlanadi.
func (d *Dispatcher) WithReachability(r Reachability) *Dispatcher {
	d.reach = r
	return d
}

// candidates — shu tsikl nomzodlari.
func (d *Dispatcher) candidates(ctx context.Context, params DispatchParams) ([]*Courier, error) {
	loc := params.RestaurantLocation
	fresh, err := d.repo.ListAvailableNear(ctx, params.CourierPool, loc.Lat, loc.Lng,
		searchRadiusMeters, locationMaxAge, maxCandidates)
	if err != nil || d.reach == nil {
		return fresh, err
	}
	out := make([]*Courier, 0, len(fresh))
	seen := make(map[string]struct{}, len(fresh))
	for _, c := range fresh {
		seen[c.ID] = struct{}{}
		if d.reach.Online(c.ID) || d.reach.CanPush(ctx, c.ID) {
			out = append(out, c)
		}
	}
	// Ilovasi yopilgan (joylashuvi eskirgan), lekin push bilan
	// uyg'otsa bo'ladigan kuryerlar.
	extended, err := d.repo.ListAvailableNear(ctx, params.CourierPool, loc.Lat, loc.Lng,
		searchRadiusMeters, closedAppLocationMaxAge, maxCandidates)
	if err != nil {
		slog.Warn("dispatch: ilovasi yopiq kuryerlarni o'qib bo'lmadi", "err", err)
		return out, nil
	}
	for _, c := range extended {
		if _, ok := seen[c.ID]; ok {
			continue
		}
		if !d.reach.Online(c.ID) && d.reach.CanPush(ctx, c.ID) {
			out = append(out, c)
		}
	}
	return out, nil
}

// preferOnline — ilovasi OCHIQ kuryerlar navbatda oldinda (ular taklifni
// darhol ko'radi), push bilan uyg'otiladiganlar keyin. Ball tartibi har
// guruh ichida saqlanadi.
func (d *Dispatcher) preferOnline(ranked []ScoredCandidate) []ScoredCandidate {
	if d.reach == nil || len(ranked) < 2 {
		return ranked
	}
	online := make(map[string]bool, len(ranked))
	for _, c := range ranked {
		online[c.Courier.ID] = d.reach.Online(c.Courier.ID)
	}
	out := make([]ScoredCandidate, len(ranked))
	copy(out, ranked)
	sort.SliceStable(out, func(i, j int) bool {
		return online[out[i].Courier.ID] && !online[out[j].Courier.ID]
	})
	return out
}

// PendingOffer — kuryerga HOZIR ochiq turgan taklif; `ExpiresIn` — qolgan vaqt.
//
// Ilova qayta ochilganda, oldinga chiqqanda yoki push bosilganda taklifni
// TIKLASH uchun (`GET /couriers/{id}/offer`). Avval taklif faqat WebSocket
// xabari sifatida ilova xotirasida turardi va ilovadan chiqilsa yo'qolardi,
// server esa uni hamon ochiq deb kutib turardi.
//
// Rad etgan kuryerga qaytarilmaydi; muddati bir soniyadan kam qolgan
// taklif ham (javob baribir ulgurmaydi).
func (d *Dispatcher) PendingOffer(courierID string) (OfferInfo, bool) {
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, o := range d.pending {
		if !o.inWave(courierID) {
			continue
		}
		if _, declined := o.declined[courierID]; declined {
			continue
		}
		left := o.expiresAt.Sub(now)
		if left < time.Second {
			continue
		}
		info := o.info
		info.ExpiresIn = left
		return info, true
	}
	return OfferInfo{}, false
}

// OfferTTL — bitta to'lqinga beriladigan to'liq vaqt.
func (d *Dispatcher) OfferTTL() time.Duration { return d.offerTTL }

// Payload — kuryer ilovasiga boradigan "offer" xabari. WebSocket va
// `GET /couriers/{id}/offer` AYNAN shu shakldan foydalanadi — ilova ikkala
// yo'ldan kelganini bir xil ishlaydi.
func (i OfferInfo) Payload(total time.Duration) map[string]any {
	return map[string]any{
		"type":                "offer",
		"order_id":            i.OrderID,
		"expires_in_sec":      int(i.ExpiresIn.Seconds()),
		"total_sec":           int(total.Seconds()),
		"restaurant_id":       i.RestaurantID,
		"restaurant_name":     i.RestaurantName,
		"restaurant_address":  i.RestaurantAddress,
		"restaurant_lat":      i.RestaurantLat,
		"restaurant_lng":      i.RestaurantLng,
		"restaurant_logo_url": i.RestaurantLogoURL,
	}
}
