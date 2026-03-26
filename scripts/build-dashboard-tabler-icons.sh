#!/usr/bin/env bash
# Rebuild api/embedded/tabler_icons.bundle from Tabler Icons (MIT).
# Source: https://github.com/tabler/tabler-icons — npm @tabler/icons outline SVGs via jsDelivr.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/api/embedded/tabler_icons.bundle"
VER="${TABLER_ICONS_VERSION:-3.40.0}"
BASE="https://cdn.jsdelivr.net/npm/@tabler/icons@${VER}/icons/outline"

# minify_svg: single-line SVG, collapsed whitespace (keeps required spaces in path data).
minify_svg() {
	tr -d '\n' | tr -s '[:space:]' ' ' | sed 's/^ //;s/ $//'
}

# Each line: logical_key<TAB>tabler-icon-filename (without .svg)
fetch_pair() {
	local key="$1"
	local tabler_name="$2"
	local tmp
	tmp="$(mktemp)"
	if ! curl -fsSL "$BASE/${tabler_name}.svg" -o "$tmp"; then
		echo "build-dashboard-tabler-icons: failed to fetch ${tabler_name}.svg" >&2
		exit 1
	fi
	printf '%s\t%s\n' "$key" "$(minify_svg <"$tmp")"
	rm -f "$tmp"
}

mkdir -p "$(dirname "$OUT")"
{
	echo "# tabler_icons.bundle — built by scripts/build-dashboard-tabler-icons.sh"
	echo "# Tabler Icons: MIT License — https://github.com/tabler/tabler-icons"
	echo "# Package: @tabler/icons@${VER} (outline)"
	echo "#"
	fetch_pair uptime clock
	fetch_pair version versions
	fetch_pair queries activity
	fetch_pair answered circle-check
	fetch_pair forwarded arrow-forward
	fetch_pair upstreams server-2
	fetch_pair cluster network
	fetch_pair cache_hits bolt
	fetch_pair cache_ratio chart-pie
	fetch_pair cache_entries database
	fetch_pair cache_compact vacuum-cleaner
	fetch_pair avg_resolve gauge
	fetch_pair upstream_wins chart-arrows
	fetch_pair perf_samples chart-dots
	fetch_pair out_local home
	fetch_pair out_cache archive
	fetch_pair out_upstream cloud-upload
	fetch_pair out_none ban
	fetch_pair block_rate shield-exclamation
	fetch_pair blocks shield-x
	fetch_pair fs_domains world
	fetch_pair fs_requesters users
	fetch_pair fs_dom_session world-search
	fetch_pair fs_req_session users-group
	fetch_pair chart_replies chart-line
	fetch_pair chart_latency timeline
	fetch_pair log_resolutions list-details
	# Section kickers (h2)
	fetch_pair sec_traffic route
	fetch_pair sec_cache database
	fetch_pair sec_fast bolt
	fetch_pair sec_outcomes chart-dots
	fetch_pair sec_adblock shield
	fetch_pair sec_fullstats chart-bar
	fetch_pair sec_trends chart-area-line
} >"$OUT"

echo "Wrote $OUT"
