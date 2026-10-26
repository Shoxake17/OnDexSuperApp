package storage

import (
	"context"
	"testing"
	"time"

	"chustapp/internal/ratings"
)

func TestMemoryRatingRepoCRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRatingRepo()

	r1 := &ratings.Rating{ID: "r1", OrderID: "o1", RestaurantID: "rest-a",
		CustomerID: "cust-1", Stars: 5, CreatedAt: time.Now()}
	if err := repo.Create(ctx, r1); err != nil {
		t.Fatal(err)
	}

	// Bitta buyurtmaga ikkinchi baho — rad etiladi.
	dup := &ratings.Rating{ID: "r2", OrderID: "o1", RestaurantID: "rest-a",
		CustomerID: "cust-1", Stars: 1, CreatedAt: time.Now()}
	if err := repo.Create(ctx, dup); err != ratings.ErrAlreadyRated {
		t.Fatalf("ErrAlreadyRated kutilgan, keldi: %v", err)
	}

	got, err := repo.GetByOrder(ctx, "o1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Stars != 5 {
		t.Fatalf("stars=5 kutilgan, keldi: %d", got.Stars)
	}

	if _, err := repo.GetByOrder(ctx, "yoq"); err != ratings.ErrNotFound {
		t.Fatalf("ErrNotFound kutilgan, keldi: %v", err)
	}

	r3 := &ratings.Rating{ID: "r3", OrderID: "o2", RestaurantID: "rest-a",
		CustomerID: "cust-2", Stars: 3, CreatedAt: time.Now()}
	if err := repo.Create(ctx, r3); err != nil {
		t.Fatal(err)
	}

	avg, count, err := repo.Summary(ctx, "rest-a")
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 || avg != 4 {
		t.Fatalf("avg=4 count=2 kutilgan, keldi avg=%v count=%d", avg, count)
	}

	avg, count, err = repo.Summary(ctx, "rest-bosh")
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 || avg != 0 {
		t.Fatalf("baho yo'q restoran uchun 0,0 kutilgan, keldi avg=%v count=%d", avg, count)
	}
}
