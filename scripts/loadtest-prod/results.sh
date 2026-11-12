#!/bin/sh
cd "$HOME/loadtest-data"
R="$1"
echo "====== K6"; tail -32 "k6-$R.txt"
echo "====== WS"; tail -15 "ws-$R.txt"
echo "====== STATS"
grep -c 'load=' "stats-$R.log"
grep "TO'XTATGICH" "stats-$R.log" || echo "to'xtatgich ishlamadi"
awk '/ load=/{split($2,a,"="); if (a[2]+0 > m) m = a[2]+0} END{print "max load=" m}' "stats-$R.log"
echo "--- api/db cpu va xotira (eng yuqori)"
grep -E 'ondex-api-1|ondex-db-1|ondex-redis-1' "stats-$R.log" | sed 's/%//' \
 | awk '{n=$1; split($2,c,"="); if (c[2]+0 > mx[n]) {mx[n]=c[2]+0; mm[n]=$3}} END{for (k in mx) print k, "max_cpu=" mx[k] "%", mm[k]}'
echo "--- vaqt bo'yicha yuk (har 30 s)"
awk '/ load=/{i++; if (i%6==1) print $1, $2, $3}' "stats-$R.log"
