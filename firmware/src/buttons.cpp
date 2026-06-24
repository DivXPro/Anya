#include "buttons.h"
#include <M5Unified.h>

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

    if (M5.BtnA.wasPressed() && pttPressCb) {
        pttHeld = true;
        pttPressCb();
    }
    if (M5.BtnA.wasReleased() && pttReleaseCb && pttHeld) {
        pttHeld = false;
        pttReleaseCb();
    }
}

void btn_on_ptt_press(ButtonCallback cb) { pttPressCb = cb; }
void btn_on_ptt_release(ButtonCallback cb) { pttReleaseCb = cb; }
void btn_on_confirm(ButtonCallback cb) { confirmCb = cb; }
void btn_on_next(ButtonCallback cb) { nextCb = cb; }
