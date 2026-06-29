#pragma once
#include <cstdint>
#include <cstddef>

enum class State { WIFI_SETUP, WIFI_CONNECTING, PAIR_READY, PAIRING, IDLE, LISTENING, SENDING, PROCESSING, PLAYING, MENU, CONFIRM, UPDATING };

struct ConfirmOption {
    char id[32];
    char label[64];
};

struct ConfirmState {
    char requestId[64];
    char prompt[256];
    ConfirmOption options[8];
    int optionCount;
    int selected;
    bool active;
};

void state_init();
void state_transition(State s);
void state_set_agent(const char* name);
void state_set_summary(const char* text);
void state_force_idle();
void state_play_audio(const uint8_t* data, size_t len);
void state_update_status(int8_t rssi, bool wifiConnected, bool wsConnected);
void state_set_ota_progress(int8_t percent, const char* version);
State state_current();

void state_set_confirm(const char* requestId, const char* prompt, const ConfirmOption* options, int optionCount);
const ConfirmState& state_confirm_state();
void state_confirm_next();
void state_confirm_select();
