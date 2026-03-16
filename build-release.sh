#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${ROOT_DIR}"

mkdir -p ./bin

if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
  COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo none)}"
else
  VERSION="${VERSION:-dev}"
  COMMIT="${COMMIT:-none}"
fi
BUILD_DATE="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

CGO_ENABLED=0 \
GOOS="${GOOS:-linux}" \
go build \
  -trimpath \
  -buildvcs=false \
  -mod=vendor \
  -tags timetzdata \
  -ldflags="-s -w -X 'main.Version=${VERSION}' -X 'main.Commit=${COMMIT}' -X 'main.BuildDate=${BUILD_DATE}'" \
  -o ./bin/codex-proxy \
  ./cmd/server
