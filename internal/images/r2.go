package images

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// R2Store — Cloudflare R2 (S3-mos API) object storage. Rasmlar CDN orqali
// tarqatiladi, server holatidan mustaqil — production uchun to'g'ri yechim.
type R2Store struct {
	client        *s3.Client
	bucket        string
	publicBaseURL string
}

// NewR2Store — accountID, kalitlar va bucket nomi bo'yicha R2 klientini
// tayyorlaydi. publicBaseURL — bucket uchun yoqilgan ochiq domen
// (masalan https://pub-xxxx.r2.dev yoki maxsus domen).
func NewR2Store(ctx context.Context, accountID, accessKeyID, secretAccessKey, bucket, publicBaseURL string) (*R2Store, error) {
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("auto"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
		config.WithBaseEndpoint(endpoint),
	)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true // Cloudflare R2 uchun tavsiya etilgan rejim
	})
	return &R2Store{
		client:        client,
		bucket:        bucket,
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
	}, nil
}

// EnsurePublicReadCORS — bucket'ga "istalgan origin'dan O'QISH mumkin"
// qoidasini qo'yadi (agar hali qo'yilmagan bo'lsa).
//
// ┌─ NEGA KERAK: 3D MODELLAR ─────────────────────────────────────────┐
// Rasmlar `<img>` orqali ko'rsatiladi va `<img>` CORS TALAB QILMAYDI —
// shuning uchun bucket CORS'siz ham yillar davomida ishlab keldi.
//
// 3D model esa boshqacha: `<model-viewer>` (three.js GLTFLoader) GLB ni
// `fetch` orqali oladi, `fetch` esa boshqa domendan javob olish uchun
// `Access-Control-Allow-Origin` sarlavhasini TALAB qiladi. U bo'lmasa
// brauzer javobni butunlay bloklaydi va model hech qachon
// ko'rinmaydi — mijoz ilovasida ham, vebda ham.
//
// Bucket ALLAQACHON ommaviy (rasmlar ochiq tarqatiladi), ya'ni bu
// qoida hech qanday yangi ma'lumotni ochmaydi: faqat brauzerga
// "o'qishga ruxsat bor" deb aytadi.
//
// `EnsureMongoIndexes` bilan bir xil naqsh: infratuzilma sozlamasi
// kodda turadi va ishga tushishda o'zi qo'llanadi.
// └───────────────────────────────────────────────────────────────────┘
//
// Mavjud qoida BOR bo'lsa TEGILMAYDI — qo'lda sozlangan (ehtimol
// qattiqroq) siyosatni bosib ketmasligimiz kerak.
func (s *R2Store) EnsurePublicReadCORS(ctx context.Context) error {
	cur, err := s.client.GetBucketCors(ctx, &s3.GetBucketCorsInput{
		Bucket: aws.String(s.bucket),
	})
	// Xato — odatda "qoida yo'q" degani (R2 `NoSuchCORSConfiguration`
	// qaytaradi). Xatoni ajratib o'tirmaymiz: qoida o'qilmasa ham,
	// bo'lmasa ham, natija bir xil — qo'yishga urinamiz.
	if err == nil && len(cur.CORSRules) > 0 {
		return nil
	}
	_, err = s.client.PutBucketCors(ctx, &s3.PutBucketCorsInput{
		Bucket: aws.String(s.bucket),
		CORSConfiguration: &types.CORSConfiguration{
			CORSRules: []types.CORSRule{{
				AllowedOrigins: []string{"*"},
				// FAQAT o'qish. Yozish/o'chirish hech qachon brauzerdan
				// qilinmaydi — u serverning ishi.
				AllowedMethods: []string{"GET", "HEAD"},
				AllowedHeaders: []string{"*"},
				MaxAgeSeconds:  aws.Int32(86400),
			}},
		},
	})
	return err
}

// Delete — bucket'dan obyektni o'chiradi.
//
// Bucket OMMAVIY: unga yuklangan har qanday fayl manzilni bilgan
// hammaga ochiq. Ya'ni xato yuklangan faylni olib tashlashning yo'li
// BO'LISHI shart — aks holda yagona chora bucket'ni qo'lda titish
// bo'lardi.
func (s *R2Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

// PublicURL — bucket ichidagi kalitning ochiq manzili.
//
// `Upload` allaqachon shu manzilni qaytaradi; bu yerda u ALOHIDA
// chiqarildi, chunki obyekt R2 da turgan, lekin bazadagi havolasi
// yo'qolgan holat bor (`cmd/r2upload -link`). Manzilni chaqiruvchi
// tomonda qo'lda yig'ish `publicBaseURL` qoidasini ikki nusxaga
// bo'lardi.
func (s *R2Store) PublicURL(key string) string {
	return s.publicBaseURL + "/" + key
}

func (s *R2Store) Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
	return s.UploadWithOptions(ctx, key, r, size, UploadOptions{ContentType: contentType})
}

// UploadOptions — yuklanadigan obyektning HTTP sarlavhalari.
type UploadOptions struct {
	ContentType string
	// ContentDisposition — brauzer faylni ochmasdan YUKLAB olishi uchun
	// (masalan APK: `attachment; filename="OnDex-1.0.0.apk"`).
	ContentDisposition string
	// CacheControl — CDN va brauzer keshi (o'zgarmas reliz fayli uzoq,
	// "eng so'nggi" manifesti qisqa keshlanadi).
	CacheControl string
}

// UploadWithOptions — `Upload` ning sarlavhalar bilan varianti.
func (s *R2Store) UploadWithOptions(ctx context.Context, key string, r io.Reader, size int64, opt UploadOptions) (string, error) {
	in := &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          r,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(opt.ContentType),
	}
	if opt.ContentDisposition != "" {
		in.ContentDisposition = aws.String(opt.ContentDisposition)
	}
	if opt.CacheControl != "" {
		in.CacheControl = aws.String(opt.CacheControl)
	}
	if _, err := s.client.PutObject(ctx, in); err != nil {
		return "", err
	}
	return s.PublicURL(key), nil
}
