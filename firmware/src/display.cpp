#include "display.h"
#include "audio.h"
#include "mascot.h"
#include "layout.h"
#include "lang.h"
#include "state.h"
#include <cstring>
#include <vector>

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

// Scrolling text state for the agent reply screen.
static std::vector<String> s_textLines;
static String s_lastText;
static int s_scrollOffset = 0;
static unsigned long s_textShownAt = 0;
static const int TEXT_LINE_H = 14;
static const int TEXT_AREA_MARGIN = 4;

void disp_init() {
    M5.Display.setRotation(DISPLAY_ROTATION);  // native portrait 135x240
    M5.Display.setBrightness(255);
    M5.Display.fillScreen(TFT_BLACK);
    M5.Display.setTextColor(TFT_WHITE);

    // Layout must be computed after M5.begin() so width()/height() are valid.
    computeMascotLayout(M5.Display.height(), MASCOT_IMG_H, mascotY, promptY);

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

    // Left label: show SSID when provided (e.g. on Pair screen), otherwise show
    // the agent name while connected, or "No agent" when the WebSocket is down.
    const char* label;
    if (ssid && ssid[0]) {
        label = ssid;
    } else if (wsConnected) {
        label = abbreviate_agent(agent);
    } else {
        label = tr(Str::NoAgent);
    }
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
    M5.Display.setTextDatum(textdatum_t::top_center);
    M5.Display.drawString(s, M5.Display.width() / 2, y);
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
    drawPrompt(tr(Str::ConnectToAnya), nullptr);
}

void disp_wifi_connecting(const char* ssid, const char* agent) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, false, false, agent);
    mascotVisible = true;
    drawMascot(true);
    drawPrompt(tr(Str::Connecting), nullptr);
}

void disp_pair_ready(const char* agent, const char* ssid) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, true, false, agent, ssid);
    mascotVisible = true;
    drawMascot(true);
    drawPrompt(tr(Str::ReadyToPair), nullptr);
}

void disp_pairing(const char* agent) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, true, false, agent);
    mascotVisible = true;
    drawMascot(true);
    drawPrompt(tr(Str::Pairing), nullptr);
}

void disp_idle(const char* agent, bool connected) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, true, connected, agent);
    mascotVisible = true;
    drawMascot(true);
    drawPrompt(connected ? tr(Str::ClickToSpeak) : tr(Str::Disconnected), nullptr);
}

void disp_listening(const char* agent) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, true, true, agent);
    mascotVisible = true;
    drawMascot(true);
    drawPrompt(tr(Str::Listening), nullptr);
}

void disp_sending(const char* agent) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, true, true, agent);
    mascotVisible = true;
    drawMascot(true);
    drawPrompt(tr(Str::Sending), nullptr);
}

void disp_processing(const char* agent) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, true, true, agent);
    mascotVisible = true;
    drawMascot(true);
    drawPrompt(tr(Str::Thinking), nullptr);
}

static int utf8_char_len(const char* s) {
    unsigned char c = static_cast<unsigned char>(*s);
    if ((c & 0x80) == 0) return 1;
    if ((c & 0xe0) == 0xc0) return 2;
    if ((c & 0xf0) == 0xe0) return 3;
    if ((c & 0xf8) == 0xf0) return 4;
    return 1;
}

static std::vector<String> wrapTextLines(const char* text, int maxW) {
    std::vector<String> lines;
    const char* p = text;
    while (*p) {
        String line;
        while (*p) {
            int charLen = utf8_char_len(p);
            String candidate = line;
            for (int i = 0; i < charLen; ++i) candidate += p[i];
            if (line.length() > 0 && M5.Display.textWidth(candidate.c_str()) > maxW) {
                break;
            }
            line = candidate;
            p += charLen;
        }
        // Ensure progress if a single character exceeds the width.
        if (line.length() == 0 && *p) {
            int charLen = utf8_char_len(p);
            for (int i = 0; i < charLen; ++i) line += p[i];
            p += charLen;
        }
        lines.push_back(line);
    }
    return lines;
}

static inline int textAreaTop() { return STATUS_BAR_H + 2; }
static inline int textAreaBottom() { return M5.Display.height() - TEXT_AREA_MARGIN; }
static inline int textAreaHeight() { return textAreaBottom() - textAreaTop(); }

static void drawScrollingText() {
    if (s_textLines.empty()) return;
    M5.Display.setTextSize(1);
    M5.Display.setTextDatum(textdatum_t::top_center);
    M5.Display.setTextColor(TFT_WHITE);
    int x = M5.Display.width() / 2;
    int canvasH = s_textLines.size() * TEXT_LINE_H;
    int areaH = textAreaHeight();
    int maxOffset = (canvasH > areaH) ? (canvasH - areaH) : 0;
    if (s_scrollOffset > maxOffset) s_scrollOffset = maxOffset;
    int topY = textAreaTop();
    for (size_t i = 0; i < s_textLines.size(); ++i) {
        // Top-anchored: the first line starts at the top of the text area, and
        // the content scrolls upward (s_scrollOffset) only when it overflows.
        int y = topY + (int)i * TEXT_LINE_H - s_scrollOffset;
        if (y + TEXT_LINE_H >= textAreaTop() && y <= M5.Display.height()) {
            M5.Display.drawString(s_textLines[i].c_str(), x, y);
        }
    }
}

void disp_playing(const char* summary, const char* agent) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, true, true, agent);
    mascotVisible = false;

    if (!summary || !summary[0]) return;

    if (s_lastText != summary) {
        s_lastText = summary;
        s_textLines = wrapTextLines(summary, M5.Display.width() - TEXT_AREA_MARGIN * 2);
        s_scrollOffset = 0;
        s_textShownAt = millis();
    }
    drawScrollingText();
}

void disp_animate_text() {
    if (state_current() != State::PLAYING || s_textLines.empty()) return;

    int areaH = textAreaHeight();
    int canvasH = s_textLines.size() * TEXT_LINE_H;
    int maxOffset = (canvasH > areaH) ? (canvasH - areaH) : 0;
    if (s_scrollOffset >= maxOffset) return;

    static unsigned long lastScroll = 0;
    unsigned long now = millis();
    if (now - lastScroll < 50) return;
    lastScroll = now;

    // Clear only the text area and redraw the status bar separator.
    M5.Display.fillRect(0, textAreaTop(), M5.Display.width(), textAreaHeight(), TFT_BLACK);
    M5.Display.drawFastHLine(0, STATUS_BAR_H, M5.Display.width(), TFT_DARKGREY);

    s_scrollOffset++;
    drawScrollingText();
}

static bool replyScrollFinished() {
    if (s_textLines.empty()) return true;
    int canvasH = (int)s_textLines.size() * TEXT_LINE_H;
    int areaH = textAreaHeight();
    int maxOffset = (canvasH > areaH) ? (canvasH - areaH) : 0;
    return s_scrollOffset >= maxOffset;
}

bool disp_text_showing_for(unsigned long ms) {
    if (state_current() != State::PLAYING) return false;
    // The idle countdown starts only after BOTH the TTS audio playback and the
    // text scrolling have finished — whichever ends last. While either is still
    // active, push the reference timestamp forward so the elapsed time is
    // measured from the moment the later action completes.
    if (audio_is_playing() || !replyScrollFinished()) {
        s_textShownAt = millis();
        return false;
    }
    return (millis() - s_textShownAt) >= ms;
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

void disp_updating_progress(int8_t percent) {
    const int barW = M5.Display.width() - 24;
    const int barH = 10;
    const int x = 12;
    const int y = promptY;

    // Clear only the inside of the progress bar.
    M5.Display.fillRect(x + 2, y + 2, barW - 4, barH - 4, TFT_BLACK);

    int fillW = (int)percent * (barW - 4) / 100;
    if (fillW < 0) fillW = 0;
    if (fillW > barW - 4) fillW = barW - 4;
    if (fillW > 0) {
        M5.Display.fillRect(x + 2, y + 2, fillW, barH - 4, TFT_GREEN);
    }

    // Update percentage text without redrawing the whole screen.
    const int textY = y + barH + 12;
    const int textW = 40;
    const int textX = (M5.Display.width() - textW) / 2;
    M5.Display.fillRect(textX, textY - 6, textW, 12, TFT_BLACK);

    char pct[8];
    snprintf(pct, sizeof(pct), "%d%%", percent);
    M5.Display.setTextSize(1);
    M5.Display.setTextColor(TFT_WHITE);
    M5.Display.setTextDatum(textdatum_t::middle_center);
    M5.Display.drawString(pct, M5.Display.width() / 2, textY);
}

void disp_updating(int8_t percent, const char* version, const char* agent) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, true, true, agent);
    mascotVisible = false;

    M5.Display.setTextSize(1);
    M5.Display.setTextColor(TFT_WHITE);
    M5.Display.setTextDatum(textdatum_t::middle_center);
    M5.Display.drawString(tr(Str::UpdatingFirmware), M5.Display.width() / 2, promptY - 24);

    // Draw the static bar outline once.
    const int barW = M5.Display.width() - 24;
    const int barH = 10;
    const int x = 12;
    const int y = promptY;
    M5.Display.drawRect(x, y, barW, barH, TFT_WHITE);

    disp_updating_progress(percent);

    if (version && version[0]) {
        char ver[48];
        snprintf(ver, sizeof(ver), "to %s", version);
        M5.Display.drawString(ver, M5.Display.width() / 2, y + barH + 26);
    }
}

// ── Menu ──────────────────────────────────────────────────────
void disp_menu(const char* agent, int selected, const Str* items, int count, int8_t rssi, bool wifiConnected, bool wsConnected) {
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
        const char* label = tr(items[i]);
        if (sel) {
            M5.Display.setTextColor(TFT_GREEN);
            M5.Display.drawString("> " + String(label) + " <", M5.Display.width() / 2, y + lineH / 2);
        } else {
            M5.Display.setTextColor(TFT_WHITE);
            M5.Display.drawString(label, M5.Display.width() / 2, y + lineH / 2);
        }
    }
}

// ── Confirmation ──────────────────────────────────────────────
void disp_confirm(const char* agent, const char* prompt, const char* const* options, int count, int selected) {
    M5.Display.fillScreen(TFT_BLACK);
    disp_status_bar(-1, true, true, agent);
    mascotVisible = false;

    M5.Display.setTextSize(1);
    M5.Display.setTextDatum(textdatum_t::top_center);
    M5.Display.setTextColor(TFT_WHITE);

    // Prompt at the top, below the status bar.
    int margin = 4;
    int promptY = STATUS_BAR_H + margin;
    int maxPromptH = 40;
    M5.Display.setClipRect(0, promptY, M5.Display.width(), maxPromptH);
    M5.Display.drawString(String(prompt), M5.Display.width() / 2, promptY);
    M5.Display.clearClipRect();

    // Options listed below the prompt.
    int lineH = 18;
    int startY = promptY + maxPromptH + 4;
    int availableH = M5.Display.height() - startY;
    int visibleCount = min(count, availableH / lineH);
    int offset = 0;
    if (selected >= visibleCount && count > visibleCount) {
        offset = selected - visibleCount + 1;
    }

    M5.Display.setTextDatum(textdatum_t::middle_center);
    for (int i = 0; i < visibleCount && (offset + i) < count; i++) {
        int idx = offset + i;
        int y = startY + i * lineH + lineH / 2;
        if (idx == selected) {
            M5.Display.setTextColor(TFT_GREEN);
            M5.Display.drawString("> " + String(options[idx]) + " <", M5.Display.width() / 2, y);
        } else {
            M5.Display.setTextColor(TFT_WHITE);
            M5.Display.drawString(options[idx], M5.Display.width() / 2, y);
        }
    }
}
