// Dispatch (kuryer qidirish) fon jarayoni.
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"runtime/debug"
	"time"

	"chustapp/internal/couriers"
	"chustapp/internal/geo"
)

// RecoverDispatch — server ishga tushganda topilgan "accepted, lekin
// kuryersiz" buyurtma uchun dispatch'ni QAYTA boshlaydi.
//
// Alohida eksport qilingan metod: `main()` faqat shu nomni biladi,
// `dispatchOrder` ning ichki tafsilotlarini emas.
func (s *Server) RecoverDispatch(orderID, restaurantID string, prepMinutes int) {
	safeGo("dispatch-recovery:"+orderID, func() {
		s.dispatchOrder(orderID, restaurantID, prepMinutes)
	})
}

// dispatchOrder — ETA-asoslangan matching engine'ni fon rejimida ishga
// tushiradi: restoran koordinatasini oladi, s.Dispatcher orqali ENG MOS
// (ETA/reyting/tajriba bo'yicha saralangan) kuryerlarga ketma-ket
// taklif yuboradi. TO'LIQ AVTOMATLASHTIRILGAN (foydalanuvchi so'rovi,
// 2026-07-30): restoranda "kuryer chaqirish" tugmasi UMUMAN yo'q —
// `s.Dispatcher.Dispatch` biror kuryer qabul qilguncha ICHKI TOMONDAN
// CHEKSIZ qayta uradi (`internal/couriers/dispatch.go`ga qarang),
// shuning uchun bu yerda muddat (timeout) YO'Q — faqat buyurtma
// boshqa sabab bilan (mijoz/admin bekor qilsa) terminal holatga
// o'tsa, `IsOrderCancelled` orqali to'xtaydi (aks holda zombie
// goroutine bo'lib qolar edi).
func (s *Server) dispatchOrder(orderID, restaurantID string, prepMinutes int) {
	ctx := context.Background()

	rest, err := s.CatalogRepo.GetRestaurant(ctx, restaurantID)
	if err != nil {
		slog.Error("dispatch: restoran topilmadi", "order", orderID, "err", err)
		s.Hub.Send(restaurantTopic(restaurantID), map[string]any{"type": "dispatch_failed", "order_id": orderID})
		return
	}

	courierID, err := s.Dispatcher.Dispatch(ctx, orderID, couriers.DispatchParams{
		RestaurantLocation: geo.LatLng{Lat: rest.Lat, Lng: rest.Lng},
		PreparationTime:    time.Duration(prepMinutes) * time.Minute,
		RestaurantID:       rest.ID,
		RestaurantName:     rest.Name,
		RestaurantAddress:  rest.Address,
		RestaurantLogoURL:  rest.LogoURL,
		IsOrderCancelled: func(ctx context.Context) (bool, error) {
			o, err := s.OrderSvc.Get(ctx, orderID)
			if err != nil {
				return false, err
			}
			return o.IsTerminal(), nil
		},
	})
	if err != nil {
		if errors.Is(err, couriers.ErrOrderCancelled) {
			slog.Info("dispatch: buyurtma bekor qilingani uchun to'xtatildi", "order", orderID)
			return
		}
		slog.Error("dispatch muvaffaqiyatsiz (haqiqiy infratuzilma xatosi)", "order", orderID, "err", err)
		// Restoranga jonli xabar — bu ENDI faqat haqiqiy xato (masalan
		// repo/DB xatosi) holatida keladi, oddiy "hozircha kuryer yo'q"
		// holatida EMAS (o'sha holatda Dispatch o'zi ichkarida cheksiz
		// qayta urinishda davom etadi va bu yerga umuman qaytmaydi).
		s.Hub.Send(restaurantTopic(restaurantID), map[string]any{
			"type":     "dispatch_failed",
			"order_id": orderID,
		})
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
func safeGo(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("fon vazifasida panic (server ishlashda davom etadi)",
					"vazifa", name, "panic", r, "stack", string(debug.Stack()))
			}
		}()
		fn()
	}()
}
