# OnDex Go API — production image.
#
# ┌─ NEGA IKKI BOSQICH ───────────────────────────────────────────────┐
# Go kompilyatori, modul keshi va manba kod ~800 MB joy egallaydi va
# ularning HECH BIRI ishlash vaqtida kerak emas. Yakuniy image'da
# faqat bitta statik binar qoladi (~30 MB).
#
# Xavfsizlik nuqtai nazaridan bu asosiy narsa: image'ga tushmagan
# kutubxonada zaiflik ham bo'lmaydi.
# └───────────────────────────────────────────────────────────────────┘

# ---------- 1-bosqich: yig'ish ----------
FROM golang:1.25-alpine AS build

WORKDIR /src

# Bog'liqliklar ALOHIDA qatlamda: manba kod o'zgarganda ular qayta
# yuklab olinmaydi (Docker qatlam keshi).
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

# CGO_ENABLED=0 — to'liq statik binar. Busiz distroless/scratch
# image'da `no such file or directory` chiqadi (libc yo'q).
#
# -trimpath — binardan qurish mashinasining fayl yo'llari olib
# tashlanadi (masalan /home/runner/work/...). Ular xato izlarida
# ko'rinib, ichki tuzilma haqida ma'lumot berardi.
#
# -s -w — debug belgilari olib tashlanadi (kichikroq binar, teskari
# muhandislik biroz qiyinlashadi).
#
# Migratsiyalar `go:embed` bilan binar ICHIDA (internal/storage/
# migrate.go), shuning uchun .sql fayllarni nusxalash SHART EMAS.
RUN CGO_ENABLED=0 GOOS=linux go build \
        -trimpath \
        -ldflags="-s -w" \
        -o /out/api \
        ./cmd/api

# ---------- 2-bosqich: ishlash ----------
#
# distroless/static — ichida shell, paket menejeri, coreutils YO'Q.
# Hujumchi RCE topsa ham `sh`, `curl`, `wget` topa olmaydi.
#
# `:nonroot` tegi — konteyner root'dan EMAS, uid 65532 ostida ishlaydi.
FROM gcr.io/distroless/static-debian12:nonroot

# TLS sertifikatlari (Google FCM, R2, SMTP, Telegram — hammasi HTTPS).
# distroless/static ularni o'zi olib keladi, lekin aniq yozib
# qo'yamiz: bazani almashtirganda unutilmasin.
COPY --from=build /out/api /api

USER nonroot:nonroot
EXPOSE 8080

# ENTRYPOINT (CMD emas) — `docker run <image> sh` bilan buyruqni
# almashtirib bo'lmaydi.
ENTRYPOINT ["/api"]
