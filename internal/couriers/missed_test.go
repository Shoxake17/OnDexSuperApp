package couriers

import (
	"context"
	"sync"
	"testing"
	"time"
)

type offlineNotifier struct {
	fakeNotifier
	offMu    sync.Mutex
	offlined []string
}

func (n *offlineNotifier) CourierAutoOffline(courierID string, _ int) {
	n.offMu.Lock()
	defer n.offMu.Unlock()
	n.offlined = append(n.offlined, courierID)
}

func (n *offlineNotifier) wasOfflined(id string) bool {
	n.offMu.Lock()
	defer n.offMu.Unlock()
	for _, o := range n.offlined {
		if o == id {
			return true
		}
	}
	return false
}

func availableOf(r *fakeRepo, id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.available[id]
}

// Liniyadan chiqishni unutgan kuryer: ketma-ket javobsiz takliflardan keyin
// liniyadan chiqariladi, xabar oladi va unga boshqa taklif yuborilmaydi.
func TestAutoOfflineAfterMissedOffers(t *testing.T) {
	repo := newFakeRepo("jim")
	n := &offlineNotifier{}
	d := NewDispatcher(repo, n, newFakeGeoClient(map[string]time.Duration{"jim": 5 * time.Minute}),
		60*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _, _ = d.Dispatch(ctx, "o1", DispatchParams{PreparationTime: 20 * time.Minute}) }()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !n.wasOfflined("jim") {
		time.Sleep(5 * time.Millisecond)
	}
	if !n.wasOfflined("jim") {
		t.Fatal("javobsiz kuryer liniyadan chiqarilmadi")
	}
	if availableOf(repo, "jim") {
		t.Fatal("kuryer hamon liniyada")
	}
	time.Sleep(200 * time.Millisecond)
	if got := n.offerCount("jim"); got != maxMissedOffers {
		t.Fatalf("taklif soni: kutilgan %d, keldi %d", maxMissedOffers, got)
	}
}

// Javob (rad etish ham) hisobni nolga qaytaradi — oradagi javobsizliklar
// qo'shilib ketmaydi.
func TestAnswerResetsMissedCount(t *testing.T) {
	repo := newFakeRepo("k")
	d := NewDispatcher(repo, &fakeNotifier{}, newFakeGeoClient(nil), time.Second)
	ctx := context.Background()

	d.recordMissed(ctx, "k")
	d.recordMissed(ctx, "k")
	d.resetMissed("k")
	d.recordMissed(ctx, "k")
	d.recordMissed(ctx, "k")
	if !availableOf(repo, "k") {
		t.Fatal("javob bergan kuryer liniyadan chiqarildi")
	}
	d.recordMissed(ctx, "k")
	if availableOf(repo, "k") {
		t.Fatal("ketma-ket 3 ta javobsizlikdan keyin liniyadan chiqarilishi kerak edi")
	}
}
