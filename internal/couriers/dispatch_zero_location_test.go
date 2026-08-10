package couriers

import (
	"context"
	"errors"
	"testing"
	"time"

	"chustapp/internal/geo"
)

// TestDispatchSkipsCourierWithZeroLocation — jonli sinovda topilgan haqiqiy
// holat: kuryer ilovasi ro'yxatdan o'tgan-u, hali joylashuvini yubormagan
// (Lat=Lng=0, "Gvineya ko'rfazi"). Bunday nomzod dispatch navbatiga
// UMUMAN kirmasligi, mantiqsiz uzoq ETA bilan real kuryerlarga navbatni
// to'smasligi kerak.
func TestDispatchSkipsCourierWithZeroLocation(t *testing.T) {
	repo := newFakeRepo("no_location", "real")
	repo.couriers[0].Lat, repo.couriers[0].Lng = 0, 0       // "no_location"
	repo.couriers[1].Lat, repo.couriers[1].Lng = 41.0, 71.0 // "real"
	n := &fakeNotifier{}
	geoClient := newFakeGeoClient(map[string]time.Duration{"real": 7 * time.Minute})
	d := NewDispatcher(repo, n, geoClient, defaultOfferTTL)

	go func() {
		waitOffered(t, n, "real")
		d.HandleResponse("o1", Response{CourierID: "real", Accepted: true})
	}()

	got, err := d.Dispatch(context.Background(), "o1", DispatchParams{
		RestaurantLocation: geo.LatLng{Lat: 41.0, Lng: 71.0},
		PreparationTime:    20 * time.Minute,
	})
	if err != nil || got != "real" {
		t.Fatalf("kutilgan 'real', olindi %q, xato: %v", got, err)
	}
	for _, o := range n.offered() {
		if o == "no_location" {
			t.Error("joylashuvi noma'lum (0,0) kuryerga taklif YUBORILMASLIGI kerak edi")
		}
	}
}

// TestDispatchAllCandidatesHaveZeroLocation — hammasi (0,0) bo'lsa,
// dispatch "kuryer topilmadi" deb TASLIM BO'LMAYDI (restoranda qayta
// urinish tugmasi yo'q, tizim o'zi cheksiz qayta uradi) — faqat `ctx`
// tugaganda to'xtaydi.
func TestDispatchAllCandidatesHaveZeroLocation(t *testing.T) {
	repo := newFakeRepo("ghost1", "ghost2")
	repo.couriers[0].Lat, repo.couriers[0].Lng = 0, 0
	repo.couriers[1].Lat, repo.couriers[1].Lng = 0, 0
	geoClient := newFakeGeoClient(nil)
	d := NewDispatcher(repo, &fakeNotifier{}, geoClient, 20*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_, err := d.Dispatch(ctx, "o1", DispatchParams{PreparationTime: 20 * time.Minute})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("context.DeadlineExceeded kutilgan edi, olindi: %v", err)
	}
}
