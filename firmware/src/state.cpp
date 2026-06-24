#include "state.h"
#include "display.h"
#include "audio.h"
#include "wifi.h"
#include <cstring>

static State current = State::IDLE;
static char agentName[32] = "Claude Code";
static bool connected = false;

void state_init() {
    disp_init();
    disp_idle(agentName, false);
}

void state_transition(State s) {
    current = s;
    switch (s) {
        case State::WIFI_SETUP:      disp_wifi_setup("Elf-hotspot"); break;
        case State::WIFI_CONNECTING: disp_wifi_connecting(wifi_ssid.c_str()); break;
        case State::PAIR_READY:      disp_pair_ready(); break;
        case State::PAIRING:         disp_pairing(); break;
        case State::IDLE:            disp_idle(agentName, connected); break;
        case State::LISTENING:       disp_listening(); break;
        case State::SENDING:         disp_sending(); break;
        case State::PROCESSING:      disp_processing(); break;
        case State::PLAYING:         break;
    }
}

void state_set_agent(const char* name) {
    strncpy(agentName, name, sizeof(agentName)-1);
    connected = true;
}

void state_set_summary(const char* text) {
    disp_playing(text);
}

void state_force_idle() {
    connected = false;
    current = State::IDLE;
    disp_idle(agentName, false);
}

void state_play_audio(const uint8_t* data, size_t len) {
    audio_play(data, len);
}

State state_current() { return current; }
