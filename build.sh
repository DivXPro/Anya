#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# Elf Build Script
# Compiles both the desktop app and firmware
# Usage: ./build.sh [desktop|firmware|all]
# ============================================================

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DESKTOP_DIR="$SCRIPT_DIR/desktop"
FIRMWARE_DIR="$SCRIPT_DIR/firmware"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "${GREEN}[BUILD]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
err()  { echo -e "${RED}[ERROR]${NC} $1"; }

build_desktop() {
    log "Building desktop app..."

    export PATH="$PATH:$(go env GOPATH)/bin"

    if ! command -v wails3 &>/dev/null; then
        err "wails3 not found in PATH. Install with: go install github.com/wailsapp/wails/v3/cmd/wails3@latest"
        exit 1
    fi

    if ! command -v node &>/dev/null; then
        err "node not found. Install Node.js 18+."
        exit 1
    fi

    if ! command -v go &>/dev/null; then
        err "go not found. Install Go 1.22+."
        exit 1
    fi

    cd "$DESKTOP_DIR"

    # Install frontend deps if needed
    if [ ! -d "frontend/node_modules" ]; then
        log "Installing frontend dependencies..."
        (cd frontend && npm install)
    fi

    # Full Wails build (bindings + frontend + Go)
    wails3 build

    if [ -f "bin/elf" ]; then
        log "Desktop binary: $(realpath bin/elf) ($(du -sh bin/elf | cut -f1))"
        file bin/elf
    else
        err "Desktop build failed — bin/elf not found"
        exit 1
    fi
}

build_firmware() {
    log "Building firmware..."

    if ! command -v pio &>/dev/null; then
        err "PlatformIO CLI not found. Install with: pip install platformio"
        exit 1
    fi

    cd "$FIRMWARE_DIR"

    pio run --project-dir "$FIRMWARE_DIR"

    log "Firmware build complete."
    warn "To upload to device: cd firmware && pio run --target upload"
}

case "${1:-all}" in
    desktop)
        build_desktop
        ;;
    firmware)
        build_firmware
        ;;
    all)
        build_desktop
        echo ""
        build_firmware
        ;;
    *)
        echo "Usage: $0 [desktop|firmware|all]"
        exit 1
        ;;
esac

log "Done."
