#!/usr/bin/env bash
#
# OnDex â€” SIRLAR VA SOZLAMALAR zaxirasi (.env, secrets/, docker-compose.yml,
# crontab), SHIFRLANGAN holda R2 `secrets/` ga.
#
# â”Œâ”€ NEGA KERAK â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
# Bazalar zaxirada, lekin `.env` (JWT siri, Octo/Multicard/R2 kalitlari)
# va `secrets/fcm.json` YO'Q edi. Server butunlay yo'qolsa bazani
# tiklash mumkin, lekin JWT siri ketadi (hamma sessiya yaroqsiz bo'ladi),
# to'lov provayderlari kalitlarini qaytadan olish kerak bo'ladi.
# â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
#
# â”Œâ”€ SHIFRLASH: NEGA ASIMMETRIK â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
# Parol (simmetrik kalit) serverda tursa, serverni qo'lga kiritgan odam
# zaxirani ham ochadi - shifrlashning foydasi yo'q. Shuning uchun
# serverda FAQAT OCHIQ SERTIFIKAT (`~/ondex/backup-cert.pem`) turadi:
# u bilan shifrlash mumkin, ochish mumkin EMAS. Maxfiy kalit (`.key.pem`)
# FAQAT sizda: parol menejerida / flesh-diskda, serverda va repoda YO'Q.
# â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
#
# KALIT JUFTINI YARATISH (o'z kompyuteringizda, bir marta):
#   openssl req -x509 -newkey rsa:4096 -sha256 -days 3650 -nodes \
#       -keyout ondex-backup-key.pem -out ondex-backup-cert.pem -subj "/CN=ondex-backup"
#   -> ondex-backup-key.pem ni XAVFSIZ joyga saqlang (YO'QOLSA ZAXIRANI OCHIB BO'LMAYDI)
#   -> faqat sertifikatni serverga: scp ondex-backup-cert.pem shajara:~/ondex/backup-cert.pem
#
# O'RNATISH (VPS'da): cron, kuniga bir marta, kunlik zaxiradan keyin:
#   30 3 * * * /bin/bash ~/ondex/backup-secrets.sh >> ~/ondex/backups/secrets.log 2>&1
#
# OCHISH (o'z kompyuteringizda, R2'dan yuklab olingan faylni):
#   openssl cms -decrypt -inform DER -in 2026-10-01_033000.tar.gz.cms \
#       -inkey ondex-backup-key.pem | tar -xzf - -C tiklangan/

set -euo pipefail
umask 077
export PATH="/usr/local/bin:/usr/bin:/bin:$PATH"

ONDEX_DIR="${ONDEX_DIR:-$HOME/ondex}"
CERT="${BACKUP_CERT:-$ONDEX_DIR/backup-cert.pem}"
KEEP="${KEEP_SECRET_BACKUPS:-30}"

cd "$ONDEX_DIR"
log() { echo "[$(date '+%F %T')] $*"; }

if [ ! -f "$CERT" ]; then
    log "XATO: shifrlash sertifikati yo'q: $CERT (yuqoridagi KALIT JUFTINI YARATISH ga qarang). Zaxira OLINMADI - ochiq holda saqlanmaydi."
    exit 1
fi

set -a
# shellcheck disable=SC1091
. ./.env
set +a

if [ -z "${BACKUP_R2_BUCKET:-}" ] || ! command -v aws >/dev/null 2>&1; then
    log "XATO: BACKUP_R2_BUCKET yoki aws CLI yo'q - sirlar zaxirasi yuboriladigan joy yo'q"
    exit 1
fi
if [ "${BACKUP_R2_BUCKET}" = "${R2_BUCKET:-}" ]; then
    log "XATO: BACKUP_R2_BUCKET rasmlar bucket'i bilan BIR XIL (ommaviy). Yuklanmadi."
    exit 1
fi

STAMP="$(date +%Y-%m-%d_%H%M%S)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
crontab -l > "$WORK/crontab.txt" 2>/dev/null || echo "# crontab yo'q" > "$WORK/crontab.txt"
FILE="$WORK/$STAMP.tar.gz.cms"

# Ochiq tar diskka YOZILMAYDI: to'g'ridan-to'g'ri shifrlovchiga uzatiladi.
tar -czf - -C "$HOME" ondex/.env ondex/secrets ondex/docker-compose.yml -C "$WORK" crontab.txt \
    | openssl cms -encrypt -binary -aes256 -outform DER -out "$FILE" "$CERT"

size=$(stat -c%s "$FILE")
if [ "$size" -lt 512 ]; then
    log "XATO: shifrlangan fayl juda kichik ($size bayt)"
    exit 1
fi
# Shifrlangan fayl haqiqatan shifrlangan: ichida .env dagi aniq qator ko'rinmasligi kerak.
if grep -aq "JWT_SECRET=" "$FILE"; then
    log "XATO: shifrlangan faylda ochiq matn topildi - to'xtatildi"
    exit 1
fi

export AWS_ACCESS_KEY_ID="${BACKUP_R2_ACCESS_KEY_ID:-${R2_ACCESS_KEY_ID:?R2 kaliti kerak}}"
export AWS_SECRET_ACCESS_KEY="${BACKUP_R2_SECRET_ACCESS_KEY:-${R2_SECRET_ACCESS_KEY:?R2 maxfiy kaliti kerak}}"
export AWS_DEFAULT_REGION="auto"
export AWS_REQUEST_CHECKSUM_CALCULATION="when_required"
export AWS_RESPONSE_CHECKSUM_VALIDATION="when_required"
EP="https://${R2_ACCOUNT_ID:?R2_ACCOUNT_ID kerak}.r2.cloudflarestorage.com"

aws s3 cp "$FILE" "s3://$BACKUP_R2_BUCKET/secrets/$STAMP.tar.gz.cms" --endpoint-url "$EP" --only-show-errors
remote=$(aws s3 ls "s3://$BACKUP_R2_BUCKET/secrets/$STAMP.tar.gz.cms" --endpoint-url "$EP" 2>/dev/null | awk '{print $3}')
if [ "${remote:-0}" != "$size" ]; then
    log "XATO: R2'dagi hajm ($remote) yuborilganga ($size) teng emas"
    exit 1
fi
log "sirlar zaxirasi OK: secrets/$STAMP.tar.gz.cms ($size bayt, shifrlangan)"

# Eski nusxalarni o'chirish: so'nggi $KEEP tasi qoladi.
aws s3 ls "s3://$BACKUP_R2_BUCKET/secrets/" --endpoint-url "$EP" 2>/dev/null \
    | awk '{print $4}' | grep -E '\.tar\.gz\.cms$' | sort | head -n "-$KEEP" \
    | while read -r old; do
        aws s3 rm "s3://$BACKUP_R2_BUCKET/secrets/$old" --endpoint-url "$EP" --only-show-errors || true
    done

METRIC_DIR="/var/lib/node_exporter/textfile"
if [ -d "$METRIC_DIR" ] && [ -w "$METRIC_DIR" ]; then
    tmp="$METRIC_DIR/.ondex_backup_secrets.$$"
    {
        echo "# HELP ondex_backup_secrets_last_success_timestamp_seconds Oxirgi muvaffaqiyatli sirlar zaxirasi (unix)."
        echo "# TYPE ondex_backup_secrets_last_success_timestamp_seconds gauge"
        echo "ondex_backup_secrets_last_success_timestamp_seconds $(date +%s)"
    } > "$tmp" && mv -f "$tmp" "$METRIC_DIR/ondex_backup_secrets.prom"
fi
