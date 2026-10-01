package secretstore

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

const testKeyHex = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func mustBox(t *testing.T, key string) *Box {
	t.Helper()
	b, err := New(key)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return b
}

func TestSealOpenRoundTrip(t *testing.T) {
	b := mustBox(t, testKeyHex)
	for _, plain := range []string{"", "Pw18axeBFo8V7NamKHXX", "o'zbekcha matn — sinov", strings.Repeat("x", 4096)} {
		enc, err := b.Seal(plain)
		if err != nil {
			t.Fatalf("Seal: %v", err)
		}
		if enc == plain && plain != "" {
			t.Fatal("shifrlanmagan: natija asl matn bilan bir xil")
		}
		got, err := b.Open(enc)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if got != plain {
			t.Errorf("Open: %q kutildi, %q keldi", plain, got)
		}
	}
}

// Bir xil matn ikki marta shifrlansa natija HAR XIL bo'lishi kerak —
// aks holda bazaga qarab "bu ikki restoranning kaliti bir xil" degan
// xulosa chiqarish mumkin bo'lardi.
func TestSealIsNonDeterministic(t *testing.T) {
	b := mustBox(t, testKeyHex)
	a, err := b.Seal("bir xil")
	if err != nil {
		t.Fatal(err)
	}
	c, err := b.Seal("bir xil")
	if err != nil {
		t.Fatal(err)
	}
	if a == c {
		t.Error("ikki shifr bir xil chiqdi — nonce tasodifiy emas")
	}
}

// Boshqa kalit bilan ochib bo'lmasligi kerak: baza nusxasi o'z-o'zidan
// sirlarni bermaydi.
func TestOpenWithOtherKeyFails(t *testing.T) {
	enc, err := mustBox(t, testKeyHex).Seal("sir")
	if err != nil {
		t.Fatal(err)
	}
	other := mustBox(t, strings.Repeat("ab", KeyLen))
	if _, err := other.Open(enc); err == nil {
		t.Fatal("boshqa kalit bilan ochildi")
	}
}

// GCM butunlikni ham tekshiradi: o'zgartirilgan shifr ochilmasligi
// kerak (CBC'da bu jimgina o'tib ketardi).
func TestTamperedCiphertextFails(t *testing.T) {
	b := mustBox(t, testKeyHex)
	enc, err := b.Seal("sir")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] ^= 0x01
	if _, err := b.Open(base64.StdEncoding.EncodeToString(raw)); err == nil {
		t.Fatal("buzilgan shifr ochildi")
	}
}

func TestOpenRejectsGarbage(t *testing.T) {
	b := mustBox(t, testKeyHex)
	for _, bad := range []string{"", "bu base64 emas!!!", base64.StdEncoding.EncodeToString([]byte("qisqa"))} {
		if _, err := b.Open(bad); err == nil {
			t.Errorf("%q ochildi", bad)
		}
	}
}

func TestNewKeyFormats(t *testing.T) {
	raw, err := hex.DecodeString(testKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	// hex, base64 (padding bilan va padsiz) — uchalasi ham qabul qilinadi.
	for _, key := range []string{
		testKeyHex,
		base64.StdEncoding.EncodeToString(raw),
		base64.RawStdEncoding.EncodeToString(raw),
		"  " + testKeyHex + "  ", // bo'sh joy kesiladi (`.env` da uchraydi)
	} {
		if _, err := New(key); err != nil {
			t.Errorf("%q rad etildi: %v", key, err)
		}
	}
}

func TestNewRejectsBadKeys(t *testing.T) {
	cases := map[string]string{
		"bo'sh":        "",
		"qisqa hex":    "00112233",
		"uzun hex":     testKeyHex + "00",
		"hex ham emas": "bu umuman kalit emas",
	}
	for name, key := range cases {
		if _, err := New(key); err == nil {
			t.Errorf("%s: qabul qilindi", name)
		}
	}
	// Bo'sh kalit ALOHIDA xato: chaqiruvchi buni "imkoniyat o'chiq" deb
	// qarashi kerak, "noto'g'ri sozlangan" deb emas.
	if _, err := New(""); err != ErrNoKey {
		t.Errorf("bo'sh kalit uchun ErrNoKey kutildi, %v keldi", err)
	}
}

// `nil` Box — kalit sozlanmagan holat. Panika bermasligi kerak.
func TestNilBoxIsSafe(t *testing.T) {
	var b *Box
	if _, err := b.Seal("x"); err != ErrNoKey {
		t.Errorf("Seal: ErrNoKey kutildi, %v", err)
	}
	if _, err := b.Open("x"); err != ErrNoKey {
		t.Errorf("Open: ErrNoKey kutildi, %v", err)
	}
}

func TestMask(t *testing.T) {
	cases := map[string]string{
		"":                     "",
		"ab":                   "**",
		"abcd":                 "****",
		"Pw18axeBFo8V7NamKHXX": "****************KHXX",
	}
	for in, want := range cases {
		if got := Mask(in); got != want {
			t.Errorf("Mask(%q): %q kutildi, %q keldi", in, want, got)
		}
	}
}
