package storage

import (
	"context"
	"testing"
	"time"

	"chustapp/internal/banners"
)

func TestMemoryBannerRepoCRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryBannerRepo()

	b1 := &banners.Banner{ID: "b1", ImageURL: "https://x/1.webp", Active: true, SortOrder: 2,
		CreatedAt: time.Now(), UpdatedAt: time.Now()}
	b2 := &banners.Banner{ID: "b2", ImageURL: "https://x/2.webp", Active: false, SortOrder: 1,
		CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := repo.Create(ctx, b1); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, b2); err != nil {
		t.Fatal(err)
	}

	all, err := repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Tartib: sort_order bo'yicha — b2 (1) b1 (2) dan OLDIN.
	if len(all) != 2 || all[0].ID != "b2" || all[1].ID != "b1" {
		t.Fatalf("tartib noto'g'ri: %+v", all)
	}

	active, err := repo.ListActive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].ID != "b1" {
		t.Fatalf("faqat FAOL banner qaytishi kerak: %+v", active)
	}

	// Yangilash — qaytarilgan nusxa ICHKI xaritaga ta'sir qilmasligi kerak.
	got, err := repo.Get(ctx, "b1")
	if err != nil {
		t.Fatal(err)
	}
	got.Active = false
	if still, _ := repo.Get(ctx, "b1"); !still.Active {
		t.Fatal("qaytarilgan nusxani o'zgartirish ombordagi yozuvga ta'sir qildi")
	}

	b1.Active = false
	if err := repo.Update(ctx, b1); err != nil {
		t.Fatal(err)
	}
	if active, _ := repo.ListActive(ctx); len(active) != 0 {
		t.Fatalf("yangilashdan keyin FAOL banner qolmasligi kerak: %+v", active)
	}

	if err := repo.Delete(ctx, "b1"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, "b1"); err != banners.ErrNotFound {
		t.Fatalf("o'chirilgandan keyin ErrNotFound kutilgan, keldi: %v", err)
	}
	if err := repo.Delete(ctx, "yoq"); err != banners.ErrNotFound {
		t.Fatalf("mavjud bo'lmagan ID uchun ErrNotFound kutilgan, keldi: %v", err)
	}
	if err := repo.Update(ctx, &banners.Banner{ID: "yoq"}); err != banners.ErrNotFound {
		t.Fatalf("mavjud bo'lmagan ID uchun ErrNotFound kutilgan, keldi: %v", err)
	}
}
