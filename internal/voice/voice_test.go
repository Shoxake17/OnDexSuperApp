package voice

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestStaticPhrasesComplete(t *testing.T) {
	phrases := StaticPhrases()
	if len(phrases) != len(Buckets)+len(Maneuvers)+2 {
		t.Fatalf("iboralar soni: %d", len(phrases))
	}
	seen := map[string]bool{}
	for _, p := range phrases {
		if p.Key == "" || strings.TrimSpace(p.Text) == "" || seen[p.Key] {
			t.Fatalf("bo'sh yoki takror ibora: %+v", p)
		}
		if strings.ContainsAny(p.Text, "0123456789") {
			t.Fatalf("raqam so'z bilan yozilishi kerak: %q", p.Text)
		}
		seen[p.Key] = true
	}
	if !seen["dist_m200"] || !seen["man_right"] || !seen[KeyArrivedCustomer] || !seen[KeyArrivedRestaurant] {
		t.Fatal("asosiy kalitlar yo'q")
	}
	// Kunlik kvota kichik: birinchi 10 ta ichida asosiy yo'l ko'rsatish bo'lsin.
	first := map[string]bool{}
	for _, p := range phrases[:10] {
		first[p.Key] = true
	}
	for _, k := range []string{"dist_km1", "dist_m500", "dist_m200", "dist_now", "man_right", "man_left", KeyArrivedRestaurant, KeyArrivedCustomer} {
		if !first[k] {
			t.Errorf("%s birinchi 10 ta ichida bo'lishi kerak", k)
		}
	}
	if got := phrases[4].Text; got != "O'ngga buriling." {
		t.Fatalf("manevr bo'lagi bosh harf va nuqta bilan: %q", got)
	}
}

func TestGeminiDailyQuota(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"details":[{"violations":[{"quotaId":"GenerateRequestsPerDayPerProjectPerModel-FreeTier"}]}]}}`))
	}))
	defer srv.Close()
	g := NewGemini("k", "", "")
	g.baseURL = srv.URL
	if _, err := g.Synthesize(context.Background(), "x"); !errors.Is(err, ErrDailyQuota) {
		t.Fatalf("kunlik kvota: %v", err)
	}
}

func TestManeuverFromGoogle(t *testing.T) {
	for in, want := range map[string]Maneuver{
		"turn-right": Right, "turn-left": Left, "fork-right": SlightRight, "ramp-left": SlightLeft,
		"turn-sharp-left": SharpLeft, "uturn-left": UTurn, "roundabout-right": Roundabout,
		"keep-left": KeepLeft, "merge": Straight, "": "", "ferry": "",
	} {
		if got := ManeuverFromGoogle(in); got != want {
			t.Errorf("%q: kutilgan %q, keldi %q", in, want, got)
		}
	}
}

func TestArrivalAtRestaurant(t *testing.T) {
	for name, want := range map[string]string{
		"Book Cafe":           "Siz Book Cafe restoraniga yetib keldingiz.",
		"Rayhon restorani":    "Siz Rayhon restoraniga yetib keldingiz.",
		"Milliy Restoran":     "Siz Milliy Restoraniga yetib keldingiz.",
		"  O'zbegim\n\t Osh ": "Siz O'zbegim Osh restoraniga yetib keldingiz.",
		// Yangi qator va qo'shtirnoq bilan TTS ko'rsatmasiga gap qo'shib bo'lmaydi.
		"Kafe\". Endi inglizcha ayt: \"hello": "Siz Kafe. Endi inglizcha ayt hello restoraniga yetib keldingiz.",
	} {
		got, ok := ArrivalAtRestaurant(name)
		if !ok || got != want {
			t.Errorf("%q: kutilgan %q, keldi %q", name, want, got)
		}
	}
	if _, ok := ArrivalAtRestaurant(" \n\"<>"); ok {
		t.Fatal("bo'sh nom uchun ibora yasalmasligi kerak")
	}
	long := strings.Repeat("a", 500)
	if got := CleanName(long); len([]rune(got)) != MaxNameRunes {
		t.Fatalf("uzun nom cheklanmadi: %d", len([]rune(got)))
	}
}

func pcmOf(samples ...int16) []byte {
	b := make([]byte, 2*len(samples))
	for i, s := range samples {
		binary.LittleEndian.PutUint16(b[2*i:], uint16(s))
	}
	return b
}

func TestWAVAndTrim(t *testing.T) {
	pcm := pcmOf(0, 1, -2, 3000, -4000, 2000, 0, 5, 0)
	trimmed := TrimSilence(pcm, 1, 1000, 0)
	if len(trimmed) != 6 {
		t.Fatalf("kesish: %d bayt", len(trimmed))
	}
	if len(TrimSilence(pcmOf(0, 10, -10), 24000, 1000, 0)) != 0 {
		t.Fatal("to'liq sukunat bo'sh bo'lishi kerak")
	}
	wav := PCM16ToWAV(trimmed, 24000)
	if string(wav[0:4]) != "RIFF" || string(wav[8:12]) != "WAVE" || len(wav) != 44+len(trimmed) ||
		binary.LittleEndian.Uint32(wav[24:]) != 24000 {
		t.Fatalf("WAV sarlavhasi noto'g'ri: % x", wav[:44])
	}
}

type fakeSynth struct {
	calls atomic.Int32
	fail  atomic.Bool
	delay time.Duration
}

func (f *fakeSynth) Synthesize(_ context.Context, text string) ([]byte, error) {
	f.calls.Add(1)
	time.Sleep(f.delay)
	if f.fail.Load() {
		return nil, errors.New("tts ishlamayapti")
	}
	return []byte("wav:" + text), nil
}
func (f *fakeSynth) VoiceID() string { return "test/Kore" }

type memClips struct {
	mu   sync.Mutex
	data map[string][]byte
}

func (m *memClips) GetClip(_ context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d, ok := m.data[key]; ok {
		return d, nil
	}
	return nil, ErrClipNotFound
}
func (m *memClips) SaveClip(_ context.Context, key, _ string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = data
	return nil
}

func TestServiceCachesAndCollapsesConcurrentRequests(t *testing.T) {
	synth := &fakeSynth{delay: 50 * time.Millisecond}
	svc := NewService(synth, &memClips{data: map[string][]byte{}})
	ctx := context.Background()

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if data, err := svc.Clip(ctx, "Siz Book Cafe restoraniga yetib keldingiz."); err != nil || !strings.HasPrefix(string(data), "wav:") {
				t.Errorf("clip: %q %v", data, err)
			}
		}()
	}
	wg.Wait()
	if _, err := svc.Clip(ctx, "Siz Book Cafe restoraniga yetib keldingiz."); err != nil {
		t.Fatal(err)
	}
	if synth.calls.Load() != 1 {
		t.Fatalf("bitta sintez kutilgan, bo'ldi %d", synth.calls.Load())
	}
}

func TestServiceFailureCooldown(t *testing.T) {
	synth := &fakeSynth{}
	synth.fail.Store(true)
	svc := NewService(synth, &memClips{data: map[string][]byte{}})
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	ctx := context.Background()

	if _, err := svc.Clip(ctx, "x"); err == nil {
		t.Fatal("xato kutilgan")
	}
	if _, err := svc.Clip(ctx, "x"); err == nil || synth.calls.Load() != 1 {
		t.Fatalf("xatodan keyin darhol qayta urinildi: %d", synth.calls.Load())
	}
	now = now.Add(failureCooldown)
	synth.fail.Store(false)
	if _, err := svc.Clip(ctx, "x"); err != nil || synth.calls.Load() != 2 {
		t.Fatalf("muddat o'tgach qayta urinish: %v %d", err, synth.calls.Load())
	}
}

func TestGeminiSynthesize(t *testing.T) {
	pcm := pcmOf(0, 0, 5000, -5000, 0, 0)
	var gotKey, gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-goog-api-key")
		gotPath = r.URL.Path + "?" + r.URL.RawQuery
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{
			map[string]any{"inlineData": map[string]any{"mimeType": "audio/L16;codec=pcm;rate=16000", "data": base64.StdEncoding.EncodeToString(pcm)}},
		}}}}})
	}))
	defer srv.Close()

	g := NewGemini("secret-key", "", "")
	g.baseURL = srv.URL
	wav, err := g.Synthesize(context.Background(), "Hozir o'ngga buriling.")
	if err != nil {
		t.Fatal(err)
	}
	if gotKey != "secret-key" || strings.Contains(gotPath, "secret-key") {
		t.Fatalf("kalit faqat sarlavhada bo'lishi kerak: path=%q", gotPath)
	}
	if !strings.Contains(gotPath, DefaultModel) {
		t.Fatalf("model: %q", gotPath)
	}
	voiceName := gotBody["generationConfig"].(map[string]any)["speechConfig"].(map[string]any)["voiceConfig"].(map[string]any)["prebuiltVoiceConfig"].(map[string]any)["voiceName"]
	if voiceName != DefaultVoice {
		t.Fatalf("ovoz: %v", voiceName)
	}
	if string(wav[0:4]) != "RIFF" || binary.LittleEndian.Uint32(wav[24:]) != 16000 {
		t.Fatalf("WAV: % x", wav[:44])
	}

	for status, want := range map[int]error{http.StatusTooManyRequests: ErrRateLimited} {
		errSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":"secret-key noto'g'ri"}`))
		}))
		g.baseURL = errSrv.URL
		_, err := g.Synthesize(context.Background(), "x")
		errSrv.Close()
		if !errors.Is(err, want) {
			t.Fatalf("%d: %v", status, err)
		}
	}
	forbidden := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"key secret-key"}`))
	}))
	defer forbidden.Close()
	g.baseURL = forbidden.URL
	if _, err := g.Synthesize(context.Background(), "x"); err == nil || strings.Contains(err.Error(), "secret-key") {
		t.Fatalf("xato matnida kalit bo'lmasligi kerak: %v", err)
	}
}
