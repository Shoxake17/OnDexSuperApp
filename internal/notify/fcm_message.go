package notify

import (
	"math"
	"strconv"
)

// ┌─ KURYER TAKLIFI — FAQAT MA'LUMOT (DATA-ONLY) PUSH ────────────────┐
// Birinchi versiyada taklif oddiy `notification` push edi: ilova yopiq
// bo'lsa Android uni O'ZI ko'rsatardi va tovush BIR MARTA qisqa chalinardi
// — kuryer sezmasdi (foydalanuvchi sinovi, 2026-09-15).
//
// Endi xabar faqat `data` bilan ketadi: telefon ilovani fonda uyg'otadi va
// kuryer ilovasi (`apps/courier_app/lib/offer_alert.dart`) TAKRORLANUVCHI
// (insistent) bildirishnoma chiqaradi — ilovaning o'z mp3 ovozi kuryer
// bosguncha yoki taklif muddati tugaguncha qayta-qayta chalinadi.
// Taklif boshqa kuryerga ketsa `courier_offer_cancelled` o'sha signalni
// o'chiradi. Muddat (`ttl`) taklif muddatiga teng: telefon shu paytda
// oflayn bo'lsa, eskirgan taklif keyin kelib chalg'itmaydi.
// └───────────────────────────────────────────────────────────────────┘
const (
	courierOfferKind          = "courier_offer"
	courierOfferCancelledKind = "courier_offer_cancelled"
	courierAutoOfflineKind    = "courier_auto_offline"
)

// dataOnlyKinds — `notification` blokisiz yuboriladigan turlar (ko'rsatishni
// ilovaning o'zi boshqaradi).
func dataOnlyKind(kind string) bool {
	return kind == courierOfferKind || kind == courierOfferCancelledKind
}

// ┌─ AFFITSIANTNING "TAOM TAYYOR" BILDIRISHNOMASI ────────────────────┐
// Avval barcha push'lar bir xil yuborilardi: kanal ko'rsatilmasdi.
// Affitsiant ilovasi fonda yoki yopiq bo'lganda Android bildirishnomani
// o'zining STANDART kanalida ko'rsatardi — u ko'p telefonlarda ovozsiz
// va vibratsiyasiz. Zalda yurgan affitsiant taom tayyor bo'lganini
// sezmas, taom sovib qolardi.
//
// Endi shu turdagi xabar aniq kanalga yuboriladi. Kanalning ovozi
// (5 soniya) va vibratsiyasi ilovada yaratiladi —
// `apps/waiter_app/lib/push.dart` (`kReadyChannelId`). Nomlar IKKI
// joyda bir xil bo'lishi SHART: mavjud bo'lmagan kanalga kelgan xabarni
// Android yana standart (jim) kanalda ko'rsatadi.
// └───────────────────────────────────────────────────────────────────┘
const (
	waiterReadyKind    = "table_order_ready"
	waiterReadyChannel = "ondex_waiter_ready_v2"
	// waiterReadySound — `res/raw/waiter_ready.ogg` (kengaytmasiz).
	// Android 8+ da ovoz kanaldan olinadi; bu maydon eski Android uchun.
	waiterReadySound = "waiter_ready"
)

// waiterReadyVibration — ~5 soniya: 0.6 s titrash, 0.3 s pauza (eski
// Android uchun; yangilarida kanalning vibratsiyasi ishlaydi).
var waiterReadyVibration = []string{"0s", "0.6s", "0.3s", "0.6s", "0.3s", "0.6s", "0.3s", "0.6s", "0.3s", "0.6s"}

// ┌─ QOLGAN HAMMA XABAR UCHUN KANAL ──────────────────────────────────┐
// Avval `channel_id` FAQAT affitsiantning "taom tayyor" xabarida
// ko'rsatilardi. Qolganlari (buyurtma holati, kuryer yo'lga chiqdi,
// to'lov) kanalsiz ketardi va Android ularni O'ZI yaratadigan
// `fcm_fallback_notification_channel` da ko'rsatardi: IMPORTANCE
// DEFAULT, ya'ni ekranda banner chiqmaydi va ko'p telefonda (jumladan
// Samsung One UI) ovoz ham bermaydi. Qurilma dumpsys'i aynan shuni
// ko'rsatdi: `channel=fcm_fallback_notification_channel importance=3
// sound=null vibrate=null` (2026-10-20).
//
// Endi xabar aniq kanalga tushadi. Kanalning O'ZI ilovada yaratiladi
// (`apps/customer_app/android/.../MainActivity.kt`) va manifestda FCM
// uchun standart kanal sifatida ko'rsatiladi. ID uch joyda ham AYNAN
// bir xil bo'lishi SHART — bo'lmagan kanalga kelgan xabar yana jim
// fallback kanaliga tushadi.
// └───────────────────────────────────────────────────────────────────┘
const orderChannel = "ondex_orders_v1"

// orderVibration — qisqa, ikki zarbli naqsh (eski Android uchun;
// yangilarida kanalning o'z vibratsiyasi ishlaydi).
var orderVibration = []string{"0s", "0.4s", "0.2s", "0.4s"}

// fcmMessage — bitta qurilma uchun FCM v1 `message` obyekti.
func fcmMessage(token string, e Event) map[string]any {
	// FCM `data` FAQAT satr qabul qiladi.
	data := make(map[string]string, len(e.Data)+4)
	for k, v := range e.Data {
		data[k] = v
	}
	data["module"] = e.Module
	data["kind"] = e.Kind

	android := map[string]any{
		// Buyurtma/taklif xabarlari kechiktirilmasin. Data-only xabar
		// yuqori ustuvorlik bilangina yopiq ilovani uyg'otadi.
		"priority": "high",
	}
	if e.TTL > 0 {
		android["ttl"] = strconv.Itoa(int(math.Ceil(e.TTL.Seconds()))) + "s"
	}
	msg := map[string]any{
		"token":   token,
		"data":    data,
		"android": android,
	}

	if dataOnlyKind(e.Kind) {
		// Matnni ilova o'zi ko'rsatadi.
		data["title"] = e.Title
		data["body"] = e.Body
		return msg
	}

	msg["notification"] = map[string]any{
		"title": e.Title,
		"body":  e.Body,
	}
	if e.Kind == waiterReadyKind {
		android["notification"] = map[string]any{
			"channel_id":              waiterReadyChannel,
			"sound":                   waiterReadySound,
			"notification_priority":   "PRIORITY_MAX",
			"default_vibrate_timings": false,
			"vibrate_timings":         waiterReadyVibration,
			// Qulflangan ekranda ham ko'rinsin: matnda faqat stol nomi
			// va "taom tayyor" — mijoz haqida hech narsa yo'q.
			"visibility": "PUBLIC",
		}
	} else {
		android["notification"] = map[string]any{
			"channel_id":              orderChannel,
			"sound":                   "default",
			"notification_priority":   "PRIORITY_MAX",
			"default_vibrate_timings": false,
			"vibrate_timings":         orderVibration,
			// Qulflangan ekranda sarlavha ko'rinadi, matn yashiriladi:
			// bildirishnomada manzil yoki summa bo'lishi mumkin.
			"visibility": "PRIVATE",
		}
	}
	msg["apns"] = map[string]any{
		"headers": map[string]string{"apns-priority": "10"},
		"payload": map[string]any{
			"aps": map[string]any{"sound": "default"},
		},
	}
	return msg
}
