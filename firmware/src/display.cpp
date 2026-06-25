#include "display.h"
#include "mascot_img.h"
#include <cstring>

// Layout — landscape 240×135 (M5StickC Plus rotation 1)
//   0-16    status bar
//  16-24    gap
//  24-104   mascot area (80×80)
// 104-112   gap
// 112+      prompt text (bottom margin ~15px)
static const int STATUS_BAR_H = 16;
static const int MASCOT_GAP   = 8;

void disp_init() {
    M5.Display.setRotation(1);  // portrait 135x240
    M5.Display.setBrightness(255);
    M5.Display.fillScreen(TFT_BLACK);
    M5.Display.setTextColor(TFT_WHITE);
}

// ── Status Bar ────────────────────────────────────────────────
void disp_status_bar(int8_t rssi, bool wifiConnected, bool wsConnected, const char* agent, const char* ssid) {
    M5.Display.fillRect(0, 0, M5.Display.width(), STATUS_BAR_H, TFT_BLACK);
    M5.Display.drawFastHLine(0, STATUS_BAR_H, M5.Display.width(), TFT_DARKGREY);

    // WiFi signal bars, right-aligned
    int bars = 0;
    if (wifiConnected) {
        if (rssi > -50)      bars = 4;
        else if (rssi > -60) bars = 3;
        else if (rssi > -70) bars = 2;
        else if (rssi > -80) bars = 1;
    }

    int barW = 2, gap = 1, baseY = STATUS_BAR_H - 2;
    int x = M5.Display.width() - 2;
    for (int i = 0; i < 4; i++) {
        int h = 2 + i * 2;
        uint16_t color = (i < bars) ? TFT_WHITE : TFT_DARKGREY;
        M5.Display.fillRect(x - barW, baseY - h, barW, h, color);
        x -= barW + gap;
    }

    // Connection dot: clearly visible states
    //   grey   = no WiFi
    //   yellow = WiFi up, not connected to desktop
    //   green  = connected to desktop
    const int DOT_R = 3;
    const int DOT_Y = STATUS_BAR_H / 2;
    const int DOT_X = 6;
    if (wsConnected) {
        M5.Display.fillCircle(DOT_X, DOT_Y, DOT_R, TFT_GREEN);
    } else if (wifiConnected) {
        M5.Display.fillCircle(DOT_X, DOT_Y, DOT_R, TFT_YELLOW);
    } else {
        M5.Display.drawCircle(DOT_X, DOT_Y, DOT_R, TFT_DARKGREY);
    }

    // Left label: show SSID when provided (e.g. on Pair screen), otherwise agent name.
    // Use middle-left datum so the text is vertically centered in the 16px bar.
    const char* label = (ssid && ssid[0]) ? ssid : agent;
    if (label && label[0]) {
        int maxChars = (M5.Display.width() - 50) / 6;
        int len = strlen(label);
        if (len > maxChars) len = maxChars;
        char buf[64];
        if (len >= (int)sizeof(buf)) len = (int)sizeof(buf) - 1;
        strncpy(buf, label, len);
        buf[len] = '\0';
        M5.Display.setTextSize(1);
        M5.Display.setTextColor(TFT_WHITE);
        M5.Display.setTextDatum(textdatum_t::middle_left);
        M5.Display.drawString(buf, 13, STATUS_BAR_H / 2);
    }
}

// ── Mascot ───────────────────────────────────────────────────
static const int MASCOT_Y = STATUS_BAR_H + MASCOT_GAP;
static int  mascotFrame  = 0;
static unsigned long mascotLastSwitch = 0;

static void drawMascot() {
    // Animate: switch to random frame every 2.5–4 seconds
    unsigned long now = millis();
    if (mascotLastSwitch == 0) mascotLastSwitch = now;
    if (now - mascotLastSwitch > 60000 + (esp_random() % 30000)) {  // 60–90s
        int next;
        do { next = (esp_random() % MASCOT_FRAMES); } while (next == mascotFrame && MASCOT_FRAMES > 1);
        mascotFrame = next;
        mascotLastSwitch = now;
    }

    int x = (M5.Display.width() - MASCOT_IMG_W) / 2;
    M5.Display.pushImage(x, MASCOT_Y, MASCOT_IMG_W, MASCOT_IMG_H, mascot_frames[mascotFrame]);
}

// ── Prompt ───────────────────────────────────────────────────
static const int PROMPT_Y = MASCOT_Y + MASCOT_IMG_H + MASCOT_GAP;

static void centerPrint(const char* s, int y) {
    int w = (int)strlen(s) * 6;
    M5.Display.setCursor((M5.Display.width() - w) / 2, y);
    M5.Display.print(s);
}

static void drawPrompt(const char* line1, const char* line2) {
    M5.Display.setTextSize(1);
    M5.Display.setTextColor(TFT_WHITE);
    centerPrint(line1, PROMPT_Y);
    if (line2) {
        centerPrint(line2, PROMPT_Y + 18);
    }
}

// ── State Screens ────────────────────────────────────────────
void disp_wifi_setup(const char* hotspotSsid, const char* agent) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, false, false, agent);
    drawMascot();
    drawPrompt("Connect to Elf-hotspot", nullptr);
}

void disp_wifi_connecting(const char* ssid, const char* agent) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, false, false, agent);
    drawMascot();
    drawPrompt("Connecting...", nullptr);
}

void disp_pair_ready(const char* agent) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, true, false, agent);
    drawMascot();
    drawPrompt("Ready to pair", nullptr);
}

void disp_pairing(const char* agent) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, true, false, agent);
    drawMascot();
    drawPrompt("Pairing...", nullptr);
}

void disp_idle(const char* agent, bool connected) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, true, connected, agent);
    drawMascot();
    drawPrompt("Click to speak", nullptr);
}

void disp_listening(const char* agent) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, true, true, agent);
    drawMascot();
    drawPrompt("Listening...", nullptr);
}

void disp_sending(const char* agent) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, true, true, agent);
    drawMascot();
    drawPrompt("Sending...", nullptr);
}

void disp_processing(const char* agent) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, true, true, agent);
    drawMascot();
    drawPrompt("Thinking...", nullptr);
}

void disp_playing(const char* summary, const char* agent) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, true, true, agent);
    M5.Display.setTextColor(TFT_WHITE);
    M5.Display.setTextSize(1);
    M5.Display.setCursor(4, PROMPT_Y);
    M5.Display.print(summary);
}

void disp_connecting(const char* desktopName, const char* agent) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, true, false, agent);
    drawMascot();
    M5.Display.setTextSize(1);
    M5.Display.setTextColor(TFT_WHITE);
    M5.Display.setCursor(4, PROMPT_Y);
    M5.Display.print(desktopName);
}

void disp_error(const char* msg, const char* agent) {
    M5.Display.fillScreen(TFT_RED);
    disp_status_bar(-1, false, false, agent);
    M5.Display.setTextSize(1);
    M5.Display.setCursor(4, PROMPT_Y);
    M5.Display.print(msg);
}
