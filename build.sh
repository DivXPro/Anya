#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# Anya Desktop Build Script
# Compiles the Wails desktop app and wraps it into a .app bundle
# Usage: ./build.sh
# ============================================================

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DESKTOP_DIR="$SCRIPT_DIR/desktop"
FIRMWARE_DIR="$SCRIPT_DIR/firmware"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo -e "${GREEN}[BUILD]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
err() { echo -e "${RED}[ERROR]${NC} $1"; }

export PATH="$PATH:$(go env GOPATH)/bin"

for cmd in wails3 node go; do
    if ! command -v "$cmd" &>/dev/null; then
        err "$cmd not found."
        exit 1
    fi
done

# Build firmware binary and embed it into the desktop app.
if command -v pio &>/dev/null; then
    FIRMWARE_VERSION="$(git -C "$SCRIPT_DIR" describe --tags --always --dirty 2>/dev/null || echo 'unknown')"
    log "Building firmware version $FIRMWARE_VERSION..."

    # Sync the firmware version into platformio.ini so the compiled binary reports it.
    sed -i.bak "s|'\-DFIRMWARE_VERSION=\".*\"'|'-DFIRMWARE_VERSION=\"$FIRMWARE_VERSION\"'|" "$FIRMWARE_DIR/platformio.ini"
    rm -f "$FIRMWARE_DIR/platformio.ini.bak"

    (cd "$FIRMWARE_DIR" && pio run)

    FIRMWARE_BIN="$FIRMWARE_DIR/.pio/build/m5stickc-s3/firmware.bin"
    ASSETS_DIR="$DESKTOP_DIR/internal/firmware/assets"
    mkdir -p "$ASSETS_DIR"

    if [ -f "$FIRMWARE_BIN" ]; then
        cp "$FIRMWARE_BIN" "$ASSETS_DIR/firmware.bin"
        echo -n "$FIRMWARE_VERSION" > "$ASSETS_DIR/version.txt"
        log "Firmware embedded: $FIRMWARE_VERSION ($(du -sh "$ASSETS_DIR/firmware.bin" | cut -f1))"
    else
        err "Firmware binary not found at $FIRMWARE_BIN"
        exit 1
    fi
else
    warn "PlatformIO not found. Skipping firmware build; embedded firmware will be empty."
    warn "To include firmware, install PlatformIO: pip install platformio"
fi

cd "$DESKTOP_DIR"

# Install frontend deps if needed
if [ ! -d "frontend/node_modules" ]; then
    log "Installing frontend dependencies..."
    (cd frontend && npm install)
fi

# Generate app icons from the source anya.png so the .app bundle and status bar
# always use the current logo instead of a stale Wails default.
ICON_SRC="$DESKTOP_DIR/frontend/public/anya.png"
ICON_DIR="$DESKTOP_DIR/build"
APPICON_PNG="$ICON_DIR/appicon.png"
ICNS_PATH="$ICON_DIR/darwin/icons.icns"

if [[ "$OSTYPE" == "darwin"* ]] && [ -f "$ICON_SRC" ]; then
    log "Generating app icons from $ICON_SRC..."
    mkdir -p "$ICON_DIR/darwin"

    # Remove stale Wails Assets.car so it doesn't override the new icons.icns.
    rm -f "$ICON_DIR/darwin/Assets.car"

    # 1024x1024 app icon used by the bundle script.
    sips -Z 1024 "$ICON_SRC" --out "$APPICON_PNG" >/dev/null 2>&1

    # macOS .icns set for the .app bundle and status bar.
    ICONSET_DIR="$(mktemp -d)/anya.iconset"
    mkdir -p "$ICONSET_DIR"
    for size in 16 32 64 128 256 512; do
        sips -Z "$size" "$ICON_SRC" --out "$ICONSET_DIR/icon_${size}x${size}.png" >/dev/null 2>&1
        sips -Z "$((size*2))" "$ICON_SRC" --out "$ICONSET_DIR/icon_${size}x${size}@2x.png" >/dev/null 2>&1
    done
    sips -Z 1024 "$ICON_SRC" --out "$ICONSET_DIR/icon_512x512@2x.png" >/dev/null 2>&1
    iconutil -c icns "$ICONSET_DIR" -o "$ICNS_PATH"
    rm -rf "$ICONSET_DIR"
fi

# Clean stale build artifacts from previous names so the user doesn't accidentally
# run an old "elf" binary that still has the Wails icon.
rm -f "$DESKTOP_DIR/bin/elf" "$DESKTOP_DIR/bin/elf.dev.app"
rm -rf "$DESKTOP_DIR/bin/Elf.app"

# Full Wails build (bindings + frontend + Go)
wails3 build

# Wrap into macOS .app bundle
if [[ "$(uname)" == "Darwin" ]]; then
    log "Wrapping into .app bundle..."
    bash bundle.sh
    log "App: $(realpath bin/Anya.app)"
fi

log "Desktop binary: $(realpath bin/anya) ($(du -sh bin/anya | cut -f1))"
log "Done."
