#!/usr/bin/env bash
SERVER="${1:-192.168.178.22}"
NAME="${2:-example.com}"
COUNT="${3:-500}"

for i in $(seq 1 "$COUNT"); do
  dig @"$SERVER" "$NAME" A +time=3 +tries=1 +noall +answer +stats 2>/dev/null \
    | awk -v n="$i" '/Query time:/ { if ($4+0 > 10) printf "%4d  %s ms\n", n, $4 }'
  sleep 4
done
