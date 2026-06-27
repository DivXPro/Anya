#include "display.h"
#include "mascot.h"
#include <cstring>

// Layout — portrait 135×240 (M5StickC S3 native)
//   0-16     status bar
//   16-?     mascot area (original GIF size, centred)
//   ?-?      prompt text (bottom)
static const int STATUS_BAR_H = 16;
static const int MASCOT_GAP   = 4;
static const int PROMPT_H     = 12;

static const char* abbreviate_agent(const char* name) {
    static char buf[32];
    if (!name || !name[0]) return "";
    const char* end = strchr(name, ' ');
    size_t len = end ? (size_t)(end - name) : strlen(name);
    if (len >= sizeof(buf)) len = sizeof(buf) - 1;
    memcpy(buf, name, len);
    buf[len] = '\0';
    return buf;
}

static int mascotY = 0;
static int promptY = 0;

void disp_init() {
    M5.Display.setRotation(DISPLAY_ROTATION);  // native portrait 135x240
    M5.Display.setBrightness(255);
    M5.Display.fillScreen(TFT_BLACK);
    M5.Display.setTextColor(TFT_WHITE);

    // Layout must be computed after M5.begin() so width()/height() are valid.
    mascotY = STATUS_BAR_H +
              (M5.Display.height() - STATUS_BAR_H - MASCOT_IMG_H - PROMPT_H - MASCOT_GAP) / 2;
    promptY = mascotY + MASCOT_IMG_H + MASCOT_GAP;

    mascot_init();
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
    //   grey  = no WiFi
    //   red   = WiFi up, not connected to desktop
    //   green = connected to desktop
    const int DOT_R = 3;
    const int DOT_Y = STATUS_BAR_H / 2;
    const int DOT_X = 6;
    if (wsConnected) {
        M5.Display.fillCircle(DOT_X, DOT_Y, DOT_R, TFT_GREEN);
    } else if (wifiConnected) {
        M5.Display.fillCircle(DOT_X, DOT_Y, DOT_R, TFT_RED);
    } else {
        M5.Display.drawCircle(DOT_X, DOT_Y, DOT_R, TFT_DARKGREY);
    }

    // Left label: show SSID when provided (e.g. on Pair screen), otherwise abbreviated agent name.
    const char* label = (ssid && ssid[0]) ? ssid : abbreviate_agent(agent);
    if (label && label[0]) {
        int maxChars = (M5.Display.width() - 44) / 6;
        int len = strlen(label);
        if (len > maxChars) len = maxChars;
        char buf[32];
        if (len >= (int)sizeof(buf)) len = (int)sizeof(buf) - 1;
        strncpy(buf, label, len);
        buf[len] = '\0';
        M5.Display.setTextSize(1);
        M5.Display.setTextColor(TFT_WHITE);
        M5.Display.setTextDatum(textdatum_t::middle_left);
        M5.Display.drawString(buf, 16, STATUS_BAR_H / 2);
    }
}

// ── Mascot ───────────────────────────────────────────────────
// Centre the original-size mascot vertically between the status bar and prompt.
// mascotY is computed at runtime in disp_init() because M5.Display.height()
// is not valid before M5.begin() (static initialisation order issue).
static int  mascotFrame  = 0;
static unsigned long mascotFrameStart = 0;
static int  lastDrawnMascotFrame = -1;
static bool mascotVisible = false;

static inline int mascot_x() {
    return (M5.Display.width() - MASCOT_IMG_W) / 2;
}

static void updateMascotFrame() {
    unsigned long now = millis();
    if (mascotFrameStart == 0) mascotFrameStart = now;
    if (now - mascotFrameStart >= MASCOT_FRAME_DURATIONS[mascotFrame]) {
        mascotFrame = (mascotFrame + 1) % MASCOT_FRAMES;
        mascotFrameStart = now;
    }
}

static void drawMascot(bool force = false) {
    updateMascotFrame();
    if (!force && mascotFrame == lastDrawnMascotFrame) return;
    lastDrawnMascotFrame = mascotFrame;
    mascot_draw(mascotFrame, mascot_x(), mascotY);
}

void disp_animate_mascot() {
    if (mascotVisible) {
        drawMascot(false);
    }
}



static void centerPrint(const char* s, int y) {
    M5.Display.setTextDatum(textdatum_t::top_left);
    int w = (int)strlen(s) * 6;
    M5.Display.setCursor((M5.Display.width() - w) / 2, y);
    M5.Display.print(s);
}

static void drawPrompt(const char* line1, const char* line2) {
    M5.Display.setTextSize(1);
    M5.Display.setTextColor(TFT_WHITE);
    centerPrint(line1, promptY);
    if (line2) {
        centerPrint(line2, promptY + 18);
    }
}

// ── State Screens ────────────────────────────────────────────
void disp_wifi_setup(const char* hotspotSsid, const char* agent) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, false, false, agent);
    mascotVisible = true;
    drawMascot(true);
    drawPrompt("Connect to Elf-hotspot", nullptr);
}

void disp_wifi_connecting(const char* ssid, const char* agent) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, false, false, agent);
    mascotVisible = true;
    drawMascot(true);
    drawPrompt("Connecting...", nullptr);
}

void disp_pair_ready(const char* agent, const char* ssid) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, true, false, agent, ssid);
    mascotVisible = true;
    drawMascot(true);
    drawPrompt("Ready to pair", nullptr);
}

void disp_pairing(const char* agent) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, true, false, agent);
    mascotVisible = true;
    drawMascot(true);
    drawPrompt("Pairing...", nullptr);
}

void disp_idle(const char* agent, bool connected) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, true, connected, agent);
    mascotVisible = true;
    drawMascot(true);
    drawPrompt(connected ? "Click to speak" : "Disconnect", nullptr);
}

void disp_listening(const char* agent) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, true, true, agent);
    mascotVisible = true;
    drawMascot(true);
    drawPrompt("Listening...", nullptr);
}

void disp_sending(const char* agent) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, true, true, agent);
    mascotVisible = true;
    drawMascot(true);
    drawPrompt("Sending...", nullptr);
}

void disp_processing(const char* agent) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, true, true, agent);
    mascotVisible = true;
    drawMascot(true);
    drawPrompt("Thinking...", nullptr);
}

void disp_playing(const char* summary, const char* agent) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, true, true, agent);
    mascotVisible = false;
    M5.Display.setTextColor(TFT_WHITE);
    M5.Display.setTextSize(1);
    M5.Display.setCursor(4, promptY);
    M5.Display.print(summary);
}

void disp_connecting(const char* desktopName, const char* agent) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, true, false, agent);
    mascotVisible = true;
    drawMascot(true);
    M5.Display.setTextSize(1);
    M5.Display.setTextColor(TFT_WHITE);
    M5.Display.setCursor(4, promptY);
    M5.Display.print(desktopName);
}

void disp_error(const char* msg, const char* agent) {
    M5.Display.fillScreen(TFT_RED);
    disp_status_bar(-1, false, false, agent);
    mascotVisible = false;
    M5.Display.setTextSize(1);
    M5.Display.setCursor(4, promptY);
    M5.Display.print(msg);
}

// ── Menu ──────────────────────────────────────────────────────
void disp_menu(const char* agent, int selected, const char* const* items, int count, int8_t rssi, bool wifiConnected, bool wsConnected) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(rssi, wifiConnected, wsConnected, agent);
    mascotVisible = false;

    // No mascot in menu — vertical list centred under the status bar.
    int lineH = 22;
    int totalH = count * lineH;
    int startY = STATUS_BAR_H + (M5.Display.height() - STATUS_BAR_H - totalH) / 2;

    M5.Display.setTextSize(1);
    M5.Display.setTextDatum(textdatum_t::middle_center);

    for (int i = 0; i < count; i++) {
        int y = startY + i * lineH;
        bool sel = (i == selected);
        if (sel) {
            M5.Display.setTextColor(TFT_GREEN);
            M5.Display.drawString("> " + String(items[i]) + " <", M5.Display.width() / 2, y + lineH / 2);
        } else {
            M5.Display.setTextColor(TFT_WHITE);
            M5.Display.drawString(items[i], M5.Display.width() / 2, y + lineH / 2);
        }
    }
}
