package assistant

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

// Shaddiy — til modeli provayderi (`LLM` interfeysining
// implementatsiyasi).
//
// ┌─ SHADDIY NIMANI KO'RADI, NIMANI KO'RMAYDI ─────────────────────────┐
// KO'RADI: suhbat matni, tool ta'riflari va tool NATIJALARI
// (menyu — allaqachon ochiq ma'lumot; buyurtma holati).
//
// KO'RMAYDI: telefon raqami, manzil, ism, to'lov ma'lumoti, boshqa
// foydalanuvchilar. Bular tool javoblariga UMUMAN qo'shilmaydi
// (`tools.go` ga qarang) — ya'ni maxfiylik "so'ramaymiz" degan
// va'da bilan emas, ma'lumotni JO'NATMASLIK bilan ta'minlanadi.
//
// `user_ref` — foydalanuvchining PSEVDONIMI (ichki ID emas). U faqat
// Shaddiy tomonidagi tezlik cheklovi uchun; qaytib bizning tizimga
// yo'l bermaydi. Qarang: `pseudonym`.
// └────────────────────────────────────────────────────────────────────┘
type Shaddiy struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// NewShaddiyFromEnv — `.env` dan o'qiydi.
//
// Ikkala qiymat ham bo'lmasa `nil` qaytadi va yordamchi BUTUNLAY
// o'chadi (503). Bu R2/Telegram/Tripo bilan bir xil falsafa:
// sozlanmagan funksiya jimgina "yarim ishlaydigan" holatda
// qolmaydi.
//
// ⚠️ `SHADDIY_API_KEY` FAQAT shu serverda turadi. U mijoz
// ilovasiga hech qachon yuborilmaydi — ilova o'z JWT'si bilan
// OnDex'ga murojaat qiladi, Shaddiy'ga esa OnDex serveri boradi.
func NewShaddiyFromEnv() (*Shaddiy, bool) {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("SHADDIY_AI_URL")), "/")
	key := strings.TrimSpace(os.Getenv("SHADDIY_API_KEY"))
	if base == "" || key == "" {
		return nil, false
	}
	return &Shaddiy{
		baseURL: base,
		apiKey:  key,
		// 35s — provayder o'zi 30s kutadi, ustiga tarmoq vaqti.
		// Cheksiz kutish HTTP handler'ni bog'lab qo'yardi.
		http: &http.Client{Timeout: 35 * time.Second},
	}, true
}

type completeRequest struct {
	Messages []Message `json:"messages"`
	Tools    []Tool    `json:"tools,omitempty"`
	UserRef  string    `json:"user_ref,omitempty"`
}

type completeResponse struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error"`
	Message struct {
		Content   string     `json:"content"`
		ToolCalls []ToolCall `json:"tool_calls"`
	} `json:"message"`
}

// pseudonym — foydalanuvchi ID'sining tashqariga chiqadigan shakli.
//
// ┌─ NEGA ICHKI ID YUBORILMAYDI ───────────────────────────────────────┐
// Shaddiy'ga `user_ref` FAQAT uning tezlik cheklovi uchun kerak: unga
// barqaror, lekin MA'NOSIZ satr yetarli. Ichki ID esa bizning
// tizimimizda hamma joyda uchraydi — buyurtma yozuvlari, qo'llab-
// quvvatlash murojaatlari, `/agent/v1` grantlari, loglar. Uni tashqi
// provayderga o'z holicha berish shu barcha manbalarni BIR-BIRIGA
// BOG'LASH imkonini ochadi (provayder buzilsa yoki loglari oshkor
// bo'lsa).
//
// HMAC-SHA256 kaliti sifatida `SHADDIY_API_KEY` ishlatiladi: u
// allaqachon FAQAT shu serverda turadi va yordamchi yoqilgan bo'lsa
// har doim mavjud — ya'ni yangi majburiy sozlama qo'shilmaydi.
// Provayder tomonda natija barqaror (bir foydalanuvchi — bir satr),
// lekin teskari yo'nalishda hech narsa bermaydi.
//
// Kalit almashtirilsa psevdonimlar ham yangilanadi — bu faqat tezlik
// cheklovi hisoblagichini nolga tushiradi, boshqa oqibati yo'q.
// └────────────────────────────────────────────────────────────────────┘
func (s *Shaddiy) pseudonym(userID string) string {
	if userID == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(s.apiKey))
	mac.Write([]byte("ondex:assistant:user:" + userID))
	// 128 bit — to'qnashuv amalda imkonsiz, satr esa qisqa.
	return hex.EncodeToString(mac.Sum(nil)[:16])
}

// Complete — suhbatning keyingi qadami.
//
// `userRef` bu yerga ICHKI ID sifatida keladi va tashqariga
// psevdonim ko'rinishida chiqadi (`pseudonym` izohiga qarang).
func (s *Shaddiy) Complete(ctx context.Context, userRef string,
	messages []Message, tools []Tool) (*Reply, error) {

	body, err := json.Marshal(completeRequest{
		Messages: messages, Tools: tools, UserRef: s.pseudonym(userRef),
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.baseURL+"/api/ai/complete", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.http.Do(req)
	if err != nil {
		// Tarmoq xatosi matnida host/port bo'ladi — u
		// foydalanuvchiga chiqmasligi kerak (`httpError` 5xx ni
		// yashiradi, lekin bu xato 502 sifatida ham ma'noli
		// bo'lishi kerak).
		slog.Error("assistant: Shaddiy'ga ulanib bo'lmadi", "err", err)
		return nil, errors.New("yordamchi hozir javob bera olmadi")
	}
	defer resp.Body.Close()

	var out completeResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		slog.Error("assistant: javobni o'qib bo'lmadi", "status", resp.StatusCode, "err", err)
		return nil, errors.New("yordamchi hozir javob bera olmadi")
	}
	if resp.StatusCode >= 400 || !out.OK {
		msg := strings.TrimSpace(out.Error)
		if msg == "" {
			msg = fmt.Sprintf("xato %d", resp.StatusCode)
		}
		slog.Warn("assistant: Shaddiy xato qaytardi", "status", resp.StatusCode, "err", msg)
		// 429 — foydalanuvchi juda tez yozyapti; boshqasi —
		// vaqtinchalik nosozlik. Ikkalasida ham matn ma'noli.
		return nil, errors.New(msg)
	}
	return &Reply{Content: out.Message.Content, ToolCalls: out.Message.ToolCalls}, nil
}
