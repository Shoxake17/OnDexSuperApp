package tables

import (
	"context"
	"strings"
	"testing"
	"time"
)

type memRepo struct {
	byID map[string]Table
}

func (m *memRepo) Create(_ context.Context, t *Table) error {
	if m.byID == nil {
		m.byID = map[string]Table{}
	}
	for _, x := range m.byID {
		if x.RestaurantID == t.RestaurantID &&
			strings.EqualFold(x.Zone, t.Zone) &&
			strings.EqualFold(x.Label, t.Label) {
			return ErrDuplicate
		}
	}
	m.byID[t.ID] = *t
	return nil
}

func (m *memRepo) GetByID(_ context.Context, id string) (*Table, error) {
	x, ok := m.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := x
	return &cp, nil
}

func (m *memRepo) GetByToken(_ context.Context, token string) (*Table, error) {
	for _, x := range m.byID {
		if x.QRToken == token {
			cp := x
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}

func (m *memRepo) ListByRestaurant(_ context.Context, restaurantID string) ([]*Table, error) {
	var out []*Table
	for _, x := range m.byID {
		if x.RestaurantID == restaurantID {
			cp := x
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (m *memRepo) Update(_ context.Context, t *Table) error {
	old, ok := m.byID[t.ID]
	if !ok {
		return ErrNotFound
	}
	cp := *t
	cp.QRToken = old.QRToken
	m.byID[t.ID] = cp
	return nil
}

func (m *memRepo) Delete(_ context.Context, id string) error {
	if _, ok := m.byID[id]; !ok {
		return ErrNotFound
	}
	delete(m.byID, id)
	return nil
}

func TestCreateInZoneAllowsSameNumberInDifferentZones(t *testing.T) {
	svc := NewService(&memRepo{})
	svc.now = func() time.Time { return time.Unix(1, 0) }
	a, err := svc.CreateInZone(context.Background(), "r1", DefaultZone, "5")
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.CreateInZone(context.Background(), "r1", "Ayvon", "5")
	if err != nil {
		t.Fatal(err)
	}
	if a.QRToken == b.QRToken {
		t.Fatal("har bir stol o'zining abadiy tokeniga ega bo'lishi shart")
	}
	if _, err := svc.CreateInZone(context.Background(), "r1", DefaultZone, "5"); err != ErrDuplicate {
		t.Fatalf("bir zonada takroriy raqam: %v", err)
	}
}

func TestQRTokenDoesNotChangeOnRenameOrZoneMove(t *testing.T) {
	svc := NewService(&memRepo{})
	t0, err := svc.CreateInZone(context.Background(), "r1", DefaultZone, "1")
	if err != nil {
		t.Fatal(err)
	}
	token := t0.QRToken
	renamed, err := svc.Rename(context.Background(), t0.ID, "12")
	if err != nil {
		t.Fatal(err)
	}
	moved, err := svc.SetZone(context.Background(), t0.ID, "VIP")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.QRToken != token || moved.QRToken != token {
		t.Fatalf("QR token o'zgardi: %q → rename=%q zone=%q",
			token, renamed.QRToken, moved.QRToken)
	}
}

func TestDisplayLabel(t *testing.T) {
	got := (&Table{Zone: DefaultZone, Label: "3"}).DisplayLabel()
	if got != "Asosiy zal · 3" {
		t.Fatalf("got %q", got)
	}
}
