package notify

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"chustapp/internal/couriers"
	"chustapp/internal/orders"
)

// Live — `orders.Notifier` va `couriers.OfferNotifier` interfeyslarini
// UMUMIY bildirishnoma xizmati ustida bajaradi.
//
// ┌─ BU QATLAM NEGA BOR ──────────────────────────────────────────────┐
// `Service` modulga bog'liq EMAS (`Event` bilan ishlaydi). `orders` va
// `couriers` esa o'z interfeyslarini talab qiladi. Shu ikkisi
// o'rtasidagi tarjimon — mana shu fayl.
//
// Yangi modul qo'shilganda `Service` ga TEGILMAYDI: o'sha modul
// shunchaki `Notify(ctx, userID, Event{Module: "shop", ...})` ni
// chaqiradi.
// └───────────────────────────────────────────────────────────────────┘
type Live struct {
	svc *Service
	// waiterLookup — restoran ID'sidan o'sha restoranda ishlaydigan
	// affitsiantlarning foydalanuvchi ID'larini beradi.
	//
	// ┌─ NEGA FUNKSIYA, `users.Repository` EMAS ──────────────────────┐
	// `notify` paketi `users` ga bog'lanmasligi kerak: u allaqachon
	// `orders` va `couriers` ga bog'liq va uchinchi bog'liqlik
	// aylanma import xavfini oshiradi (`users` kelajakda
	// bildirishnoma yuborishi tabiiy).
	//
	// Funksiya esa bog'liqlikni TESKARI qiladi — `main` uni ulaydi.
	// Bu `Verifier.ContactHook` bilan bir xil naqsh.
	// └───────────────────────────────────────────────────────────────┘
	waiterLookup func(ctx context.Context, restaurantID string) ([]string, error)
	// courierUser — kuryer ID'sidan uning akkaunti (push tokenlari
	// akkauntga bog'langan). `waiterLookup` bilan bir xil sabab.
	courierUser func(ctx context.Context, courierID string) (string, error)
}

func NewLive(svc *Service) *Live { return &Live{svc: svc} }

// WithCourierUserLookup — kuryer akkauntini topish funksiyasini ulaydi.
// Ulanmasa taklif push'i yuborilmaydi va ilovasi yopiq kuryerlar
// "yetib bormaydi" deb hisoblanadi (`CanPush`).
func (l *Live) WithCourierUserLookup(fn func(ctx context.Context, courierID string) (string, error)) *Live {
	l.courierUser = fn
	return l
}

// ---------- couriers.Reachability ----------

// Online — kuryer ilovasi ochiq (kuryerning ish kanaliga ulangan).
func (l *Live) Online(courierID string) bool {
	return l.svc.Online(Entity(ModuleFood, courierID))
}

// CanPush — ilova yopiq bo'lsa ham push bilan uyg'otsa bo'ladi.
func (l *Live) CanPush(ctx context.Context, courierID string) bool {
	if l.courierUser == nil || !l.svc.PushEnabled() {
		return false
	}
	userID, err := l.courierUser(ctx, courierID)
	if err != nil || userID == "" {
		return false
	}
	return l.svc.HasPushTokens(ctx, userID)
}

// WithWaiterLookup — affitsiantlarni topish funksiyasini ulaydi.
// Ulanmasa, stol buyurtmalari uchun push shunchaki yuborilmaydi
// (jonli WS kanali baribir ishlaydi).
func (l *Live) WithWaiterLookup(fn func(ctx context.Context, restaurantID string) ([]string, error)) *Live {
	l.waiterLookup = fn
	return l
}

// ---------- orders.Notifier ----------

func (l *Live) OrderCreated(o *orders.Order) {
	LogNotifier{}.OrderCreated(o)
	// Restoran xodimlariga — ENTITY kanali (kim ishlab tursa, o'sha
	// eshitadi), shaxsiy emas.
	event := map[string]any{
		"type":        "new_order",
		"order_id":    o.ID,
		"total_tiyin": o.TotalTiyin,
	}
	// Stol buyurtmasi bo'lsa — panel qaysi stol ekanini DARHOL
	// ko'rsatishi uchun. Busiz panel har bir yangi buyurtma uchun
	// alohida so'rov yuborishga majbur bo'lardi.
	if o.IsDineIn() {
		event["order_type"] = string(orders.TypeDineIn)
		event["table_label"] = o.TableLabel
		event["party_size"] = o.PartySize
	}
	l.svc.Broadcast(Entity(ModuleFood, o.RestaurantID), event)
	// Superadmin paneli — o'sha eventning nusxasi. Busiz panel yangi
	// buyurtmani faqat keyingi so'rov siklida ko'rardi (`Admin()`
	// izohiga qarang).
	l.svc.Broadcast(Admin(), event)
}

func (l *Live) OrderStatusChanged(o *orders.Order, from orders.Status) {
	LogNotifier{}.OrderStatusChanged(o, from)

	event := map[string]any{
		"type":       "order_status",
		"order_id":   o.ID,
		"from":       from,
		"status":     o.Status,
		"courier_id": o.CourierID,
	}
	// Stol buyurtmasida affitsiant ilovasi ham shu kanalni eshitadi
	// (u ham `Entity(food, restaurantID)` ga obuna) — shuning uchun
	// stol ma'lumoti eventga qo'shiladi.
	if o.IsDineIn() {
		event["order_type"] = string(orders.TypeDineIn)
		event["table_label"] = o.TableLabel
		event["party_size"] = o.PartySize
	}
	// Restoran va kuryer — ish kanallari (jonli, tarixsiz).
	l.svc.Broadcast(Entity(ModuleFood, o.RestaurantID), event)
	l.svc.Broadcast(Entity(ModuleFood, o.CourierID), event)
	// Superadmin paneli (buyurtmalar ro'yxati va boshqaruv
	// ko'rsatkichlari shu eventdan yangilanadi).
	l.svc.Broadcast(Admin(), event)

	// ── Affitsiantga PUSH: taom tayyor ──
	//
	// Jonli kanal (yuqorida) FAQAT ilova ochiq bo'lsa ishlaydi.
	// Affitsiant esa zал bo'ylab yuradi va telefoni cho'ntagida,
	// ekrani o'chiq bo'ladi — aynan shu holatda xabar YETIB
	// BORISHI kerak, aks holda taom oshxonada sovib qoladi.
	if o.IsDineIn() && o.Status == orders.StatusReady {
		l.notifyWaiters(o)
	}

	// MIJOZGA — shaxsiy bildirishnoma: saqlanadi va ilova yopiq
	// bo'lsa push ketadi. Buyurtma holati — foydalanuvchi
	// O'TKAZIB YUBORMASLIGI kerak bo'lgan yagona narsa.
	ctx := context.Background()
	l.svc.Notify(ctx, o.CustomerID, Event{
		Module: ModuleFood,
		Kind:   "order_status",
		Title:  orderStatusTitle(o),
		Body:   orderStatusText(o),
		Data: map[string]string{
			"order_id": o.ID,
			"status":   string(o.Status),
		},
	})
}

// notifyWaiters — tayyor bo'lgan stol buyurtmasi haqida restoranning
// BARCHA affitsiantlariga xabar beradi.
//
// ┌─ NEGA HAMMASIGA, BITTASIGA EMAS ──────────────────────────────────┐
// Buyurtma hech qaysi affitsiantga BIRIKTIRILMAGAN: stolni kim bo'sh
// bo'lsa o'sha xizmat qiladi. Kimga yuborishni tanlamoqchi bo'lsak,
// "stol ↔ affitsiant" jadvali va navbatchilik grafigi kerak bo'lardi
// — bu restoran uchun ortiqcha ma'muriyatchilik.
//
// Hammasiga yuborish esa oddiy va ISHONCHLI: taom sovib qolgandan
// ko'ra ikki kishi bir vaqtda kelgani yaxshiroq. Kim birinchi
// "berildi" bossa, buyurtma ro'yxatdan chiqadi.
// └───────────────────────────────────────────────────────────────────┘
func (l *Live) notifyWaiters(o *orders.Order) {
	if l.waiterLookup == nil {
		return
	}
	ctx := context.Background()
	ids, err := l.waiterLookup(ctx, o.RestaurantID)
	if err != nil {
		slog.Error("affitsiantlarni topishda xato — push yuborilmadi",
			"restaurant", o.RestaurantID, "order", o.ID, "err", err)
		return
	}
	table := o.TableLabel
	if table == "" {
		table = "—"
	}
	for _, id := range ids {
		l.svc.Notify(ctx, id, Event{
			Module: ModuleFood,
			Kind:   "table_order_ready",
			Title:  fmt.Sprintf("%s-stol: buyurtma tayyor", table),
			Body:   "Taomni stolga olib boring",
			Data: map[string]string{
				"order_id":    o.ID,
				"table_label": o.TableLabel,
				"status":      string(o.Status),
			},
		})
	}
}

// orderStatusText — foydalanuvchi ko'radigan matn (push'da ham shu).
// orderStatusTitle — mijoz bildirishnomasining sarlavhasi.
//
// Affitsiant "Yetkazdim" bosganda mijoz telefonida umumiy "Buyurtma
// holati" emas, aniq "Buyurtmangiz keldi" chiqadi — bu eng kutilgan
// xabar.
func orderStatusTitle(o *orders.Order) string {
	if o.IsDineIn() && o.Status == orders.StatusServed {
		return "Buyurtmangiz keldi"
	}
	return "Buyurtma holati"
}

func orderStatusText(o *orders.Order) string {
	// Stolda ovqatlanishda "yetkazish" atamalari ma'nosiz — mijoz
	// restoranning o'zida o'tiribdi.
	if o.IsDineIn() {
		switch o.Status {
		case orders.StatusAccepted:
			return "Restoran buyurtmangizni qabul qildi"
		case orders.StatusPreparing:
			return "Buyurtmangiz tayyorlanmoqda"
		case orders.StatusReady:
			return "Buyurtmangiz tayyor — hozir olib kelishadi"
		case orders.StatusServed:
			return "Buyurtmangiz stolingizga olib kelindi. Yoqimli ishtaha!"
		}
	}
	switch o.Status {
	case orders.StatusAccepted:
		return "Restoran buyurtmangizni qabul qildi"
	case orders.StatusPreparing:
		return "Buyurtmangiz tayyorlanmoqda"
	case orders.StatusReady:
		return "Buyurtmangiz tayyor"
	case orders.StatusPickedUp:
		return "Kuryer buyurtmangizni oldi"
	case orders.StatusDelivered:
		return "Buyurtmangiz yetkazildi"
	case orders.StatusCancelled:
		return "Buyurtma bekor qilindi"
	case orders.StatusRejected:
		return "Restoran buyurtmani rad etdi"
	default:
		return fmt.Sprintf("Buyurtma holati: %s", o.Status)
	}
}

// ---------- couriers.OfferNotifier ----------

// SendOffer — kuryerga taklif.
//
// SHAXSIY bildirishnoma sifatida ketadi: taklif 20 soniya amal
// qiladi va uni O'TKAZIB YUBORISH kuryer uchun yo'qotilgan pul
// demak. Ilova yopiq bo'lsa push bilan uyg'otiladi.
func (l *Live) SendOffer(courierID string, info couriers.OfferInfo) {
	LogNotifier{}.SendOffer(courierID, info.OrderID, info.ExpiresIn)

	// Jonli kanal — taklif kartochkasi uchun to'liq kontekst (shakl
	// `GET /couriers/{id}/offer` bilan bir xil).
	topic := Entity(ModuleFood, courierID)
	l.svc.Broadcast(topic, info.Payload(info.ExpiresIn))

	// ┌─ ILOVA YOPIQ — PUSH (2026-09-15) ─────────────────────────────┐
	// Avval faqat WebSocket bor edi: kuryer ilovani yopsa (liniyada
	// bo'lsa ham) taklif hech qayerga yetmasdi. Endi ilova ulanmagan
	// bo'lsa push yuboriladi; kuryer bosganda ilova taklifni serverdan
	// tiklaydi. Ilova ochiq bo'lsa push YO'Q — ekranda allaqachon bor.
	// └───────────────────────────────────────────────────────────────┘
	if l.courierUser == nil || l.svc.Online(topic) || !l.svc.PushEnabled() {
		return
	}
	safeGo("notify.courier_offer", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		userID, err := l.courierUser(ctx, courierID)
		if err != nil || userID == "" {
			if err != nil {
				slog.Warn("notify: kuryer akkaunti topilmadi — taklif push'i yuborilmadi",
					"courier", courierID, "err", err)
			}
			return
		}
		l.svc.Push(ctx, userID, courierOfferEvent(info))
	})
}

// courierOfferEvent — taklif push'i. Matnda faqat restoran nomi (qulflangan
// ekranda ham ko'rinadi — mijoz haqida hech narsa yo'q).
func courierOfferEvent(info couriers.OfferInfo) Event {
	body := "Qabul qilish uchun ilovani oching"
	if name := strings.TrimSpace(info.RestaurantName); name != "" {
		body = name + " — qabul qilish uchun ilovani oching"
	}
	return Event{
		Module: ModuleFood,
		Kind:   courierOfferKind,
		Title:  "Yangi buyurtma",
		Body:   body,
		Data: map[string]string{
			"order_id": info.OrderID,
			// Ilova signalni shu paytgacha chaladi (muddat telefonga
			// kechikib yetsa ham ortiqcha chalinmaydi).
			"expires_at": strconv.FormatInt(time.Now().Add(info.ExpiresIn).UnixMilli(), 10),
		},
		TTL: info.ExpiresIn,
	}
}

func (l *Live) CancelOffer(courierID, orderID string) {
	LogNotifier{}.CancelOffer(courierID, orderID)
	topic := Entity(ModuleFood, courierID)
	l.svc.Broadcast(topic, map[string]any{
		"type":     "offer_cancelled",
		"order_id": orderID,
	})
	// Ilova yopiq bo'lsa takrorlanayotgan signal o'chirilsin: taklifni
	// boshqa kuryer olgan yoki muddati tugagan.
	if l.courierUser == nil || l.svc.Online(topic) || !l.svc.PushEnabled() {
		return
	}
	safeGo("notify.courier_offer_cancelled", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		userID, err := l.courierUser(ctx, courierID)
		if err != nil || userID == "" {
			return
		}
		l.svc.Push(ctx, userID, Event{
			Module: ModuleFood,
			Kind:   courierOfferCancelledKind,
			Data:   map[string]string{"order_id": orderID},
			TTL:    30 * time.Second,
		})
	})
}

// CourierAutoOffline — kuryer ketma-ket javobsiz takliflardan keyin liniyadan
// chiqarildi (`couriers/missed.go`). Ilova ochiq bo'lsa jonli kanal holatni
// darhol yangilaydi; push esa yopiq/fondagi ilovada kuryerga sababini
// aytadi — aks holda u "nega buyurtma kelmayapti" deb kutib o'tirardi.
// Oddiy (data-only emas) bildirishnoma: bir marta ko'rinadi, jiringlamaydi.
func (l *Live) CourierAutoOffline(courierID string, missed int) {
	l.svc.Broadcast(Entity(ModuleFood, courierID), map[string]any{
		"type":   "auto_offline",
		"missed": missed,
	})
	if l.courierUser == nil || !l.svc.PushEnabled() {
		return
	}
	safeGo("notify.courier_auto_offline", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		userID, err := l.courierUser(ctx, courierID)
		if err != nil || userID == "" {
			return
		}
		l.svc.Push(ctx, userID, Event{
			Module: ModuleFood,
			Kind:   courierAutoOfflineKind,
			Title:  "Liniyadan chiqarildingiz",
			Body: fmt.Sprintf("Ketma-ket %d ta taklifga javob berilmadi. "+
				"Ishlashga tayyor bo'lsangiz, ilovada qayta liniyaga chiqing.", missed),
			TTL: time.Hour,
		})
	})
}
