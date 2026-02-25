#!/usr/bin/env bash
set -e
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "=== gofmt ==="
if [ -n "$(gofmt -l .)" ]; then
	echo "gofmt: the following files need formatting:"
	gofmt -l .
	exit 1
fi
echo "gofmt: OK"

echo ""
echo "=== golangci-lint ==="
GOLANGCI_LINT="${GOLANGCI_LINT:-$(go env GOPATH)/bin/golangci-lint}"
if [ ! -x "$GOLANGCI_LINT" ]; then
	echo "golangci-lint not found at $GOLANGCI_LINT (install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)"
	exit 1
fi
"$GOLANGCI_LINT" run --timeout=5m
echo "golangci-lint: OK"
