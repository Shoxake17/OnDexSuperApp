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

// Boshqa bildirishnomalar (mijoz, kuryer) o'zgarishsiz qoladi.
func TestFCMMessage_OtherKindsUnchanged(t *testing.T) {
	msg := fcmMessage("tok", Event{Module: ModuleFood, Kind: "order_status", Title: "t", Body: "b"})
	android := msg["android"].(map[string]any)
	if _, ok := android["notification"]; ok {
		t.Fatal("mijoz xabariga affitsiant kanali qo'shilib qoldi")
	}
	if _, ok := msg["apns"]; ok {
		t.Fatal("mijoz xabariga apns qo'shilib qoldi")
	}
	if android["priority"] != "high" {
		t.Fatal("priority yo'qoldi")
	}
}
