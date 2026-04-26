#!/usr/bin/env bash
# Build dnsplane-${VERSION_BASE}.tar.gz for rpmbuild SOURCES (prefix matches %autosetup).
# Uses git archive when inside a git work tree; otherwise copies the tree (e.g. CI
# container checkouts without .git).
set -euo pipefail

VERSION_BASE="${1:?usage: $0 <version_base> <output.tar.gz>}"
OUT="${2:?usage: $0 <version_base> <output.tar.gz>}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PREFIX="dnsplane-${VERSION_BASE}"

if git -C "$ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
	git -C "$ROOT" archive --format=tar --prefix="${PREFIX}/" HEAD | gzip -c >"$OUT"
	exit 0
fi

stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT
mkdir -p "$stage/$PREFIX"
(
	cd "$ROOT"
	shopt -s dotglob nullglob
	for p in *; do
		[[ "$p" == .git ]] && continue
		cp -a "$p" "$stage/$PREFIX/"
	done
)
tar -C "$stage" -czf "$OUT" "$PREFIX"
