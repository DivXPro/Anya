#pragma once
#include <M5Unified.h>
#include "lang.h"

// Portrait rotation for M5StickC S3.
// 0 = USB connector at the bottom (native portrait).
// 2 = USB connector at the top (portrait flipped 180°).
constexpr uint8_t DISPLAY_ROTATION = 0;

void disp_init();
void disp_status_bar(int8_t rssi, bool wifiConnected, bool wsConnected, const char* agent, const char* ssid = nullptr);
void disp_wifi_setup(const char* hotspotSsid, const char* agent);
void disp_wifi_connecting(const char* ssid, const char* agent);
void disp_pair_ready(const char* agent, const char* ssid = nullptr);
void disp_pairing(const char* agent);
void disp_idle(const char* agent, bool connected);
void disp_connecting(const char* desktopName, const char* agent);
void disp_listening(const char* agent);
void disp_sending(const char* agent);
void disp_processing(const char* agent);
void disp_playing(const char* summary, const char* agent);
void disp_error(const char* msg, const char* agent);
void disp_menu(const char* agent, int selected, const Str* items, int count, int8_t rssi, bool wifiConnected, bool wsConnected);
void disp_agent_session_menu(const char* agent, int selected, const char* const* titles, const char* const* cwd, int count, int8_t rssi, bool wifiConnected, bool wsConnected);
void disp_confirm(const char* agent, const char* prompt, const char* const* options, int count, int selected);
void disp_updating(int8_t percent, const char* version, const char* agent);
void disp_updating_progress(int8_t percent);

// Call from the main loop to keep the mascot animating on screens that show it.
void disp_animate_mascot();
void disp_animate_text();
bool disp_text_showing_for(unsigned long ms);
