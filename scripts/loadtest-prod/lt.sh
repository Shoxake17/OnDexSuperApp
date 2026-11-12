#!/bin/sh
# loadtest vositasini bir martalik konteynerda ishga tushiradi.
# Sirlar faqat server ichidagi vaqtinchalik faylda (600), ish tugagach o'chiriladi;
# hech narsa ekranga chiqarilmaydi. Foydalanish: sh lt.sh <loadtest argumentlari>
set -eu
IMG=$(docker inspect -f '{{.Config.Image}}' ondex-api-1)
ENVF=$(mktemp /tmp/lt.env.XXXXXX)
chmod 600 "$ENVF"
trap 'rm -f "$ENVF"' EXIT
docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' ondex-api-1 \
  | grep -E '^(DATABASE_URL|MONGODB_URI|MONGODB_DB|JWT_SECRET)=' > "$ENVF"
mkdir -p "$HOME/loadtest-data"
chmod 700 "$HOME/loadtest-data"
docker run --rm --network container:ondex-api-1 --read-only \
  --user "$(id -u):$(id -g)" \
  --env-file "$ENVF" \
  -v /tmp/loadtest:/lt:ro -v "$HOME/loadtest-data:/data" \
  --entrypoint /lt "$IMG" "$@" -dir /data
