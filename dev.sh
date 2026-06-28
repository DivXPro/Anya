#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# Anya Development Script
# Runs the desktop app in dev mode (hot reload) or directly
# Usage: ./dev.sh [dev|run|test]
# ============================================================

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DESKTOP_DIR="$SCRIPT_DIR/desktop"

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { echo -e "${GREEN}[DEV]${NC} $1"; }
info() { echo -e "${BLUE}[INFO]${NC} $1"; }

check_deps() {
    local missing=()

    export PATH="$PATH:$(go env GOPATH)/bin"

    command -v go      &>/dev/null || missing+=("go (1.22+)")
    command -v node    &>/dev/null || missing+=("node (18+)")
    command -v wails3  &>/dev/null || missing+=("wails3")
    command -v ffmpeg  &>/dev/null || missing+=("ffmpeg")
    command -v faster-whisper &>/dev/null || missing+=("faster-whisper (pip install faster-whisper)")
    command -v edge-tts &>/dev/null || missing+=("edge-tts (pip install edge-tts)")

    if [ ${#missing[@]} -gt 0 ]; then
        echo -e "${RED}Missing dependencies:${NC}"
        for m in "${missing[@]}"; do
            echo "  ✗ $m"
        done
        echo ""
        echo "Speech tools (ffmpeg, faster-whisper, edge-tts) are optional for basic UI testing."
        return 1
    fi
    return 0
}

run_dev() {
    log "Starting development mode (hot reload)..."
    info "Frontend: http://localhost:9245"
    info "Edit Go/TSX files → auto rebuild + reload"

    cd "$DESKTOP_DIR"
    wails3 dev
}

run_app() {
    log "Starting Anya desktop app..."

    if [ ! -f "$DESKTOP_DIR/bin/anya" ]; then
        log "No binary found, building first..."
        "$SCRIPT_DIR/build.sh" desktop
    fi

    cd "$DESKTOP_DIR"
    ./bin/anya &
    ANYA_PID=$!
    info "Anya running (PID=$ANYA_PID)"
    info "WebSocket server: ws://localhost:9876"
    info "Data directory: ~/.elf/"
    info "Press Ctrl+C to stop"

    trap "kill $ANYA_PID 2>/dev/null; log 'Stopped.'" EXIT
    wait $ANYA_PID
}

run_tests() {
    log "Running all tests..."

    cd "$DESKTOP_DIR"

    echo ""
    go test ./internal/... -v -count=1 2>&1 | while IFS= read -r line; do
        if [[ "$line" == "--- PASS"* ]] || [[ "$line" == "ok"* ]]; then
            echo -e "${GREEN}$line${NC}"
        elif [[ "$line" == "--- FAIL"* ]] || [[ "$line" == "FAIL"* ]]; then
            echo -e "${RED}$line${NC}"
        else
            echo "$line"
        fi
    done

    echo ""
    log "Running go vet..."
    go vet ./internal/...
    log "go vet: clean"
}

case "${1:-dev}" in
    dev)
        check_deps || warn "Proceeding despite missing deps..."
        run_dev
        ;;
    run)
        run_app
        ;;
    test)
        run_tests
        ;;
    *)
        echo "Usage: $0 [dev|run|test]"
        echo ""
        echo "  dev   — Start in development mode with hot reload (default)"
        echo "  run   — Build (if needed) and run the desktop app"
        echo "  test  — Run all tests and go vet"
        exit 1
        ;;
esac
