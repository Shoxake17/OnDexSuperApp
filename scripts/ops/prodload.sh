#!/bin/bash
# Production ondex-api'ga yuklama sinovi, serverning o'zidan (ichki tarmoq orqali).
#
# Rejimlar:
#   * ONDEX_TOKEN BERILMASA: faqat OCHIQ GET (menyu, restoranlar...), yozuv yo'q.
#   * ONDEX_TOKEN BERILSA : tokenli o'qish/hisoblash ham (quote, me/orders, unread).
#     quote BUYURTMA YARATMAYDI (faqat narx hisoblaydi), me/orders va unread - GET.
#     Skript POST /orders YUBORMAYDI: production bazasiga yozuv tushmaydi.
#
# Ishlatish:
#   STEPS=300,600,1000,1500 STAGE_S=20 bash prodload.sh
#   ONDEX_TOKEN=... STEPS=300,600,1000 STAGE_S=20 bash prodload.sh
#
# TOKEN HAQIDA: u buyruq satriga YOZILMAYDI (`docker run -e ONDEX_TOKEN` qiymatni muhitdan
# oladi), shuning uchun `ps` va `docker inspect`da tokenning o'zi ko'rinmaydi. Tokenni
# yuklama tugagach o'chiring: `unset ONDEX_TOKEN` (va u kirgan akkauntdan chiqish mumkin).
#
# QO'RIQCHI: server yuki (1 daqiqalik o'rtacha) 5.0 dan oshsa k6 ga SIGINT yuboriladi:
# k6 to'xtaydi VA hisobotni yozadi (SIGKILL hisobotni yo'qotardi). Chegarani ko'tarmang.
set -u
OUT="${OUT:-/tmp/prodload}"
NET="${NET:-shox-global-net}"
API_URL="${API_URL:-http://ondex-api:8080}"
mkdir -p "$OUT"; rm -f "$OUT"/*; chmod 777 "$OUT"
cp "${CAPACITY_JS:-/tmp/capacity.js}" /tmp/capacity.js 2>/dev/null || true
echo "image=$(docker inspect ondex-api-1 --format '{{.Config.Image}}')" > "$OUT/info.txt"
if [ -n "${ONDEX_TOKEN:-}" ]; then echo "rejim=tokenli" >> "$OUT/info.txt"; else echo "rejim=ochiq" >> "$OUT/info.txt"; fi
docker image inspect grafana/k6 >/dev/null 2>&1 || docker pull -q grafana/k6 >> "$OUT/info.txt" 2>&1

( while true; do
    echo "$(date +%T) load=$(cut -d' ' -f1 /proc/loadavg) $(docker stats --no-stream --format '{{.Name}}={{.CPUPerc}}/{{.MemUsage}}' ondex-api-1 ondex-db-1 ondex-redis-1 ondex-mongo-1 | tr '\n' ' ')" >> "$OUT/res.log"
    sleep 4
  done ) &
SAMP=$!

( while true; do
    L=$(cut -d' ' -f1 /proc/loadavg)
    if awk "BEGIN{exit !($L > 5.0)}"; then
      echo "QORIQCHI: load=$L > 5.0, k6 yumshoq to'xtatildi (hisobot saqlanadi)" >> "$OUT/info.txt"
      docker kill -s INT prodload-k6 >/dev/null 2>&1
      break
    fi
    sleep 2
  done ) &
GUARD=$!

ENVARGS=(-e "ONDEX_API=$API_URL" -e "STEPS=${STEPS:-300,600,1000,1500}" -e "STAGE_S=${STAGE_S:-20}" -e MAX_VUS=2000 -e SUMMARY_OUT=/out/summary.json)
if [ -n "${ONDEX_TOKEN:-}" ]; then ENVARGS+=(-e ONDEX_TOKEN); else ENVARGS+=(-e PUBLIC_ONLY=1); fi

docker run --rm --name prodload-k6 --network "$NET" --cpus 2 \
  -v /tmp/capacity.js:/capacity.js:ro -v "$OUT":/out \
  "${ENVARGS[@]}" grafana/k6 run --quiet /capacity.js > "$OUT/k6.txt" 2>&1
kill $SAMP $GUARD 2>/dev/null
echo DONE >> "$OUT/info.txt"
echo "--- $OUT/info.txt"; cat "$OUT/info.txt"
echo "--- $OUT/k6.txt"; tail -26 "$OUT/k6.txt"
echo "--- cho'qqi yuk"; sed -E 's/.*load=([0-9.]+).*/\1/' "$OUT/res.log" | sort -n | tail -1 | sed 's/^/eng yuqori load: /'
