#!/usr/bin/env bash
# Emit dnsplane version parts for RPM/DEB/CI. FULL = BASE-SHORTSHA (e.g. 1.4.175-d977f1b).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

short_sha() {
	# GitHub Actions container checkouts often have no .git; use workflow commit SHA.
	if [[ -n "${GITHUB_SHA:-}" ]] && [[ "${#GITHUB_SHA}" -ge 7 ]]; then
		echo "${GITHUB_SHA:0:7}"
		return
	fi
	if git -C "$ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
		git -C "$ROOT" rev-parse --short=7 HEAD
		return
	fi
	echo "unknown"
}

base_version() {
	if git -C "$ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
		local tag
		if tag=$(git -C "$ROOT" describe --tags --match 'v[0-9]*.[0-9]*.[0-9]*' --abbrev=0 HEAD 2>/dev/null); then
			local stripped="${tag#v}"
			# Use semver core only (e.g. v1.4.175-rc1 or v1.4.175-d977f1b → 1.4.175).
			if [[ "$stripped" =~ ^([0-9]+\.[0-9]+\.[0-9]+) ]]; then
				echo "${BASH_REMATCH[1]}"
				return
			fi
			echo "$stripped"
			return
		fi
	fi
	# Tag builds in CI without .git (e.g. container jobs).
	if [[ -n "${GITHUB_REF_NAME:-}" ]]; then
		case "$GITHUB_REF_NAME" in
		v[0-9]*.[0-9]*.[0-9]*)
			local stripped="${GITHUB_REF_NAME#v}"
			if [[ "$stripped" =~ ^([0-9]+\.[0-9]+\.[0-9]+) ]]; then
				echo "${BASH_REMATCH[1]}"
				return
			fi
			echo "$stripped"
			return
			;;
		esac
	fi
	if [[ -f "$ROOT/VERSION" ]]; then
		tr -d ' \n\r\t' <"$ROOT/VERSION"
		return
	fi
	echo "0.0.0"
}

SHORTSHA="$(short_sha)"
BASE="$(base_version)"
FULL="${BASE}-${SHORTSHA}"

cmd="${1:-export}"
case "$cmd" in
export)
	printf "export DNSPLANE_VERSION_BASE=%q\n" "$BASE"
	printf "export DNSPLANE_GIT_SHORT=%q\n" "$SHORTSHA"
	printf "export DNSPLANE_VERSION_FULL=%q\n" "$FULL"
	;;
base) echo "$BASE" ;;
short | sha) echo "$SHORTSHA" ;;
full) echo "$FULL" ;;
*)
	echo "usage: $0 {export|base|short|full}" >&2
	exit 1
	;;
esac
