#include "display.h"

void disp_init() {
    M5.Display.setRotation(1);
    M5.Display.fillScreen(TFT_WHITE);
    M5.Display.setTextColor(TFT_BLACK);
}

static void drawMascot() {
    M5.Display.fillCircle(80, 42, 34, TFT_LIGHTGREY);
    M5.Display.setTextColor(TFT_BLACK);
    M5.Display.setTextSize(2);
    M5.Display.setCursor(54, 36);
    M5.Display.print("Elf");
}

static void drawPrompt(const char* line1, const char* line2 = nullptr) {
    M5.Display.setTextSize(2);
    M5.Display.setTextColor(TFT_BLACK);
    M5.Display.setCursor(18, 98);
    M5.Display.print(line1);
    if (line2) {
        M5.Display.setCursor(42, 122);
        M5.Display.print(line2);
    }
}

void disp_wifi_setup(const char* hotspotSsid) {
    M5.Display.fillScreen(TFT_WHITE);
    drawMascot();
    drawPrompt("Connect Elf-hotspot", "and setup");
}

void disp_wifi_connecting(const char* ssid) {
    M5.Display.fillScreen(TFT_WHITE);
    drawMascot();
    drawPrompt("Connecting WiFi...", nullptr);
}

void disp_pair_ready() {
    M5.Display.fillScreen(TFT_WHITE);
    drawMascot();
    drawPrompt("Click below to pair", nullptr);
}

void disp_pairing() {
    M5.Display.fillScreen(TFT_WHITE);
    drawMascot();
    drawPrompt("Pairing", "Click to stop");
}

void disp_idle(const char* agentName, bool connected) {
    M5.Display.fillScreen(TFT_WHITE);
    drawMascot();
    drawPrompt("Click to speak", nullptr);
    M5.Display.setTextSize(1);
    M5.Display.setCursor(8, 8);
    M5.Display.printf("%s %s", connected ? "online" : "offline", agentName);
}

void disp_listening() {
    M5.Display.fillScreen(TFT_WHITE);
    drawMascot();
    drawPrompt("Listening...", nullptr);
}

void disp_sending() {
    M5.Display.fillScreen(TFT_WHITE);
    drawMascot();
    drawPrompt("Sending...", nullptr);
}

void disp_processing() {
    M5.Display.fillScreen(TFT_WHITE);
    drawMascot();
    drawPrompt("Processing...", nullptr);
}

void disp_playing(const char* summary) {
    M5.Display.fillScreen(TFT_WHITE);
    M5.Display.setTextColor(TFT_BLACK);
    M5.Display.setTextSize(2);
    M5.Display.setCursor(10, 28);
    M5.Display.print(summary);
}

void disp_connecting(const char* desktopName) {
    M5.Display.fillScreen(TFT_WHITE);
    drawMascot();
    M5.Display.setCursor(10, 98);
    M5.Display.printf("Connecting:\n%s", desktopName);
}

void disp_error(const char* msg) {
    M5.Display.fillScreen(TFT_RED);
    M5.Display.setCursor(10, 30);
    M5.Display.print(msg);
}
