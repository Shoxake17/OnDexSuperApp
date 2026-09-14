package notify

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

// fcmMessage — bitta qurilma uchun FCM v1 `message` obyekti.
func fcmMessage(token string, e Event) map[string]any {
	// FCM `data` FAQAT satr qabul qiladi.
	data := make(map[string]string, len(e.Data)+2)
	for k, v := range e.Data {
		data[k] = v
	}
	data["module"] = e.Module
	data["kind"] = e.Kind

	android := map[string]any{
		// Buyurtma/taklif xabarlari kechiktirilmasin.
		"priority": "high",
	}
	msg := map[string]any{
		"token": token,
		"notification": map[string]any{
			"title": e.Title,
			"body":  e.Body,
		},
		"data":    data,
		"android": android,
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
		msg["apns"] = map[string]any{
			"headers": map[string]string{"apns-priority": "10"},
			"payload": map[string]any{
				"aps": map[string]any{"sound": "default"},
			},
		}
	}
	return msg
}
