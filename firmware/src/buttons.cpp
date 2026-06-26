#include "buttons.h"
#include <M5Unified.h>
#include <esp_log.h>

static ButtonCallback pttPressCb = nullptr;
static ButtonCallback pttReleaseCb = nullptr;
static ButtonCallback confirmCb = nullptr;
static ButtonCallback nextCb = nullptr;
static bool pttHeld = false;

void btn_init() {
    M5.update();
}

void btn_loop() {
    M5.update();

    bool pressed = M5.BtnA.wasPressed() || M5.BtnB.wasPressed();
    bool released = M5.BtnA.wasReleased() || M5.BtnB.wasReleased();

    if (pressed && pttPressCb) {
        ESP_LOGI("btn", "PTT pressed");
        pttHeld = true;
        pttPressCb();
    }
    if (released && pttReleaseCb && pttHeld) {
        ESP_LOGI("btn", "PTT released");
        pttHeld = false;
        pttReleaseCb();
    }
}

void btn_on_ptt_press(ButtonCallback cb) { pttPressCb = cb; }
void btn_on_ptt_release(ButtonCallback cb) { pttReleaseCb = cb; }
void btn_on_confirm(ButtonCallback cb) { confirmCb = cb; }
void btn_on_next(ButtonCallback cb) { nextCb = cb; }
