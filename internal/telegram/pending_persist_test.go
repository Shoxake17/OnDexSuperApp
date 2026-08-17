package telegram

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis YO'Q bo'lganda hech narsa buzilmasligi kerak — bu loyihaning
// umumiy qoidasi: Redis ixtiyoriy, u hech qachon ilovani to'xtatmaydi.
func TestPendingStoreWorksWithoutRedis(t *testing.T) {
	s := newPendingStore()

	p := &Pending{Token: "t1", Phone: "+998901234567", ExpiresAt: time.Now().Add(time.Minute)}
	s.put(p)

	if got, ok := s.getByToken("t1"); !ok || got.Phone != "+998901234567" {
		t.Fatalf("token bo'yicha topilmadi: %+v ok=%v", got, ok)
	}
	if _, ok := s.bindChat("t1", 555); !ok {
		t.Fatal("chat bog'lanmadi")
	}
	if got, ok := s.getByChat(555); !ok || got.Token != "t1" {
		t.Fatalf("chat bo'yicha topilmadi: %+v ok=%v", got, ok)
	}
	s.markLoggedIn("t1", "+998901234567")
	if got, _ := s.getByToken("t1"); !got.Done {
		t.Fatal("Done belgilanmadi")
	}
	s.drop("t1")
	if _, ok := s.getByToken("t1"); ok {
		t.Fatal("o'chirilgandan keyin ham topildi")
	}
}

// `WithRedis(nil)` — hech narsa qilmasligi kerak, yiqilmasligi ham.
func TestWithRedisNilIsNoop(t *testing.T) {
	v := &Verifier{store: newPendingStore()}
	if v.WithRedis(nil) != v {
		t.Fatal("nil bilan o'zini qaytarishi kerak")
	}
	if v.store.rdb != nil {
		t.Fatal("nil client yozilib qolgan")
	}
}

// testRedis — lokal Redis bo'lsa ulanadi, bo'lmasa testni o'tkazib
// yuboradi. CI'da `go test` uchun Redis ko'tarilmaydi, shuning uchun
// bu test u yerda JIMGINA o'tkaziladi — lekin ishlab chiqish
// mashinasida (`ondex run`) haqiqiy tekshiruv beradi.
func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6380" // docker-compose.yml dagi port
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		rdb.Close()
		t.Skipf("Redis yo'q (%s) — integratsiya testi o'tkazib yuborildi", addr)
	}
	return rdb
}

// ASOSIY SENARIY: server qayta ishga tushdi.
//
// Foydalanuvchi botda hammasini bajardi (Done=true), lekin ilova hali
// natijani so'rab ulgurmadi — aynan shu paytda deploy bo'ldi. Ilgari
// sessiya izsiz yo'qolardi va foydalanuvchi "havola eskirgan" ko'rardi.
func TestPendingSurvivesRestart(t *testing.T) {
	rdb := testRedis(t)
	defer rdb.Close()

	token := "test-restart-" + time.Now().Format("150405.000000")
	ctx := context.Background()
	defer rdb.Del(ctx, pendingTokenKey+token, pendingChatKey+"777")

	// --- "eski" server ---
	old := newPendingStore()
	old.rdb = rdb
	old.put(&Pending{
		Token:         token,
		ExpiresAt:     time.Now().Add(5 * time.Minute),
		ConfirmSecret: "sirli-kalit",
	})
	if _, ok := old.bindChat(token, 777); !ok {
		t.Fatal("chat bog'lanmadi")
	}
	old.markLoggedIn(token, "+998901112233")

	// --- server o'chdi va yangisi ko'tarildi ---
	fresh := newPendingStore()
	fresh.rdb = rdb
	fresh.load()

	got, ok := fresh.getByToken(token)
	if !ok {
		t.Fatal("restartdan keyin sessiya tiklanmadi — foydalanuvchi 'havola eskirgan' ko'radi")
	}
	if !got.Done || got.VerifiedPhone != "+998901112233" {
		t.Fatalf("tasdiqlangan holat yo'qolgan: Done=%v phone=%q", got.Done, got.VerifiedPhone)
	}
	// Fishingga qarshi maxfiy kalit ham saqlanishi SHART — busiz
	// natijani olish uchun tekshiruv o'tmay qoladi.
	if got.ConfirmSecret != "sirli-kalit" {
		t.Fatalf("ConfirmSecret yo'qolgan: %q", got.ConfirmSecret)
	}
	// Chat indeksi ham tiklanishi kerak: bot yangiligi faqat chat ID
	// bilan keladi.
	if byChat, ok := fresh.getByChat(777); !ok || byChat.Token != token {
		t.Fatal("chat -> token indeksi tiklanmadi")
	}
}

// `drop` Redis'dan ham o'chirishi kerak — aks holda tugagan sessiya
// keyingi restartda "tirilib" qaytardi.
func TestDropRemovesFromRedis(t *testing.T) {
	rdb := testRedis(t)
	defer rdb.Close()

	token := "test-drop-" + time.Now().Format("150405.000000")
	ctx := context.Background()

	s := newPendingStore()
	s.rdb = rdb
	s.put(&Pending{Token: token, ExpiresAt: time.Now().Add(5 * time.Minute)})
	s.bindChat(token, 888)
	s.drop(token)

	if n, _ := rdb.Exists(ctx, pendingTokenKey+token).Result(); n != 0 {
		t.Error("token kaliti Redis'da qolib ketdi")
	}
	if n, _ := rdb.Exists(ctx, pendingChatKey+"888").Result(); n != 0 {
		t.Error("chat kaliti Redis'da qolib ketdi")
	}
}
