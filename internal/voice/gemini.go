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
	"strings"
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

// ErrNoAudio — javob muvaffaqiyatli (HTTP 200), lekin ichida audio yo'q.
// Xato matnida modelning sababi bor (`finishReason` va model audio o'rniga
// qaytargan matn).
//
// ┌─ NEGA SABAB KO'RSATILADI (2026-09-22) ────────────────────────────┐
// Avval faqat "audio yo'q" deyilardi. "Chapga buriling." va "Keskin
// chapga buriling." bir necha kun ketma-ket shunday yiqildi, har safar
// 3 urinish bilan kunlik 10 so'rovning ko'pini yeb — sababini esa hech
// kim ko'rmadi.
// └───────────────────────────────────────────────────────────────────┘
var ErrNoAudio = errors.New("TTS javobida audio yo'q")

// ErrBlocked — model iborani ATAYLAB rad etdi (xavfsizlik filtri va h.k.).
// Qayta urinish natijani o'zgartirmaydi, faqat kvota yeydi.
var ErrBlocked = errors.New("TTS iborani rad etdi")

// blockedFinish — qayta urinish foydasiz bo'lgan `finishReason` lar.
var blockedFinish = map[string]bool{
	"SAFETY": true, "PROHIBITED_CONTENT": true, "BLOCKLIST": true,
	"SPII": true, "RECITATION": true,
}

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

	var out ttsResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("TTS javobini o'qib bo'lmadi: %w", err)
	}
	for _, c := range out.Candidates {
		for _, p := range c.Content.Parts {
			if p.InlineData != nil && p.InlineData.Data != "" {
				return decodeWAV(p.InlineData.MimeType, p.InlineData.Data)
			}
		}
	}
	return nil, out.noAudioErr()
}

// ttsResponse — `generateContent` javobining bizga kerakli qismi.
type ttsResponse struct {
	Candidates []struct {
		FinishReason string `json:"finishReason"`
		Content      struct {
			Parts []struct {
				Text       string `json:"text"`
				InlineData *struct {
					MimeType string `json:"mimeType"`
					Data     string `json:"data"`
				} `json:"inlineData"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	PromptFeedback *struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
}

// decodeWAV — base64 PCM ni WAV ga aylantiradi (oldi-ortidagi sukunat kesiladi).
func decodeWAV(mimeType, data string) ([]byte, error) {
	pcm, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, fmt.Errorf("TTS audio buzilgan: %w", err)
	}
	rate := 24000
	if m := rateRe.FindStringSubmatch(mimeType); m != nil {
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

// noAudioErr — audio yo'q javob: model NEGA bermaganini xatoga qo'shadi.
func (r *ttsResponse) noAudioErr() error {
	var finish, said []string
	base := ErrNoAudio
	for _, c := range r.Candidates {
		if c.FinishReason != "" {
			finish = append(finish, c.FinishReason)
			if blockedFinish[c.FinishReason] {
				base = ErrBlocked
			}
		}
		for _, p := range c.Content.Parts {
			if t := strings.TrimSpace(p.Text); t != "" {
				said = append(said, t)
			}
		}
	}
	block := ""
	if r.PromptFeedback != nil && r.PromptFeedback.BlockReason != "" {
		block = r.PromptFeedback.BlockReason
		base = ErrBlocked
	}
	return fmt.Errorf("%w (finishReason=%s, blockReason=%s, model matni=%q)",
		base, orDash(strings.Join(finish, ",")), orDash(block), truncate(strings.Join(said, " "), 120))
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// truncate — xato matnini qisqartiradi (rune bo'yicha, harf bo'linmasin).
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
