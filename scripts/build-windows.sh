#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CD="$ROOT/desktop"
export PATH="$PATH:$(go env GOPATH)/bin"

PACKAGE=false
INSTALL_SCOPE="${INSTALL_SCOPE:-user}"
ARCH="${ARCH:-amd64}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --package)
      PACKAGE=true
      shift
      ;;
    --machine)
      INSTALL_SCOPE=machine
      shift
      ;;
    --user)
      INSTALL_SCOPE=user
      shift
      ;;
    -h|--help)
      echo "Usage: $0 [--package] [--machine|--user]"
      echo ""
      echo "Options:"
      echo "  --package   Also build an NSIS installer (requires wails3 + makensis)"
      echo "  --machine   Install for all users (default: --user)"
      echo "  --user      Install for current user only"
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      exit 1
      ;;
  esac
done

echo "[build-windows] building frontend..."
cd "$CD/frontend"
npm install
npm run build

cd "$CD"
mkdir -p "$CD/bin"

build_exe() {
  if command -v wails3 >/dev/null 2>&1; then
    echo "[build-windows] wails3 found; building with Wails task..."
    wails3 task windows:build ARCH="$ARCH"
    cp "$CD/bin/anya.exe" "$CD/bin/anya-windows-$ARCH.exe"
  else
    echo "[build-windows] wails3 not found; falling back to go build..."
    GOOS=windows GOARCH="$ARCH" CGO_ENABLED=0 go build \
      -tags production \
      -trimpath \
      -ldflags="-w -s -H windowsgui" \
      -o "$CD/bin/anya-windows-$ARCH.exe" \
      .
  fi
}

build_exe

if [[ "$PACKAGE" == true ]]; then
  if ! command -v wails3 >/dev/null 2>&1; then
    echo "[build-windows] error: --package requires wails3." >&2
    exit 1
  fi
  if ! command -v makensis >/dev/null 2>&1; then
    echo "[build-windows] error: --package requires makensis (e.g. brew install nsis)." >&2
    exit 1
  fi
  echo "[build-windows] building NSIS installer (scope=$INSTALL_SCOPE, arch=$ARCH)..."
  wails3 task windows:package INSTALL_SCOPE="$INSTALL_SCOPE" ARCH="$ARCH"
  cp "$CD/bin/anya-$ARCH-installer.exe" "$CD/bin/anya-windows-$ARCH-installer.exe"
  echo "[build-windows] done: $CD/bin/anya-windows-$ARCH-installer.exe"
else
  echo "[build-windows] done: $CD/bin/anya-windows-$ARCH.exe"
fi
