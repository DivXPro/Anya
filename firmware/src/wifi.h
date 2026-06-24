#pragma once
#include <WiFi.h>
#include <Preferences.h>

extern String wifi_local_ip;
extern String wifi_ssid;

bool wifi_init();
bool wifi_connected();
bool wifi_has_credentials();
int8_t wifi_rssi();
void wifi_save_credentials(const char* ssid, const char* password);
void wifi_clear_credentials();

void wifi_save_desktop_ip(const char* ip, uint16_t port);
String wifi_get_desktop_ip();
uint16_t wifi_get_desktop_port();
String wifi_get_local_ip();

void wifi_save_bound_desktop(const char* desktop_id, const char* ip, uint16_t port);
String wifi_get_bound_desktop_id();
String wifi_get_bound_desktop_ip();
uint16_t wifi_get_bound_desktop_port();
