package couriers

import (
	"context"
	"testing"
	"time"
)

// Production'da (2026-08-27) topilgan haqiqiy nosozlikning regressiya
// testi.
//
// ┌─ NIMA BO'LGAN EDI ────────────────────────────────────────────────┐
// Kuryerning `vehicle_type` = "bike" -> `geo.ModeBicycling`. Google
// Distance Matrix O'zbekistonda velosiped yo'nalishlarini umuman
// qo'llamaydi va element darajasida ZERO_RESULTS qaytaradi — LEKIN
// so'rovning yuqori statusi "OK" bo'ladi.
//
// `rankCandidates` esa "OK emas" nomzodni JIM tashlab yuborardi, ya'ni:
//
//   - bazada bo'sh, yaqin, onlayn kuryer BOR
//   - `err != nil` zaxira shoxi ishlamaydi (xato yo'q)
//   - saralangan ro'yxat bo'sh -> "hozircha bo'sh onlayn kuryer yo'q"
//   - tsikl 1076 marta aylandi, BIRORTA taklif yuborilmadi
//
// Test aynan shu holatni qayta yaratadi: yagona nomzodga Google ETA
// bermaydi. Kuryer baribir taklif olishi SHART.
// └───────────────────────────────────────────────────────────────────┘
func TestDispatchOffersWhenGoogleReturnsZeroResults(t *testing.T) {
	repo := newFakeRepo("bike1")
	n := &fakeNotifier{}

	geoClient := newFakeGeoClient(nil)
	geoClient.notOK = map[string]bool{"bike1": true}

	d := NewDispatcher(repo, n, geoClient, defaultOfferTTL)

	go func() {
		_, _ = d.Dispatch(context.Background(), "order1", DispatchParams{
			PreparationTime: 20 * time.Minute,
		})
	}()

	// Tuzatishdan OLDIN bu yerda taklif hech qachon kelmasdi.
	waitOffered(t, n, "bike1")
}

// Aralash holat: bir nomzodga ETA bor, ikkinchisiga yo'q. Ikkalasi ham
// taklif olishi kerak — ETA'siz qolgani zaxira hisob bilan.
func TestDispatchKeepsBothRoutableAndUnroutableCandidates(t *testing.T) {
	repo := newFakeRepo("withETA", "noETA")
	n := &fakeNotifier{}

	geoClient := newFakeGeoClient(map[string]time.Duration{
		"withETA": 7 * time.Minute,
	})
	geoClient.notOK = map[string]bool{"noETA": true}

	d := NewDispatcher(repo, n, geoClient, 60*time.Millisecond)

	go func() {
		_, _ = d.Dispatch(context.Background(), "order2", DispatchParams{
			PreparationTime: 20 * time.Minute,
		})
	}()

	waitOffered(t, n, "withETA")
	waitOffered(t, n, "noETA")
}
