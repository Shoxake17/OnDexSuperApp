package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/voice"
)

func TestClipStoreMemory(t *testing.T) {
	runClipStoreContract(t, NewMemoryClipStore())
}

func TestClipStorePostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("CI'da TEST_DATABASE_URL BO'LISHI SHART")
		}
		t.Skip("TEST_DATABASE_URL berilmagan — Postgres testi o'tkazib yuborildi")
	}
	ctx := context.Background()
	root, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	defer root.Close()
	schema := fmt.Sprintf("voice_it_%d", time.Now().UnixNano())
	if _, err := root.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("sxema: %v", err)
	}
	defer func() {
		if _, err := root.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("sxema o'chirilmadi: %v", err)
		}
	}()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	sqlBytes, err := migrationFS.ReadFile("migrations/0055_voice_clips.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
		t.Fatalf("migratsiya: %v", err)
	}
	runClipStoreContract(t, NewPgClipStore(pool))
}

func runClipStoreContract(t *testing.T, store voice.ClipStore) {
	t.Helper()
	ctx := context.Background()
	if _, err := store.GetClip(ctx, "k1"); !errors.Is(err, voice.ErrClipNotFound) {
		t.Fatalf("yo'q fayl: %v", err)
	}
	if err := store.SaveClip(ctx, "k1", "matn", []byte("RIFF-1")); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveClip(ctx, "k1", "matn", []byte("RIFF-2")); err != nil {
		t.Fatal(err)
	}
	if got, err := store.GetClip(ctx, "k1"); err != nil || string(got) != "RIFF-1" {
		t.Fatalf("birinchi yozuv qolishi kerak: %q %v", got, err)
	}
	if err := store.SaveClip(ctx, "k2", "matn", nil); err == nil {
		t.Fatal("bo'sh fayl saqlanmasligi kerak")
	}
	if err := store.SaveClip(ctx, "k3", "matn", []byte(strings.Repeat("x", voice.MaxClipBytes+1))); err == nil {
		t.Fatal("juda katta fayl saqlanmasligi kerak")
	}
}
