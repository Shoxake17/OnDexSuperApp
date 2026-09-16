// Package voice — kuryer ilovasining ovozli yo'l ko'rsatishi: o'zbekcha
// iboralar va ularni ovozga aylantirish (Gemini TTS, "Kore" ovozi).
//
// ┌─ ARXITEKTURA (2026-09-15) ─────────────────────────────────────────┐
// Bitta iborani sintez qilish ~8–12 soniya oladi — "200 metrdan keyin"
// degan gapni o'sha paytda so'rab bo'lmaydi. Shuning uchun:
//   - burilish va "yetib keldingiz" BO'LAKLARI oldindan yasalib ilovaga
//     joylanadi (`cmd/voicegen`, kalit = fayl nomi); ilova masofa va manevr
//     bo'lagini ketma-ket chaladi;
//   - faqat restoran NOMI bor ibora serverda bir marta yasalib bazada
//     saqlanadi (`Service.Clip`), matnni ilova bermaydi — nom buyurtmadan.
//
// Kalitlar Dart tomonda ham bir xil (`apps/courier_app/lib/voice_navigation.dart`).
// └────────────────────────────────────────────────────────────────────┘
package voice

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Maneuver — kanonik manevr (Google Directions `maneuver` ning soddalashtirilgani).
type Maneuver string

const (
	Right       Maneuver = "right"
	Left        Maneuver = "left"
	SlightRight Maneuver = "slight_right"
	SlightLeft  Maneuver = "slight_left"
	SharpRight  Maneuver = "sharp_right"
	SharpLeft   Maneuver = "sharp_left"
	UTurn       Maneuver = "uturn"
	Roundabout  Maneuver = "roundabout"
	KeepRight   Maneuver = "keep_right"
	KeepLeft    Maneuver = "keep_left"
	Straight    Maneuver = "straight"
)

// Maneuvers — barcha kanonik manevrlar (ilovadagi ovoz fayllari shular uchun).
var Maneuvers = []Maneuver{Right, Left, SlightRight, SlightLeft, SharpRight, SharpLeft,
	UTurn, Roundabout, KeepRight, KeepLeft, Straight}

// ManeuverFromGoogle — Google `maneuver` qiymati; aytiladigan manevr
// bo'lmasa (bo'sh, parom va h.k.) — "".
func ManeuverFromGoogle(m string) Maneuver {
	switch m {
	case "turn-right":
		return Right
	case "turn-left":
		return Left
	case "turn-slight-right", "ramp-right", "fork-right":
		return SlightRight
	case "turn-slight-left", "ramp-left", "fork-left":
		return SlightLeft
	case "turn-sharp-right":
		return SharpRight
	case "turn-sharp-left":
		return SharpLeft
	case "uturn-right", "uturn-left":
		return UTurn
	case "roundabout-right", "roundabout-left":
		return Roundabout
	case "keep-right":
		return KeepRight
	case "keep-left":
		return KeepLeft
	case "straight", "merge":
		return Straight
	}
	return ""
}

// Bucket — manevrgacha masofa bosqichi.
type Bucket string

const (
	Km1  Bucket = "km1"
	M500 Bucket = "m500"
	M200 Bucket = "m200"
	Now  Bucket = "now"
)

// Buckets — uzoqdan yaqinga.
var Buckets = []Bucket{Km1, M500, M200, Now}

// Raqamlar SO'Z bilan: o'zbek tili rasmiy qo'llanmagan TTS "300" ni boshqa
// tilda o'qib yuborishi mumkin.
var bucketText = map[Bucket]string{
	Km1:  "Bir kilometrdan keyin",
	M500: "Besh yuz metrdan keyin",
	M200: "Ikki yuz metrdan keyin",
	Now:  "Hozir",
}

var maneuverText = map[Maneuver]string{
	Right:       "o'ngga buriling",
	Left:        "chapga buriling",
	SlightRight: "biroz o'ngga buriling",
	SlightLeft:  "biroz chapga buriling",
	SharpRight:  "keskin o'ngga buriling",
	SharpLeft:   "keskin chapga buriling",
	UTurn:       "orqaga qayriling",
	Roundabout:  "aylanma yo'lga kiring",
	KeepRight:   "o'ng tomondan yuring",
	KeepLeft:    "chap tomondan yuring",
	Straight:    "to'g'riga yuring",
}

// Yetib kelish iboralarining kalitlari.
const (
	KeyArrivedRestaurant = "arrived_restaurant"
	KeyArrivedCustomer   = "arrived_customer"
)

// StaticPhrase — ilovaga joylanadigan bo'lak: kalit (fayl nomi) va matn.
type StaticPhrase struct {
	Key  string
	Text string
}

// DistanceKey — masofa bo'lagi, masalan "dist_m200".
func DistanceKey(b Bucket) string { return "dist_" + string(b) }

// ManeuverKey — manevr bo'lagi, masalan "man_right".
func ManeuverKey(m Maneuver) string { return "man_" + string(m) }

// StaticPhrases — ilovaga joylanadigan barcha bo'laklar, MUHIMLIK tartibida.
//
// ┌─ NEGA BO'LAKLAR (egasining qarori, 2026-09-15) ───────────────────┐
// Gemini TTS bepul tarifi kuniga 10 so'rov. 46 ta yaxlit ibora ("Ikki yuz
// metrdan keyin o'ngga buriling") o'rniga 17 ta bo'lak yasaladi, ilova
// ularni ketma-ket chaladi. Tartib — eng ko'p kerak bo'ladiganlari
// birinchi: bir kunlik kvotada ham asosiy yo'l ko'rsatish ishlaydi.
// └───────────────────────────────────────────────────────────────────┘
func StaticPhrases() []StaticPhrase {
	out := make([]StaticPhrase, 0, len(Buckets)+len(Maneuvers)+2)
	for _, b := range Buckets {
		out = append(out, StaticPhrase{Key: DistanceKey(b), Text: bucketText[b]})
	}
	out = append(out, maneuverPhrase(Right), maneuverPhrase(Left),
		StaticPhrase{Key: KeyArrivedRestaurant, Text: "Siz restoranga yetib keldingiz."},
		StaticPhrase{Key: KeyArrivedCustomer, Text: "Siz mijoz manziliga yetib keldingiz."},
	)
	for _, m := range Maneuvers {
		if m != Right && m != Left {
			out = append(out, maneuverPhrase(m))
		}
	}
	return out
}

func maneuverPhrase(m Maneuver) StaticPhrase {
	t := maneuverText[m]
	return StaticPhrase{Key: ManeuverKey(m), Text: strings.ToUpper(t[:1]) + t[1:] + "."}
}

// MaxNameRunes — ovozga aylantiriladigan restoran nomining eng ko'p uzunligi.
const MaxNameRunes = 60

// CleanName — restoran nomini TTS uchun tozalaydi: faqat harf, raqam va
// nomlarda uchraydigan belgilar (apostrof, chiziqcha, &, nuqta); qolgani —
// bitta bo'sh joy. Yangi qator, qo'shtirnoq va boshqa belgilar orqali TTS
// ko'rsatmasiga "gap qo'shib" bo'lmasin.
func CleanName(name string) string {
	var b strings.Builder
	space := true
	for _, r := range name {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r),
			r == '\'', r == 'ʻ', r == 'ʼ', r == '`', r == '-', r == '&', r == '.':
			b.WriteRune(r)
			space = false
		default:
			if !space {
				b.WriteByte(' ')
				space = true
			}
		}
	}
	// Olib tashlangan belgi o'rnidagi bo'sh joy nuqtadan oldin qolmasin
	// ("Kafe ." emas, "Kafe.").
	s := strings.TrimSpace(strings.ReplaceAll(b.String(), " .", "."))
	if utf8.RuneCountInString(s) > MaxNameRunes {
		s = strings.TrimSpace(string([]rune(s)[:MaxNameRunes]))
	}
	return s
}

// ArrivalAtRestaurant — "Siz Book Cafe restoraniga yetib keldingiz."
// (egasi tasdiqlagan namunadagi shakl). Nom "restoran(i)" bilan tugasa
// so'z takrorlanmaydi.
func ArrivalAtRestaurant(name string) (string, bool) {
	clean := CleanName(name)
	if clean == "" {
		return "", false
	}
	lower := strings.ToLower(clean)
	var place string
	switch {
	case strings.HasSuffix(lower, "restorani"):
		place = clean + "ga"
	case strings.HasSuffix(lower, "restoran"):
		place = clean + "iga"
	default:
		place = clean + " restoraniga"
	}
	return "Siz " + place + " yetib keldingiz.", true
}
