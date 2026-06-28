#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CD="$ROOT/desktop"
export PATH="$PATH:$(go env GOPATH)/bin"

echo "[build-windows] building frontend..."
cd "$CD/frontend"
npm install
npm run build

cd "$CD"
mkdir -p "$CD/bin"

if command -v wails3 >/dev/null 2>&1; then
	echo "[build-windows] wails3 found; building with Wails task..."
	wails3 task windows:build ARCH=amd64
	cp "$CD/bin/anya.exe" "$CD/bin/anya-windows-amd64.exe"
else
	echo "[build-windows] wails3 not found; falling back to go build..."
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build \
	  -tags production \
	  -trimpath \
	  -ldflags="-w -s -H windowsgui" \
	  -o "$CD/bin/anya-windows-amd64.exe" \
	  .
fi

echo "[build-windows] done: $CD/bin/anya-windows-amd64.exe"
