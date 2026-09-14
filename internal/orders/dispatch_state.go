package orders

import (
	"context"
	"errors"
	"time"
)

// DispatchState — yetkazish buyurtmasi uchun kuryer qidiruvi holati.
//
// ┌─ NEGA HOLAT BUYURTMA YOZUVIDA ────────────────────────────────────┐
// Avval qidiruv holati FAQAT server xotirasida (Dispatcher goroutine'i)
// yashardi va bu ikki jiddiy xatoga olib keldi:
//
//  1. Qidiruv HECH QACHON tugamasdi. Onlayn kuryer bo'lmasa buyurtma
//     panelda "Kuryer qidirilmoqda..." bo'lib abadiy turardi, restoran
//     esa na bekor qila olardi, na qayta urina olardi.
//  2. Server qayta ishga tushsa `preparing`/`ready` holatidagi
//     buyurtmaning qidiruvi umuman TIKLANMASDI — tiklash faqat
//     `accepted` ni ko'rardi.
//
// Endi holat va muddat bazada. Qaror (topilmadi, avtomatik bekor)
// xotiradagi taymerga emas, shu ikki maydonga tayanadi — restartdan
// keyin ham aynan o'sha joydan davom etadi.
// └───────────────────────────────────────────────────────────────────┘
type DispatchState string

const (
	// DispatchNone — qidiruv boshlanmagan yoki kuryer biriktirilgan.
	DispatchNone DispatchState = ""
	// DispatchSearching — kuryer qidirilmoqda.
	DispatchSearching DispatchState = "searching"
	// DispatchNotFound — muddat ichida kuryer topilmadi: restoran qaror
	// qilishi kerak (bekor qilish yoki qayta qidirish).
	DispatchNotFound DispatchState = "not_found"
)

const (
	// CourierSearchWindow — taom TAYYOR bo'lgach shu vaqt ichida kuryer
	// topilmasa, restoran paneliga "Kuryer topilmadi" chiqadi.
	//
	// Hisob "tayyor" paytidan boshlanadi, qabul qilingan paytdan emas:
	// tayyorlash davomida kuryer yo'qligi muammo emas (u hali kerak
	// emas), taom tayyor turib sovishi esa muammo.
	CourierSearchWindow = 10 * time.Minute
	// CourierNotFoundCancelAfter — "Kuryer topilmadi" holatida restoran
	// hech narsa bosmasa, buyurtma shundan keyin AVTOMATIK bekor
	// qilinadi: mijoz noma'lum muddat kutib qolmaydi, karta to'lovi
	// qaytariladi.
	CourierNotFoundCancelAfter = 30 * time.Minute
)

// ErrDispatchNotRestartable — qidiruvni qayta boshlash faqat "kuryer
// topilmadi" holatidagi tayyor buyurtmada mumkin.
var ErrDispatchNotRestartable = errors.New(
	"kuryer qidiruvini qayta boshlab bo'lmaydi: buyurtma \"kuryer topilmadi\" holatida emas")

// AwaitsCourier — kuryer kutayotgan yetkazish buyurtmasimi: restoran
// qabul qilgan, yakunlanmagan, kuryer hali biriktirilmagan.
//
// Holatlar ro'yxati `storage.PgOrderRepo.ListAwaitingCourier` dagi SQL
// va migration 0048 dagi qisman indeks bilan BIR XIL bo'lishi shart.
func (o *Order) AwaitsCourier() bool {
	if o.IsDineIn() || o.CourierID != "" {
		return false
	}
	switch o.Status {
	case StatusAccepted, StatusPreparing, StatusReady:
		return true
	}
	return false
}

// CourierSearchExpired — tayyor buyurtmaga qidiruv muddati ichida kuryer
// topilmadimi.
func (o *Order) CourierSearchExpired(now time.Time) bool {
	return o.Status == StatusReady && o.AwaitsCourier() &&
		o.DispatchState != DispatchNotFound &&
		o.DispatchDeadline != nil && !now.Before(*o.DispatchDeadline)
}

// CourierNotFoundExpired — "kuryer topilmadi" holatida restoran javob
// bermadi va avtomatik bekor qilish vaqti keldimi.
func (o *Order) CourierNotFoundExpired(now time.Time) bool {
	return o.Status == StatusReady && o.AwaitsCourier() &&
		o.DispatchState == DispatchNotFound &&
		o.DispatchDeadline != nil && !now.Before(*o.DispatchDeadline)
}

// lastChangeTo — buyurtma shu holatga OXIRGI marta qachon o'tgani.
func (o *Order) lastChangeTo(s Status) (time.Time, bool) {
	for i := len(o.History) - 1; i >= 0; i-- {
		if o.History[i].To == s {
			return o.History[i].At, true
		}
	}
	return time.Time{}, false
}

// applyDispatchPolicy — holat o'zgarganda qidiruv holatini moslaydi.
//
// `ChangeStatus` ichida, `Save` dan OLDIN chaqiriladi: holat va qidiruv
// muddati BITTA atomik yozuvda saqlanadi — oraliqda "tayyor, lekin
// muddatsiz" buyurtma bo'lib qolmaydi.
func applyDispatchPolicy(o *Order, to Status, now time.Time) {
	if o.IsDineIn() {
		return
	}
	switch {
	case IsTerminal(to):
		o.DispatchDeadline = nil
	case o.CourierID != "":
		// Kuryer allaqachon biriktirilgan — qidiruv yo'q.
	case to == StatusAccepted:
		o.DispatchState = DispatchSearching
		o.DispatchDeadline = nil
	case to == StatusReady:
		o.DispatchState = DispatchSearching
		deadline := now.Add(CourierSearchWindow)
		o.DispatchDeadline = &deadline
	}
}

// Now — xizmat soati. Kuryer qidiruvi nazoratchisi (`httpapi`) muddatlarni
// AYNAN shu soat bilan solishtiradi — testda ikkalasi bitta soatga
// bog'lanadi.
func (s *Service) Now() time.Time { return s.now() }

// WithClock — soatni almashtiradi (testlar uchun).
func (s *Service) WithClock(now func() time.Time) *Service {
	s.now = now
	return s
}

// MarkCourierNotFound — qidiruv muddati tugagan tayyor buyurtmani
// "kuryer topilmadi" holatiga o'tkazadi va avtomatik bekor qilish
// muddatini qo'yadi.
//
// changed=false xato EMAS: muddat hali tugamagan yoki buyurtma shu
// oraliqda boshqa holatga o'tgan (kuryer topildi, bekor qilindi).
func (s *Service) MarkCourierNotFound(ctx context.Context, orderID string) (*Order, bool, error) {
	return s.updateDispatch(ctx, orderID, func(o *Order, now time.Time) (bool, error) {
		if !o.CourierSearchExpired(now) {
			return false, nil
		}
		o.DispatchState = DispatchNotFound
		deadline := now.Add(CourierNotFoundCancelAfter)
		o.DispatchDeadline = &deadline
		return true, nil
	})
}

// RestartCourierSearch — restoran "Kuryer qidirish" bosdi: qidiruv yangi
// muddat bilan qaytadan boshlanadi. Qidiruvning o'zini chaqiruvchi
// (HTTP qatlami) ishga tushiradi.
func (s *Service) RestartCourierSearch(ctx context.Context, orderID string) (*Order, error) {
	o, _, err := s.updateDispatch(ctx, orderID, func(o *Order, now time.Time) (bool, error) {
		if o.Status != StatusReady || !o.AwaitsCourier() || o.DispatchState != DispatchNotFound {
			return false, ErrDispatchNotRestartable
		}
		o.DispatchState = DispatchSearching
		deadline := now.Add(CourierSearchWindow)
		o.DispatchDeadline = &deadline
		return true, nil
	})
	return o, err
}

// EnsureCourierSearchDeadline — muddatsiz qolgan tayyor buyurtmaga
// (migration 0048 dan OLDIN tayyor bo'lganlar) muddat beradi. Hisob
// buyurtma haqiqatan tayyor bo'lgan paytdan boshlanadi — ya'ni uzoq
// osilib qolgan buyurtma darhol "kuryer topilmadi" bo'ladi.
func (s *Service) EnsureCourierSearchDeadline(ctx context.Context, orderID string) (*Order, bool, error) {
	return s.updateDispatch(ctx, orderID, func(o *Order, now time.Time) (bool, error) {
		if o.Status != StatusReady || !o.AwaitsCourier() ||
			o.DispatchState == DispatchNotFound || o.DispatchDeadline != nil {
			return false, nil
		}
		start := now
		if at, ok := o.lastChangeTo(StatusReady); ok && at.Before(now) {
			start = at
		}
		deadline := start.Add(CourierSearchWindow)
		o.DispatchState = DispatchSearching
		o.DispatchDeadline = &deadline
		return true, nil
	})
}

// AutoCancelCourierNotFound — "kuryer topilmadi" muddati tugagan
// buyurtmani TIZIM nomidan bekor qiladi.
//
// Bekor qilish oddiy `ChangeStatus` orqali o'tadi: to'lov qaytariladi,
// mijoz, restoran, admin va kuryer ilovalari jonli xabar oladi. Oraliqda
// restoran "Kuryer qidirish" bosgan yoki kuryer topilgan bo'lsa,
// `ValidateOrderTransition` bekor qilishni rad etadi — bu changed=false.
func (s *Service) AutoCancelCourierNotFound(ctx context.Context, orderID string) (*Order, bool, error) {
	o, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return nil, false, err
	}
	if !o.CourierNotFoundExpired(s.now()) {
		return o, false, nil
	}
	cancelled, err := s.ChangeStatus(ctx, orderID, StatusCancelled, ActorSystem)
	if err != nil {
		var terr *TransitionError
		if errors.As(err, &terr) {
			return o, false, nil
		}
		return nil, false, err
	}
	return cancelled, true, nil
}

// updateDispatch — qidiruv maydonlarini optimistik qulf bilan yangilaydi
// (`ChangeStatus` bilan bir xil naqsh). `mutate` false qaytarsa hech
// narsa yozilmaydi.
func (s *Service) updateDispatch(ctx context.Context, orderID string,
	mutate func(o *Order, now time.Time) (bool, error)) (*Order, bool, error) {

	for attempt := 0; attempt < maxOptimisticRetries; attempt++ {
		o, err := s.repo.GetByID(ctx, orderID)
		if err != nil {
			return nil, false, err
		}
		now := s.now()
		changed, err := mutate(o, now)
		if err != nil {
			return nil, false, err
		}
		if !changed {
			return o, false, nil
		}
		o.UpdatedAt = now
		err = s.repo.Save(ctx, o)
		if errors.Is(err, ErrConflict) {
			continue
		}
		if err != nil {
			return nil, false, err
		}
		return o, true, nil
	}
	return nil, false, ErrConflict
}
