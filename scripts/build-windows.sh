#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CD="$ROOT/desktop"

echo "[build-windows] building frontend..."
cd "$CD/frontend"
npm install
npm run build

echo "[build-windows] cross-compiling Windows amd64 binary..."
cd "$CD"
mkdir -p "$CD/bin"
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build \
  -tags production \
  -trimpath \
  -ldflags="-w -s -H windowsgui" \
  -o "$CD/bin/anya-windows-amd64.exe" \
  .

echo "[build-windows] done: $CD/bin/anya-windows-amd64.exe"
