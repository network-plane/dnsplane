#!/usr/bin/env sh
# Examples: REST API stats dashboard (requires "api": true and apiport reachable).
# Optional: Authorization: Bearer <api_auth_token> when api_auth_token is set in dnsplane.json.
#
# Resolutions log JSON (in-memory ring; size = dashboard_resolution_log_cap, default 1000):
#   cap, count, resolutions (newest first; client_ip, qname, qtype, outcome, upstream, record, duration_ms, at)

set -eu
BASE="${BASE:-http://127.0.0.1:8080}"

curl -sS "${BASE}/stats/dashboard"
printf '\n'

curl -sS "${BASE}/stats/dashboard/data"
printf '\n'

curl -sS "${BASE}/stats/dashboard/resolutions"
printf '\n'

curl -sS -X POST "${BASE}/stats/dashboard/resolutions/purge"
printf '\n'
