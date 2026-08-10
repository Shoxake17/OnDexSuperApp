// Package images — mahsulot rasmlarini saqlash qatlami. Ikki implementatsiya:
// LocalStore (dev, disk) va R2Store (production, Cloudflare object storage).
// Qaysi biri ishlatilishi cmd/api/main.go da .env orqali tanlanadi.
package images

import (
	"context"
	"io"
)

// Store — rasm faylini saqlaydi va unga ochiq (public) URL qaytaradi.
type Store interface {
	Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) (url string, err error)
}
