module chustapp

// PATCH versiyasi ATAYLAB aniq ko'rsatilgan (1.25.0 emas).
//
// Bu qator toolchain uchun ENG KAM talab. `go 1.25.0` turganda CI
// `setup-go` ga aynan shuni o'rnatardi — ya'ni 1.25 ning ENG BIRINCHI
// relizini, 12 ta xavfsizlik patchisiz. `govulncheck` shunda standart
// kutubxonada 23 ta CVE topgandi (crypto/tls, crypto/x509, net/url,
// net/http) — ularning hammasi 1.25.6...1.25.12 da tuzatilgan.
//
// Yangi CVE chiqqanda bu raqamni oshiring. CI mos `1.26.x` ni ishlatadi
// va eng yangi patchni o'zi oladi, shuning uchun kunlik skanerlash
// ogohlantirishni oldindan beradi.
//
// 2026-yil ko'tarilishi: `govulncheck` 1.26.5 standart kutubxonasida
// kod HAQIQATAN chaqiradigan ikki zaiflikni topdi —
//   * GO-2026-5972 (encoding/asn1 rekursiya chuqurligi; FCM kalitini
//     o'qishda x509.ParsePKCS8PrivateKey orqali),
//   * GO-2026-5026 (net/http + x/net/idna Punycode; har http.Client.Do).
// Ikkalasi ham 1.26.6 da tuzatilgan, shuning uchun eng kam talab shu
// versiyaga ko'tarildi (GOTOOLCHAIN=auto uni o'zi yuklaydi).
go 1.26.6

require (
	github.com/aws/aws-sdk-go-v2 v1.45.1
	github.com/aws/aws-sdk-go-v2/config v1.33.2
	github.com/aws/aws-sdk-go-v2/credentials v1.20.2
	github.com/aws/aws-sdk-go-v2/service/s3 v1.110.0
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/gorilla/websocket v1.5.3
	github.com/jackc/pgx/v5 v5.10.0
	github.com/ledongthuc/pdf v0.0.0-20250511090121-5959a4027728
	github.com/mayahiro/go-webp v0.3.0
	github.com/pdfcpu/pdfcpu v0.15.0
	github.com/redis/go-redis/v9 v9.22.0
	go.mongodb.org/mongo-driver v1.17.9
	golang.org/x/crypto v0.56.0
	golang.org/x/image v0.45.0
	google.golang.org/genai v1.71.0
)

require (
	cloud.google.com/go v0.116.0 // indirect
	cloud.google.com/go/auth v0.18.2 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.20 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.19.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.11.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.20.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.8.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.36.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.41.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.48.0 // indirect
	github.com/aws/smithy-go v1.28.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.11 // indirect
	github.com/googleapis/gax-go/v2 v2.17.0 // indirect
	github.com/hhrutter/tiff v1.0.6 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/klauspost/compress v1.20.0 // indirect
	github.com/mattn/go-runewidth v0.0.27 // indirect
	github.com/montanaflynn/stats v0.7.1 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.2 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.61.0 // indirect
	go.opentelemetry.io/otel v1.44.0 // indirect
	go.opentelemetry.io/otel/metric v1.44.0 // indirect
	go.opentelemetry.io/otel/trace v1.44.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/grpc v1.83.2 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
