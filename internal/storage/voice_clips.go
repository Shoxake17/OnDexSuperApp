package storage

import (
	"context"
	"errors"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/voice"
)

// PgClipStore — ovoz fayllari (migration 0055, `voice_clips`).
type PgClipStore struct{ pool *pgxpool.Pool }

func NewPgClipStore(pool *pgxpool.Pool) *PgClipStore { return &PgClipStore{pool: pool} }

func (s *PgClipStore) GetClip(ctx context.Context, key string) ([]byte, error) {
	var data []byte
	err := s.pool.QueryRow(ctx, `SELECT audio FROM voice_clips WHERE key = $1`, key).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, voice.ErrClipNotFound
	}
	return data, err
}

func (s *PgClipStore) SaveClip(ctx context.Context, key, text string, data []byte) error {
	if len(data) == 0 || len(data) > voice.MaxClipBytes {
		return errors.New("ovoz fayli hajmi noto'g'ri")
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO voice_clips (key, text, audio) VALUES ($1, $2, $3)
		 ON CONFLICT (key) DO NOTHING`, key, text, data)
	return err
}

// MemoryClipStore — DATABASE_URL berilmagan holat uchun.
type MemoryClipStore struct {
	mu   sync.RWMutex
	data map[string][]byte
}

func NewMemoryClipStore() *MemoryClipStore { return &MemoryClipStore{data: map[string][]byte{}} }

func (s *MemoryClipStore) GetClip(_ context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.data[key]
	if !ok {
		return nil, voice.ErrClipNotFound
	}
	return append([]byte(nil), d...), nil
}

func (s *MemoryClipStore) SaveClip(_ context.Context, key, _ string, data []byte) error {
	if len(data) == 0 || len(data) > voice.MaxClipBytes {
		return errors.New("ovoz fayli hajmi noto'g'ri")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[key]; !exists {
		s.data[key] = append([]byte(nil), data...)
	}
	return nil
}
