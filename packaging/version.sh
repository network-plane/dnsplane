#!/usr/bin/env bash
# Emit dnsplane version parts for RPM/DEB/CI. FULL = BASE-SHORTSHA (e.g. 1.4.175-d977f1b).
# Go version: ./packaging/version.sh go → from go.mod (toolchain line, else "go" directive).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Effective Go version for builds (toolchain line wins, else "go" directive). Matches go.mod only.
go_mod_go_version() {
	awk '
		/^toolchain go/ {
			t = $2
			sub(/^go/, "", t)
			print t
			exit
		}
		/^go[ \t]/ { g = $2 }
		END {
			if (g != "") {
				print g
				exit
			}
			print "go.mod: missing go directive" > "/dev/stderr"
			exit 1
		}
	' "$ROOT/go.mod"
}

# RPM package repos typically provide Go at major.minor granularity.
# Convert go.mod version (e.g. 1.26.2) to 1.26 for BuildRequires.
go_rpm_min_version() {
	local v="$1"
	awk -v ver="$v" 'BEGIN {
		n = split(ver, a, ".")
		if (n < 2) {
			print "invalid go version: " ver > "/dev/stderr"
			exit 1
		}
		print a[1] "." a[2]
	}'
}

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
GO_MOD="$(go_mod_go_version)"
GO_RPM_MIN="$(go_rpm_min_version "$GO_MOD")"

cmd="${1:-export}"
case "$cmd" in
export)
	printf "export DNSPLANE_VERSION_BASE=%q\n" "$BASE"
	printf "export DNSPLANE_GIT_SHORT=%q\n" "$SHORTSHA"
	printf "export DNSPLANE_VERSION_FULL=%q\n" "$FULL"
	printf "export DNSPLANE_GO_MOD=%q\n" "$GO_MOD"
	printf "export DNSPLANE_GO_RPM_MIN=%q\n" "$GO_RPM_MIN"
	;;
base) echo "$BASE" ;;
short | sha) echo "$SHORTSHA" ;;
full) echo "$FULL" ;;
go) echo "$GO_MOD" ;;
*)
	echo "usage: $0 {export|base|short|full|go}" >&2
	exit 1
	;;
esac
