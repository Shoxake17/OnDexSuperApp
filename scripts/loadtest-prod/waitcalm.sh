#!/bin/sh
# Qoldiq test konteynerlarini tekshiradi va server yuki 2 dan tushguncha (max ~7 daqiqa) kutadi.
docker ps -a --format '{{.Names}}' | grep -E 'k6-loadtest|wsload-loadtest' || echo "qoldiq konteyner yo'q"
i=0
while awk -v l="$(cut -d' ' -f1 /proc/loadavg)" 'BEGIN{exit !(l>2)}' && [ "$i" -lt 42 ]; do
  sleep 10; i=$((i+1))
done
uptime
