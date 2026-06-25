#include "display.h"
#include <cstring>

// Layout — portrait 80×160
static const int STATUS_BAR_H = 16;
static const int MASCOT_CX   = 40;  // center of 80px screen
static const int MASCOT_CY   = 58;
static const int MASCOT_R    = 24;
static const int PROMPT_Y    = 108; // below mascot: 58+24=82, +26px gap

void disp_init() {
    M5.Display.setRotation(0);
    M5.Display.fillScreen(TFT_WHITE);
    M5.Display.setTextColor(TFT_BLACK);
}

// ── Status Bar ────────────────────────────────────────────────
void disp_status_bar(int8_t rssi, bool wsConnected, const char* agent) {
    M5.Display.fillRect(0, 0, M5.Display.width(), STATUS_BAR_H, TFT_WHITE);
    M5.Display.drawFastHLine(0, STATUS_BAR_H, M5.Display.width(), TFT_LIGHTGREY);

    // WiFi signal bars, right-aligned
    int bars = 0;
    if (rssi > -50)      bars = 4;
    else if (rssi > -60) bars = 3;
    else if (rssi > -70) bars = 2;
    else if (rssi > -80) bars = 1;

    int barW = 2, gap = 1, baseY = STATUS_BAR_H - 2;
    int x = M5.Display.width() - 2;
    for (int i = 0; i < 4; i++) {
        int h = 2 + i * 2;
        uint16_t color = (i < bars) ? TFT_BLACK : TFT_LIGHTGREY;
        M5.Display.fillRect(x - barW, baseY - h, barW, h, color);
        x -= barW + gap;
    }

    // Connection dot
    M5.Display.setTextSize(1);
    if (wsConnected) {
        M5.Display.setTextColor(TFT_GREEN);
        M5.Display.setCursor(3, 3);
        M5.Display.print("●");
    } else {
        M5.Display.setTextColor(TFT_DARKGREY);
        M5.Display.setCursor(3, 3);
        M5.Display.print("○");
    }

    // Agent name, right after the dot
    M5.Display.setTextColor(TFT_BLACK);
    M5.Display.setCursor(11, 3);
    if (agent && agent[0]) {
        M5.Display.print(agent);
    }
}

// ── Mascot ───────────────────────────────────────────────────
static void drawMascot() {
    M5.Display.fillCircle(MASCOT_CX, MASCOT_CY, MASCOT_R, TFT_LIGHTGREY);
    M5.Display.setTextColor(TFT_BLACK);
    M5.Display.setTextSize(1);
    // "Elf" is ~18px wide at text size 1, centered in circle
    M5.Display.setCursor(MASCOT_CX - 9, MASCOT_CY - 4);
    M5.Display.print("Elf");
}

// ── Prompt ───────────────────────────────────────────────────
static void centerPrint(const char* s, int y) {
    // ~6px per char at text size 1
    int w = (int)strlen(s) * 6;
    M5.Display.setCursor((M5.Display.width() - w) / 2, y);
    M5.Display.print(s);
}

static void drawPrompt(const char* line1, const char* line2) {
    M5.Display.setTextSize(1);
    M5.Display.setTextColor(TFT_BLACK);
    centerPrint(line1, PROMPT_Y);
    if (line2) {
        centerPrint(line2, PROMPT_Y + 18);
    }
}

// ── State Screens ────────────────────────────────────────────
void disp_wifi_setup(const char* hotspotSsid, const char* agent) {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, false, agent);
    drawMascot();
    drawPrompt("Connect hotspot", "Elf-hotspot");
}

void disp_wifi_connecting(const char* ssid, const char* agent) {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, false, agent);
    drawMascot();
    drawPrompt("Connecting...", nullptr);
}

void disp_pair_ready(const char* agent) {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, false, agent);
    drawMascot();
    drawPrompt("Ready to pair", nullptr);
}

void disp_pairing(const char* agent) {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, false, agent);
    drawMascot();
    drawPrompt("Pairing...", nullptr);
}

void disp_idle(const char* agent, bool connected) {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, connected, agent);
    drawMascot();
    drawPrompt("Click to speak", nullptr);
}

void disp_listening(const char* agent) {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, false, agent);
    drawMascot();
    drawPrompt("Listening...", nullptr);
}

void disp_sending(const char* agent) {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, false, agent);
    drawMascot();
    drawPrompt("Sending...", nullptr);
}

void disp_processing(const char* agent) {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, false, agent);
    drawMascot();
    drawPrompt("Thinking...", nullptr);
}

void disp_playing(const char* summary, const char* agent) {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, false, agent);
    M5.Display.setTextColor(TFT_BLACK);
    M5.Display.setTextSize(1);
    M5.Display.setCursor(4, STATUS_BAR_H + 12);
    M5.Display.print(summary);
}

void disp_connecting(const char* desktopName, const char* agent) {
    M5.Display.fillScreen(TFT_WHITE);
    disp_status_bar(-1, false, agent);
    drawMascot();
    M5.Display.setTextSize(1);
    M5.Display.setTextColor(TFT_BLACK);
    M5.Display.setCursor(4, PROMPT_Y);
    M5.Display.print(desktopName);
}

void disp_error(const char* msg, const char* agent) {
    M5.Display.fillScreen(TFT_RED);
    disp_status_bar(-1, false, agent);
    M5.Display.setTextSize(1);
    M5.Display.setCursor(4, STATUS_BAR_H + 14);
    M5.Display.print(msg);
}
