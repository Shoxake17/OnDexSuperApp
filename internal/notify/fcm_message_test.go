package notify

import (
	"encoding/json"
	"testing"
	"time"
)

// Affitsiantning "taom tayyor" push'i ovoz va vibratsiya kanaliga
// ketishi SHART — aks holda Android uni jim standart kanalda ko'rsatadi.
func TestFCMMessage_WaiterReadyHasSoundChannel(t *testing.T) {
	msg := fcmMessage("tok", Event{
		Module: ModuleFood, Kind: "table_order_ready",
		Title: "Stol: buyurtma tayyor", Body: "Taomni stolga olib boring",
		Data: map[string]string{"order_id": "o1"},
	})
	android := msg["android"].(map[string]any)
	n, ok := android["notification"].(map[string]any)
	if !ok {
		t.Fatal("android.notification yo'q — kanal ko'rsatilmagan")
	}
	if n["channel_id"] != "ondex_waiter_ready_v2" || n["sound"] != "waiter_ready" {
		t.Fatalf("kanal/ovoz noto'g'ri: %v", n)
	}
	if n["default_vibrate_timings"] != false {
		t.Fatal("standart vibratsiya o'chirilmagan")
	}
	var total time.Duration
	for _, s := range n["vibrate_timings"].([]string) {
		d, err := time.ParseDuration(s)
		if err != nil {
			t.Fatalf("FCM davomiylik formati noto'g'ri: %q", s)
		}
		total += d
	}
	if total < 4*time.Second || total > 6*time.Second {
		t.Fatalf("vibratsiya ~5 soniya bo'lishi kerak, keldi %v", total)
	}
	data := msg["data"].(map[string]string)
	if data["order_id"] != "o1" || data["kind"] != "table_order_ready" {
		t.Fatalf("data noto'g'ri: %v", data)
	}
	if _, err := json.Marshal(map[string]any{"message": msg}); err != nil {
		t.Fatal(err)
	}
}

// Mijoz/kuryer xabarlari ham O'Z kanaliga tushishi SHART.
//
// Avval bu yerda kanal UMUMAN ko'rsatilmasdi va Android xabarni
// o'zining `fcm_fallback_notification_channel` ida (IMPORTANCE_DEFAULT,
// ovozsiz) ko'rsatardi — telefonda tekshirilgan, 2026-10-20.
func TestFCMMessage_OrderKindsUseOrderChannel(t *testing.T) {
	msg := fcmMessage("tok", Event{Module: ModuleFood, Kind: "order_status", Title: "t", Body: "b"})
	android := msg["android"].(map[string]any)
	n, ok := android["notification"].(map[string]any)
	if !ok {
		t.Fatal("android.notification yo'q — kanal ko'rsatilmagan")
	}
	if n["channel_id"] != orderChannel {
		t.Fatalf("kanal noto'g'ri: %v", n["channel_id"])
	}
	if n["sound"] != "default" || n["notification_priority"] != "PRIORITY_MAX" {
		t.Fatalf("ovoz/ustuvorlik noto'g'ri: %v", n)
	}
	for _, s := range n["vibrate_timings"].([]string) {
		if _, err := time.ParseDuration(s); err != nil {
			t.Fatalf("FCM davomiylik formati noto'g'ri: %q", s)
		}
	}
	if android["priority"] != "high" {
		t.Fatal("priority yo'qoldi")
	}
	if _, ok := msg["apns"]; !ok {
		t.Fatal("apns bloki yo'q — iOS'da ovoz chalinmaydi")
	}
	if _, err := json.Marshal(map[string]any{"message": msg}); err != nil {
		t.Fatal(err)
	}
}

// Data-only xabarlarga (kuryer taklifi) kanal QO'SHILMAYDI: ularni
// ilovaning o'zi ko'rsatadi, Android esa umuman ko'rsatmasligi kerak.
func TestFCMMessage_DataOnlyHasNoNotificationBlock(t *testing.T) {
	msg := fcmMessage("tok", Event{Module: ModuleFood, Kind: courierOfferKind, Title: "t", Body: "b"})
	if _, ok := msg["notification"]; ok {
		t.Fatal("kuryer taklifiga notification bloki qo'shilib qoldi")
	}
	android := msg["android"].(map[string]any)
	if _, ok := android["notification"]; ok {
		t.Fatal("kuryer taklifiga android.notification qo'shilib qoldi")
	}
}
