// Dispatch (kuryer qidirish) fon jarayoni.
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"chustapp/internal/couriers"
	"chustapp/internal/geo"
	"chustapp/internal/orders"
	"chustapp/internal/safego"
)

// launchDispatch — kuryer kutayotgan buyurtma uchun qidiruvni fon
// rejimida boshlaydi (qayta qidirish va nazoratchi tiklashi).
//
// Tayyorlash vaqtining faqat QOLGAN qismi beriladi: taom allaqachon
// tayyor bo'lsa kuryer ETA'si o'tib ketgan vaqtga emas, "hozir"ga
// moslanishi kerak (nol — moslashtirish yo'q, eng yaqini yaxshi).
func (s *Server) launchDispatch(o *orders.Order) {
	if s.Dispatcher == nil || o == nil || !o.NeedsDispatchRecovery() {
		return
	}
	prep := 0
	if o.Status != orders.StatusReady && o.ReadyAt != nil {
		if left := time.Until(*o.ReadyAt); left > 0 {
			prep = int(left.Minutes())
		}
	}
	oID, rID := o.ID, o.RestaurantID
	safeGo("dispatch:"+oID, func() { s.dispatchOrder(oID, rID, prep) })
}

// stopDispatch — ishlayotgan qidiruvni DARHOL to'xtatadi: kuryerlardagi
// ochiq takliflar ham yopiladi. Qidiruv bo'lmasa hech narsa qilmaydi.
func (s *Server) stopDispatch(orderID string) {
	if s.Dispatcher != nil {
		s.Dispatcher.Cancel(orderID)
	}
}

// dispatchStateEvent — qidiruv holati o'zgarganda panellarga ketadigan
// xabar. Faqat holat va muddat — mijoz ma'lumoti yo'q.
func dispatchStateEvent(eventType string, o *orders.Order) map[string]any {
	event := map[string]any{
		"type":           eventType,
		"order_id":       o.ID,
		"order_number":   o.OrderNumber,
		"dispatch_state": o.DispatchState,
	}
	if o.DispatchDeadline != nil {
		event["dispatch_deadline"] = o.DispatchDeadline.UTC()
	}
	return event
}

// publishDispatchState — restoran va superadmin panellariga jonli xabar.
func (s *Server) publishDispatchState(eventType string, o *orders.Order) {
	if s.Hub == nil || o == nil {
		return
	}
	event := dispatchStateEvent(eventType, o)
	s.Hub.Send(restaurantTopic(o.RestaurantID), event)
	s.Hub.Send(adminTopic(), event)
}

// dispatchOrder — ETA-asoslangan matching engine'ni ishga tushiradi:
// restoran koordinatasini oladi, s.Dispatcher orqali ENG MOS
// (ETA/reyting/tajriba bo'yicha saralangan) kuryerlarga taklif yuboradi.
//
// Bu yerda muddat YO'Q: qidiruvni vaqt bo'yicha to'xtatish
// nazoratchining ishi (`dispatch_watchdog.go`) — u buyurtmani "kuryer
// topilmadi" qilgach `stopDispatch` chaqiradi. Qidiruv yana har tsiklda
// buyurtmani bazadan tekshiradi (`IsOrderCancelled`), ya'ni boshqa
// server nusxasi yoki qo'lda o'zgartirish ham uni to'xtatadi.
func (s *Server) dispatchOrder(orderID, restaurantID string, prepMinutes int) {
	ctx := context.Background()

	rest, err := s.CatalogRepo.GetRestaurant(ctx, restaurantID)
	if err != nil {
		slog.Error("dispatch: restoran topilmadi", "order", orderID, "err", err)
		s.Hub.Send(restaurantTopic(restaurantID), map[string]any{"type": "dispatch_failed", "order_id": orderID})
		if s.AlertsSvc != nil {
			s.AlertsSvc.DispatchFailed(restaurantID, orderID)
		}
		return
	}

	courierID, err := s.Dispatcher.Dispatch(ctx, orderID, couriers.DispatchParams{
		RestaurantLocation: geo.LatLng{Lat: rest.Lat, Lng: rest.Lng},
		PreparationTime:    time.Duration(prepMinutes) * time.Minute,
		RestaurantID:       rest.ID,
		RestaurantName:     rest.Name,
		RestaurantAddress:  rest.Address,
		RestaurantLogoURL:  rest.LogoURL,
		// Qidiruv davom etishi kerakmi: buyurtma yakunlanmagan, kuryer
		// biriktirilmagan va "kuryer topilmadi" holatiga o'tmagan.
		IsOrderCancelled: func(ctx context.Context) (bool, error) {
			o, err := s.OrderSvc.Get(ctx, orderID)
			if err != nil {
				return false, err
			}
			return !o.NeedsDispatchRecovery(), nil
		},
	})
	if err != nil {
		switch {
		case errors.Is(err, couriers.ErrOrderCancelled), errors.Is(err, context.Canceled):
			slog.Info("dispatch: qidiruv to'xtatildi (buyurtma endi kuryer kutmayapti)", "order", orderID)
			return
		case errors.Is(err, couriers.ErrAlreadyRunning):
			// Shu buyurtma uchun qidiruv allaqachon ishlayapti (masalan
			// nazoratchi va restoran bir vaqtda boshladi) — ikkinchisi
			// kerak emas va bu xato EMAS.
			return
		}
		slog.Error("dispatch muvaffaqiyatsiz (haqiqiy infratuzilma xatosi)", "order", orderID, "err", err)
		// Restoranga jonli xabar — faqat haqiqiy xato (masalan DB xatosi)
		// holatida. Qidiruvni nazoratchi keyinroq qayta boshlaydi.
		s.Hub.Send(restaurantTopic(restaurantID), map[string]any{
			"type":     "dispatch_failed",
			"order_id": orderID,
		})
		// Bildirishnomalar tarixida ham qolsin: panel yopiq bo'lsa
		// jonli xabar yo'qolardi.
		if s.AlertsSvc != nil {
			s.AlertsSvc.DispatchFailed(restaurantID, orderID)
		}
		return
	}
	assigned, err := s.OrderSvc.AssignCourier(ctx, orderID, courierID)
	if err != nil {
		slog.Error("kuryer biriktirishda xato", "order", orderID, "err", err)
		// Dispatch.Dispatch() kuryerni g'olib deb SetAvailable(false)
		// qilib ulgurgan edi (dispatch.go), lekin buyurtma shu oraliqda
		// terminal holatga o'tib ketgan bo'lishi mumkin (masalan admin/
		// mijoz bekor qildi) — shuning uchun AssignCourier xato qaytardi.
		// Buni orqaga qaytarmasak, kuryer bazada abadiy "band" bo'lib
		// qolardi (yangi takliflar olmay, sababini bilmasdan).
		if err := s.CourierRepo.SetAvailable(ctx, courierID, true); err != nil {
			slog.Error("kuryerni band holatidan chiqarishda xato", "courier", courierID, "err", err)
		}
		return
	}
	// Restoran va mijozga jonli xabar — kuryer topilgani darhol
	// ko'rinishi uchun.
	event := map[string]any{
		"type":       "courier_assigned",
		"order_id":   orderID,
		"courier_id": courierID,
	}
	s.Hub.Send(restaurantTopic(assigned.RestaurantID), event)
	s.Hub.Send(userTopic(assigned.CustomerID), event)
	// Superadmin paneli — buyurtmalar jadvalidagi "Kuryer" ustuni va
	// boshqaruv ko'rsatkichlari shu eventdan yangilanadi.
	s.Hub.Send(adminTopic(), event)
}

// safeGo — fon goroutine'ini panikadan himoyalab ishga tushiradi.
//
// MUHIM: `net/http` FAQAT so'rov goroutine'idagi panikani ushlaydi.
// Qo'lda ochilgan `go ...` ichidagi panic (masalan nil-pointer) BUTUN
// jarayonni tugatadi — bitta buyurtma butun API'ni yiqitishi mumkin edi.
//
// Amalga oshirilishi `internal/safego` da (bug.md 44-band): avval bu
// naqsh loyihada FAQAT shu faylda bor edi va qolgan beshta yalang'och
// `go ...` himoyasiz qolgandi. Endi hamma joy bitta nusxadan oladi.
func safeGo(name string, fn func()) { safego.Go(name, fn) }
