package main

import (
	"crypto/hmac"
	"crypto/sha256"
)

// deriveKey — bitta sirdan MAQSADGA bog'langan alohida kalit (HMAC-SHA256).
// Bir kalitni ikki vazifada ishlatmaslik uchun: biri oshkor bo'lsa ham
// ikkinchisi saqlanadi, va bir maqsadning imzosi boshqasida yaroqsiz.
func deriveKey(secret, purpose string) []byte {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(purpose))
	return m.Sum(nil)
}
