#!/bin/sh
# Yuklama testidan OLDIN OnDex Postgres va Mongo zaxirasi (qiymatlar chiqarilmaydi).
set -eu
TS=$(date +%Y%m%d-%H%M)
D="$HOME/backups"
mkdir -p "$D"
docker exec ondex-db-1 sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' > "$D/ondex-pg-pre-loadtest-$TS.dump"
docker exec ondex-mongo-1 sh -c 'mongodump --quiet --archive --gzip ${MONGO_INITDB_ROOT_USERNAME:+--username "$MONGO_INITDB_ROOT_USERNAME" --password "$MONGO_INITDB_ROOT_PASSWORD" --authenticationDatabase admin}' > "$D/ondex-mongo-pre-loadtest-$TS.archive.gz"
chmod 600 "$D"/ondex-*-pre-loadtest-$TS.*
ls -la "$D"/ondex-*-pre-loadtest-$TS.*
gzip -t "$D/ondex-mongo-pre-loadtest-$TS.archive.gz" && echo MONGO_GZIP_OK
docker exec -i ondex-db-1 pg_restore -l < "$D/ondex-pg-pre-loadtest-$TS.dump" > /dev/null && echo PG_DUMP_OK
