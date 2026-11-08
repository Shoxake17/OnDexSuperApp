package storage

import (
	"context"
	"errors"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgAvatarStore — `user_avatars` (migratsiya 0073; `httpapi.AvatarStore`).
// Bo'sh satr — rasm yo'q; `Set(ctx, id, "")` rasmni olib tashlaydi.
type PgAvatarStore struct{ pool *pgxpool.Pool }

func NewPgAvatarStore(pool *pgxpool.Pool) *PgAvatarStore { return &PgAvatarStore{pool: pool} }

func (s *PgAvatarStore) Get(ctx context.Context, userID string) (string, error) {
	var url string
	err := s.pool.QueryRow(ctx, `SELECT url FROM user_avatars WHERE user_id = $1`, userID).Scan(&url)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return url, err
}

func (s *PgAvatarStore) Set(ctx context.Context, userID, url string) error {
	if url == "" {
		_, err := s.pool.Exec(ctx, `DELETE FROM user_avatars WHERE user_id = $1`, userID)
		return err
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO user_avatars (user_id, url) VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET url = EXCLUDED.url, updated_at = now()`, userID, url)
	return err
}

// MemoryAvatarStore — testlar va Postgres'siz dev.
type MemoryAvatarStore struct {
	mu sync.Mutex
	m  map[string]string
}

func NewMemoryAvatarStore() *MemoryAvatarStore { return &MemoryAvatarStore{m: map[string]string{}} }

func (s *MemoryAvatarStore) Get(_ context.Context, userID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[userID], nil
}

func (s *MemoryAvatarStore) Set(_ context.Context, userID, url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if url == "" {
		delete(s.m, userID)
	} else {
		s.m[userID] = url
	}
	return nil
}
