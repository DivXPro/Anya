#include "state.h"
#include "display.h"
#include "audio.h"
#include "elf_wifi.h"
#include <cstring>

static State current = State::WIFI_SETUP;
static char agentName[32] = "Claude";
static bool connected = false;
static int8_t lastRssi = 0;
static bool lastWsConnected = false;

void state_init() {
    disp_init();
}

void state_update_status(int8_t rssi, bool wsConnected) {
    if (rssi == lastRssi && wsConnected == lastWsConnected) return;
    lastRssi = rssi;
    lastWsConnected = wsConnected;
    disp_status_bar(rssi, wsConnected, agentName);
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
    }
    disp_status_bar(lastRssi, lastWsConnected, agentName);
}

void state_set_agent(const char* name) {
    strncpy(agentName, name, sizeof(agentName)-1);
    agentName[sizeof(agentName)-1] = '\0';
    connected = true;
    lastWsConnected = true;
}

void state_set_summary(const char* text) {
    disp_playing(text, agentName);
    disp_status_bar(lastRssi, lastWsConnected, agentName);
}

void state_force_idle() {
    connected = false;
    lastWsConnected = false;
    current = State::IDLE;
    disp_idle(agentName, false);
    disp_status_bar(lastRssi, false, agentName);
}

void state_play_audio(const uint8_t* data, size_t len) {
    audio_play(data, len);
}

State state_current() { return current; }
