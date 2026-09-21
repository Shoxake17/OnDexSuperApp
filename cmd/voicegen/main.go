// voicegen — kuryer ilovasiga joylanadigan ovozli yo'l ko'rsatish iboralarini
// yasaydi (Gemini TTS, `internal/voice`).
//
//	go run ./cmd/voicegen                       # yetishmaganlarini yasaydi
//	go run ./cmd/voicegen -force                # hammasini qayta
//	go run ./cmd/voicegen -only dist_m200,man_right
//	go run ./cmd/voicegen -delay 25s            # bepul tarifda
//
// Bepul tarifda kuniga 10 so'rov: kvota tugasa to'xtaydi, ertasi kuni shu
// buyruq qolganlarini yasaydi (bo'laklar muhimlik tartibida).
//
// Kalit `GEMINI_API_KEY` (muhitdan yoki loyiha ildizidagi `.env` dan) —
// fayllarga yozilmaydi va ekranga chiqarilmaydi.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chustapp/internal/voice"
)

func main() {
	out := flag.String("out", filepath.Join("apps", "courier_app", "assets", "voice"), "fayllar papkasi")
	force := flag.Bool("force", false, "mavjud fayllarni ham qayta yasash")
	only := flag.String("only", "", "faqat shu kalitlar (vergul bilan)")
	// Bepul tarifda TTS daqiqasiga bir necha so'rov qabul qiladi: oraliqsiz
	// yuborilsa deyarli har so'rov 429 olib, kutish vaqti o'sib boradi.
	delay := flag.Duration("delay", 0, "muvaffaqiyatli so'rovlar orasidagi oraliq (masalan 25s)")
	flag.Parse()

	env := readDotEnv(".env")
	get := func(name string) string {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			return v
		}
		return env[name]
	}
	key := get("GEMINI_API_KEY")
	if key == "" {
		fmt.Fprintln(os.Stderr, "GEMINI_API_KEY topilmadi (muhitda ham, .env da ham)")
		os.Exit(1)
	}
	tts := voice.NewGemini(key, get("GEMINI_TTS_MODEL"), get("GEMINI_TTS_VOICE"))
	fmt.Printf("Ovoz: %s\n", tts.VoiceID())

	wanted := map[string]bool{}
	for _, k := range strings.Split(*only, ",") {
		if k = strings.TrimSpace(k); k != "" {
			wanted[k] = true
		}
	}
	if err := os.MkdirAll(*out, 0o750); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	phrases := voice.StaticPhrases()
	failed := 0
	for i, p := range phrases {
		if len(wanted) > 0 && !wanted[p.Key] {
			continue
		}
		path := filepath.Join(*out, p.Key+".wav")
		if _, err := os.Stat(path); err == nil && !*force {
			fmt.Printf("[%d/%d] %s — bor, o'tkazildi\n", i+1, len(phrases), p.Key)
			continue
		}
		data, err := synthWithRetry(tts, p.Text)
		if errors.Is(err, voice.ErrDailyQuota) {
			fmt.Fprintf(os.Stderr, "[%d/%d] %s — kunlik kvota tugadi. Ertaga (Toshkent vaqti bilan ~12:00 dan keyin) shu buyruqni qayta ishga tushiring.\n",
				i+1, len(phrases), p.Key)
			os.Exit(2)
		}
		if err != nil {
			failed++
			fmt.Fprintf(os.Stderr, "[%d/%d] %s — XATO: %v\n", i+1, len(phrases), p.Key, err)
			continue
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			failed++
			fmt.Fprintf(os.Stderr, "[%d/%d] %s — yozilmadi: %v\n", i+1, len(phrases), p.Key, err)
			continue
		}
		fmt.Printf("[%d/%d] %s — %.0f KB  «%s»\n", i+1, len(phrases), p.Key, float64(len(data))/1024, p.Text)
		if *delay > 0 {
			time.Sleep(*delay)
		}
	}
	if failed > 0 {
		fmt.Fprintf(os.Stderr, "%d ta ibora yasalmadi — qayta ishga tushiring (mavjudlari o'tkaziladi)\n", failed)
		os.Exit(1)
	}
	fmt.Println("Tayyor.")
}

// synthWithRetry — chegara (429) va vaqtinchalik xatolarda kutib qayta urinadi.
func synthWithRetry(tts *voice.Gemini, text string) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= 8; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		data, err := tts.Synthesize(ctx, text)
		cancel()
		if err == nil {
			return data, nil
		}
		if errors.Is(err, voice.ErrDailyQuota) {
			return nil, err // qayta urinish foydasiz
		}
		lastErr = err
		// "Audio yo'q" kabi xatoda HAR urinish kunlik kvotadan (bepul tarifda 10)
		// bitta so'rov yeydi: 8 marta urinish butun kunlik kvotani bitta ibora
		// uchun sarflashi mumkin. Uch urinishdan keyin keyingi iboraga o'tiladi.
		if !errors.Is(err, voice.ErrRateLimited) && attempt >= 3 {
			break
		}
		wait := 5 * time.Second
		if errors.Is(err, voice.ErrRateLimited) {
			wait = time.Duration(attempt*20) * time.Second
		}
		fmt.Printf("    urinish %d: %v — %s kutiladi\n", attempt, err, wait)
		time.Sleep(wait)
	}
	return nil, lastErr
}

// readDotEnv — oddiy KEY=VALUE o'quvchi (izohlar va qo'shtirnoqlar hisobga olinadi).
func readDotEnv(path string) map[string]string {
	out := map[string]string{}
	f, err := os.Open(path) //nolint:gosec // G304: loyiha ildizidagi .env
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(name)] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return out
}
