package httpapi

import (
	"context"
	"io"
	"testing"

	"chustapp/internal/images"
)

// recordingStore — `images.Store` (sarlavhasiz) va `optionUploader` ni yozib oladi.
type recordingStore struct {
	plainCalls int
}

func (s *recordingStore) Upload(_ context.Context, key string, r io.Reader, _ int64, _ string) (string, error) {
	s.plainCalls++
	_, _ = io.Copy(io.Discard, r)
	return "/uploads/" + key, nil
}

type optionStore struct {
	recordingStore
	gotOpt  images.UploadOptions
	gotKey  string
	gotSize int64
}

func (s *optionStore) UploadWithOptions(_ context.Context, key string, r io.Reader, size int64, opt images.UploadOptions) (string, error) {
	s.gotOpt, s.gotKey, s.gotSize = opt, key, size
	_, _ = io.Copy(io.Discard, r)
	return "https://cdn.example/" + key, nil
}

// R2 (sarlavhalarni biladi): immutable Cache-Control va to'g'ri Content-Type yuboriladi.
func TestUploadImmutable_UsesHeadersWhenSupported(t *testing.T) {
	st := &optionStore{}
	url, err := uploadImmutable(context.Background(), st, "logos/abc.webp", []byte("12345"), "image/webp")
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://cdn.example/logos/abc.webp" {
		t.Fatalf("url = %q", url)
	}
	if st.gotOpt.CacheControl != "public, max-age=31536000, immutable" {
		t.Errorf("CacheControl = %q", st.gotOpt.CacheControl)
	}
	if st.gotOpt.ContentType != "image/webp" || st.gotSize != 5 || st.gotKey != "logos/abc.webp" {
		t.Errorf("noto'g'ri parametrlar: %+v size=%d key=%s", st.gotOpt, st.gotSize, st.gotKey)
	}
	if st.plainCalls != 0 {
		t.Error("sarlavhali yo'l bor edi, oddiy Upload chaqirilmasligi kerak")
	}
}

// Lokal disk (dev): sarlavha tushunchasi yo'q, oddiy Upload ishlaydi.
func TestUploadImmutable_FallsBackToPlainUpload(t *testing.T) {
	st := &recordingStore{}
	url, err := uploadImmutable(context.Background(), st, "logos/abc.webp", []byte("12345"), "image/webp")
	if err != nil {
		t.Fatal(err)
	}
	if url != "/uploads/logos/abc.webp" || st.plainCalls != 1 {
		t.Fatalf("url=%q plainCalls=%d", url, st.plainCalls)
	}
}
