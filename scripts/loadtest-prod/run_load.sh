#!/bin/sh
# PROD yuklama testi: resurs kuzatuvi + xavfsizlik to'xtatgichi + 10k WebSocket + aralash HTTP.
# Foydalanish: sh run_load.sh <run> <steps> <stage_s>
set -u
RUN="$1"; STEPS="$2"; STAGE="$3"
D="$HOME/loadtest-data"
STATS="$D/stats-$RUN.log"
: > "$STATS"
touch "$D/.running-$RUN"

# 1) Kuzatuv + to'xtatgich: har 5 s â€” yuk, RAM, konteynerlar. Yuk 18 dan (3x yadro) oshsa k6 va wsload to'xtatiladi.
(
  while [ -f "$D/.running-$RUN" ]; do
    L=$(cut -d' ' -f1 /proc/loadavg)
    echo "$(date +%T) load=$L mem=$(free -m | awk '/Mem:/{print $3"/"$2"MB"}')" >> "$STATS"
    docker stats --no-stream --format '  {{.Name}} cpu={{.CPUPerc}} mem={{.MemUsage}}' ondex-api-1 ondex-db-1 ondex-redis-1 ondex-mongo-1 k6-loadtest wsload-loadtest 2>/dev/null >> "$STATS"
    if awk -v l="$L" 'BEGIN{exit !(l>18)}'; then
      echo "$(date +%T) TO'XTATGICH: load=$L > 18 â€” test to'xtatildi" >> "$STATS"
      docker kill --signal=SIGINT k6-loadtest >/dev/null 2>&1; sleep 3; docker kill --signal=SIGINT wsload-loadtest >/dev/null 2>&1; rm -f "$D/.running-$RUN"
    fi
    sleep 5
  done
) &

# 2) 10 000 WebSocket ulanish (sekundiga 500), HTTP test davomida ushlab turiladi.
TOTAL_S=$(( ($(echo "$STEPS" | tr ',' '\n' | wc -l)) * (STAGE + 10) + 30 ))
docker rm -f wsload-loadtest >/dev/null 2>&1; docker run -d --name wsload-loadtest --network container:ondex-api-1 --cpus 1 \
  --user "$(id -u):$(id -g)" -v /tmp/wsload:/wsload:ro -v "$D:/data" \
  --entrypoint /wsload "$(docker inspect -f '{{.Config.Image}}' ondex-api-1)" \
  -url ws://localhost:8080/ws -tokens "/data/tokens-$RUN.json" -n 10000 -rate 500 -hold "${TOTAL_S}s" > /dev/null
sleep 30

# 3) Aralash HTTP (k6), 2 CPU bilan cheklangan.
docker run --rm --name k6-loadtest --network container:ondex-api-1 --cpus 2 \
  --user "$(id -u):$(id -g)" -v /tmp/k6scripts:/scripts:ro -v "$D:/data" -e K6_NO_USAGE_REPORT=true \
  grafana/k6:latest run --quiet -e ONDEX_API=http://localhost:8080 \
  -e RESTAURANT_ID="loadtest-$RUN-rest" -e TOKENS_FILE="/data/tokens-$RUN.json" \
  -e STEPS="$STEPS" -e STAGE_S="$STAGE" -e SUMMARY_OUT="/data/k6-$RUN.json" \
  /scripts/prod_load.js > "$D/k6-$RUN.txt" 2>&1
echo "k6 exit=$?" >> "$D/k6-$RUN.txt"

docker wait wsload-loadtest > /dev/null 2>&1 || true
docker logs wsload-loadtest > "$D/ws-$RUN.txt" 2>&1 || true
docker rm -f wsload-loadtest > /dev/null 2>&1 || true
rm -f "$D/.running-$RUN"
sleep 6
echo "TUGADI"
