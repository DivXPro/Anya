#include "display.h"

// Layout — portrait 80×160
static const int STATUS_BAR_H = 14;
static const int MASCOT_CX   = 40;
static const int MASCOT_CY   = 54;
static const int MASCOT_R    = 20;
static const int PROMPT_Y    = 86;

void disp_init() {
    M5.Display.setRotation(0);  // portrait
    M5.Display.fillScreen(TFT_WHITE);
    M5.Display.setTextColor(TFT_BLACK);
}

// ── Status Bar ──────────────────────────────────────────────
void disp_status_bar(int8_t rssi, bool wsConnected) {
    M5.Display.fillRect(0, 0, M5.Display.width(), STATUS_BAR_H, TFT_WHITE);
    M5.Display.drawFastHLine(0, STATUS_BAR_H, M5.Display.width(), TFT_LIGHTGREY);

    // WiFi signal bars (right-aligned)
    int bars = 0;
    if (rssi > -50)      bars = 4;
    else if (rssi > -60) bars = 3;
    else if (rssi > -70) bars = 2;
    else if (rssi > -80) bars = 1;

    int x = M5.Display.width() - 2;
    int barW = 2;
    int gap  = 1;
    for (int i = 0; i < 4; i++) {
        int h = 2 + i * 2;
        int y = STATUS_BAR_H - 2 - h;
        uint16_t color = (i < bars) ? TFT_BLACK : TFT_LIGHTGREY;
        M5.Display.fillRect(x - barW, y, barW, h, color);
        x -= barW + gap;
    }

    // Connection dot
    M5.Display.setTextSize(1);
    M5.Display.setCursor(2, 2);
    if (wsConnected) {
        M5.Display.setTextColor(TFT_GREEN);
        M5.Display.print("●");
    } else {
        M5.Display.setTextColor(TFT_LIGHTGREY);
        M5.Display.print("○");
    }
}

// ── Mascot ──────────────────────────────────────────────────
static void drawMascot() {
    M5.Display.fillCircle(MASCOT_CX, MASCOT_CY, MASCOT_R, TFT_LIGHTGREY);
    M5.Display.setTextColor(TFT_BLACK);
    M5.Display.setTextSize(1);
    M5.Display.setCursor(MASCOT_CX - 10, MASCOT_CY - 5);
    M5.Display.print("Elf");
}

// ── Prompt ──────────────────────────────────────────────────
static void drawPrompt(const char* line1, const char* line2 = nullptr) {
    M5.Display.setTextSize(1);
    M5.Display.setTextColor(TFT_BLACK);
    int tw = strlen(line1) * 6;  // approx width for size 1
    M5.Display.setCursor((M5.Display.width() - tw) / 2, PROMPT_Y);
    M5.Display.print(line1);
    if (line2) {
        tw = strlen(line2) * 6;
        M5.Display.setCursor((M5.Display.width() - tw) / 2, PROMPT_Y + 16);
        M5.Display.print(line2);
    }
}

// ── State Screens ───────────────────────────────────────────
void disp_wifi_setup(const char* hotspotSsid) {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, false);
    drawMascot();
    drawPrompt("Connect hotspot", "Elf-hotspot");
}

void disp_wifi_connecting(const char* ssid) {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, false);
    drawMascot();
    drawPrompt("Connecting...", nullptr);
}

void disp_pair_ready() {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, false);
    drawMascot();
    drawPrompt("Ready to pair", nullptr);
}

void disp_pairing() {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, false);
    drawMascot();
    drawPrompt("Pairing...", nullptr);
}

void disp_idle(const char* agentName, bool connected) {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, connected);
    drawMascot();
    drawPrompt("Click to speak", nullptr);
    M5.Display.setTextSize(1);
    M5.Display.setCursor(4, PROMPT_Y + 20);
    M5.Display.printf("%s:%s", connected ? "on" : "off", agentName);
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
    drawPrompt("Thinking...", nullptr);
}

void disp_playing(const char* summary) {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, false);
    M5.Display.setTextColor(TFT_BLACK);
    M5.Display.setTextSize(1);
    M5.Display.setCursor(4, STATUS_BAR_H + 12);
    M5.Display.print(summary);
}

void disp_connecting(const char* desktopName) {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, false);
    drawMascot();
    M5.Display.setCursor(4, PROMPT_Y);
    M5.Display.printf("To: %s", desktopName);
}

void disp_error(const char* msg) {
    M5.Display.fillScreen(TFT_RED);
    disp_status_bar(-1, false);
    M5.Display.setTextSize(1);
    M5.Display.setCursor(4, STATUS_BAR_H + 14);
    M5.Display.print(msg);
}
