package storage

import (
	"context"
	"sort"
	"sync"

	"chustapp/internal/banners"
)

// MemoryBannerRepo — Postgres ulanmagan rejim (lokal sinov, demo) uchun.
type MemoryBannerRepo struct {
	mu   sync.RWMutex
	byID map[string]banners.Banner
}

func NewMemoryBannerRepo() *MemoryBannerRepo {
	return &MemoryBannerRepo{byID: map[string]banners.Banner{}}
}

func sortedBanners(list []*banners.Banner) []*banners.Banner {
	sort.Slice(list, func(i, j int) bool {
		if list[i].SortOrder != list[j].SortOrder {
			return list[i].SortOrder < list[j].SortOrder
		}
		return list[i].CreatedAt.Before(list[j].CreatedAt)
	})
	return list
}

func (r *MemoryBannerRepo) List(_ context.Context) ([]*banners.Banner, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]*banners.Banner, 0, len(r.byID))
	for _, b := range r.byID {
		cp := b
		list = append(list, &cp)
	}
	return sortedBanners(list), nil
}

func (r *MemoryBannerRepo) ListActive(ctx context.Context) ([]*banners.Banner, error) {
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*banners.Banner, 0, len(all))
	for _, b := range all {
		if b.Active {
			out = append(out, b)
		}
	}
	return out, nil
}

func (r *MemoryBannerRepo) Get(_ context.Context, id string) (*banners.Banner, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	b, ok := r.byID[id]
	if !ok {
		return nil, banners.ErrNotFound
	}
	cp := b
	return &cp, nil
}

func (r *MemoryBannerRepo) Create(_ context.Context, b *banners.Banner) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[b.ID] = *b
	return nil
}

func (r *MemoryBannerRepo) Update(_ context.Context, b *banners.Banner) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[b.ID]; !ok {
		return banners.ErrNotFound
	}
	r.byID[b.ID] = *b
	return nil
}

func (r *MemoryBannerRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[id]; !ok {
		return banners.ErrNotFound
	}
	delete(r.byID, id)
	return nil
}
