package couriers

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeRepo struct {
	mu        sync.Mutex
	couriers  []*Courier
	available map[string]bool
}

func newFakeRepo(ids ...string) *fakeRepo {
	r := &fakeRepo{available: make(map[string]bool)}
	for _, id := range ids {
		r.couriers = append(r.couriers, &Courier{ID: id, Available: true, Approved: true})
		r.available[id] = true
	}
	return r
}

func (r *fakeRepo) Create(_ context.Context, c *Courier) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.couriers = append(r.couriers, c)
	r.available[c.ID] = c.Available
	return nil
}

func (r *fakeRepo) ListAll(_ context.Context) ([]*Courier, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]*Courier(nil), r.couriers...), nil
}

func (r *fakeRepo) SetApproved(_ context.Context, id string, approved bool) error {
	for _, c := range r.couriers {
		if c.ID == id {
			c.Approved = approved
			return nil
		}
	}
	return ErrNoCourier
}

func (r *fakeRepo) GetByID(_ context.Context, id string) (*Courier, error) {
	for _, c := range r.couriers {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, ErrNoCourier
}

func (r *fakeRepo) FindNearby(_ context.Context, _, _ float64, limit int) ([]*Courier, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*Courier
	for _, c := range r.couriers {
		if r.available[c.ID] && len(out) < limit {
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *fakeRepo) SetAvailable(_ context.Context, id string, v bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.available[id] = v
	return nil
}

// fakeNotifier — taklif yuborilgan kuryerlarni yozib boradi.
type fakeNotifier struct {
	mu     sync.Mutex
	offers []string
}

func (n *fakeNotifier) SendOffer(courierID, _ string, _ time.Duration) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.offers = append(n.offers, courierID)
}
func (n *fakeNotifier) CancelOffer(_, _ string) {}

func (n *fakeNotifier) offered() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.offers...)
}

// respondWhenOffered — kuryer nomidan javob beruvchi yordamchi: taklif shu
// kuryerga yetib borishini kutadi, keyin javob yuboradi.
func respondWhenOffered(t *testing.T, d *Dispatcher, n *fakeNotifier, orderID, courierID string, accept bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, id := range n.offered() {
			if id == courierID {
				if d.HandleResponse(orderID, Response{CourierID: courierID, Accepted: accept}) {
					return
				}
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Errorf("kuryer %s ga taklif yetib bormadi", courierID)
}

func TestDispatchFirstAccepts(t *testing.T) {
	repo := newFakeRepo("c1", "c2")
	n := &fakeNotifier{}
	d := NewDispatcher(repo, n, time.Second, 5)

	go respondWhenOffered(t, d, n, "o1", "c1", true)

	got, err := d.Dispatch(context.Background(), "o1", 0, 0)
	if err != nil || got != "c1" {
		t.Fatalf("kutilgan c1, olindi %q, xato: %v", got, err)
	}
	if repo.available["c1"] {
		t.Error("qabul qilgan kuryer band bo'lishi kerak edi")
	}
}

func TestDispatchDeclineMovesToNext(t *testing.T) {
	repo := newFakeRepo("c1", "c2")
	n := &fakeNotifier{}
	d := NewDispatcher(repo, n, time.Second, 5)

	go func() {
		respondWhenOffered(t, d, n, "o1", "c1", false) // c1 rad etadi
		respondWhenOffered(t, d, n, "o1", "c2", true)  // c2 qabul qiladi
	}()

	got, err := d.Dispatch(context.Background(), "o1", 0, 0)
	if err != nil || got != "c2" {
		t.Fatalf("kutilgan c2, olindi %q, xato: %v", got, err)
	}
}

func TestDispatchTimeoutMovesToNext(t *testing.T) {
	repo := newFakeRepo("c1", "c2")
	n := &fakeNotifier{}
	d := NewDispatcher(repo, n, 50*time.Millisecond, 5) // c1 javob bermaydi

	go respondWhenOffered(t, d, n, "o1", "c2", true)

	got, err := d.Dispatch(context.Background(), "o1", 0, 0)
	if err != nil || got != "c2" {
		t.Fatalf("kutilgan c2 (c1 timeout), olindi %q, xato: %v", got, err)
	}
}

func TestDispatchAllDeclineReturnsError(t *testing.T) {
	repo := newFakeRepo("c1", "c2")
	n := &fakeNotifier{}
	d := NewDispatcher(repo, n, time.Second, 5)

	go func() {
		respondWhenOffered(t, d, n, "o1", "c1", false)
		respondWhenOffered(t, d, n, "o1", "c2", false)
	}()

	if _, err := d.Dispatch(context.Background(), "o1", 0, 0); !errors.Is(err, ErrNoCourier) {
		t.Fatalf("ErrNoCourier kutilgan edi, olindi: %v", err)
	}
}

func TestStaleResponseRejected(t *testing.T) {
	repo := newFakeRepo("c1")
	n := &fakeNotifier{}
	d := NewDispatcher(repo, n, time.Second, 5)

	// Hech qanday dispatch yo'q — javob qabul qilinmasligi kerak.
	if d.HandleResponse("yoq-order", Response{CourierID: "c1", Accepted: true}) {
		t.Error("mavjud bo'lmagan taklifga javob qabul qilindi")
	}

	// Taklif c1 da turganda boshqa kuryer javob berolmasligi kerak.
	done := make(chan struct{})
	go func() {
		defer close(done)
		d.Dispatch(context.Background(), "o1", 0, 0)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for len(n.offered()) == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if d.HandleResponse("o1", Response{CourierID: "c99", Accepted: true}) {
		t.Error("begona kuryer javobi qabul qilindi")
	}
	d.HandleResponse("o1", Response{CourierID: "c1", Accepted: true})
	<-done
}

func TestNoCouriersAvailable(t *testing.T) {
	repo := newFakeRepo() // bo'sh
	d := NewDispatcher(repo, &fakeNotifier{}, time.Second, 5)
	if _, err := d.Dispatch(context.Background(), "o1", 0, 0); !errors.Is(err, ErrNoCourier) {
		t.Fatalf("ErrNoCourier kutilgan edi, olindi: %v", err)
	}
}
