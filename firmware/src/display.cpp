#include "display.h"

// Layout constants
static const int STATUS_BAR_H = 16;
static const int MASCOT_CX   = 80;
static const int MASCOT_CY   = 44;
static const int MASCOT_R    = 28;
static const int PROMPT_Y    = 82;

void disp_init() {
    M5.Display.setRotation(1);
    M5.Display.fillScreen(TFT_WHITE);
    M5.Display.setTextColor(TFT_BLACK);
}

// ── Status Bar ──────────────────────────────────────────────
void disp_status_bar(int8_t rssi, bool wsConnected) {
    // Background
    M5.Display.fillRect(0, 0, M5.Display.width(), STATUS_BAR_H, TFT_WHITE);
    M5.Display.drawFastHLine(0, STATUS_BAR_H, M5.Display.width(), TFT_LIGHTGREY);

    // ── WiFi signal icon (left) ──
    int bars = 0;
    if (rssi > -50)      bars = 4;
    else if (rssi > -60) bars = 3;
    else if (rssi > -70) bars = 2;
    else if (rssi > -80) bars = 1;

    int x = M5.Display.width() - 4;
    int barW = 3;
    int gap  = 1;
    for (int i = 0; i < 4; i++) {
        int h = 3 + i * 2;
        int y = STATUS_BAR_H - 3 - h;
        uint16_t color = (i < bars) ? TFT_BLACK : TFT_LIGHTGREY;
        M5.Display.fillRect(x - barW, y, barW, h, color);
        x -= barW + gap;
    }

    // ── Desktop connection indicator (right side of top bar) ──
    M5.Display.setTextSize(1);
    M5.Display.setCursor(4, 2);
    if (wsConnected) {
        M5.Display.setTextColor(TFT_GREEN);
        M5.Display.print("● Elf");
    } else {
        M5.Display.setTextColor(TFT_LIGHTGREY);
        M5.Display.print("○ Elf");
    }

    // ── RSSI text ──
    M5.Display.setTextColor(TFT_DARKGREY);
    M5.Display.setCursor(52, 2);
    if (rssi < 0) {
        M5.Display.printf("%d dBm", rssi);
    } else {
        M5.Display.print("--- dBm");
    }
}

// ── Mascot ──────────────────────────────────────────────────
static void drawMascot() {
    M5.Display.fillCircle(MASCOT_CX, MASCOT_CY, MASCOT_R, TFT_LIGHTGREY);
    M5.Display.setTextColor(TFT_BLACK);
    M5.Display.setTextSize(2);
    M5.Display.setCursor(MASCOT_CX - 14, MASCOT_CY - 8);
    M5.Display.print("Elf");
}

// ── Prompt ──────────────────────────────────────────────────
static void drawPrompt(const char* line1, const char* line2 = nullptr) {
    M5.Display.setTextSize(2);
    M5.Display.setTextColor(TFT_BLACK);
    M5.Display.setCursor(18, PROMPT_Y);
    M5.Display.print(line1);
    if (line2) {
        M5.Display.setCursor(42, PROMPT_Y + 24);
        M5.Display.print(line2);
    }
}

// ── State Screens ───────────────────────────────────────────
void disp_wifi_setup(const char* hotspotSsid) {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, false);
    drawMascot();
    drawPrompt("Connect Elf-hotspot", "and setup");
}

void disp_wifi_connecting(const char* ssid) {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, false);
    drawMascot();
    drawPrompt("Connecting WiFi...", nullptr);
}

void disp_pair_ready() {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, false);
    drawMascot();
    drawPrompt("Click below to pair", nullptr);
}

void disp_pairing() {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, false);
    drawMascot();
    drawPrompt("Pairing", "Click to stop");
}

void disp_idle(const char* agentName, bool connected) {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, connected);
    drawMascot();
    drawPrompt("Click to speak", nullptr);
    M5.Display.setTextSize(1);
    M5.Display.setCursor(8, PROMPT_Y + 50);
    M5.Display.printf("%s %s", connected ? "online" : "offline", agentName);
}

void disp_listening() {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, false);
    drawMascot();
    drawPrompt("Listening...", nullptr);
}

void disp_sending() {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, false);
    drawMascot();
    drawPrompt("Sending...", nullptr);
}

void disp_processing() {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, false);
    drawMascot();
    drawPrompt("Processing...", nullptr);
}

void disp_playing(const char* summary) {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, false);
    M5.Display.setTextColor(TFT_BLACK);
    M5.Display.setTextSize(2);
    M5.Display.setCursor(10, STATUS_BAR_H + 10);
    M5.Display.print(summary);
}

void disp_connecting(const char* desktopName) {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, false);
    drawMascot();
    M5.Display.setCursor(10, PROMPT_Y);
    M5.Display.printf("Connecting:\n%s", desktopName);
}

void disp_error(const char* msg) {
    M5.Display.fillScreen(TFT_RED);
    disp_status_bar(-1, false);
    M5.Display.setCursor(10, STATUS_BAR_H + 14);
    M5.Display.print(msg);
}
