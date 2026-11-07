// Command authkey — markaziy sessiya uchun Ed25519 imzolash kalitini yaratadi.
//
//	go run ./cmd/authkey -out secrets/auth_ed25519.pem
//
// Fayl FAQAT egasi o'qiy oladigan qilib yoziladi (0600) va allaqachon mavjud
// bo'lsa USTIDAN YOZILMAYDI (tasodifan ishlab turgan kalitni almashtirib,
// barcha sessiyalarni yaroqsiz qilmaslik uchun). Maxfiy kalit ekranga
// chiqarilmaydi — faqat ochiq `kid`.
package main

import (
	"flag"
	"fmt"
	"os"

	"chustapp/internal/identity"
)

func main() {
	out := flag.String("out", "", "kalit fayli yo'li (masalan secrets/auth_ed25519.pem)")
	flag.Parse()
	if *out == "" {
		fmt.Fprintln(os.Stderr, "foydalanish: go run ./cmd/authkey -out secrets/auth_ed25519.pem")
		os.Exit(2)
	}
	key, err := identity.GenerateKey()
	if err != nil {
		fmt.Fprintln(os.Stderr, "kalit yaratilmadi:", err)
		os.Exit(1)
	}
	pemBytes, err := identity.EncodePrivatePEM(key)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kalit kodlanmadi:", err)
		os.Exit(1)
	}
	f, err := os.OpenFile(*out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fayl yaratilmadi (mavjud bo'lsa ustidan yozilmaydi):", err)
		os.Exit(1)
	}
	if _, err := f.Write(pemBytes); err != nil {
		_ = f.Close()
		fmt.Fprintln(os.Stderr, "yozilmadi:", err)
		os.Exit(1)
	}
	if err := f.Close(); err != nil {
		fmt.Fprintln(os.Stderr, "yopilmadi:", err)
		os.Exit(1)
	}
	fmt.Printf("tayyor: %s (kid=%s)\nServerga: AUTH_SIGNING_KEY_FILE=%s\n", *out, key.KID, *out)
}
