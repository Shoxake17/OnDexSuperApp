package couriers

import (
	"context"
	"sync"
	"testing"
	"time"
)

// To'lqinli dispatch testlari (`waveOffer`, `awaitWave`).
//
// Eng muhimi — POYGA: to'lqindagi bir necha kuryer bir vaqtda "qabul
// qilaman" bosishi mumkin. Faqat BITTASI g'olib bo'lishi shart, aks
// holda bitta buyurtmaga ikki kuryer biriktirilardi.

// TestWaveOffersAllSimultaneously — to'lqindagi HAMMA kuryer taklif
// oladi (bittalab emas). Bu — o'zgarishning asosiy maqsadi.
func TestWaveOffersAllSimultaneously(t *testing.T) {
	repo := newFakeRepo("a", "b", "c", "d", "e")
	n := &fakeNotifier{}
	geoClient := newFakeGeoClient(map[string]time.Duration{
		"a": 7 * time.Minute, "b": 8 * time.Minute, "c": 9 * time.Minute,
		"d": 20 * time.Minute, "e": 25 * time.Minute,
	})
	d := NewDispatcher(repo, n, geoClient, 200*time.Millisecond)

	go func() {
		waitOffered(t, n, "a")
		// Birinchi to'lqin to'liq yuborilishini kutamiz.
		time.Sleep(30 * time.Millisecond)
		d.HandleResponse("o1", Response{CourierID: "a", Accepted: true})
	}()

	got, err := d.Dispatch(context.Background(), "o1",
		DispatchParams{PreparationTime: 20 * time.Minute})
	if err != nil || got != "a" {
		t.Fatalf("kutilgan 'a', olindi %q (%v)", got, err)
	}

	// Birinchi to'lqinda AYNAN waveSize ta kuryer bo'lishi kerak.
	offered := 0
	for _, id := range []string{"a", "b", "c"} {
		offered += n.offerCount(id)
	}
	if offered != waveSize {
		t.Fatalf("birinchi to'lqinda %d ta taklif kutilgan, olindi %d (%v)",
			waveSize, offered, n.offered())
	}
	// Ikkinchi to'lqin umuman ketmasligi kerak.
	if n.offerCount("d")+n.offerCount("e") != 0 {
		t.Errorf("g'olib birinchi to'lqinda topilgan — ikkinchisi ketmasligi kerak: %v",
			n.offered())
	}
}

// ★ POYGA TESTI
//
// To'lqindagi UCHALA kuryer ham bir vaqtda "qabul qilaman" bosadi.
// Natijada FAQAT BITTASI g'olib bo'lishi va FAQAT BITTASI band
// qilinishi kerak.
func TestWaveOnlyOneWinnerWhenAllAcceptAtOnce(t *testing.T) {
	repo := newFakeRepo("a", "b", "c")
	n := &fakeNotifier{}
	geoClient := newFakeGeoClient(map[string]time.Duration{
		"a": 7 * time.Minute, "b": 8 * time.Minute, "c": 9 * time.Minute,
	})
	d := NewDispatcher(repo, n, geoClient, time.Second)

	go func() {
		waitOffered(t, n, "a")
		waitOffered(t, n, "b")
		waitOffered(t, n, "c")
		// Uchalasi BIR VAQTDA bosadi.
		var wg sync.WaitGroup
		for _, id := range []string{"a", "b", "c"} {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				d.HandleResponse("o1", Response{CourierID: id, Accepted: true})
			}(id)
		}
		wg.Wait()
	}()

	got, err := d.Dispatch(context.Background(), "o1",
		DispatchParams{PreparationTime: 20 * time.Minute})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if got == "" {
		t.Fatal("g'olib aniqlanmadi")
	}

	// FAQAT g'olib band bo'lishi kerak — qolganlari hamon bo'sh.
	repo.mu.Lock()
	defer repo.mu.Unlock()
	busy := 0
	for id, avail := range repo.available {
		if !avail {
			busy++
			if id != got {
				t.Errorf("g'olib bo'lmagan kuryer (%s) band qilingan", id)
			}
		}
	}
	if busy != 1 {
		t.Fatalf("AYNAN bitta kuryer band bo'lishi kerak edi, band: %d", busy)
	}
}

// Qabul qilgan kuryer shu lahzada BOSHQA buyurtmani olib ulgurgan
// bo'lsa (ClaimIfAvailable=false), to'lqin TO'XTAMASLIGI va keyingi
// javobni kutishda davom etishi kerak — aks holda taklif behuda
// yo'qolardi.
func TestWaveContinuesWhenWinnerAlreadyBusy(t *testing.T) {
	repo := newFakeRepo("busy", "free")
	n := &fakeNotifier{}
	geoClient := newFakeGeoClient(map[string]time.Duration{
		"busy": 7 * time.Minute, "free": 8 * time.Minute,
	})
	d := NewDispatcher(repo, n, geoClient, time.Second)

	go func() {
		waitOffered(t, n, "busy")
		waitOffered(t, n, "free")
		// "busy" boshqa buyurtmani olib ulgurdi — endi u bo'sh emas.
		repo.mu.Lock()
		repo.available["busy"] = false
		repo.mu.Unlock()

		d.HandleResponse("o1", Response{CourierID: "busy", Accepted: true})
		time.Sleep(20 * time.Millisecond)
		d.HandleResponse("o1", Response{CourierID: "free", Accepted: true})
	}()

	got, err := d.Dispatch(context.Background(), "o1",
		DispatchParams{PreparationTime: 20 * time.Minute})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if got != "free" {
		t.Fatalf("band kuryer o'rniga bo'shi kutilgan, olindi %q", got)
	}
}

// Hammasi rad etsa — to'lqin muddati TUGASHINI KUTMASDAN keyingisiga
// o'tilishi kerak. Busiz har to'lqin bekorga TTL kutardi.
func TestWaveMovesOnImmediatelyWhenAllReject(t *testing.T) {
	repo := newFakeRepo("a", "b", "c", "d")
	n := &fakeNotifier{}
	geoClient := newFakeGeoClient(map[string]time.Duration{
		"a": 7 * time.Minute, "b": 8 * time.Minute,
		"c": 9 * time.Minute, "d": 10 * time.Minute,
	})
	// TTL ATAYLAB uzun: agar kod muddatni kutsa, test uni sezadi.
	const ttl = 3 * time.Second
	d := NewDispatcher(repo, n, geoClient, ttl)

	go func() {
		for _, id := range []string{"a", "b", "c"} {
			waitOffered(t, n, id)
		}
		for _, id := range []string{"a", "b", "c"} {
			d.HandleResponse("o1", Response{CourierID: id, Accepted: false})
		}
		waitOffered(t, n, "d")
		d.HandleResponse("o1", Response{CourierID: "d", Accepted: true})
	}()

	start := time.Now()
	got, err := d.Dispatch(context.Background(), "o1",
		DispatchParams{PreparationTime: 20 * time.Minute})
	elapsed := time.Since(start)

	if err != nil || got != "d" {
		t.Fatalf("kutilgan 'd', olindi %q (%v)", got, err)
	}
	if elapsed >= ttl {
		t.Fatalf("hammasi rad etgach to'lqin muddati KUTILDI (%v) — "+
			"darhol keyingisiga o'tishi kerak edi", elapsed)
	}
}

// Yutqazganlarga taklif BEKOR qilinishi kerak — aks holda ularning
// ilovasida taklif kartochkasi muddati tugaguncha osilib turardi.
func TestWaveCancelsLosers(t *testing.T) {
	repo := newFakeRepo("a", "b", "c")
	n := &fakeNotifier{}
	geoClient := newFakeGeoClient(map[string]time.Duration{
		"a": 7 * time.Minute, "b": 8 * time.Minute, "c": 9 * time.Minute,
	})
	d := NewDispatcher(repo, n, geoClient, time.Second)

	go func() {
		waitOffered(t, n, "c")
		d.HandleResponse("o1", Response{CourierID: "a", Accepted: true})
	}()

	got, _ := d.Dispatch(context.Background(), "o1",
		DispatchParams{PreparationTime: 20 * time.Minute})
	if got != "a" {
		t.Fatalf("kutilgan 'a', olindi %q", got)
	}
	for _, id := range []string{"b", "c"} {
		if !n.wasCancelled(id) {
			t.Errorf("yutqazgan %s ga bekor qilish yuborilmadi (%v)", id, n.cancelled)
		}
	}
	if n.wasCancelled("a") {
		t.Error("g'olibga bekor qilish yuborilmasligi kerak edi")
	}
}
