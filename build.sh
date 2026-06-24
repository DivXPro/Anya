#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# Elf Desktop Build Script
# Compiles the Wails desktop app and wraps it into a .app bundle
# Usage: ./build.sh
# ============================================================

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DESKTOP_DIR="$SCRIPT_DIR/desktop"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

log() { echo -e "${GREEN}[BUILD]${NC} $1"; }
err() { echo -e "${RED}[ERROR]${NC} $1"; }

export PATH="$PATH:$(go env GOPATH)/bin"

for cmd in wails3 node go; do
    if ! command -v "$cmd" &>/dev/null; then
        err "$cmd not found."
        exit 1
    fi
done

cd "$DESKTOP_DIR"

# Install frontend deps if needed
if [ ! -d "frontend/node_modules" ]; then
    log "Installing frontend dependencies..."
    (cd frontend && npm install)
fi

# Full Wails build (bindings + frontend + Go)
wails3 build

# Wrap into macOS .app bundle
if [[ "$(uname)" == "Darwin" ]]; then
    log "Wrapping into .app bundle..."
    bash bundle.sh
    log "App: $(realpath bin/Elf.app)"
fi

log "Desktop binary: $(realpath bin/elf) ($(du -sh bin/elf | cut -f1))"
log "Done."
