#pragma once
#include <M5Unified.h>

void disp_init();
void disp_status_bar(int8_t rssi, bool wifiConnected, bool wsConnected, const char* agent, const char* ssid = nullptr);
void disp_wifi_setup(const char* hotspotSsid, const char* agent);
void disp_wifi_connecting(const char* ssid, const char* agent);
void disp_pair_ready(const char* agent);
void disp_pairing(const char* agent);
void disp_idle(const char* agent, bool connected);
void disp_connecting(const char* desktopName, const char* agent);
void disp_listening(const char* agent);
void disp_sending(const char* agent);
void disp_processing(const char* agent);
void disp_playing(const char* summary, const char* agent);
void disp_error(const char* msg, const char* agent);
void disp_menu(const char* agent, int selected, const char* const* items, int count);
