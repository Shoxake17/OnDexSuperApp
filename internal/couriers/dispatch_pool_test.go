package couriers

import (
	"context"
	"testing"
	"time"
)

// XAVFSIZLIK CHEGARASI: restoran buyurtmasi FAQAT shu restoranning o'z
// kuryerlariga taklif qilinadi. Boshqa restoranning kuryeri ham, OnDex
// platforma kuryeri ham (hatto eng yaqini bo'lsa ham) taklif olmaydi va
// javob bera olmaydi.
func TestDispatchOffersOnlyOwnPool(t *testing.T) {
	repo := newFakeRepo("own-1", "own-2", "boshqa", "platforma")
	for _, c := range repo.couriers {
		switch c.ID {
		case "own-1", "own-2":
			c.RestaurantID = "rest-a"
		case "boshqa":
			c.RestaurantID = "rest-b"
		}
	}
	n := &fakeNotifier{}
	// Begonalar ATAYLAB eng yaqin: havuz filtri ballashdan oldin ishlashi kerak.
	geoClient := newFakeGeoClient(map[string]time.Duration{
		"own-1": 9 * time.Minute, "own-2": 8 * time.Minute,
		"boshqa": time.Minute, "platforma": time.Minute,
	})
	d := NewDispatcher(repo, n, geoClient, 500*time.Millisecond)

	foreign := make(chan bool, 1)
	go func() {
		waitOffered(t, n, "own-2")
		time.Sleep(30 * time.Millisecond)
		foreign <- d.HandleResponse("o-pool", Response{CourierID: "boshqa", Accepted: true}) ||
			d.HandleResponse("o-pool", Response{CourierID: "platforma", Accepted: true})
		d.HandleResponse("o-pool", Response{CourierID: "own-2", Accepted: true})
	}()

	got, err := d.Dispatch(context.Background(), "o-pool", DispatchParams{
		PreparationTime: 20 * time.Minute, CourierPool: "rest-a", RestaurantID: "rest-a",
	})
	if err != nil || got != "own-2" {
		t.Fatalf("kutilgan 'own-2', olindi %q (%v)", got, err)
	}
	if n.offerCount("boshqa")+n.offerCount("platforma") != 0 {
		t.Fatalf("begona havuz kuryeriga taklif ketdi: %v", n.offered())
	}
	if <-foreign {
		t.Fatal("begona havuz kuryerining javobi qabul qilindi")
	}
}
