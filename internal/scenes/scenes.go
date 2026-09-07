// 3D maket fayllarini XAVFSIZ tarqatish.
//
// ┌─ MUAMMO ───────────────────────────────────────────────────────────┐
// Maket (`.pck`, ~100-220 MB) ilgari rasmlar bilan BIR XIL ommaviy R2
// bucket'ida turardi va manzili `GET /restaurants` javobida
// autentifikatsiyasiz qaytardi. Ya'ni havolani bir marta ko'rgan har
// kim uni cheksiz yuklab olardi — qidiruv botlari ham.
//
// Rasm uchun bu me'yor (u baribir ommaviy), maket uchun esa yo'q: u
// restoranning ichki maketi va ichida BAJARILADIGAN kod bor.
// └────────────────────────────────────────────────────────────────────┘
//
// ┌─ YECHIM VA UNING CHEGARASI ────────────────────────────────────────┐
// Maket ALOHIDA, YOPIQ bucket'da saqlanadi va faqat tizimga kirgan
// foydalanuvchiga MUDDATI TUGAYDIGAN imzolangan havola beriladi.
//
// Bu "ko'rish mumkin, yuklash mumkin emas" DEGANI EMAS — bunday narsa
// printsipial jihatdan mumkin emas: faylni ilova ko'rsatishi uchun u
// qurilmaga tushishi shart. Erishiladigan narsa boshqa va u haqiqiy:
//
//   - havola ommaviy katalogda TURMAYDI;
//   - faqat kirgan foydalanuvchi oladi;
//   - havola qisqa muddatdan keyin O'LADI, ya'ni uni ulashib
//     bo'lmaydi va u qidiruvga tushmaydi.
//
// └────────────────────────────────────────────────────────────────────┘
package scenes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// DefaultTTL — imzolangan havolaning amal qilish muddati.
//
// Tanlov: maket 220 MB gacha va sekin internetda uzoq yuklanadi,
// shuning uchun bir necha daqiqa YETARLI EMAS. Bir soat — yuklashga
// bemalol yetadi, lekin havolani "doimiy" ham qilmaydi.
const DefaultTTL = time.Hour

// Signer — yopiq bucket'dagi obyekt uchun vaqtinchalik havola beradi.
type Signer struct {
	presign *s3.PresignClient
	bucket  string
	ttl     time.Duration
}

// New — R2 hisob ma'lumotlari bo'yicha imzolovchi.
//
// `bucket` — maketlar uchun ALOHIDA, ommaviy bo'lmagan bucket. Uni
// rasmlar bucket'i bilan bir xil qilib bo'lmaydi: R2 da ommaviylik
// butun bucket uchun yoqiladi, ya'ni bitta joyda saqlansa maket ham
// ommaviy bo'lib qolardi.
func New(ctx context.Context, accountID, accessKeyID, secretAccessKey, bucket string, ttl time.Duration) (*Signer, error) {
	if accountID == "" || accessKeyID == "" || secretAccessKey == "" || bucket == "" {
		return nil, errors.New("scenes: R2 sozlamalari to'liq emas")
	}
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("auto"),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
		config.WithBaseEndpoint(endpoint),
	)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true // R2 uchun tavsiya etilgan rejim
	})
	// ┌─ NEGA BU YERDA TEKSHIRUV KERAK ────────────────────────────────┐
	// Imzolash LOKAL amal: `PresignGetObject` R2 ga UMUMAN murojaat
	// qilmaydi, u shunchaki URL ni imzolaydi. Ya'ni kalitda bu
	// bucket'ga ruxsat bo'lmasa ham server "to'g'ri ko'rinadigan"
	// havola beraveradi va xato faqat MIJOZ yuklab olmoqchi
	// bo'lganda, 403 bo'lib chiqadi — serverda esa hech qanday iz
	// qolmaydi.
	//
	// Shuning uchun ruxsat ishga tushishda BIR MARTA tekshiriladi.
	// R2 API tokeni ko'pincha aniq bucket'larga bog'lanadi, ya'ni
	// yangi bucket ochilganda tokenni ham yangilash kerak bo'ladi —
	// bu eng ko'p uchraydigan xato.
	// └────────────────────────────────────────────────────────────────┘
	if _, err := client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &bucket}); err != nil {
		return nil, fmt.Errorf("scenes: %q bucket'iga kirib bo'lmadi — "+
			"R2 API tokenida shu bucket uchun ruxsat bormi? (%w)", bucket, err)
	}

	return &Signer{
		presign: s3.NewPresignClient(client),
		bucket:  bucket,
		ttl:     ttl,
	}, nil
}

// SignedURL — obyekt kaliti uchun muddatli havola.
func (s *Signer) SignedURL(ctx context.Context, key string) (string, error) {
	key = strings.TrimPrefix(strings.TrimSpace(key), "/")
	if key == "" {
		return "", errors.New("scenes: obyekt kaliti bo'sh")
	}
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	}, s3.WithPresignExpires(s.ttl))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// ObjectKey — saqlangan qiymatdan R2 obyekt kalitini ajratadi.
//
// ┌─ NEGA IKKI SHAKL QO'LLAB-QUVVATLANADI ─────────────────────────────┐
// Admin panel maket manzilini TO'LIQ URL sifatida yuboradi va u
// bazada shundayligicha saqlangan. Panelni o'zgartirmaslik uchun bu
// yerda ikkala shakl ham qabul qilinadi:
//
//	https://pub-xxx.r2.dev/scenes/bookcafe.pck  ->  scenes/bookcafe.pck
//	scenes/bookcafe.pck                         ->  scenes/bookcafe.pck
//
// Ya'ni mavjud yozuvlar ham, kelajakda kalit sifatida kiritilganlar
// ham ishlaydi.
// └────────────────────────────────────────────────────────────────────┘
func ObjectKey(stored, publicBaseURL string) string {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return ""
	}
	base := strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
	if base != "" && strings.HasPrefix(stored, base+"/") {
		return strings.TrimPrefix(stored, base+"/")
	}
	// To'liq URL, lekin boshqa domen — kalitni ajratib bo'lmaydi.
	// Bunday qiymat ATAYLAB rad etiladi: noma'lum domenga imzo
	// qo'yish mumkin emas va uni shundayligicha qaytarish ham
	// xavfsiz emas (`isAllowedSceneURL` uni kiritishga qo'ymaydi).
	if strings.Contains(stored, "://") {
		return ""
	}
	return strings.TrimPrefix(stored, "/")
}
