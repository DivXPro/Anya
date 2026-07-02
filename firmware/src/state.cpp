#include "state.h"
#include "display.h"
#include "audio.h"
#include "elf_wifi.h"
#include "ota.h"
#include <cstring>

static State current = State::WIFI_SETUP;
static char agentName[32] = "Claude";
static char summaryText[256] = "";
static bool connected = false;
static int8_t lastRssi = 0;
static bool lastWifiConnected = false;
static bool lastWsConnected = false;

static const char* status_ssid() {
    return (current == State::PAIR_READY) ? wifi_ssid.c_str() : nullptr;
}

void state_init() {
    disp_init();
    // Draw empty status bar immediately so layout is consistent from the start
    disp_status_bar(-1, false, false, "", status_ssid());
}

void state_update_status(int8_t rssi, bool wifiConnected, bool wsConnected) {
    if (rssi == lastRssi && wifiConnected == lastWifiConnected && wsConnected == lastWsConnected) return;
    lastRssi = rssi;
    lastWifiConnected = wifiConnected;
    lastWsConnected = wsConnected;
    bool idlePromptChanged = (current == State::IDLE && connected != wsConnected);
    connected = wsConnected;
    if (idlePromptChanged) {
        disp_idle(agentName, connected);
        disp_status_bar(rssi, wifiConnected, wsConnected, agentName, status_ssid());
        return;
    }
    disp_status_bar(rssi, wifiConnected, wsConnected, agentName, status_ssid());
}

void state_transition(State s) {
    if (s == current) return;
    current = s;
    switch (s) {
        case State::WIFI_SETUP:
            disp_wifi_setup("Anya", agentName);
            break;
        case State::WIFI_CONNECTING:
            disp_wifi_connecting(wifi_ssid.c_str(), agentName);
            break;
        case State::PAIR_READY:
            disp_pair_ready(agentName, status_ssid());
            break;
        case State::PAIRING:
            disp_pairing(agentName);
            break;
        case State::IDLE:
            disp_idle(agentName, connected);
            break;
        case State::LISTENING:
            disp_listening(agentName);
            break;
        case State::SENDING:
            disp_sending(agentName);
            break;
        case State::PROCESSING:
            disp_processing(agentName);
            break;
        case State::PLAYING:
            disp_playing(summaryText, agentName);
            break;
        case State::MENU:
            // Menu items are drawn by main.cpp after it sets the menu level.
            break;
        case State::CONFIRM: {
            const ConfirmState& cs = state_confirm_state();
            if (cs.active && cs.optionCount > 0) {
                const char* opts[8];
                for (int i = 0; i < cs.optionCount; i++) {
                    opts[i] = cs.options[i].label;
                }
                disp_confirm(agentName, cs.prompt, opts, cs.optionCount, cs.selected);
            }
            break;
        }
        case State::UPDATING:
            disp_updating(0, "", agentName);
            break;
    }
    // PAIR_READY already draws the status bar with the SSID in one shot.
    if (current != State::PAIR_READY) {
        disp_status_bar(lastRssi, lastWifiConnected, lastWsConnected, agentName, status_ssid());
    }
}

void state_set_agent(const char* name) {
    strncpy(agentName, name, sizeof(agentName)-1);
    agentName[sizeof(agentName)-1] = '\0';
    connected = true;
    lastWifiConnected = true;
    lastWsConnected = true;
    // If we are already on the idle screen, refresh it so the prompt switches
    // from "Disconnected" back to "Click to speak" and the status bar shows
    // the agent name instead of "No agent".
    if (current == State::IDLE) {
        disp_idle(agentName, true);
        disp_status_bar(lastRssi, lastWifiConnected, lastWsConnected, agentName, status_ssid());
    }
}

void state_set_summary(const char* text) {
    bool changed = (strcmp(summaryText, text ? text : "") != 0);
    strncpy(summaryText, text ? text : "", sizeof(summaryText) - 1);
    summaryText[sizeof(summaryText) - 1] = '\0';
    // If we are already showing the reply, refresh the display only when the
    // text actually changes. Otherwise the transition to PLAYING will paint it.
    if (current == State::PLAYING && changed) {
        disp_playing(summaryText, agentName);
        disp_status_bar(lastRssi, lastWifiConnected, lastWsConnected, agentName, status_ssid());
    }
}

void state_force_idle() {
    if (current == State::UPDATING) {
        ota_abort();
    }
    audio_stop_recording();

    connected = lastWsConnected;
    current = State::IDLE;
    disp_idle(agentName, connected);
    disp_status_bar(lastRssi, lastWifiConnected, lastWsConnected, agentName, status_ssid());
}

void state_force_disconnected() {
    if (current == State::UPDATING) {
        ota_abort();
    }
    audio_stop_recording();

    bool wifiConnected = wifi_connected();
    int8_t rssi = wifiConnected ? wifi_rssi() : 0;
    bool alreadyDisconnectedIdle = (current == State::IDLE && !connected);
    connected = false;
    lastRssi = rssi;
    lastWifiConnected = wifiConnected;
    lastWsConnected = false;
    if (alreadyDisconnectedIdle) {
        disp_status_bar(lastRssi, lastWifiConnected, false, agentName, status_ssid());
        return;
    }

    current = State::IDLE;
    disp_idle(agentName, false);
    disp_status_bar(lastRssi, lastWifiConnected, false, agentName, status_ssid());
}

void state_play_audio(const uint8_t* data, size_t len) {
    audio_play(data, len);
}

void state_set_ota_progress(int8_t percent, const char* version) {
    if (current != State::UPDATING) return;
    disp_updating_progress(percent);
}

State state_current() { return current; }

static ConfirmState confirmState;

void state_set_confirm(const char* requestId, const char* prompt, const ConfirmOption* options, int optionCount) {
    confirmState.active = true;
    strncpy(confirmState.requestId, requestId ? requestId : "", sizeof(confirmState.requestId) - 1);
    confirmState.requestId[sizeof(confirmState.requestId) - 1] = '\0';
    strncpy(confirmState.prompt, prompt ? prompt : "", sizeof(confirmState.prompt) - 1);
    confirmState.prompt[sizeof(confirmState.prompt) - 1] = '\0';
    confirmState.optionCount = constrain(optionCount, 0, 8);
    for (int i = 0; i < confirmState.optionCount; i++) {
        strncpy(confirmState.options[i].id, options[i].id, sizeof(confirmState.options[i].id) - 1);
        confirmState.options[i].id[sizeof(confirmState.options[i].id) - 1] = '\0';
        strncpy(confirmState.options[i].label, options[i].label, sizeof(confirmState.options[i].label) - 1);
        confirmState.options[i].label[sizeof(confirmState.options[i].label) - 1] = '\0';
    }
    confirmState.selected = 0;
}

const ConfirmState& state_confirm_state() { return confirmState; }

void state_confirm_next() {
    if (!confirmState.active || confirmState.optionCount <= 0) return;
    confirmState.selected = (confirmState.selected + 1) % confirmState.optionCount;
}

void state_confirm_select() {
    confirmState.active = false;
}
