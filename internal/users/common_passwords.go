package users

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"strings"
	"sync"
	"unicode/utf8"
)

// Taqiqlangan parollar — NIST SP 800-63B 3.1.1.2 (SSO auditi, AUD-9).
//
// ┌─ MANBA ───────────────────────────────────────────────────────────────┐
// `data/common_passwords.txt.gz` — Buyuk Britaniya NCSC'ning "100k most
// used passwords" ro'yxati (SecLists, MIT), 8+ belgili, kichik harfli,
// takrorsiz (~46 ming) + mahalliy so'zlar (toshkent, parol123, ondex...).
// OFLAYN: parol hech qayerga yuborilmaydi (tashqi API — yangi xavf va
// bog'liqlik). Yangilash: ro'yxatni qayta yuklab, shu faylni almashtirish.
// └───────────────────────────────────────────────────────────────────────┘
//
//go:embed data/common_passwords.txt.gz
var commonPasswordsGz []byte

var (
	commonOnce sync.Once
	commonSet  map[string]struct{}
)

func loadCommonPasswords() {
	commonSet = make(map[string]struct{}, 50_000)
	zr, err := gzip.NewReader(bytes.NewReader(commonPasswordsGz))
	if err != nil {
		return // fayl buzuq — tekshiruvning qolgan qismi baribir ishlaydi
	}
	defer zr.Close()
	sc := bufio.NewScanner(zr)
	for sc.Scan() {
		if w := strings.TrimSpace(sc.Text()); w != "" {
			commonSet[w] = struct{}{}
		}
	}
}

// serviceWords — xizmatga xos so'zlar (NIST: "context-specific words").
var serviceWords = []string{"ondex", "chust", "shoxpos", "merchant", "parol", "password", "qwerty"}

// isWeakPassword — keng tarqalgan / sizib chiqqan, bir xil belgilar,
// oddiy ketma-ketlik yoki xizmat nomi + raqamlar.
func isWeakPassword(p string) bool {
	commonOnce.Do(loadCommonPasswords)
	low := strings.ToLower(strings.TrimSpace(p))
	if _, bad := commonSet[low]; bad {
		return true
	}
	if commonPasswords[low] {
		return true
	}
	if utf8.RuneCountInString(low) == 0 {
		return true
	}
	// Bitta belgi takrori: "aaaaaaaa", "11111111".
	first, _ := utf8.DecodeRuneInString(low)
	if strings.Count(low, string(first)) == utf8.RuneCountInString(low) {
		return true
	}
	// Oddiy ketma-ketlik: "12345678", "87654321", "abcdefgh".
	if isSequence(low) {
		return true
	}
	// Xizmat so'zi + faqat raqam/belgi: "ondex2026!", "chust1234".
	for _, w := range serviceWords {
		if strings.Contains(low, w) {
			rest := strings.ReplaceAll(low, w, "")
			if strings.Trim(rest, "0123456789!@#$%^&*._-") == "" {
				return true
			}
		}
	}
	return false
}

func isSequence(s string) bool {
	if len(s) < 4 {
		return false
	}
	up, down := true, true
	for i := 1; i < len(s); i++ {
		d := int(s[i]) - int(s[i-1])
		up = up && d == 1
		down = down && d == -1
	}
	return up || down
}
