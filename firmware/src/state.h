#pragma once
#include <cstdint>
#include <cstddef>

enum class State { WIFI_SETUP, WIFI_CONNECTING, PAIR_READY, PAIRING, IDLE, LISTENING, SENDING, PROCESSING, PLAYING, MENU };

void state_init();
void state_transition(State s);
void state_set_agent(const char* name);
void state_set_summary(const char* text);
void state_force_idle();
void state_play_audio(const uint8_t* data, size_t len);
void state_update_status(int8_t rssi, bool wifiConnected, bool wsConnected);
State state_current();
