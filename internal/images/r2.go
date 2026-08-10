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

func (s *R2Store) Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          r,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	})
	if err != nil {
		return "", err
	}
	return s.publicBaseURL + "/" + key, nil
}
