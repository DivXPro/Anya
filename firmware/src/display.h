#pragma once
#include <M5Unified.h>

void disp_init();
void disp_wifi_setup(const char* hotspotSsid);
void disp_wifi_connecting(const char* ssid);
void disp_pair_ready();
void disp_pairing();
void disp_idle(const char* agentName, bool connected);
void disp_connecting(const char* desktopName);
void disp_listening();
void disp_sending();
void disp_processing();
void disp_playing(const char* summary);
void disp_error(const char* msg);
