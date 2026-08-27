package model3d

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// TripoClient — Tripo AI (tripo3d.ai) OpenAPI klienti.
//
// ┌─ PROVAYDERGA XOS HAMMA NARSA SHU FAYLDA ──────────────────────────┐
// `Service` bu turni KO'RMAYDI — u faqat `Generator` interfeysi bilan
// ishlaydi. Provayder almashsa yoki API versiyasi o'zgarsa, o'zgarish
// shu fayldan chiqmaydi.
// └───────────────────────────────────────────────────────────────────┘
//
// API shakli (v2 OpenAPI):
//
//	POST {base}/v2/openapi/task        — vazifa ochish  → data.task_id
//	GET  {base}/v2/openapi/task/{id}   — holat         → data.status, data.output.model
//
// Autentifikatsiya: `Authorization: Bearer <API_KEY>`.
type TripoClient struct {
	apiKey  string
	baseURL string
	// modelVersion — bo'sh bo'lsa Tripo o'z standartini tanlaydi.
	// `.env` orqali sozlanadi: yangi versiya chiqqanda kod
	// o'zgartirilmaydi.
	modelVersion string
}

// DefaultTripoBaseURL — provayderning rasmiy manzili.
const DefaultTripoBaseURL = "https://api.tripo3d.ai"

func NewTripoClient(apiKey, baseURL, modelVersion string) *TripoClient {
	return &TripoClient{
		apiKey:       strings.TrimSpace(apiKey),
		baseURL:      normalizeBaseURL(baseURL),
		modelVersion: strings.TrimSpace(modelVersion),
	}
}

// normalizeBaseURL — `.env` dagi qiymatni ISHLAYDIGAN manzilga
// keltiradi.
//
// ┌─ NEGA SHUNCHA HIMOYA ─────────────────────────────────────────────┐
// Bu maydon IXTIYORIY va odatda bo'sh qoldiriladi. Lekin uni qo'lda
// to'ldirishda sxemani tushirib qoldirish juda oson ("tripo3d.ai") va
// natijada Go quyidagi tushunarsiz xatoni beradi:
//
//	Post "tripo3d.ai/v2/openapi/task": unsupported protocol scheme ""
//
// Xato tarmoq qatlamidan chiqadi, ya'ni sabab "sozlamada sxema yo'q"
// ekani hech qayerdan ko'rinmaydi. Shuning uchun:
//   - bo'sh bo'lsa           → rasmiy manzil;
//   - sxemasiz bo'lsa        → oldiga `https://` qo'yiladi;
//   - `http://` bo'lsa       → RAD ETILADI (kalit ochiq ketardi);
//   - butunlay yaroqsiz bo'lsa → rasmiy manzilga qaytiladi.
//
// Har bir tuzatish JURNALGA yoziladi — sozlama jimgina "tuzalib"
// qolmasin, odam xatosini bilsin.
// └───────────────────────────────────────────────────────────────────┘
func normalizeBaseURL(raw string) string {
	v := strings.TrimRight(strings.TrimSpace(raw), "/")
	if v == "" {
		return DefaultTripoBaseURL
	}
	if strings.HasPrefix(strings.ToLower(v), "http://") {
		slog.Warn("TRIPO_BASE_URL http:// bilan berilgan — API kaliti ochiq ketmasligi uchun rasmiy https manzil ishlatiladi",
			"berilgan", v)
		return DefaultTripoBaseURL
	}
	if !strings.HasPrefix(strings.ToLower(v), "https://") {
		slog.Warn("TRIPO_BASE_URL da sxema yo'q — https:// qo'shildi", "berilgan", v)
		v = "https://" + v
	}
	u, err := url.Parse(v)
	// Host ALOHIDA tekshiriladi: `url.Parse` juda kechirimli va
	// "https://://buzilgan" kabi satrlarni ham xatosiz qabul qiladi.
	// Faqat haqiqiy host shakli (harf/raqam/nuqta/defis, ixtiyoriy
	// port) o'tkaziladi.
	if err != nil || !validHost.MatchString(u.Host) {
		slog.Warn("TRIPO_BASE_URL yaroqsiz — rasmiy manzil ishlatiladi", "berilgan", raw)
		return DefaultTripoBaseURL
	}
	return v
}

var validHost = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?(:[0-9]{1,5})?$`)

func (c *TripoClient) Name() string { return "tripo" }

// tripoEnvelope — Tripo javoblarining umumiy qobig'i.
type tripoEnvelope[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type tripoTaskCreated struct {
	TaskID string `json:"task_id"`
}

type tripoTaskState struct {
	TaskID   string `json:"task_id"`
	Status   string `json:"status"`
	Progress int    `json:"progress"`
	Output   struct {
		Model    string `json:"model"`
		PBRModel string `json:"pbr_model"`
	} `json:"output"`
}

// Submit — rasm havolasidan `image_to_model` vazifasini ochadi.
//
// Rasm URL bilan uzatiladi (fayl yuklash emas): rasm allaqachon
// bizning ochiq R2 omborimizda turadi, uni ikkinchi marta tarmoq
// orqali haydashning ma'nosi yo'q.
func (c *TripoClient) Submit(ctx context.Context, imageURL string) (string, error) {
	if c.apiKey == "" {
		return "", errors.New("TRIPO_API_KEY sozlanmagan")
	}
	// Bizning havolamiz bo'lsa ham tekshiramiz: `ImageStore` sozlamasi
	// noto'g'ri bo'lsa (masalan lokal disk rejimida `http://localhost`)
	// tashqi xizmatga ishlamaydigan havola ketardi va xato faqat bir
	// necha daqiqadan keyin, tushunarsiz shaklda chiqardi.
	if _, err := safeHTTPSURL(imageURL); err != nil {
		return "", fmt.Errorf("rasm havolasi tashqi xizmat uchun yaroqsiz: %w", err)
	}

	body := map[string]any{
		"type": "image_to_model",
		"file": map[string]any{
			// Tripo URL orqali JPEG/PNG qabul qiladi. `Service` shuning
			// uchun WebP dan JPEG nusxa tayyorlab beradi.
			"type": "jpg",
			"url":  imageURL,
		},
	}
	if c.modelVersion != "" {
		body["model_version"] = c.modelVersion
	}

	var out tripoEnvelope[tripoTaskCreated]
	if err := c.do(ctx, http.MethodPost, "/v2/openapi/task", body, &out); err != nil {
		return "", err
	}
	if out.Data.TaskID == "" {
		return "", fmt.Errorf("provayder vazifa ID bermadi (code=%d %s)", out.Code, out.Message)
	}
	return out.Data.TaskID, nil
}

// Fetch — vazifa holatini so'raydi.
func (c *TripoClient) Fetch(ctx context.Context, taskID string) (TaskResult, error) {
	var out tripoEnvelope[tripoTaskState]
	if err := c.do(ctx, http.MethodGet, "/v2/openapi/task/"+taskID, nil, &out); err != nil {
		return TaskResult{}, err
	}

	switch strings.ToLower(out.Data.Status) {
	case "success":
		// `pbr_model` — teksturali variant, mavjud bo'lsa afzal:
		// taom uchun tekstura shakldan ko'ra muhimroq.
		url := out.Data.Output.PBRModel
		if url == "" {
			url = out.Data.Output.Model
		}
		if url == "" {
			return TaskResult{Status: TaskFailed, Err: "javobda model havolasi yo'q"}, nil
		}
		return TaskResult{Status: TaskDone, ModelURL: url}, nil

	case "failed", "banned", "expired", "cancelled":
		return TaskResult{Status: TaskFailed, Err: out.Data.Status}, nil

	case "queued", "running":
		return TaskResult{Status: TaskRunning}, nil

	default:
		// Noma'lum holat — vazifani o'ldirmaymiz, kuzatishda davom
		// etamiz. Umumiy kutish chegarasi (`PollTimeout`) baribir
		// cheksiz osilishga yo'l qo'ymaydi.
		return TaskResult{Status: TaskRunning}, nil
	}
}

func (c *TripoClient) do(ctx context.Context, method, path string, body any, out any) error {
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}

	req, err := newRequest(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Javob hajmi cheklanadi: buzilgan/dushman javob xotirani
	// to'ldirmasin (JSON qobig'i uchun 1MB dan ko'p kerak emas).
	dec := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// Xato matnini o'qishga urinamiz — jurnalda sabab ko'rinsin.
		var e tripoEnvelope[json.RawMessage]
		_ = dec.Decode(&e)
		msg := e.Message
		if msg == "" {
			msg = resp.Status
		}
		// ┌─ AJRATILADIGAN HOLATLAR ──────────────────────────────────┐
		// Ba'zi xatolar "keyinroq urinib ko'ring" TOIFASIGA KIRMAYDI
		// va foydalanuvchiga aniq aytilishi kerak. Ular sentinel
		// xatoga o'raladi, HTTP qatlami esa mos kod tanlaydi.
		//
		// Tripo `code` maydonini beradi (2010 — kredit yetishmasligi).
		// Kod o'zgarib ketishi mumkin bo'lgani uchun MATN bo'yicha ham
		// tekshiriladi — ikkalasidan biri yetadi.
		// └───────────────────────────────────────────────────────────┘
		//
		// Sentinel xato YALANG'OCH qaytariladi: uning matni
		// foydalanuvchi uchun yozilgan va ustiga inglizcha provayder
		// jumlasi qo'shilsa, restoran xodimi uchun faqat shovqin
		// bo'lardi. Provayder tafsiloti JURNALGA yoziladi.
		if e.Code == 2010 || strings.Contains(strings.ToLower(msg), "credit") {
			slog.Warn("tripo: kredit yetishmadi", "code", e.Code, "xabar", msg)
			return ErrNoCredit
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			slog.Warn("tripo: tezlik chegarasi", "xabar", msg)
			return ErrProviderBusy
		}
		// API kaliti javobga TUSHMAYDI: xabar faqat provayder matni.
		return fmt.Errorf("tripo: %s (HTTP %d)", msg, resp.StatusCode)
	}
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("tripo javobi o'qilmadi: %w", err)
	}
	return nil
}
