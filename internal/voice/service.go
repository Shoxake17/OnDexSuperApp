package voice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// ErrClipNotFound — ibora keshda yo'q.
var ErrClipNotFound = errors.New("ovoz fayli keshda yo'q")

// MaxClipBytes — bitta ovoz faylining eng ko'p hajmi (bazaga ham shu chegara).
const MaxClipBytes = 4 << 20

// failureCooldown — sintez muvaffaqiyatsiz bo'lsa shu vaqt qayta urinilmaydi
// (TTS ishlamay qolganda har so'rov pullik xizmatga urilmasin).
const failureCooldown = 30 * time.Second

// Synthesizer — matnni WAV ga aylantiruvchi (production'da `Gemini`).
type Synthesizer interface {
	Synthesize(ctx context.Context, text string) ([]byte, error)
	VoiceID() string
}

// ClipStore — tayyor ovoz fayllari ombori (Postgres yoki xotira).
type ClipStore interface {
	// GetClip — topilmasa `ErrClipNotFound`.
	GetClip(ctx context.Context, key string) ([]byte, error)
	// SaveClip — birinchi yozuv qoladi.
	SaveClip(ctx context.Context, key, text string, data []byte) error
}

// Service — iborani bir marta sintez qilib saqlaydi va qayta beradi.
type Service struct {
	synth Synthesizer
	store ClipStore

	mu       sync.Mutex
	inflight map[string]*flight
	failed   map[string]failure
	now      func() time.Time
}

type flight struct {
	done chan struct{}
	data []byte
	err  error
}

type failure struct {
	at  time.Time
	err error
}

func NewService(synth Synthesizer, store ClipStore) *Service {
	return &Service{
		synth:    synth,
		store:    store,
		inflight: map[string]*flight{},
		failed:   map[string]failure{},
		now:      time.Now,
	}
}

// ClipKey — kesh kaliti: ovoz va matn bo'yicha (matnning o'zi kalitda emas).
func ClipKey(voiceID, text string) string {
	sum := sha256.Sum256([]byte("v1\x00" + voiceID + "\x00" + text))
	return hex.EncodeToString(sum[:])
}

// Clip — matn uchun WAV: keshdan, bo'lmasa sintez qilib saqlaydi.
//
// Bir xil matn uchun bir vaqtda kelgan so'rovlar BITTA sintezni kutadi.
// Sintez so'rovchi kontekstidan mustaqil (60 s): kuryer kutmay chiqib ketsa
// ham natija keshga tushadi va keyingi safar darhol beriladi.
func (s *Service) Clip(ctx context.Context, text string) ([]byte, error) {
	key := ClipKey(s.synth.VoiceID(), text)
	data, err := s.store.GetClip(ctx, key)
	if err == nil {
		return data, nil
	}
	if !errors.Is(err, ErrClipNotFound) {
		return nil, err
	}

	s.mu.Lock()
	if f, ok := s.failed[key]; ok && s.now().Sub(f.at) < failureCooldown {
		s.mu.Unlock()
		return nil, f.err
	}
	if f, ok := s.inflight[key]; ok {
		s.mu.Unlock()
		select {
		case <-f.done:
			return f.data, f.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	f := &flight{done: make(chan struct{})}
	s.inflight[key] = f
	s.mu.Unlock()

	sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 60*time.Second)
	defer cancel()
	data, err = s.synth.Synthesize(sctx, text)
	if err == nil && len(data) > MaxClipBytes {
		data, err = nil, errors.New("ovoz fayli juda katta")
	}
	if err == nil {
		if serr := s.store.SaveClip(sctx, key, text, data); serr != nil {
			slog.Warn("voice: ovoz faylini saqlab bo'lmadi", "err", serr)
		}
	}

	s.mu.Lock()
	f.data, f.err = data, err
	delete(s.inflight, key)
	if err != nil {
		s.failed[key] = failure{at: s.now(), err: err}
	} else {
		delete(s.failed, key)
	}
	for k, v := range s.failed {
		if s.now().Sub(v.at) >= failureCooldown {
			delete(s.failed, k)
		}
	}
	s.mu.Unlock()
	close(f.done)
	return data, err
}
