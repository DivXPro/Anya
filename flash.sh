#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# Anya Firmware Flash Script
# Compiles and uploads firmware to M5StickC S3
# Usage: ./flash.sh [upload|monitor|clean]
# ============================================================

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
FIRMWARE_DIR="$SCRIPT_DIR/firmware"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log()  { echo -e "${GREEN}[FLASH]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
err()  { echo -e "${RED}[ERROR]${NC} $1"; }
info() { echo -e "${BLUE}[INFO]${NC} $1"; }

check_pio() {
    if ! command -v pio &>/dev/null; then
        err "PlatformIO CLI not found."
        echo "  Install: pip install platformio"
        echo "  Or:      brew install platformio"
        exit 1
    fi
}

do_upload() {
    log "Compiling and uploading firmware to M5StickC S3..."
    info "Make sure the device is connected via USB-C."
    echo ""

    cd "$FIRMWARE_DIR"
    pio run --target upload

    log "Upload complete!"
    echo ""
    info "To see device output, run: ./flash.sh monitor"
}

do_monitor() {
    log "Opening serial monitor (115200 baud)..."
    info "Press Ctrl+A then Ctrl+\ to exit."

    cd "$FIRMWARE_DIR"
    pio device monitor --baud 115200
}

do_clean() {
    log "Cleaning build artifacts..."

    cd "$FIRMWARE_DIR"
    pio run --target clean
    rm -rf .pio

    log "Clean complete."
}

do_build() {
    log "Compiling firmware (no upload)..."

    cd "$FIRMWARE_DIR"
    pio run

    log "Build successful. To upload: ./flash.sh upload"
}

do_erase() {
    log "Erasing flash..."
    warn "This will wipe all data on the device including WiFi credentials!"

    cd "$FIRMWARE_DIR"
    pio run --target erase

    log "Flash erased. Next upload will start fresh."
}

case "${1:-upload}" in
    upload)
        check_pio
        do_upload
        ;;
    build)
        check_pio
        do_build
        ;;
    monitor)
        check_pio
        do_monitor
        ;;
    clean)
        check_pio
        do_clean
        ;;
    erase)
        check_pio
        do_erase
        ;;
    *)
        echo "Usage: $0 [upload|build|monitor|clean|erase]"
        echo ""
        echo "  upload   — Build and upload firmware (default)"
        echo "  build    — Compile only, don't upload"
        echo "  monitor  — Open serial monitor (115200 baud)"
        echo "  clean    — Delete build artifacts"
        echo "  erase    — Full flash erase (wipes WiFi credentials)"
        exit 1
        ;;
esac
