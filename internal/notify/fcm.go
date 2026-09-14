package notify

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// FCM (Firebase Cloud Messaging) HTTP v1 orqali push.
//
// ── NEGA ADMIN SDK EMAS ────────────────────────────────────────────
// `firebaseauth` paketidagi bilan bir xil sabab: Admin SDK katta
// bog'liqlik. Bu yerda kerak bo'lgani — xizmat akkaunti bilan OAuth2
// token olish va bitta POST. Ikkalasi ham standart kutubxona bilan
// bajariladi.
//
// ── SOZLANMASA NIMA BO'LADI ────────────────────────────────────────
// `FIREBASE_SERVICE_ACCOUNT_JSON` berilmasa `NewFCM` nil qaytaradi va
// push umuman yuborilmaydi — WebSocket va DB yozuvi o'z ishini
// qilaveradi (SMTP/Eskiz bilan bir xil naqsh).

const (
	fcmScope    = "https://www.googleapis.com/auth/firebase.messaging"
	googleToken = "https://oauth2.googleapis.com/token"
)

// serviceAccount — kerakli maydonlar (JSON'da boshqalari ham bor).
type serviceAccount struct {
	ProjectID   string `json:"project_id"`
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

type FCM struct {
	sa   serviceAccount
	key  *rsa.PrivateKey
	http *http.Client

	mu        sync.Mutex
	token     string
	tokenTill time.Time
}

// NewFCM — xizmat akkaunti JSON'idan quradi.
//
// `raw` bo'sh bo'lsa (nil, nil) qaytaradi — bu XATO EMAS, push
// shunchaki o'chirilgan degani.
func NewFCM(raw string) (*FCM, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var sa serviceAccount
	if err := json.Unmarshal([]byte(raw), &sa); err != nil {
		return nil, fmt.Errorf("FIREBASE_SERVICE_ACCOUNT_JSON o'qib bo'lmadi: %w", err)
	}
	if sa.ProjectID == "" || sa.ClientEmail == "" || sa.PrivateKey == "" {
		return nil, errors.New("FIREBASE_SERVICE_ACCOUNT_JSON to'liq emas (project_id/client_email/private_key)")
	}
	// Kalit ba'zan base64 ichida beriladi (CI/secret menejerlarida
	// ko'p uchraydi) — ikkala shaklni ham qabul qilamiz.
	key, err := parsePrivateKey(decodeIfBase64(sa.PrivateKey))
	if err != nil {
		return nil, err
	}
	if sa.TokenURI == "" {
		sa.TokenURI = googleToken
	}
	return &FCM{
		sa:   sa,
		key:  key,
		http: &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func parsePrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("xizmat akkaunti kaliti PEM formatida emas")
	}
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rk, ok := k.(*rsa.PrivateKey); ok {
			return rk, nil
		}
		return nil, errors.New("xizmat akkaunti kaliti RSA emas")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// accessToken — OAuth2 token (keshlanadi, muddati tugashidan oldin
// yangilanadi).
func (f *FCM) accessToken(ctx context.Context) (string, error) {
	f.mu.Lock()
	if f.token != "" && time.Now().Before(f.tokenTill) {
		t := f.token
		f.mu.Unlock()
		return t, nil
	}
	f.mu.Unlock()

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   f.sa.ClientEmail,
		"scope": fcmScope,
		"aud":   f.sa.TokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(f.key)
	if err != nil {
		return "", err
	}

	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", signed)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.sa.TokenURI,
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := f.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("oauth2: HTTP %d", resp.StatusCode)
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	f.mu.Lock()
	f.token = out.AccessToken
	// 60 soniya zaxira — muddati tugash chegarasida ishlatib
	// qolmaslik uchun.
	f.tokenTill = time.Now().Add(time.Duration(out.ExpiresIn-60) * time.Second)
	f.mu.Unlock()
	return out.AccessToken, nil
}

// Push — har bir tokenga alohida yuboradi.
//
// FCM v1 da guruh yuborish yo'q (legacy `registration_ids` olib
// tashlangan), shuning uchun sikl. Tokenlar soni odatda 1-3 ta
// (foydalanuvchining qurilmalari).
func (f *FCM) Push(ctx context.Context, tokens []string, e Event) error {
	if f == nil || len(tokens) == 0 {
		return nil
	}
	access, err := f.accessToken(ctx)
	if err != nil {
		return fmt.Errorf("fcm: token olinmadi: %w", err)
	}
	endpoint := "https://fcm.googleapis.com/v1/projects/" +
		url.PathEscape(f.sa.ProjectID) + "/messages:send"

	var firstErr error
	for _, t := range tokens {
		// Xabar tuzilishi (kanal, ovoz, vibratsiya) — `fcm_message.go`.
		raw, _ := json.Marshal(map[string]any{"message": fcmMessage(t, e)})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")

		resp, err := f.http.Do(req)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if resp.StatusCode >= 400 {
			// 404/400 — token eskirgan yoki yaroqsiz. Chaqiruvchi
			// uni o'chirishi mumkin, lekin bu yerda faqat log —
			// tokenlar do'koni bu paketga bog'liq emas.
			slog.Warn("fcm: yuborilmadi", "status", resp.StatusCode,
				"javob", strings.TrimSpace(string(body)))
			if firstErr == nil {
				firstErr = fmt.Errorf("fcm: HTTP %d", resp.StatusCode)
			}
		}
	}
	return firstErr
}

// base64 importi PEM tahlilida kerak bo'lmasa ham, xizmat akkaunti
// kaliti ba'zan base64 holida beriladi — shu sabab qo'shimcha yordamchi.
func decodeIfBase64(s string) string {
	if strings.Contains(s, "BEGIN") {
		return s
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return string(b)
	}
	return s
}
