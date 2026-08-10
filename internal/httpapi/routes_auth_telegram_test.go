package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"chustapp/internal/storage"
	"chustapp/internal/telegram"
	"chustapp/internal/users"
)

// Bu fayl `GET /auth/telegram/login/status` ni HTTP DARAJASIDA sinaydi.
//
// NEGA ALOHIDA KERAK: `internal/telegram` testlari `ConsumeLogin` ni
// TO'G'RIDAN-TO'G'RI chaqiradi va handler mantig'iga umuman tegmaydi.
// Aynan shu bo'shliq tufayli handlerda "kalit bo'sh bo'lsa har doim
// pending" degan tekshiruv qolib ketgan edi va dev rejimida
// "Telegram bilan kirish" butunlay ishlamay qolgandi (ilova cheksiz
// yuklanib turardi). Testlar yashil edi, chunki hech biri HTTP orqali
// o'tmasdi.

// fakeBot — `telegram` paketidagi `botAPI` ni qanoatlantiradi.
// Interfeys turi eksport qilinmagan, lekin METODLARI eksport qilingan,
// shuning uchun uni tashqi paketdan ham amalga oshirish mumkin.
type fakeBot struct {
	updates chan telegram.Update
}

func (f *fakeBot) Configured() bool { return true }
func (f *fakeBot) BotUsername(context.Context) (string, error) {
	return "OndexTestBot", nil
}
func (f *fakeBot) SendMessage(context.Context, int64, string) error    { return nil }
func (f *fakeBot) AskContact(context.Context, int64, string) error     { return nil }
func (f *fakeBot) RemoveKeyboard(context.Context, int64, string) error { return nil }
func (f *fakeBot) SendReturnLink(context.Context, int64, string, string, string) error {
	return nil
}
func (f *fakeBot) GetUpdates(ctx context.Context, _ int64, _ int) ([]telegram.Update, error) {
	select {
	case u := <-f.updates:
		return []telegram.Update{u}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type noopSms struct{}

func (noopSms) Send(string, string) error { return nil }

// telegramTestServer — faqat shu endpoint uchun zarur bog'liqliklar.
func telegramTestServer(t *testing.T, secretRequired bool) (http.Handler, *telegram.Verifier, *fakeBot) {
	t.Helper()
	bot := &fakeBot{updates: make(chan telegram.Update, 4)}
	v := telegram.NewVerifier(bot,
		func(context.Context, string) (string, error) { return "123456", nil },
		users.NormalizePhone, 5,
	).WithConfirmSecretRequired(secretRequired)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go v.Run(ctx)

	userRepo := storage.NewMemoryUserRepo()
	tokens := users.NewTokenIssuer("test-secret", time.Hour)
	s := New(Deps{
		UserRepo: userRepo,
		Tokens:   tokens,
		AuthSvc: users.NewService(userRepo, storage.NewMemoryCodeStore(),
			noopSms{}, tokens, NewID),
		Telegram: v,
		DevMode:  true,
	})
	return s.Routes(nil), v, bot
}

// startUpdate / contactUpdate — botga keladigan yangilanishlar.
func startUpdate(chatID int64, token string) telegram.Update {
	var u telegram.Update
	u.Message = new(struct {
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
	})
	u.Message.Chat.ID = chatID
	u.Message.Text = "/start " + token
	return u
}

func contactUpdate(chatID int64, phone string, userID int64) telegram.Update {
	u := startUpdate(chatID, "")
	u.Message.Text = ""
	u.Message.Contact = &struct {
		PhoneNumber string `json:"phone_number"`
		UserID      int64  `json:"user_id"`
	}{PhoneNumber: phone, UserID: userID}
	u.Message.From = &struct {
		ID int64 `json:"id"`
	}{ID: userID}
	return u
}

// confirmViaBot — botdagi to'liq oqimni o'ynatadi va tasdiq yozilguncha
// kutadi.
func confirmViaBot(t *testing.T, v *telegram.Verifier, bot *fakeBot, token string) {
	t.Helper()
	bot.updates <- startUpdate(700, token)
	bot.updates <- contactUpdate(700, "998901234567", 55)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, done, _ := v.LoginResult(token); done {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("bot tasdiqni yozmadi")
}

func startLogin(t *testing.T, v *telegram.Verifier) string {
	t.Helper()
	_, token, err := v.StartLogin(context.Background())
	if err != nil {
		t.Fatalf("StartLogin: %v", err)
	}
	return token
}

func getStatus(t *testing.T, h http.Handler, url string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body
}

// ★ REGRESSIYA TESTI
//
// Dev rejimida (kalit talab qilinmaydi) ilova bo'sh `c` bilan so'raydi
// va tasdiq kelgach KIRISH TOKENINI olishi kerak. Tuzatishdan oldin bu
// yerda abadiy `{"pending":true}` qaytardi.
func TestTelegramStatusCompletesWithoutSecretInDev(t *testing.T) {
	h, v, bot := telegramTestServer(t, false)
	token := startLogin(t, v)

	code, body := getStatus(t, h, "/auth/telegram/login/status?token="+token+"&c=")
	if code != http.StatusOK || body["pending"] != true {
		t.Fatalf("tasdiqdan oldin pending kutilgan: %d %v", code, body)
	}

	confirmViaBot(t, v, bot, token)

	code, body = getStatus(t, h, "/auth/telegram/login/status?token="+token+"&c=")
	if code != http.StatusOK {
		t.Fatalf("holat kodi: %d, javob: %v", code, body)
	}
	if body["token"] == nil || body["token"] == "" {
		t.Fatalf("KIRISH TOKENI BERILMADI (cheksiz yuklanish sababi): %v", body)
	}
}

// Production rejimida kalitsiz so'rov HECH QACHON yakunlanmasligi
// kerak — fishingga qarshi to'siq shu.
func TestTelegramStatusRequiresSecretInProd(t *testing.T) {
	h, v, bot := telegramTestServer(t, true)
	token := startLogin(t, v)
	confirmViaBot(t, v, bot, token)

	for _, q := range []string{"&c=", "&c=yolgonkalit", ""} {
		code, body := getStatus(t, h, "/auth/telegram/login/status?token="+token+q)
		if code != http.StatusOK || body["pending"] != true {
			t.Fatalf("kalitsiz/soxta kalit bilan pending kutilgan (%q): %d %v",
				q, code, body)
		}
		if body["token"] != nil {
			t.Fatalf("FISHING: kalitsiz kirish tokeni berildi (%q)", q)
		}
	}
}

// Noma'lum token — 404.
func TestTelegramStatusUnknownToken(t *testing.T) {
	h, _, _ := telegramTestServer(t, false)
	code, _ := getStatus(t, h, "/auth/telegram/login/status?token=yolgon&c=")
	if code != http.StatusNotFound {
		t.Fatalf("404 kutilgan, olindi: %d", code)
	}
}
