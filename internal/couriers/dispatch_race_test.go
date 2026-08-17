package couriers

import (
	"context"
	"sync"
	"testing"
	"time"
)

// ┌─ NEGA BU TEST BOR ──────────────────────────────────────────────────┐
// To'lqinli dispatch bir vaqtda bir NECHTA kuryerga taklif yuboradi va
// "kim birinchi qabul qilsa — o'sha oladi" tamoyilida ishlaydi
// (`waveOffer` izohiga qarang). Ya'ni bir nechta kuryer AYNAN BIR
// PAYTDA "qabul qilaman" bosishi — kutilmagan holat emas, NORMAL holat.
//
// Agar ikkitasi ham yutsa, bitta buyurtmaga ikkita kuryer biriktiriladi:
// ikkalasi restoranga boradi, ikkalasi haq talab qiladi va buyurtma
// holati qaysi biri oxirgi yozgan bo'lsa o'shanikiga aylanadi.
//
// Mavjud testlar mantiqning ketma-ket yo'llarini qamragan
// (`TestDispatchStopsAfterAcceptDoesNotOfferNextWave`,
// `TestHandleResponseRejectsStaleOrWrongCandidate`), lekin HAQIQIY
// bir vaqtdalik sinalmagan edi. Bu test aynan shuni tekshiradi va
// `go test -race` ostida ma'lumot poygasini ham ushlaydi.
// └─────────────────────────────────────────────────────────────────────┘

// Bir vaqtda qabul qilgan bir necha kuryerdan FAQAT BITTASI yutishi kerak.
func TestConcurrentAcceptOnlyOneWins(t *testing.T) {
	const n = 3 // bitta to'lqinga sig'adigan nomzodlar
	ids := []string{"c1", "c2", "c3"}

	repo := newFakeRepo(ids...)
	notifier := &fakeNotifier{}
	// ETA lar deyarli teng — saralash natijasi muhim emas, hammasi
	// bitta to'lqinda taklif olishi kerak.
	geoClient := newFakeGeoClient(map[string]time.Duration{
		"c1": 7 * time.Minute,
		"c2": 7 * time.Minute,
		"c3": 7 * time.Minute,
	})
	d := NewDispatcher(repo, notifier, geoClient, defaultOfferTTL)

	// Barcha kuryerlar taklif olguncha kutamiz, so'ng ular AYNAN BIR
	// PAYTDA javob berishadi. `start` kanali — to'siq (barrier):
	// goroutine'lar tayyor bo'lib turadi va yopilganda birdan yuguradi.
	start := make(chan struct{})
	var wg sync.WaitGroup
	accepted := make([]bool, n)

	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			waitOffered(t, notifier, id)
			<-start
			accepted[i] = d.HandleResponse("o1", Response{CourierID: id, Accepted: true})
		}(i, id)
	}

	go func() {
		// Uchalasi ham taklif olgach to'siqni ochamiz.
		for _, id := range ids {
			waitOffered(t, notifier, id)
		}
		close(start)
	}()

	winner, err := d.Dispatch(context.Background(), "o1",
		DispatchParams{PreparationTime: 20 * time.Minute})
	wg.Wait()

	if err != nil {
		t.Fatalf("dispatch xatosi: %v", err)
	}
	if winner == "" {
		t.Fatal("hech kim tanlanmadi — holbuki uchalasi ham qabul qilgan edi")
	}

	// ★ ASOSIY INVARIANT: aynan bitta kuryer band bo'lishi kerak.
	busy := []string{}
	for _, id := range ids {
		if !repo.available[id] {
			busy = append(busy, id)
		}
	}
	if len(busy) != 1 {
		t.Fatalf("XATO: %d ta kuryer band bo'ldi (%v) — bitta buyurtmaga bir nechta kuryer biriktirilgan",
			len(busy), busy)
	}
	if busy[0] != winner {
		t.Errorf("band bo'lgan kuryer (%s) dispatch qaytargani (%s) bilan mos emas",
			busy[0], winner)
	}
}

// Bir xil kuryer bir necha marta "qabul qilaman" yuborsa (ilovada
// tugma ikki marta bosilishi — juda oddiy holat), natija o'zgarmasligi
// kerak.
func TestDoubleTapAcceptIsHarmless(t *testing.T) {
	repo := newFakeRepo("c1", "c2")
	notifier := &fakeNotifier{}
	geoClient := newFakeGeoClient(map[string]time.Duration{
		"c1": 7 * time.Minute,
		"c2": 9 * time.Minute,
	})
	d := NewDispatcher(repo, notifier, geoClient, defaultOfferTTL)

	go func() {
		waitOffered(t, notifier, "c1")
		// Uch marta ketma-ket — foydalanuvchi tugmani takror bosdi.
		for i := 0; i < 3; i++ {
			d.HandleResponse("o1", Response{CourierID: "c1", Accepted: true})
		}
	}()

	winner, err := d.Dispatch(context.Background(), "o1",
		DispatchParams{PreparationTime: 20 * time.Minute})
	if err != nil {
		t.Fatalf("dispatch xatosi: %v", err)
	}
	if winner != "c1" {
		t.Fatalf("kutilgan c1, olindi %q", winner)
	}
	if repo.available["c2"] == false {
		t.Error("c2 qabul qilmagan edi — u band bo'lmasligi kerak")
	}
}
