package flow

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"time"
)

// Bu fayldagi funksiyalar NASHR qiluvchi tomon uchun (`cmd/ondexflow`,
// testlar). Server shaxsiy kalitni HECH QACHON ko'rmaydi.

// GenerateKey — yangi RSA-3072 kalit jufti: shaxsiy (PKCS#8 PEM) va
// ochiq (Shorebird formati: PKCS#1 DER base64).
func GenerateKey() (privPEM []byte, pubB64 string, err error) {
	k, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		return nil, "", err
	}
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		return nil, "", err
	}
	privPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	return privPEM, PublicKeyB64(&k.PublicKey), nil
}

// PublicKeyB64 — Shorebird kutgan shakl (PKCS#1 DER, standart base64).
func PublicKeyB64(pub *rsa.PublicKey) string {
	return base64.StdEncoding.EncodeToString(x509.MarshalPKCS1PublicKey(pub))
}

// ParsePrivateKey — PKCS#8 yoki PKCS#1 PEM.
func ParsePrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	b, _ := pem.Decode(pemBytes)
	if b == nil {
		return nil, errors.New("PEM emas")
	}
	if k, err := x509.ParsePKCS8PrivateKey(b.Bytes); err == nil {
		if rk, ok := k.(*rsa.PrivateKey); ok {
			return rk, nil
		}
		return nil, errors.New("RSA kalit emas")
	}
	return x509.ParsePKCS1PrivateKey(b.Bytes)
}

// Sign — xabar ustiga RSASSA-PKCS1-v1_5 + SHA-256 (qurilma tekshiruvi bilan bir xil).
func Sign(k *rsa.PrivateKey, message []byte) (string, error) {
	sum := sha256.Sum256(message)
	sig, err := rsa.SignPKCS1v15(rand.Reader, k, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// FileRefOf — fayl uchun manifest yozuvi.
func FileRefOf(b []byte) FileRef {
	sum := sha256.Sum256(b)
	return FileRef{SHA256: hex.EncodeToString(sum[:]), Size: int64(len(b))}
}

// SignManifest — manifestga vaqt qo'yadi, JSON qiladi va imzolaydi.
func SignManifest(k *rsa.PrivateKey, m PublishManifest, now time.Time) (raw []byte, sig string, err error) {
	m.IssuedAt = now.Unix()
	raw, err = json.Marshal(m)
	if err != nil {
		return nil, "", err
	}
	sig, err = Sign(k, raw)
	return raw, sig, err
}
