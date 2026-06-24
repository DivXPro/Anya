#!/usr/bin/env bash
set -euo pipefail
# Wrap the elf binary into a macOS .app bundle so Finder treats it as a GUI app.

BIN_DIR="$(cd "$(dirname "$0")/bin" && pwd)"
APP_NAME="Elf"
APP_BUNDLE="$BIN_DIR/$APP_NAME.app"
BINARY="$BIN_DIR/elf"
ICON="$BIN_DIR/../build/appicon.png"

rm -rf "$APP_BUNDLE"
mkdir -p "$APP_BUNDLE/Contents/MacOS"
mkdir -p "$APP_BUNDLE/Contents/Resources"

# Copy binary
cp "$BINARY" "$APP_BUNDLE/Contents/MacOS/$APP_NAME"
chmod +x "$APP_BUNDLE/Contents/MacOS/$APP_NAME"

# Info.plist — LSUIElement=1 hides from Dock, making it a menu bar style app
cat > "$APP_BUNDLE/Contents/Info.plist" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>$APP_NAME</string>
    <key>CFBundleIdentifier</key>
    <string>com.elf.app</string>
    <key>CFBundleName</key>
    <string>$APP_NAME</string>
    <key>CFBundleVersion</key>
    <string>1.0.0</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>LSMinimumSystemVersion</key>
    <string>11.0</string>
    <key>LSUIElement</key>
    <true/>
    <key>NSHighResolutionCapable</key>
    <true/>
</dict>
</plist>
EOF

# Icon
if [ -f "$ICON" ]; then
    cp "$ICON" "$APP_BUNDLE/Contents/Resources/appicon.png"
fi

echo "Bundle created: $APP_BUNDLE"
