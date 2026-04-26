#!/usr/bin/env bash
# Run gosec (https://github.com/securego/gosec) against this module.
# Pin matches .github/workflows/gosec.yml and the upstream Docker action image tag.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

GOSEC_VERSION="${GOSEC_VERSION:-v2.25.0}"
GOBIN="$(go env GOPATH)/bin"
GOSEC="${GOSEC:-$GOBIN/gosec}"

if [[ ! -x "$GOSEC" ]] || [[ "${GOSEC_FORCE_INSTALL:-}" == "1" ]]; then
	echo "Installing gosec ${GOSEC_VERSION} to ${GOBIN}..."
	go install "github.com/securego/gosec/v2/cmd/gosec@${GOSEC_VERSION}"
	GOSEC="$GOBIN/gosec"
fi

extra_args=()
if [[ -f .gosec.json ]]; then
	extra_args+=(-conf .gosec.json)
fi

echo "=== gosec (${GOSEC_VERSION}) ==="
exec "$GOSEC" "${extra_args[@]}" -fmt text -sort ./...
