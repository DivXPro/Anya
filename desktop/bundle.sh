#!/usr/bin/env bash
set -euo pipefail
# Wrap the anya binary into a macOS .app bundle so Finder treats it as a GUI app.

BIN_DIR="$(cd "$(dirname "$0")/bin" && pwd)"
APP_NAME="Anya"
APP_BUNDLE="$BIN_DIR/$APP_NAME.app"
BINARY="$BIN_DIR/anya"
ICON_PNG="$BIN_DIR/../build/appicon.png"
ICON_ICNS="$BIN_DIR/../build/darwin/icons.icns"

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
    <string>com.anya.app</string>
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
    <key>CFBundleIconFile</key>
    <string>icons</string>
</dict>
</plist>
EOF

# Icons
if [ -f "$ICON_ICNS" ]; then
    cp "$ICON_ICNS" "$APP_BUNDLE/Contents/Resources/icons.icns"
elif [ -f "$ICON_PNG" ]; then
    cp "$ICON_PNG" "$APP_BUNDLE/Contents/Resources/appicon.png"
fi

echo "Bundle created: $APP_BUNDLE"
