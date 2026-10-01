#!/usr/bin/env bash
#
# OnDex â€” SOATLIK zaxira (PostgreSQL + MongoDB) -> R2 `hourly/`.
#
# â”Œâ”€ NEGA KERAK â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
# Kunlik zaxira (`backup.sh`, 03:15) bilan baza yo'qolsa 24 soatgacha
# buyurtma va to'lov ma'lumoti ketishi mumkin (RPO = 24 soat). Soatlik
# nusxa buni 1 soatga tushiradi.
#
# Baza hozir KICHIK (daraja: ~50 KB), shuning uchun to'liq dump arzon.
# Baza ~1 GB dan oshganda to'liq dump o'rniga Postgres WAL arxivlash
# (nuqtaga tiklash) ga o'tish kerak: dump vaqti va yuki baza bilan o'sadi.
# â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
#
# O'RNATISH (VPS'da, bir marta):
#   mkdir -p ~/ondex/backups-hourly
#   cp backup-hourly.sh ~/ondex/backup-hourly.sh && chmod +x ~/ondex/backup-hourly.sh
#   crontab -e
#     5 * * * * /bin/bash ~/ondex/backup-hourly.sh >> ~/ondex/backups-hourly/backup.log 2>&1
#
# Kunlik `backup.sh` ga TEGMAYDI: u mustaqil ishlayveradi (03:15).

set -euo pipefail
export PATH="/usr/local/bin:/usr/bin:/bin:$PATH"

ONDEX_DIR="${ONDEX_DIR:-$HOME/ondex}"
BACKUP_DIR="${BACKUP_DIR:-$ONDEX_DIR/backups-hourly}"
KEEP_HOURS="${KEEP_HOURS:-48}"          # lokal: so'nggi 48 soat
KEEP_REMOTE_DAYS="${KEEP_REMOTE_DAYS:-3}" # R2 `hourly/`: so'nggi 3 kun (kunlik nusxalar alohida, uzoq)

cd "$ONDEX_DIR"
mkdir -p "$BACKUP_DIR"

log() { echo "[$(date '+%F %T')] $*"; }

# Ikki ishga tushish bir-biriga tegmasin (oldingisi tugamagan bo'lsa).
exec 9>"$BACKUP_DIR/.lock"
if ! flock -n 9; then
    log "oldingi soatlik zaxira hali tugamagan - o'tkazib yuborildi"
    exit 0
fi

set -a
# shellcheck disable=SC1091
. ./.env
set +a

STAMP="$(date +%Y-%m-%d_%H%M%S)"
OUT="$BACKUP_DIR/$STAMP"
mkdir -p "$OUT"
fail=0

log "soatlik zaxira boshlandi -> $OUT"

docker compose exec -T db pg_dump \
    -U "${POSTGRES_USER:-chust}" -d "${POSTGRES_DB:-chustapp}" \
    --clean --if-exists | gzip > "$OUT/postgres.sql.gz"

docker compose exec -T mongo mongodump \
    -u "${MONGO_USER:-ondex}" -p "${MONGO_PASSWORD:?MONGO_PASSWORD kerak}" \
    --authenticationDatabase admin --db "${MONGODB_DB:-chustapp}" \
    --archive --gzip > "$OUT/mongo.archive.gz" 2>/dev/null

# Tekshiruv: hajm (kesilmaganmi) va gzip yaxlitligi. Kunlik skript bilan
# bir xil mantiq: mazmunni fayl hajmi bilan emas, yaxlitlik bilan baholaymiz.
check() {
    local f="$1" min="$2" size
    size=$(stat -c%s "$f" 2>/dev/null || echo 0)
    if [ "$size" -lt "$min" ]; then
        log "XATO: $(basename "$f") kesilgan yoki bo'sh ($size bayt)"; fail=1
    elif ! gunzip -t "$f" 2>/dev/null; then
        log "XATO: $(basename "$f") buzilgan (gunzip -t o'tmadi)"; fail=1
    else
        log "OK: $(basename "$f") - $size bayt"
    fi
}
check "$OUT/postgres.sql.gz" 1024
check "$OUT/mongo.archive.gz" 200

# R2 ga yuklash (yopiq bucket) va QAYTA O'QIB tasdiqlash.
if [ -n "${BACKUP_R2_BUCKET:-}" ] && command -v aws >/dev/null 2>&1; then
    if [ "${BACKUP_R2_BUCKET}" = "${R2_BUCKET:-}" ]; then
        log "XATO: BACKUP_R2_BUCKET rasmlar bucket'i bilan BIR XIL (ommaviy). Yuklanmadi."
        fail=1
    else
        export AWS_ACCESS_KEY_ID="${BACKUP_R2_ACCESS_KEY_ID:-${R2_ACCESS_KEY_ID:?R2 kaliti kerak}}"
        export AWS_SECRET_ACCESS_KEY="${BACKUP_R2_SECRET_ACCESS_KEY:-${R2_SECRET_ACCESS_KEY:?R2 maxfiy kaliti kerak}}"
        export AWS_DEFAULT_REGION="auto"
        export AWS_REQUEST_CHECKSUM_CALCULATION="when_required"
        export AWS_RESPONSE_CHECKSUM_VALIDATION="when_required"
        EP="https://${R2_ACCOUNT_ID:?R2_ACCOUNT_ID kerak}.r2.cloudflarestorage.com"

        if aws s3 sync "$OUT" "s3://$BACKUP_R2_BUCKET/hourly/$STAMP/" \
            --endpoint-url "$EP" --only-show-errors; then
            remote=$(aws s3 ls "s3://$BACKUP_R2_BUCKET/hourly/$STAMP/" --endpoint-url "$EP" 2>/dev/null | wc -l)
            if [ "${remote:-0}" -ge 2 ]; then
                log "R2: OK - $remote fayl tasdiqlandi"
            else
                log "XATO: R2'da fayllar topilmadi (ko'ringani: ${remote:-0})"; fail=1
            fi
        else
            log "XATO: R2 ga yuklab bo'lmadi"; fail=1
        fi

        # R2 dagi eski soatlik nusxalarni o'chirish (kamida bir nechta qoladi).
        cutoff=$(date -d "-${KEEP_REMOTE_DAYS} days" +%Y-%m-%d)
        aws s3 ls "s3://$BACKUP_R2_BUCKET/hourly/" --endpoint-url "$EP" 2>/dev/null \
            | awk '{print $2}' | tr -d '/' | while read -r d; do
                day="${d%%_*}"
                if [[ "$day" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] && [[ "$day" < "$cutoff" ]]; then
                    aws s3 rm "s3://$BACKUP_R2_BUCKET/hourly/$d/" --recursive --endpoint-url "$EP" --only-show-errors || true
                fi
            done
    fi
else
    log "OGOHLANTIRISH: BACKUP_R2_BUCKET yoki aws CLI yo'q - faqat lokal nusxa"
fi

# Lokal eski nusxalarni o'chirish.
find "$BACKUP_DIR" -mindepth 1 -maxdepth 1 -type d -mmin "+$((KEEP_HOURS * 60))" -exec rm -rf {} + 2>/dev/null || true

if [ "$fail" -ne 0 ]; then
    log "SOATLIK ZAXIRA TO'LIQ EMAS - yuqoridagi xatolarni ko'ring"
    exit 1
fi
log "soatlik zaxira tugadi"

# Kuzatuv metrikasi (atomik yozuv); Prometheus qoidasi 2 soatdan ortiq
# yangilanmasa ogohlantiradi.
METRIC_DIR="/var/lib/node_exporter/textfile"
if [ -d "$METRIC_DIR" ] && [ -w "$METRIC_DIR" ]; then
    tmp="$METRIC_DIR/.ondex_backup_hourly.$$"
    {
        echo "# HELP ondex_backup_hourly_last_success_timestamp_seconds Oxirgi muvaffaqiyatli soatlik zaxira (unix)."
        echo "# TYPE ondex_backup_hourly_last_success_timestamp_seconds gauge"
        echo "ondex_backup_hourly_last_success_timestamp_seconds $(date +%s)"
    } > "$tmp" && mv -f "$tmp" "$METRIC_DIR/ondex_backup_hourly.prom"
fi
