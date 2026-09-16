// Tashqi geokodlash manbalari va ularning javob shakllari.
//
// Bu fayl HTTP handlerlardan ATAYLAB ajratilgan: u faqat "tashqi
// xizmatdan manzil/marshrut olish" bilan shug'ullanadi va hech qanday
// HTTP javob yozmaydi. Handlerlar (routes_geo.go) shu funksiyalarni
// chaqiradi.
package httpapi

import (
	"context"
	"log/slog"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// geocodeResponse — Google Geocoding API javobining bizga kerakli qismi.
type geocodeResponse struct {
	Status  string `json:"status"`
	Results []struct {
		FormattedAddress  string `json:"formatted_address"`
		AddressComponents []struct {
			LongName string   `json:"long_name"`
			Types    []string `json:"types"`
		} `json:"address_components"`
	} `json:"results"`
}

// directionsResponse — Google Directions API javobining bizga kerakli qismi.
type directionsResponse struct {
	Status string `json:"status"`
	Routes []struct {
		OverviewPolyline struct {
			Points string `json:"points"`
		} `json:"overview_polyline"`
		Legs []struct {
			Distance struct {
				Value int `json:"value"` // metr
			} `json:"distance"`
			Duration struct {
				Value int `json:"value"` // soniya
			} `json:"duration"`
			// Steps — ovozli yo'l ko'rsatish uchun manevrlar.
			Steps []struct {
				Maneuver      string `json:"maneuver"`
				StartLocation struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"start_location"`
				Distance struct {
					Value int `json:"value"`
				} `json:"distance"`
			} `json:"steps"`
		} `json:"legs"`
	} `json:"routes"`
}

// routePoint — xaritada chizish uchun bitta nuqta.
type routePoint struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// decodePolyline — Google'ning "encoded polyline algorithm format"ini
// dekodlaydi (https://developers.google.com/maps/documentation/utilities/polylinealgorithm).
// Dekodlash SERVERDA bajariladi — shu tufayli Flutter ilovalariga polyline
// dekodlash kutubxonasi qo'shish shart emas, ular tayyor {lat,lng} nuqtalar
// ro'yxatini oladi.
// ┌─ TUZATILGAN NOSOZLIK (bug.md 22-band) ─────────────────────────────┐
// Ichki sikllarda `index < len(encoded)` tekshiruvi YO'Q edi. Tashqi
// sikl uni tekshirardi, ichkilari esa `b < 0x20` bo'lguncha o'qishda
// davom etardi. Polilinia KESILGAN bo'lsa (oxirgi bayt `>= 0x20`)
// `index` chegaradan chiqib, `index out of range` panic'i tug'ilardi.
//
// Kirish TASHQI xizmatdan keladi (Google Directions javobi), ya'ni
// biz uni nazorat qilmaymiz: qisman javob, buzilgan uzatish yoki
// xizmatning o'zgargan formati serverni yiqitishi mumkin edi.
//
// Endi ichki sikl `decodeValue` yordamchisiga chiqarildi va u
// chegaradan chiqmaydi: ma'lumot tugab qolsa, `ok=false` qaytaradi
// va dekodlash O'SHA YERDA to'xtaydi — shu paytgacha yig'ilgan
// nuqtalar saqlanadi (qisman marshrut bo'shdan yaxshiroq).
// └────────────────────────────────────────────────────────────────────┘
func decodePolyline(encoded string) []routePoint {
	var points []routePoint
	index, lat, lng := 0, 0, 0
	for index < len(encoded) {
		dlat, next, ok := decodeValue(encoded, index)
		if !ok {
			break
		}
		index = next
		lat += dlat

		dlng, next, ok := decodeValue(encoded, index)
		if !ok {
			break
		}
		index = next
		lng += dlng

		points = append(points, routePoint{Lat: float64(lat) / 1e5, Lng: float64(lng) / 1e5})
	}
	return points
}

// decodeValue — polilinia formatidagi bitta qiymatni o'qiydi.
//
// Qaytaradi: qiymat, keyingi indeks va ma'lumot TO'LIQ bo'lganini
// bildiruvchi bayroq. Kesilgan kirishda `false` — chaqiruvchi
// to'xtaydi, panic bo'lmaydi.
func decodeValue(encoded string, index int) (value, next int, ok bool) {
	shift, result := 0, 0
	for {
		if index >= len(encoded) {
			// Kesilgan ma'lumot: oxirgi bayt "davomi bor" deb
			// belgilangan, lekin davomi yo'q.
			return 0, index, false
		}
		b := int(encoded[index]) - 63
		index++
		result |= (b & 0x1f) << shift
		shift += 5
		if b < 0x20 {
			break
		}
	}
	if result&1 != 0 {
		return ^(result >> 1), index, true
	}
	return result >> 1, index, true
}

// yandexReverseGeocode — Google aniq ko'cha topa olmaganda ikkinchi manba
// sifatida so'raladi (YANDEX_GEOCODER_API_KEY sozlangan bo'lsagina; kalit
// yo'q bo'lsa jimgina `false` qaytaradi — asosiy oqim buzilmaydi). Faqat
// ko'cha/uy darajasidagi ("street"/"house") natija bo'lsa qaytariladi —
// aks holda Google'ning mahalla/shahar darajasidagi natijasi saqlanadi
// (Yandex'ning ham faqat shahar darajasidagi javobi hech qanday foyda
// bermaydi). Kalit — bepul tarif, 1000 so'rov/kun, karta talab qilmaydi
// (https://yandex.com/dev/geocode/doc/en/terms).
func yandexReverseGeocode(ctx context.Context, lat, lng float64) (string, bool) {
	key := os.Getenv("YANDEX_GEOCODER_API_KEY")
	if key == "" {
		return "", false
	}
	// DIQQAT: Yandex'da koordinata tartibi "long,lat" — Google'dagi
	// "lat,lng"ga TESKARI, chalkashtirmaslik kerak (klassik Yandex
	// hujjatlarida shunday, lekin kalit ishga tushgach haqiqiy nuqta
	// bilan sinab tasdiqlash kerak — /v1/ portali yangi bo'lgani uchun
	// 100% ishonch yo'q).
	geoURL := "https://geocode-maps.yandex.ru/v1/?apikey=" + key +
		"&format=json&kind=house&results=1&lang=uz_UZ&geocode=" +
		strconv.FormatFloat(lng, 'f', -1, 64) + "," + strconv.FormatFloat(lat, 'f', -1, 64)
	var data struct {
		Response struct {
			GeoObjectCollection struct {
				FeatureMember []struct {
					GeoObject struct {
						Name             string `json:"name"`
						Description      string `json:"description"`
						MetaDataProperty struct {
							GeocoderMetaData struct {
								Kind string `json:"kind"`
							} `json:"GeocoderMetaData"`
						} `json:"metaDataProperty"`
					} `json:"GeoObject"`
				} `json:"featureMember"`
			} `json:"GeoObjectCollection"`
		} `json:"response"`
	}
	// Timeout + hajm chegarasi `geoclient.go` da (bug.md 21-band).
	if err := fetchGeoJSON(ctx, geoURL, &data); err != nil {
		slog.Warn("Yandex Geocoder so'roviga xato", "err", err)
		return "", false
	}
	members := data.Response.GeoObjectCollection.FeatureMember
	if len(members) == 0 {
		return "", false
	}
	obj := members[0].GeoObject
	kind := obj.MetaDataProperty.GeocoderMetaData.Kind
	if kind != "house" && kind != "street" {
		return "", false // faqat mahalla/shahar darajasi — Google'nikidan yaxshiroq emas
	}
	if obj.Name == "" {
		return "", false
	}
	return obj.Name, true
}

// dgisReverseGeocode — Google va Yandex ikkalasi ham aniq ko'cha topa
// olmaganda so'nggi manba sifatida so'raladi (Markaziy Osiyoda mahalliy
// ma'lumotlar bo'yicha ko'pincha kuchliroq deb hisoblanadi — LEKIN Chust
// uchun jonli tekshirilganda 2GIS'da hozircha ko'cha/bino darajasidagi
// ma'lumot UMUMAN yo'qligi aniqlandi (faqat tuman/viloyat topildi) — bu
// kod xatosi emas, xom ma'lumot yetishmasligi; boshqa shaharlar/kelajakda
// 2GIS ma'lumoti to'ldirilsa ishlaydigan bo'lsin deb saqlab qo'yildi.
//
// DIQQAT — Yandex/Google'dan FARQLI: 2GIS'ning bepul "demo" kaliti
// yaratilgan kundan boshlab FAQAT 1 OYGA amal qiladi va 1000 so'rov bilan
// cheklangan (doimiy oylik/kunlik limit emas) — muddati o'tsa yoki DGIS_API_KEY
// bo'sh bo'lsa jimgina o'tkazib yuboriladi, xato bermaydi.
func dgisReverseGeocode(ctx context.Context, lat, lng float64) (string, bool) {
	key := os.Getenv("DGIS_API_KEY")
	if key == "" {
		return "", false
	}
	geoURL := "https://catalog.api.2gis.com/3.0/items/geocode" +
		"?lon=" + strconv.FormatFloat(lng, 'f', -1, 64) +
		"&lat=" + strconv.FormatFloat(lat, 'f', -1, 64) +
		"&radius=500&fields=items.full_name,items.address_name&locale=uz_UZ&key=" + key
	// 404 "itemNotFound" — hech narsa topilmadi, bu XATO emas, oddiy
	// bo'sh natija (JSON decode xato bermaydi, Result.Items shunchaki
	// bo'sh qoladi, quyidagi len()==0 tekshiruvi to'g'ri ishlaydi).
	var data struct {
		Result struct {
			Items []struct {
				Type        string `json:"type"`
				Name        string `json:"name"`
				FullName    string `json:"full_name"`
				AddressName string `json:"address_name"`
			} `json:"items"`
		} `json:"result"`
	}
	// Timeout + hajm chegarasi `geoclient.go` da (bug.md 21-band).
	if err := fetchGeoJSON(ctx, geoURL, &data); err != nil {
		slog.Warn("2GIS Geocoder so'roviga xato", "err", err)
		return "", false
	}
	for _, item := range data.Result.Items {
		if item.Type != "building" && item.Type != "street" {
			continue // adm_div (tuman/viloyat) kabi keng turlarni o'tkazib yuboramiz
		}
		if item.AddressName != "" {
			return item.AddressName, true
		}
		if item.FullName != "" {
			return item.FullName, true
		}
		if item.Name != "" {
			return item.Name, true
		}
	}
	return "", false
}

// shortAddress — to'liq, aniq manzil (mahalla + ko'cha + uy) quradi.
//
// MUHIM: Google reverse-geocode bitta koordinata uchun BIR NECHTA natija
// qaytaradi — birinchisi ko'pincha aynan o'sha nuqtadagi OBYEKT (masalan
// bog'dagi attraksion, do'kon) bo'lib, uning o'zida "route" (ko'cha)
// komponenti UMUMAN bo'lmaydi, faqat shahar darajasigacha ma'lumot beradi.
// Avvalgi kod FAQAT shu birinchi natijani tekshirib, undan pastdagi (aynan
// ko'cha ma'lumoti bor) natijalarga qaramay "Chust" bilan to'xtab qolar
// edi — shuning uchun BARCHA natijalar (va ularning barcha komponentlari)
// birga skanerlanadi, keyin eng aniq (mahalla+ko'cha+uy) birikma quriladi.
// highwayCodeRe — magistral yo'l kodlariga mos keladi (R-121, M-39, A373,
// P-2 va h.k.) — bular ko'cha nomi emas, rasmiy yo'l raqamlari, mijozga
// yetkazib berish manzili sifatida ko'rsatib bo'lmaydi.
var highwayCodeRe = regexp.MustCompile(`(?i)^[a-z]{1,2}-?\d{1,4}$`)

// shortAddress — to'liq, aniq manzil (mahalla + ko'cha + uy) quradi.
// Ikkinchi qaytar qiymat — haqiqiy ko'cha (route) topildimi yoki yo'qmi;
// chaqiruvchi shunga qarab boshqa geokodlash manbasini (Yandex) ham
// sinab ko'rish kerakligini hal qiladi.
//
// MUHIM: Google reverse-geocode bitta koordinata uchun BIR NECHTA natija
// qaytaradi — birinchisi ko'pincha aynan o'sha nuqtadagi OBYEKT (masalan
// bog'dagi attraksion, do'kon) bo'lib, uning o'zida "route" (ko'cha)
// komponenti UMUMAN bo'lmaydi, faqat shahar darajasigacha ma'lumot beradi.
// Avvalgi kod FAQAT shu birinchi natijani tekshirib, undan pastdagi (aynan
// ko'cha ma'lumoti bor) natijalarga qaramay "Chust" bilan to'xtab qolar
// edi — shuning uchun BARCHA natijalar (va ularning barcha komponentlari)
// birga skanerlanadi, keyin eng aniq (mahalla+ko'cha+uy) birikma quriladi.
func shortAddress(d geocodeResponse) (string, bool) {
	if d.Status != "OK" || len(d.Results) == 0 {
		return "", false
	}
	hasType := func(types []string, want string) bool {
		return slices.Contains(types, want)
	}
	var route, houseNumber, sublocality, locality, district string
	for _, res := range d.Results {
		for _, c := range res.AddressComponents {
			// "Unnamed Road" — nomlanmagan yo'llar uchun standart yorliq;
			// highwayCodeRe — magistral yo'l raqamlari (R-121 va h.k.).
			// Ikkalasi ham mijozga ko'rsatib bo'lmaydigan "ko'cha nomi".
			if route == "" && hasType(c.Types, "route") &&
				!strings.EqualFold(c.LongName, "Unnamed Road") &&
				!highwayCodeRe.MatchString(strings.TrimSpace(c.LongName)) {
				route = c.LongName
			}
			if houseNumber == "" && hasType(c.Types, "street_number") {
				houseNumber = c.LongName
			}
			if sublocality == "" && (hasType(c.Types, "sublocality") ||
				hasType(c.Types, "sublocality_level_1") ||
				hasType(c.Types, "neighborhood")) {
				sublocality = c.LongName
			}
			if locality == "" && hasType(c.Types, "locality") {
				locality = c.LongName
			}
			// Tuman/shahar darajasi — locality ham topilmagan chekka
			// nuqtalar uchun oxirgi mazmunli zaxira (masalan "Chust tumani").
			if district == "" && hasType(c.Types, "administrative_area_level_2") {
				district = c.LongName
			}
		}
	}
	var parts []string
	if sublocality != "" {
		parts = append(parts, sublocality)
	}
	switch {
	case route != "" && houseNumber != "":
		parts = append(parts, route+", "+houseNumber)
	case route != "":
		parts = append(parts, route)
	}
	if len(parts) > 0 {
		return strings.Join(parts, ", "), route != ""
	}
	if locality != "" {
		return locality, false
	}
	if district != "" {
		return district, false
	}
	// Hech qanday tuzilgan komponent topilmadi (juda kam uchraydi) —
	// Google'ning xom `formatted_address`i odatda Plus Code bilan
	// boshlanadi ("27F4+4X, Chust, ..."), bu foydalanuvchiga tushunarsiz —
	// shuning uchun uni ko'rsatishdan ko'ra bo'sh qaytaramiz (frontend
	// "manzil aniqlanmadi" holatini ko'rsatadi, chalkash kodni emas).
	return "", false
}

// withCORS — brauzerdan kelgan so'rovlar uchun CORS ruxsatlari. Xuddi
// ws.NewHub bilan bir xil ALLOWED_ORIGINS ro'yxatidan foydalanadi (bitta
// manba — bugungi kunda Next.js frontend BFF pattern orqali ishlagani
// uchun brauzer to'g'ridan-to'g'ri bu API'ga REST so'rov yubormaydi, lekin
// bu qatlam kelajakdagi to'g'ridan-to'g'ri brauzer chaqiruvlari (masalan
// boshqa mini-app'lar) uchun ham himoya bo'lib qoladi). ALLOWED_ORIGINS
// bo'sh bo'lsa (dev) hamma origin qabul qilinadi — production'da SHART
// sozlanishi kerak. Origin header umuman yo'q so'rovlar (native mobil
// ilovalar, server-to-server chaqiruvlar) har doim ruxsat etiladi.
// withBodyLimit — HAR QANDAY so'rov tanasi uchun umumiy yuqori chegara.
//
// Avval faqat `/uploads` cheklangan edi; qolgan handler'lar
// `json.NewDecoder(r.Body)`ni chegarasiz o'qirdi. Ya'ni autentifikatsiya
// TALAB QILMAYDIGAN `/auth/request-code`ga yuz megabaytlik tana yuborib,
// serverni xotira buferlashga majburlash mumkin edi.
//
