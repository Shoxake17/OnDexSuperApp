package voice

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

const (
	// DefaultModel / DefaultVoice — egasi tanlagan ovoz (2026-09-15, "Kore").
	DefaultModel = "gemini-2.5-flash-preview-tts"
	DefaultVoice = "Kore"

	maxResponseBytes = 32 << 20
)

// ErrRateLimited — TTS so'rovlar chegarasi (HTTP 429); keyinroq qayta urinish kerak.
var ErrRateLimited = errors.New("TTS so'rovlar chegarasiga yetildi")

// ErrDailyQuota — KUNLIK kvota tugadi (bepul tarifda kuniga 10 so'rov).
// Qayta urinish foydasiz: ertasi kunni kutish yoki billing kerak.
var ErrDailyQuota = errors.New("TTS kunlik kvotasi tugadi")

// instruction — o'qish uslubi. Egasi tinglab tanlagan namuna shu ko'rsatma
// bilan yasalgan; o'zgartirilsa ovoz ohangi ham o'zgaradi.
const instruction = "Aniq, xotirjam va do'stona ohangda o'zbek tilida ayt: "

// Gemini — Gemini TTS (REST `generateContent`, javob PCM 16-bit).
//
// Kalit faqat so'rov SARLAVHASIDA (`x-goog-api-key`) — manzilga
// yozilmaydi, ya'ni xato matni yoki log orqali sizmaydi.
type Gemini struct {
	apiKey    string
	model     string
	voiceName string
	baseURL   string
	client    *http.Client
}

func NewGemini(apiKey, model, voiceName string) *Gemini {
	if model == "" {
		model = DefaultModel
	}
	if voiceName == "" {
		voiceName = DefaultVoice
	}
	return &Gemini{
		apiKey:    apiKey,
		model:     model,
		voiceName: voiceName,
		baseURL:   "https://generativelanguage.googleapis.com/v1beta",
		client:    &http.Client{Timeout: 90 * time.Second},
	}
}

// VoiceID — kesh kalitining bir qismi: ovoz yoki model o'zgarsa eski
// yozuvlar ishlatilmaydi.
func (g *Gemini) VoiceID() string { return g.model + "/" + g.voiceName }

var rateRe = regexp.MustCompile(`rate=(\d+)`)

// Synthesize — matnni WAV ga aylantiradi (oldi-ortidagi sukunat kesilgan).
func (g *Gemini) Synthesize(ctx context.Context, text string) ([]byte, error) {
	body, err := json.Marshal(map[string]any{
		"contents": []any{map[string]any{"parts": []any{map[string]any{"text": instruction + text}}}},
		"generationConfig": map[string]any{
			"responseModalities": []string{"AUDIO"},
			"speechConfig": map[string]any{
				"voiceConfig": map[string]any{
					"prebuiltVoiceConfig": map[string]any{"voiceName": g.voiceName},
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	endpoint := g.baseURL + "/models/" + url.PathEscape(g.model) + ":generateContent"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", g.apiKey)
	resp, err := g.client.Do(req) //nolint:gosec // G704 emas: manzil kod ichidagi Google hostidan quriladi
	if err != nil {
		return nil, fmt.Errorf("TTS so'rovi bajarilmadi: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, err
	}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		// Javobdagi `quotaId` (masalan `GenerateRequestsPerDayPerProject...`).
		if bytes.Contains(raw, []byte("PerDay")) {
			return nil, ErrDailyQuota
		}
		return nil, ErrRateLimited
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("TTS javobi: HTTP %d", resp.StatusCode)
	}

	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					InlineData *struct {
						MimeType string `json:"mimeType"`
						Data     string `json:"data"`
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("TTS javobini o'qib bo'lmadi: %w", err)
	}
	for _, c := range out.Candidates {
		for _, p := range c.Content.Parts {
			if p.InlineData == nil || p.InlineData.Data == "" {
				continue
			}
			pcm, err := base64.StdEncoding.DecodeString(p.InlineData.Data)
			if err != nil {
				return nil, fmt.Errorf("TTS audio buzilgan: %w", err)
			}
			rate := 24000
			if m := rateRe.FindStringSubmatch(p.InlineData.MimeType); m != nil {
				if v, err := strconv.Atoi(m[1]); err == nil && v >= 8000 && v <= 48000 {
					rate = v
				}
			}
			pcm = TrimSilence(pcm, rate, 400, 120*time.Millisecond)
			if len(pcm) == 0 {
				return nil, errors.New("TTS bo'sh audio qaytardi")
			}
			return PCM16ToWAV(pcm, rate), nil
		}
	}
	return nil, errors.New("TTS javobida audio yo'q")
}
