package images

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
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

var errBadKey = errors.New("images: noto'g'ri fayl kaliti")

func (s *LocalStore) Upload(_ context.Context, key string, r io.Reader, _ int64, _ string) (string, error) {
	base, err := filepath.Abs(s.Dir)
	if err != nil {
		return "", err
	}
	path := filepath.Join(base, filepath.FromSlash(key))
	// Kalitni server o'zi yasaydi, lekin baribir tekshiriladi: `../` bilan
	// saqlash papkasidan tashqariga yozib bo'lmasin.
	rel, err := filepath.Rel(base, path)
	if err != nil || rel == "." || rel == ".." || filepath.IsAbs(rel) ||
		strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errBadKey
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", err
	}
	f, err := os.Create(path) //nolint:gosec // yo'l yuqorida saqlash papkasi ichida ekani tekshirildi
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", err
	}
	return "/uploads/" + key, nil
}
