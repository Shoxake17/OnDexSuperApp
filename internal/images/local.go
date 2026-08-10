package images

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

// LocalStore — dev muhit uchun: rasmlar diskka yoziladi, server o'zi
// /uploads/* orqali beradi. Production'da R2Store ishlatilishi kerak —
// disk server qayta tiklanganda (masalan Docker konteyner yangilanganda)
// yo'qolishi mumkin.
type LocalStore struct {
	Dir string
}

func NewLocalStore(dir string) *LocalStore {
	return &LocalStore{Dir: dir}
}

func (s *LocalStore) Upload(_ context.Context, key string, r io.Reader, _ int64, _ string) (string, error) {
	path := filepath.Join(s.Dir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", err
	}
	return "/uploads/" + key, nil
}
