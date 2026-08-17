// Package telegram — OTP kodini Telegram bot orqali yetkazish.
//
// ── NEGA DEEP LINK KERAK ───────────────────────────────────────────
// Telegram Bot API'da "shu telefon raqamiga xabar yubor" degan amal
// UMUMAN YO'Q. Bot faqat `chat_id` ga yoza oladi, `chat_id` esa
// foydalanuvchi botga BIRINCHI bo'lib yozgandan keyin paydo bo'ladi.
// Bu spamning oldini olish uchun ataylab shunday.
//
// Shu sabab oqim teskari yo'nalishda quriladi:
//
//	1. ilova backenddan bir martalik TOKEN oladi;
//	2. `t.me/<bot>?start=<token>` havolasi ochiladi;
//	3. foydalanuvchi Start bosadi -> bot tokenni ko'radi va endi
//	   `chat_id` ni biladi;
//	4. bot RAQAMNI ULASHISHNI so'raydi;
//	5. raqam mos kelsa — kod o'sha chatga yuboriladi.
//
// ── 4-QADAM NEGA MAJBURIY (eng muhim joyi) ─────────────────────────
// Deep link o'zi telefon raqamini TASDIQLAMAYDI — u faqat "kimdir
// botni ochdi" degan ma'noni beradi. Busiz hujum juda oson bo'lardi:
//
//	hujumchi ilovada QURBONNING raqamini yozadi ->
//	botni O'ZINING Telegramida ochadi -> kodni oladi ->
//	qurbonning raqami bilan ro'yxatdan o'tadi.
//
// `request_contact` tugmasi bosilganda Telegram raqamni O'ZI yuboradi
// (foydalanuvchi uni qo'lda yoza olmaydi va o'zgartira olmaydi).
// Backend uni ilovada kiritilgan raqam bilan solishtiradi — mos
// kelmasa kod UMUMAN yuborilmaydi. Shundagina bu yo'l SMS bilan teng
// kuchda bo'ladi.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client — Telegram Bot API ustidagi minimal klient.
//
// Tashqi kutubxona ataylab olinmadi: bizga bor-yo'g'i uchta metod
// kerak (`getMe`, `getUpdates`, `sendMessage`), to'liq SDK esa yana
// bitta bog'liqlik va yana bitta yangilanadigan yuza demakdir.
type Client struct {
	token string
	http  *http.Client
}

// ErrConflict — Telegram bitta botni FAQAT bitta jarayon polling
// qilishiga ruxsat beradi.
//
// ┌─ NEGA ALOHIDA XATO ────────────────────────────────────────────────┐
// Ikkita server (masalan lokal dev va production) bir xil
// `TELEGRAM_BOT_TOKEN` bilan ishlasa, ikkalasi ham `getUpdates`
// chaqiradi va Telegram navbat bilan ikkalasiga 409 qaytaradi.
//
// Oqibati foydalanuvchida ko'rinadi: "Telegram bilan kirish" bosgan
// odam botga o'tadi, Start bosadi — lekin uning yangiligi qaysi
// serverga tushishi TASODIFIY bo'ladi. Yarmi holatda kirish
// yakunlanmaydi va u "havola eskirgan" xabarini oladi.
//
// Ilgari bu oddiy tarmoq xatosi bilan bir xil loglanardi
// ("getUpdates xatosi") va sababini topish soatlab vaqt olgan edi.
// Endi log xatoning O'ZIDA yechimni aytadi.
// └────────────────────────────────────────────────────────────────────┘
var ErrConflict = errors.New("telegram: botni boshqa jarayon ham polling qilyapti")

// classifyError — Telegram'ning `description` matnini tipiga ajratadi.
//
// Alohida funksiya, chunki `call` tarmoqqa bog'liq va uni testdan
// chaqirib bo'lmaydi — bu esa mantiqning O'ZI (aynan nima 409 deb
// hisoblanadi) tekshirilishi mumkin bo'lgan yagona joy.
func classifyError(description string) error {
	// Telegram matni: "Conflict: terminated by other getUpdates request;
	// make sure that only one bot instance is running".
	if strings.Contains(description, "Conflict") {
		return fmt.Errorf("%w: %s", ErrConflict, description)
	}
	return fmt.Errorf("telegram: %s", description)
}

func NewClient(token string) *Client {
	return &Client{
		token: strings.TrimSpace(token),
		// Long-polling `getUpdates` uzoq kutadi — timeout undan
		// KATTA bo'lishi shart, aks holda har so'rov uzilib turadi.
		http: &http.Client{Timeout: 70 * time.Second},
	}
}

func (c *Client) Configured() bool { return c.token != "" }

func (c *Client) call(ctx context.Context, method string, payload any, out any) error {
	if !c.Configured() {
		return errors.New("telegram: bot token sozlanmagan")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	endpoint := "https://api.telegram.org/bot" + c.token + "/" + method
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	var envelope struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("telegram: javobni o'qib bo'lmadi: %w", err)
	}
	if !envelope.OK {
		// TOKEN LOG QILINMAYDI — xato matnida ham u bo'lmaydi, chunki
		// u faqat URL'da edi.
		//
		// 409 ALOHIDA ajratiladi: uning sababi mutlaqo boshqacha va
		// yechimi ham boshqa (pastdagi ErrConflict izohiga qarang).
		// Aks holda u boshqa tarmoq xatolari bilan bir xil ko'rinardi.
		return classifyError(envelope.Description)
	}
	if out != nil {
		return json.Unmarshal(envelope.Result, out)
	}
	return nil
}

// BotUsername — deep link qurish uchun (`t.me/<username>?start=...`).
func (c *Client) BotUsername(ctx context.Context) (string, error) {
	var me struct {
		Username string `json:"username"`
	}
	if err := c.call(ctx, "getMe", map[string]any{}, &me); err != nil {
		return "", err
	}
	return me.Username, nil
}

// SendMessage — oddiy matnli xabar.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	return c.call(ctx, "sendMessage", map[string]any{
		"chat_id": chatID,
		"text":    text,
	}, nil)
}

// AskContact — "Raqamni ulashish" tugmasi bilan so'rov.
//
// `request_contact: true` — Telegram raqamni O'ZI yuboradi; bu
// yagona ishonchli tasdiq (paket izohiga qarang).
func (c *Client) AskContact(ctx context.Context, chatID int64, text string) error {
	return c.call(ctx, "sendMessage", map[string]any{
		"chat_id": chatID,
		"text":    text,
		"reply_markup": map[string]any{
			"keyboard": []any{
				[]any{map[string]any{
					"text":            "📱 Raqamni ulashish",
					"request_contact": true,
				}},
			},
			"resize_keyboard":   true,
			"one_time_keyboard": true,
		},
	}, nil)
}

// RemoveKeyboard — tasdiqdan keyin tugmani olib tashlaydi.
func (c *Client) RemoveKeyboard(ctx context.Context, chatID int64, text string) error {
	return c.call(ctx, "sendMessage", map[string]any{
		"chat_id":      chatID,
		"text":         text,
		"reply_markup": map[string]any{"remove_keyboard": true},
	}, nil)
}

// SendReturnLink — "ilovaga qaytish" tugmasi bilan xabar.
//
// NEGA TUGMA: Android 10 dan boshlab fondagi ilova O'ZINI oldinga
// chiqara olmaydi, ya'ni "butunlay avtomatik qaytish" mumkin emas.
// Eng yaxshi variant — foydalanuvchi ilovalar orasida qidirmasin,
// tugma ko'z oldida tursin.
//
// DIQQAT: `url` FAQAT `http(s)://` yoki `tg://` bo'lishi mumkin —
// Telegramning qat'iy cheklovi. Boshqa sxema (masalan `ondex://`)
// yuborilsa butun so'rov `BUTTON_URL_INVALID` bilan rad etiladi.
// Shu sabab chaqiruvchi HTTP manzil beradi va yo'naltirishni server
// bajaradi (`verifier.go` dagi `returnPath` izohiga qarang).
//
// XATO ENDI YUTILMAYDI: avval bu funksiya rad etilganda jimgina
// tugmasiz xabarga qaytardi va natijada tugma HECH QACHON
// ko'rinmasdi — buni sezish uchun hech qanday belgi yo'q edi.
// Chaqiruvchi xatoni ko'rib, log yozadi va foydalanuvchiga aniq
// xabar beradi.
func (c *Client) SendReturnLink(ctx context.Context, chatID int64, text, url, buttonLabel string) error {
	return c.call(ctx, "sendMessage", map[string]any{
		"chat_id": chatID,
		"text":    text,
		"reply_markup": map[string]any{
			"inline_keyboard": []any{
				[]any{map[string]any{"text": buttonLabel, "url": url}},
			},
		},
	}, nil)
}

// Update — bizga kerak bo'lgan maydonlar (Telegram javobi ancha katta).
type Update struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text    string `json:"text"`
		Contact *struct {
			PhoneNumber string `json:"phone_number"`
			UserID      int64  `json:"user_id"`
		} `json:"contact"`
		From *struct {
			ID int64 `json:"id"`
		} `json:"from"`
	} `json:"message"`
}

// GetUpdates — long polling.
//
// NEGA WEBHOOK EMAS: webhook uchun OMMAVIY HTTPS manzil kerak,
// bizning server esa hozir lokal (`adb reverse` orqali). Long polling
// bunday cheklovsiz ishlaydi va production'da ham to'liq qo'llab-
// quvvatlanadi.
func (c *Client) GetUpdates(ctx context.Context, offset int64, timeoutSec int) ([]Update, error) {
	var updates []Update
	err := c.call(ctx, "getUpdates", map[string]any{
		"offset":  offset,
		"timeout": timeoutSec,
		// Faqat kerakli turlar — qolganini Telegram yubormaydi.
		"allowed_updates": []string{"message"},
	}, &updates)
	return updates, err
}

// DeepLink — `t.me/<bot>?start=<token>`.
func DeepLink(botUsername, token string) string {
	return "https://t.me/" + url.PathEscape(botUsername) + "?start=" + url.QueryEscape(token)
}
