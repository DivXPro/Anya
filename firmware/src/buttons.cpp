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
    bool pttPressedDirect = rawA && !lastRawA;
    bool pttReleasedDirect = !rawA && lastRawA;
    bool confirmPressedDirect = rawB && !lastRawB;
    lastRawA = rawA;
    lastRawB = rawB;

    // Also keep M5Unified button detection in case the board is ever recognized.
    // Only BtnA (front big button) is used for PTT speak; BtnB is reserved.
    bool pttPressed = pttPressedDirect || M5.BtnA.wasPressed();
    bool pttReleased = pttReleasedDirect || M5.BtnA.wasReleased();
    bool confirmPressed = confirmPressedDirect || M5.BtnB.wasPressed();

    if (pttPressed && pttPressCb) {
        ESP_LOGI("btn", "PTT pressed");
        pttHeld = true;
        pttPressCb();
    }
    if (pttReleased && pttReleaseCb && pttHeld) {
        ESP_LOGI("btn", "PTT released");
        pttHeld = false;
        pttReleaseCb();
    }
    if (confirmPressed) {
        ESP_LOGI("btn", "confirm/next pressed");
        if (confirmCb) confirmCb();
        else if (nextCb) nextCb();
    }
}

void btn_on_ptt_press(ButtonCallback cb) { pttPressCb = cb; }
void btn_on_ptt_release(ButtonCallback cb) { pttReleaseCb = cb; }
void btn_on_confirm(ButtonCallback cb) { confirmCb = cb; }
void btn_on_next(ButtonCallback cb) { nextCb = cb; }
