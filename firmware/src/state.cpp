#include "state.h"
#include "display.h"
#include "audio.h"
#include "elf_wifi.h"
#include <cstring>

static State current = State::WIFI_SETUP;
static char agentName[32] = "Claude";
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
    disp_status_bar(rssi, wifiConnected, wsConnected, agentName, status_ssid());
}

void state_transition(State s) {
    if (s == current) return;
    current = s;
    switch (s) {
        case State::WIFI_SETUP:
            disp_wifi_setup("Elf-hotspot", agentName);
            break;
        case State::WIFI_CONNECTING:
            disp_wifi_connecting(wifi_ssid.c_str(), agentName);
            break;
        case State::PAIR_READY:
            disp_pair_ready(agentName);
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
            break;
        case State::MENU:
            disp_menu(agentName, 0, nullptr, 0, lastRssi, lastWifiConnected, lastWsConnected);
            break;
    }
    disp_status_bar(lastRssi, lastWifiConnected, lastWsConnected, agentName, status_ssid());
}

void state_set_agent(const char* name) {
    strncpy(agentName, name, sizeof(agentName)-1);
    agentName[sizeof(agentName)-1] = '\0';
    connected = true;
    lastWifiConnected = true;
    lastWsConnected = true;
}

void state_set_summary(const char* text) {
    disp_playing(text, agentName);
    disp_status_bar(lastRssi, lastWifiConnected, lastWsConnected, agentName, status_ssid());
}

void state_force_idle() {
    connected = false;
    lastWifiConnected = false;
    lastWsConnected = false;
    audio_stop_recording();
    current = State::IDLE;
    disp_idle(agentName, false);
    disp_status_bar(lastRssi, false, false, agentName, status_ssid());
}

void state_play_audio(const uint8_t* data, size_t len) {
    audio_play(data, len);
}

State state_current() { return current; }
