package agentapi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

// ┌─ SIRLAR BILAN ISHLASHNING YAGONA JOYI ─────────────────────────────┐
// Kalit/token yaratish va hashlash FAQAT shu faylda. Sabab: bu kod
// bir necha joyga tarqalsa, kimdir bir kun `math/rand` ishlatib
// qo'yadi yoki hashlashni unutib ochiq saqlaydi. Bitta joyda
// bo'lganda esa butun modelni bitta faylni o'qib tekshirish mumkin.
// └────────────────────────────────────────────────────────────────────┘

const (
	// keyPrefixLive / keyPrefixTest — kalitning turi NOMIDAN
	// ko'rinadi. Bu ataylab: `.env` ga sinov kaliti o'rniga jonli
	// kalit yopishtirilgani ko'z bilan darhol sezilishi kerak.
	keyPrefixLive = "ondex_live_"
	keyPrefixTest = "ondex_test_"
	// grantPrefix — foydalanuvchi granti.
	grantPrefix = "ondexg_"
	// linkSecretPrefix — ulanish so'rovini poll qilish siri.
	linkSecretPrefix = "ondexl_"

	// keyBytes — 24 bayt = 192 bit. Brute-force amalda imkonsiz.
	keyBytes = 24
	// tokenBytes — grant/link sirlari uchun 32 bayt (256 bit).
	tokenBytes = 32

	// idBytes — ichki identifikatorlar (sir EMAS, lekin taxmin
	// qilinmasligi kerak: link ID loglarga tushadi).
	idBytes = 16
)

// randomHex — `crypto/rand`. `math/rand` EMAS.
//
// Bu farq hal qiluvchi: `math/rand` urug'i taxmin qilinsa, hujumchi
// barcha kalit va tokenlarni hisoblab chiqarardi. Xato yutilmaydi —
// tasodifiylik manbai ishlamasa, sir yaratmaslik yagona to'g'ri
// xatti-harakat.
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", errors.New("tasodifiylik manbai ishlamadi: " + err.Error())
	}
	return hex.EncodeToString(b), nil
}

// NewID — ichki identifikator.
func NewID() (string, error) { return randomHex(idBytes) }

// HashSecret — sirning bazada saqlanadigan ko'rinishi.
//
// NEGA SHA-256, ARGON2 EMAS: bular parol emas, 192-256 bitli
// TASODIFIY qiymatlar. Lug'at hujumi ularga ta'sir qilmaydi, sekin
// hash esa har so'rovda protsessorni yoqib, DoS yuzasi ochardi
// (parollar uchun `internal/users/password.go` ataylab Argon2id
// ishlatadi — u yerda kirish maydoni kichik va taxmin qilinadigan).
func HashSecret(s string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(s)))
	return hex.EncodeToString(sum[:])
}

// NewPartnerKey — sherik uchun API kalit yaratadi.
//
// Qaytaradi: to'liq kalit (BIR MARTA ko'rsatiladi va boshqa hech
// qachon tiklanmaydi) va uning ochiq prefiksi.
func NewPartnerKey(env Environment) (full, prefix string, err error) {
	p := keyPrefixLive
	if env == EnvTest {
		p = keyPrefixTest
	}
	secret, err := randomHex(keyBytes)
	if err != nil {
		return "", "", err
	}
	full = p + secret
	// Prefiks: tur + sirning birinchi 8 belgisi. 8 hex = 32 bit —
	// kalitlarni bir-biridan ajratish uchun yetarli, sirni ochish
	// uchun esa mutlaqo yetarsiz (qolgan 160 bit noma'lum).
	prefix = p + secret[:8]
	return full, prefix, nil
}

// EnvironmentOfKey — kalitning turini NOMIDAN aniqlaydi.
//
// Bu faqat qulaylik: haqiqiy tur bazadagi yozuvdan olinadi. Kalit
// nomiga ISHONMASLIK kerak — u tashqaridan keladi.
func EnvironmentOfKey(key string) Environment {
	if strings.HasPrefix(key, keyPrefixTest) {
		return EnvTest
	}
	return EnvLive
}

// LooksLikePartnerKey — bazaga bemaqsad so'rov yubormaslik uchun
// arzon tekshiruv (`tables.Resolve` dagi uzunlik tekshiruvi bilan bir
// xil mantiq).
func LooksLikePartnerKey(key string) bool {
	if !strings.HasPrefix(key, keyPrefixLive) && !strings.HasPrefix(key, keyPrefixTest) {
		return false
	}
	return len(key) == len(keyPrefixLive)+keyBytes*2
}

// NewGrantToken — foydalanuvchi granti tokeni.
func NewGrantToken() (string, error) {
	s, err := randomHex(tokenBytes)
	if err != nil {
		return "", err
	}
	return grantPrefix + s, nil
}

func LooksLikeGrantToken(t string) bool {
	return strings.HasPrefix(t, grantPrefix) && len(t) == len(grantPrefix)+tokenBytes*2
}

// NewLinkSecret — ulanish so'rovini poll qilish siri.
func NewLinkSecret() (string, error) {
	s, err := randomHex(tokenBytes)
	if err != nil {
		return "", err
	}
	return linkSecretPrefix + s, nil
}

// ── Foydalanuvchi teradigan kod ──

// userCodeAlphabet — Crockford Base32 (I, L, O, U yo'q).
//
// NEGA SHUNDAY: kodni odam EKRANDAN O'QIB, klaviaturada teradi.
// "0/O", "1/I/L" chalkashligi eng ko'p uchraydigan xato manbai;
// "U" esa tasodifan haqoratli so'z hosil qilmasligi uchun
// tashlangan (Crockford standartidagi sabab).
const userCodeAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// userCodeLen — 8 belgi = 32^8 ≈ 1.1×10^12 variant.
//
// Bu kalit uchun kam, LEKIN kod bor-yo'g'i 10 daqiqa yashaydi va
// terish urinishlari IP bo'yicha cheklangan — ya'ni taxmin qilishga
// amalda imkoniyat qolmaydi. Uzunroq kod esa foydalanuvchini
// charchatardi (uni qo'lda ko'chirish kerak).
const userCodeLen = 8

// NewUserCode — "K7P2-9QMX" ko'rinishidagi kod.
//
// Ajratuvchi chiziqcha faqat KO'RINISH uchun; `NormalizeUserCode` uni
// olib tashlaydi, ya'ni foydalanuvchi chiziqchasiz tersa ham ishlaydi.
func NewUserCode() (string, error) {
	b := make([]byte, userCodeLen)
	if _, err := rand.Read(b); err != nil {
		return "", errors.New("tasodifiylik manbai ishlamadi: " + err.Error())
	}
	out := make([]byte, 0, userCodeLen+1)
	for i, v := range b {
		if i == userCodeLen/2 {
			out = append(out, '-')
		}
		// Modul bilan taqsimot bir oz notekis bo'ladi (256 % 32 == 0
		// bo'lgani uchun bu yerda AYNAN tekis — alifbo 32 ta).
		out = append(out, userCodeAlphabet[int(v)%len(userCodeAlphabet)])
	}
	return string(out), nil
}

// NormalizeUserCode — foydalanuvchi kiritgan kodni solishtirishga
// tayyorlaydi: katta harf, chiziqcha/bo'shliqsiz, chalkash belgilar
// to'g'rilangan.
//
// "O" → "0" va "I"/"L" → "1" almashtirish MAQSADLI: bu belgilar
// alifboda umuman yo'q, ya'ni ular faqat NOTO'G'RI o'qishdan
// paydo bo'ladi. To'g'rilamasak, foydalanuvchi "kod ishlamayapti"
// degan xulosaga kelardi.
func NormalizeUserCode(code string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(code)) {
		switch r {
		case '-', ' ', '_':
			continue
		case 'O':
			r = '0'
		case 'I', 'L':
			r = '1'
		}
		if strings.ContainsRune(userCodeAlphabet, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ValidUserCode — normallashtirilgan kod to'g'ri shakldami.
func ValidUserCode(normalized string) bool {
	return len(normalized) == userCodeLen
}
