// Buyurtma kuzatuvi: yetkazish yo'li, kuryer joylashuvi va qolgan vaqt.
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/couriers"
	"chustapp/internal/delivery"
	"chustapp/internal/geo"
	"chustapp/internal/orders"
	"chustapp/internal/ratelimit"
	"chustapp/internal/tracking"
	"chustapp/internal/users"
)

// Kuzatuv bosqichlari (`deliveryTrackingView.Phase`).
const (
	// trackingNone — kuzatiladigan narsa yo'q (kuryer yo'q, bekor qilingan).
	trackingNone = "none"
	// trackingWaiting — kuryer biriktirilgan, taom hali restoranda.
	trackingWaiting = "waiting"
	// trackingLive — kuryer taomni oldi va mijozga kelmoqda.
	trackingLive = "live"
	// trackingDelivered — yetkazildi; yo'l buyurtmada saqlanib qoladi.
	trackingDelivered = "delivered"
)

const (
	// liveRouteFresh — shu vaqtdan yangi yo'l kuryer qayerga siljishidan
	// qat'i nazar qayta ishlatiladi.
	liveRouteFresh = 30 * time.Second
	// liveRouteMaxAge / liveRouteMoveMeters — kuryer deyarli joyida bo'lsa
	// (tirbandlik, svetofor) yo'l shuncha vaqtgacha qayta hisoblanmaydi.
	liveRouteMaxAge     = 3 * time.Minute
	liveRouteMoveMeters = 100.0
	// directionsRetryGap — Google xatosidan keyin shu vaqt qayta so'ralmaydi.
	directionsRetryGap = 15 * time.Second
	// plannedRouteRetryGap — A→B yo'lini hisoblash muvaffaqiyatsiz bo'lsa.
	plannedRouteRetryGap = time.Minute
	// liveRouteForget — shuncha vaqt so'ralmagan yozuv xotiradan o'chadi.
	liveRouteForget = time.Hour
	// courierArrivedMeters — kuryer manzilga shuncha yaqin bo'lsa "yetib
	// keldi" (qolgan vaqt 0), yo'l so'ralmaydi.
	courierArrivedMeters = 60.0
)

// trackingLimiter — foydalanuvchi bo'yicha: ekran har joylashuv hodisasida
// (≈10 s) so'raydi; 30 ta bir zumda, keyin soniyasiga bitta.
var trackingLimiter = ratelimit.New(1, 30)

type trackingRoute struct {
	Points          []tracking.Point `json:"points"`
	DistanceMeters  int              `json:"distance_meters"`
	DurationSeconds int              `json:"duration_seconds"`
}

// deliveryTrackingView — `GET /orders/{id}/tracking` javobi.
type deliveryTrackingView struct {
	Phase string `json:"phase"`
	// RestaurantName / RestaurantLogoURL — A nuqta belgisi (restoran logosi).
	// Faqat to'liq javobda (`planned` bilan): o'zgarmaydi, mijoz saqlab qo'yadi.
	RestaurantName    string `json:"restaurant_name,omitempty"`
	RestaurantLogoURL string `json:"restaurant_logo_url,omitempty"`
	// Origin — A nuqta (restoran), Destination — B nuqta (mijoz manzili).
	Origin      *tracking.Point `json:"origin,omitempty"`
	Destination *tracking.Point `json:"destination,omitempty"`
	// Courier — faqat `live` bosqichida.
	Courier *tracking.Point `json:"courier,omitempty"`
	// PlannedRoute — A→B yo'li (buyurtmada saqlanadi).
	PlannedRoute *trackingRoute `json:"planned_route,omitempty"`
	// RemainingRoute — kuryerdan B gacha qolgan yo'l (faqat `live`).
	RemainingRoute *trackingRoute `json:"remaining_route,omitempty"`
	// ETASeconds — `ComputedAt` paytidagi qolgan vaqt; mijoz ilovasi
	// o'tgan vaqtni o'zi ayiradi.
	ETASeconds  *int       `json:"eta_seconds,omitempty"`
	ComputedAt  *time.Time `json:"computed_at,omitempty"`
	PickedUpAt  *time.Time `json:"picked_up_at,omitempty"`
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
}

func (s *Server) registerTrackingRoutes(mux *http.ServeMux) {
	// GET /orders/{id}/tracking — mijoz ilovasidagi kuzatuv xaritasi.
	//
	// Kim ko'radi: buyurtma egasi (mijoz), o'sha restoran va admin.
	// Kuryerning o'ziga kerak emas (u o'z yo'lini ilovasida hisoblaydi).
	// Begona buyurtma — 404 (mavjudligi oshkor bo'lmasin, `GET /orders/{id}`
	// dagi izoh).
	mux.HandleFunc("GET /orders/{id}/tracking", s.auth(
		[]users.Role{users.RoleCustomer, users.RoleRestaurant, users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			claims := claimsFrom(r)
			if !trackingLimiter.Allow(claims.Subject) {
				httpError(w, http.StatusTooManyRequests, errors.New("kuzatuv juda tez-tez so'ralmoqda"))
				return
			}
			o, err := s.OrderSvc.Get(r.Context(), r.PathValue("id"))
			if err != nil || !canSeeOrder(claims, o) || o.IsDineIn() {
				httpError(w, http.StatusNotFound, orders.ErrNotFound)
				return
			}
			// `?planned=0` — mijozda A→B yo'li allaqachon bor: u o'zgarmaydi,
			// yuzlab nuqtani har 15 soniyada qayta yuborish mobil trafikni
			// behuda sarflardi (optimizatsiya, 2026-09-15).
			includePlanned := r.URL.Query().Get("planned") != "0"
			writeJSON(w, http.StatusOK, s.deliveryTracking(r.Context(), o, includePlanned))
		}))
}

func trackingPhase(o *orders.Order) string {
	if o.CourierID == "" || o.IsDineIn() {
		return trackingNone
	}
	switch o.Status {
	case orders.StatusAccepted, orders.StatusPreparing, orders.StatusReady:
		return trackingWaiting
	case orders.StatusPickedUp:
		return trackingLive
	case orders.StatusDelivered:
		return trackingDelivered
	}
	return trackingNone
}

// deliveryTracking — buyurtma holatiga mos kuzatuv ma'lumoti.
//
// Tashqi xizmat (Google) xatosi javobni BUZMAYDI: yo'l bo'lmasa ham
// nuqtalar va kuryer joylashuvi qaytadi, mijoz ilovasi belgilarni chizadi.
//
// includePlanned `false` — A→B nuqtalari javobga qo'shilmaydi (A nuqtasi
// qoladi).
func (s *Server) deliveryTracking(ctx context.Context, o *orders.Order, includePlanned bool) deliveryTrackingView {
	view := deliveryTrackingView{Phase: trackingPhase(o)}
	if view.Phase == trackingNone || view.Phase == trackingWaiting {
		// Taom olinmaguncha yo'l chizilmaydi (foydalanuvchi talabi);
		// kuryer joylashuvi `GET /orders/{id}` va jonli hodisada bor.
		return view
	}
	view.PickedUpAt = lastStatusAt(o, orders.StatusPickedUp)
	view.DeliveredAt = lastStatusAt(o, orders.StatusDelivered)
	if includePlanned {
		if rest := s.restaurantFor(ctx, o.RestaurantID); rest != nil {
			view.RestaurantName, view.RestaurantLogoURL = rest.Name, rest.LogoURL
		}
	}
	dest, hasDest := trackingPoint(o.DeliveryLat, o.DeliveryLng)
	if hasDest {
		view.Destination = &dest
	}

	var courier *couriers.Courier
	if s.CourierRepo != nil {
		if c, err := s.CourierRepo.GetByID(ctx, o.CourierID); err == nil {
			courier = c
		}
	}
	mode := routeModeFor(courier)

	if planned := s.plannedRoute(ctx, o, dest, hasDest, mode); planned != nil {
		origin := planned.Origin
		view.Origin = &origin
		if includePlanned {
			view.PlannedRoute = &trackingRoute{
				Points: planned.Points, DistanceMeters: planned.DistanceMeters, DurationSeconds: planned.DurationSeconds,
			}
		}
	} else if origin, ok := s.restaurantPoint(ctx, o.RestaurantID); ok {
		view.Origin = &origin
	}

	if view.Phase != trackingLive {
		// Yetkazildi: kuryer endi boshqa buyurtmada — uning joylashuvi
		// bu mijozga BERILMAYDI.
		s.liveRoutes.forget(o.ID)
		return view
	}
	if courier == nil {
		return view
	}
	from, ok := trackingPoint(courier.Lat, courier.Lng)
	if !ok {
		return view
	}
	view.Courier = &from
	if !hasDest {
		return view
	}
	if distanceMeters(from, dest) <= courierArrivedMeters {
		zero, now := 0, s.liveRoutes.now()
		view.ETASeconds, view.ComputedAt = &zero, &now
		return view
	}
	res, at, ok := s.liveRoutes.get(ctx, o.ID, from, func(ctx context.Context) (DirectionsResult, error) {
		return s.routeBetween(ctx, directionsTrackingLive, o.ID, from, dest, mode)
	})
	if ok {
		view.RemainingRoute = &trackingRoute{
			Points: res.Points, DistanceMeters: res.DistanceMeters, DurationSeconds: res.DurationSeconds,
		}
		eta := res.DurationSeconds
		view.ETASeconds, view.ComputedAt = &eta, &at
	}
	return view
}

// plannedRoute — buyurtmaga biriktirilgan A→B yo'li: saqlangani, bo'lmasa
// bir marta hisoblanib saqlanadi.
func (s *Server) plannedRoute(ctx context.Context, o *orders.Order, dest tracking.Point, hasDest bool, mode string) *tracking.Route {
	if s.RouteRepo == nil {
		return nil
	}
	rt, err := s.RouteRepo.GetRoute(ctx, o.ID)
	if err == nil {
		return rt
	}
	if !errors.Is(err, tracking.ErrNotFound) {
		slog.Warn("yetkazish yo'lini o'qib bo'lmadi", "order", o.ID, "err", err)
		return nil
	}
	if !hasDest {
		return nil
	}
	origin, ok := s.restaurantPoint(ctx, o.RestaurantID)
	if !ok || !s.liveRoutes.allowPlanned(o.ID) {
		return nil
	}
	res, err := s.routeBetween(ctx, directionsTrackingPlanned, o.ID, origin, dest, mode)
	if err != nil {
		slog.Warn("yetkazish yo'lini hisoblab bo'lmadi", "order", o.ID, "err", err)
		return nil
	}
	rt = &tracking.Route{
		OrderID: o.ID, Origin: origin, Destination: dest,
		Points:         tracking.Downsample(res.Points),
		DistanceMeters: res.DistanceMeters, DurationSeconds: res.DurationSeconds,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.RouteRepo.SaveRoute(ctx, rt); err != nil {
		slog.Warn("yetkazish yo'lini saqlab bo'lmadi", "order", o.ID, "err", err)
		return rt
	}
	// Parallel so'rov boshqa yozuvni birinchi saqlagan bo'lishi mumkin —
	// bazadagisi haqiqat.
	if saved, err := s.RouteRepo.GetRoute(ctx, o.ID); err == nil {
		return saved
	}
	return rt
}

func (s *Server) restaurantFor(ctx context.Context, restaurantID string) *catalog.Restaurant {
	if s.CatalogRepo == nil {
		return nil
	}
	rest, err := s.CatalogRepo.GetRestaurant(ctx, restaurantID)
	if err != nil {
		return nil
	}
	return rest
}

func (s *Server) restaurantPoint(ctx context.Context, restaurantID string) (tracking.Point, bool) {
	rest := s.restaurantFor(ctx, restaurantID)
	if rest == nil {
		return tracking.Point{}, false
	}
	return trackingPoint(rest.Lat, rest.Lng)
}

// routeModeFor — Google yo'nalish rejimi. Velosiped ham "driving": Google
// O'zbekistonda velosiped yo'nalishlarini bermaydi (ZERO_RESULTS,
// `couriers/dispatch.go` dagi izoh).
func routeModeFor(c *couriers.Courier) string {
	if c != nil && c.VehicleType == couriers.VehicleFoot {
		return "walking"
	}
	return "driving"
}

// trackingPoint — to'g'ri koordinata; "0,0" — joylashuv hali noma'lum.
func trackingPoint(lat, lng float64) (tracking.Point, bool) {
	if (lat == 0 && lng == 0) || !delivery.ValidCoords(lat, lng) {
		return tracking.Point{}, false
	}
	return tracking.Point{Lat: lat, Lng: lng}, true
}

func distanceMeters(a, b tracking.Point) float64 {
	return geo.HaversineMeters(geo.LatLng{Lat: a.Lat, Lng: a.Lng}, geo.LatLng{Lat: b.Lat, Lng: b.Lng})
}

func lastStatusAt(o *orders.Order, st orders.Status) *time.Time {
	for i := len(o.History) - 1; i >= 0; i-- {
		if o.History[i].To == st {
			at := o.History[i].At
			return &at
		}
	}
	return nil
}

// liveRouteCache — kuryerdan manzilgacha qolgan yo'l, buyurtma bo'yicha.
//
// ┌─ NEGA KESH ────────────────────────────────────────────────────────┐
// Mijoz ilovasi har joylashuv hodisasida (≈10 s) kuzatuvni so'raydi. Har
// safar Google Directions'ga borilsa bitta yetkazish ~100 ta PULLIK so'rov
// bo'lardi, ikki qurilmada ochilsa — ikki barobar. Kesh buyurtma bo'yicha:
// nechta mijoz/panel ochib turmasin, Google'ga ko'pi bilan 30 soniyada bir.
// Bir vaqtda kelgan so'rovlar bitta hisobni kutmaydi — tayyor bo'lgunicha
// oldingi natijani oladi (`inflight`).
// └────────────────────────────────────────────────────────────────────┘
type liveRouteCache struct {
	mu      sync.Mutex
	entries map[string]*liveRouteEntry
	planned map[string]time.Time // A→B hisoblashga oxirgi urinish
	now     func() time.Time
}

type liveRouteEntry struct {
	from      tracking.Point
	res       DirectionsResult
	at        time.Time
	ok        bool
	attempted time.Time
	inflight  bool
}

func newLiveRouteCache() *liveRouteCache {
	return &liveRouteCache{
		entries: map[string]*liveRouteEntry{},
		planned: map[string]time.Time{},
		now:     time.Now,
	}
}

// get — qolgan yo'l: natija, hisoblangan vaqt va natija borligi.
func (c *liveRouteCache) get(ctx context.Context, orderID string, from tracking.Point,
	fetch func(context.Context) (DirectionsResult, error)) (DirectionsResult, time.Time, bool) {
	now := c.now()
	c.mu.Lock()
	c.sweepLocked(now)
	e := c.entries[orderID]
	if e == nil {
		e = &liveRouteEntry{}
		c.entries[orderID] = e
	}
	if e.ok && e.reusable(from, now) {
		res, at := e.res, e.at
		c.mu.Unlock()
		return res, at, true
	}
	if e.inflight || (!e.attempted.IsZero() && now.Sub(e.attempted) < directionsRetryGap) {
		res, at, ok := e.res, e.at, e.ok
		c.mu.Unlock()
		return res, at, ok
	}
	e.inflight, e.attempted = true, now
	c.mu.Unlock()

	res, err := fetch(ctx)

	c.mu.Lock()
	defer c.mu.Unlock()
	e.inflight = false
	if err != nil {
		slog.Warn("kuryer yo'lini hisoblab bo'lmadi — oldingi natija qoldi", "order", orderID, "err", err)
		return e.res, e.at, e.ok
	}
	e.from, e.res, e.at, e.ok = from, res, now, true
	// `forget` hisob paytida yozuvni o'chirgan bo'lsa — qayta qo'yilmaydi.
	return res, now, true
}

func (e *liveRouteEntry) reusable(from tracking.Point, now time.Time) bool {
	age := now.Sub(e.at)
	if age < liveRouteFresh {
		return true
	}
	return age < liveRouteMaxAge && distanceMeters(e.from, from) < liveRouteMoveMeters
}

// allowPlanned — A→B yo'lini hisoblashga urinish mumkinmi (xatodan keyin
// daqiqasiga bir).
func (c *liveRouteCache) allowPlanned(orderID string) bool {
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sweepLocked(now)
	if last, ok := c.planned[orderID]; ok && now.Sub(last) < plannedRouteRetryGap {
		return false
	}
	c.planned[orderID] = now
	return true
}

func (c *liveRouteCache) forget(orderID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[orderID]; ok && !e.inflight {
		delete(c.entries, orderID)
	}
}

// sweepLocked — uzoq so'ralmagan yozuvlar (yetkazma tugagan, ekran yopilgan).
func (c *liveRouteCache) sweepLocked(now time.Time) {
	for id, e := range c.entries {
		last := e.at
		if e.attempted.After(last) {
			last = e.attempted
		}
		if !e.inflight && now.Sub(last) > liveRouteForget {
			delete(c.entries, id)
		}
	}
	for id, at := range c.planned {
		if now.Sub(at) > liveRouteForget {
			delete(c.planned, id)
		}
	}
}
