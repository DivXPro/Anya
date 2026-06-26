#include "buttons.h"
#include <M5Unified.h>
#include <esp_log.h>

// M5StickC S3 physical button GPIOs.
// M5Unified (as of 0.2.17) does not auto-detect M5StickC S3 when the board
// is set to esp32-s3-devkitc-1, so BtnA/BtnB are not wired to these pins.
// Read them directly with pull-ups; the buttons are active-low.
#ifndef BTN_A_PIN
#define BTN_A_PIN GPIO_NUM_11
#endif
#ifndef BTN_B_PIN
#define BTN_B_PIN GPIO_NUM_12
#endif

static ButtonCallback pttPressCb = nullptr;
static ButtonCallback pttReleaseCb = nullptr;
static ButtonCallback confirmCb = nullptr;
static ButtonCallback nextCb = nullptr;
static bool pttHeld = false;
static bool lastRawA = false;
static bool lastRawB = false;

void btn_init() {
    M5.update();
    pinMode(BTN_A_PIN, INPUT_PULLUP);
    pinMode(BTN_B_PIN, INPUT_PULLUP);
    lastRawA = digitalRead(BTN_A_PIN) == LOW;
    lastRawB = digitalRead(BTN_B_PIN) == LOW;
}

void btn_loop() {
    M5.update();

    bool rawA = digitalRead(BTN_A_PIN) == LOW;
    bool rawB = digitalRead(BTN_B_PIN) == LOW;

    // Edge detection for direct GPIO buttons.
    bool pressedDirect = (rawA && !lastRawA) || (rawB && !lastRawB);
    bool releasedDirect = (!rawA && lastRawA) || (!rawB && lastRawB);
    lastRawA = rawA;
    lastRawB = rawB;

    // Also keep M5Unified button detection in case the board is ever recognized.
    bool pressed = pressedDirect || M5.BtnA.wasPressed() || M5.BtnB.wasPressed();
    bool released = releasedDirect || M5.BtnA.wasReleased() || M5.BtnB.wasReleased();

    if (pressed && pttPressCb) {
        ESP_LOGI("btn", "PTT pressed (A=%d B=%d)", rawA, rawB);
        pttHeld = true;
        pttPressCb();
    }
    if (released && pttReleaseCb && pttHeld) {
        ESP_LOGI("btn", "PTT released (A=%d B=%d)", rawA, rawB);
        pttHeld = false;
        pttReleaseCb();
    }
}

void btn_on_ptt_press(ButtonCallback cb) { pttPressCb = cb; }
void btn_on_ptt_release(ButtonCallback cb) { pttReleaseCb = cb; }
void btn_on_confirm(ButtonCallback cb) { confirmCb = cb; }
void btn_on_next(ButtonCallback cb) { nextCb = cb; }
