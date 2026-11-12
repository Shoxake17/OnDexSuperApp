#!/bin/sh
# k6 ni API konteyneri tarmog'ida ishga tushiradi (Cloudflare/Caddy'siz, origin o'lchovi).
# Foydalanish: sh k6run.sh <skript.js> <cpus> [k6 -e argumentlari...]
set -eu
SCRIPT="$1"; CPUS="$2"; shift 2
docker run --rm --network container:ondex-api-1 --cpus "$CPUS" \
  --user "$(id -u):$(id -g)" \
  -v /tmp/k6scripts:/scripts:ro -v "$HOME/loadtest-data:/data" \
  -e K6_NO_USAGE_REPORT=true \
  grafana/k6:latest run --quiet -e ONDEX_API=http://localhost:8080 "$@" "/scripts/$SCRIPT"
