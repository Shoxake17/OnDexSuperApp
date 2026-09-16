package couriers

import (
	"context"
	"testing"
	"time"
)

type fakeReach struct {
	online map[string]bool
	push   map[string]bool
}

func (f fakeReach) Online(id string) bool                     { return f.online[id] }
func (f fakeReach) CanPush(_ context.Context, id string) bool { return f.push[id] }

// agedRepo — joylashuv yoshini hisobga oladigan soxta ombor.
type agedRepo struct {
	*fakeRepo
	ages map[string]time.Duration
}

func (r *agedRepo) ListAvailableNear(ctx context.Context, pool string, lat, lng float64,
	radius float64, maxAge time.Duration, limit int) ([]*Courier, error) {
	all, err := r.fakeRepo.ListAvailableNear(ctx, pool, lat, lng, radius, maxAge, limit)
	if err != nil {
		return nil, err
	}
	out := make([]*Courier, 0, len(all))
	for _, c := range all {
		if r.ages[c.ID] <= maxAge {
			out = append(out, c)
		}
	}
	return out, nil
}

func idsOf(cs []*Courier) map[string]bool {
	m := make(map[string]bool, len(cs))
	for _, c := range cs {
		m[c.ID] = true
	}
	return m
}

// Ilova yopiq va push yo'q — taklif YETIB BORMAYDI, to'lqin behuda
// band qilinmaydi. Ilova yopiq, lekin push bor — joylashuvi eskirgan
// bo'lsa ham (smena davomida, 12 soatgacha) uyg'otiladi.
func TestCandidatesRespectReachability(t *testing.T) {
	repo := &agedRepo{
		fakeRepo: newFakeRepo("ochiq", "yopiq-push", "yopiq-pushsiz", "juda-eski"),
		ages: map[string]time.Duration{
			// "yopiq-push" — 45 daqiqa: avvalgi 30 daqiqalik chegara aynan
			// shunday kuryerni jimgina tashlab yuborardi.
			"ochiq": 0, "yopiq-push": 45 * time.Minute, "yopiq-pushsiz": 0, "juda-eski": 13 * time.Hour,
		},
	}
	reach := fakeReach{
		online: map[string]bool{"ochiq": true},
		push:   map[string]bool{"yopiq-push": true, "juda-eski": true},
	}
	ctx := context.Background()

	d := NewDispatcher(repo, &fakeNotifier{}, newFakeGeoClient(nil), time.Second).WithReachability(reach)
	got, err := d.candidates(ctx, DispatchParams{})
	if err != nil {
		t.Fatal(err)
	}
	ids := idsOf(got)
	if len(ids) != 2 || !ids["ochiq"] || !ids["yopiq-push"] {
		t.Fatalf("kutilgan {ochiq, yopiq-push}, keldi %v", ids)
	}

	// Tekshiruvchi ulanmagan — avvalgi xatti-harakat (faqat yangi joylashuv).
	plain := NewDispatcher(repo, &fakeNotifier{}, newFakeGeoClient(nil), time.Second)
	got, _ = plain.candidates(ctx, DispatchParams{})
	if ids := idsOf(got); len(ids) != 2 || !ids["ochiq"] || !ids["yopiq-pushsiz"] {
		t.Fatalf("tekshiruvchisiz: kutilgan {ochiq, yopiq-pushsiz}, keldi %v", ids)
	}
}

func TestPreferOnlineKeepsScoreOrderWithinGroups(t *testing.T) {
	c := func(id string) ScoredCandidate { return ScoredCandidate{Courier: &Courier{ID: id}} }
	d := NewDispatcher(newFakeRepo(), &fakeNotifier{}, newFakeGeoClient(nil), time.Second).
		WithReachability(fakeReach{online: map[string]bool{"b": true, "d": true}})
	got := d.preferOnline([]ScoredCandidate{c("a"), c("b"), c("c"), c("d")})
	want := []string{"b", "d", "a", "c"}
	for i, id := range want {
		if got[i].Courier.ID != id {
			t.Fatalf("tartib: kutilgan %v, keldi %v", want, []string{got[0].Courier.ID, got[1].Courier.ID, got[2].Courier.ID, got[3].Courier.ID})
		}
	}
}

// Ilovadan chiqib qaytgan kuryer taklifni SERVERDAN tiklaydi.
func TestPendingOfferRecovery(t *testing.T) {
	repo := newFakeRepo("k1", "k2")
	n := &fakeNotifier{}
	d := NewDispatcher(repo, n, newFakeGeoClient(map[string]time.Duration{
		"k1": 5 * time.Minute, "k2": 6 * time.Minute,
	}), 2*time.Second)

	done := make(chan string, 1)
	go func() {
		got, _ := d.Dispatch(context.Background(), "o-rec", DispatchParams{
			PreparationTime: 20 * time.Minute, RestaurantName: "Book Cafe",
		})
		done <- got
	}()
	waitOffered(t, n, "k2")

	info, ok := d.PendingOffer("k1")
	if !ok || info.OrderID != "o-rec" || info.RestaurantName != "Book Cafe" {
		t.Fatalf("ochiq taklif tiklanmadi: %+v %v", info, ok)
	}
	if info.ExpiresIn <= 0 || info.ExpiresIn > d.OfferTTL() {
		t.Fatalf("qolgan vaqt noto'g'ri: %v", info.ExpiresIn)
	}
	p := info.Payload(d.OfferTTL())
	if p["type"] != "offer" || p["total_sec"] != 2 || p["expires_in_sec"].(int) > 2 {
		t.Fatalf("payload: %v", p)
	}
	if _, ok := d.PendingOffer("begona"); ok {
		t.Fatal("to'lqinda bo'lmagan kuryerga taklif qaytdi")
	}

	// Rad etgan kuryerga qaytarilmaydi.
	d.HandleResponse("o-rec", Response{CourierID: "k2", Accepted: false})
	if _, ok := d.PendingOffer("k2"); ok {
		t.Fatal("rad etilgan taklif qayta tiklandi")
	}

	d.HandleResponse("o-rec", Response{CourierID: "k1", Accepted: true})
	if got := <-done; got != "k1" {
		t.Fatalf("g'olib: %q", got)
	}
	if _, ok := d.PendingOffer("k1"); ok {
		t.Fatal("yopilgan taklif hamon ochiq")
	}
}
