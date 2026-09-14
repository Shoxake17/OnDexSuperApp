package orders

import "fmt"

// Buyurtma holatlari o'rtasidagi ruxsat etilgan o'tishlar.
//
// ┌─ NEGA TUR BO'YICHA IKKI JADVAL ───────────────────────────────────┐
// Yetkazish va stolda ovqatlanish buyurtmalari hayot siklining KATTA
// qismini bo'lishadi (created → accepted → preparing → ready) va faqat
// oxirgi qadamda ajraladi:
//
//	delivery: ready → picked_up → delivered   (kuryer)
//	dine_in:  ready → served                  (affitsiant)
//
// Bitta umumiy jadvalga ikkala tugashni ham qo'shish MUMKIN EDI, lekin
// o'shanda affitsiant YETKAZISH buyurtmasini "served" deb belgilay
// olardi (yoki kuryer stol buyurtmasini "picked_up"). Ikkalasi ham
// buyurtmani noto'g'ri terminal holatga tiqib qo'yardi va uni
// qaytarishning iloji yo'q edi — shuning uchun tur jadval TANLAYDI,
// keyin aktor tekshiriladi.
// └───────────────────────────────────────────────────────────────────┘

// Umumiy boshlanish — ikkala turda ham bir xil.
func commonTransitions() map[Status]map[Status][]Actor {
	return map[Status]map[Status][]Actor{
		StatusCreated: {
			StatusAccepted:  {ActorRestaurant},
			StatusRejected:  {ActorRestaurant},
			StatusCancelled: {ActorCustomer, ActorAdmin},
		},
		StatusAccepted: {
			StatusPreparing: {ActorRestaurant},
			StatusCancelled: {ActorCustomer, ActorAdmin},
		},
		StatusPreparing: {
			StatusReady:     {ActorRestaurant},
			StatusCancelled: {ActorAdmin},
		},
	}
}

// deliveryTransitions — kuryer yetkazadigan buyurtma.
var deliveryTransitions = func() map[Status]map[Status][]Actor {
	t := commonTransitions()
	t[StatusReady] = map[Status][]Actor{
		// Yandex Eats uslubi (foydalanuvchi so'rovi bo'yicha, 2026-07-30):
		// alohida tasdiqlash kodi YO'Q, restoran ham hech qanday tugma
		// bosmaydi — kuryer restoranda buyurtma raqamining OXIRGI 4
		// xonasini xodimga og'zaki aytadi, taomni qo'lga olgach O'ZI
		// "Buyurtma olindi" tugmasini bosadi. Dastur darajasida bu
		// tekshirilmaydi (kamroq ishqalanish — restoran xodimi doim
		// tugma bosib turishga majbur bo'lmaydi); bu ataylab tanlangan
		// murosaga kelish, xavfsizlikdan qulaylik foydasiga.
		StatusPickedUp: {ActorCourier},
		// Restoran va tizim FAQAT kuryer topilmaganda bekor qila oladi.
		// Bu shart buyurtma maydonlariga bog'liq va jadvalda
		// ifodalanmaydi — `ValidateOrderTransition` da tekshiriladi.
		StatusCancelled: {ActorAdmin, ActorRestaurant, ActorSystem},
	}
	t[StatusPickedUp] = map[Status][]Actor{
		StatusDelivered: {ActorCourier},
		StatusCancelled: {ActorAdmin},
	}
	return t
}()

// dineInTransitions — stol QR kodi orqali berilgan buyurtma.
var dineInTransitions = func() map[Status]map[Status][]Actor {
	t := commonTransitions()
	t[StatusReady] = map[Status][]Actor{
		// Affitsiant taomni stolga olib borgach bosadi. Restoran ham
		// bosa oladi: kichik oshxonalarda affitsiant va oshpaz bir
		// odam bo'lishi mumkin va o'shanda buyurtma "ready" holatida
		// abadiy osilib qolardi.
		StatusServed:    {ActorWaiter, ActorRestaurant},
		StatusCancelled: {ActorAdmin},
	}
	return t
}()

// delivered/served/rejected/cancelled — terminal, hech qayerga o'tmaydi.

// IsTerminal — buyurtma yakunlanganmi (boshqa holatga o'tmaydi).
//
// NEGA EKSPORT QILINGAN: "faol buyurtmasi bormi" savoli endi
// buyurtmalar modulidan TASHQARIDA ham beriladi — masalan superadmin
// akkauntni o'chirishdan oldin (`routes_admin_users.go`). Har bir
// chaqiruvchi terminal holatlar ro'yxatini o'zi yozsa, yangi holat
// qo'shilganda ro'yxatlarning biri yangilanmasdan qolardi va xato
// JIM bo'lardi (yakunlangan buyurtma "faol" deb ko'rinardi).
func IsTerminal(s Status) bool {
	switch s {
	case StatusDelivered, StatusServed, StatusRejected, StatusCancelled:
		return true
	default:
		return false
	}
}

type TransitionError struct {
	From, To Status
	By       Actor
	Reason   string
}

func (e *TransitionError) Error() string {
	return fmt.Sprintf("transition %s -> %s by %s not allowed: %s", e.From, e.To, e.By, e.Reason)
}

// ValidateTransition — o'tish mumkinligini va aktor huquqini tekshiradi.
//
// `orderType` bo'sh bo'lsa `delivery` deb qaraladi (Type.Normalized()).
func ValidateTransition(orderType Type, from, to Status, by Actor) error {
	table := deliveryTransitions
	if orderType.Normalized() == TypeDineIn {
		table = dineInTransitions
	}
	targets, ok := table[from]
	if !ok {
		return &TransitionError{from, to, by, "holat terminal"}
	}
	actors, ok := targets[to]
	if !ok {
		return &TransitionError{from, to, by, "bunday o'tish mavjud emas"}
	}
	for _, a := range actors {
		if a == by {
			return nil
		}
	}
	return &TransitionError{from, to, by, "bu aktor uchun ruxsat yo'q"}
}

// ValidateOrderTransition — `ValidateTransition` + buyurtma maydonlariga
// bog'liq qoidalar. `Service.ChangeStatus` FAQAT shuni chaqiradi.
func ValidateOrderTransition(o *Order, to Status, by Actor) error {
	if err := ValidateTransition(o.Type, o.Status, to, by); err != nil {
		return err
	}
	return checkCourierNotFoundCancel(o, to, by)
}

// checkCourierNotFoundCancel — tayyor YETKAZISH buyurtmasini restoran
// yoki tizim faqat "kuryer topilmadi" holatida bekor qila oladi.
//
// ┌─ NEGA ────────────────────────────────────────────────────────────┐
// Usiz restoran kuryer kelayotgan (yoki hali qidirilayotgan) buyurtmani
// ham bekor qila olardi: kuryer bo'sh qo'l bilan qaytardi, mijoz esa
// sababsiz bekor qilingan buyurtma olardi. Admin cheklanmaydi — u
// nizolarni hal qiladi.
// └───────────────────────────────────────────────────────────────────┘
func checkCourierNotFoundCancel(o *Order, to Status, by Actor) error {
	if to != StatusCancelled || o.Status != StatusReady || o.IsDineIn() {
		return nil
	}
	if by != ActorRestaurant && by != ActorSystem {
		return nil
	}
	if o.CourierID == "" && o.DispatchState == DispatchNotFound {
		return nil
	}
	return &TransitionError{o.Status, to, by,
		"tayyor buyurtmani faqat kuryer topilmaganda bekor qilish mumkin"}
}
