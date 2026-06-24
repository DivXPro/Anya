#pragma once
#include <Arduino.h>
#include <ESPmDNS.h>
#include <WebServer.h>

void mdns_start_advertise(const char* deviceID, const char* deviceName);
void mdns_stop_advertise();

void http_setup_connect_endpoint();
void http_loop();
bool connect_is_requested();
void connect_clear_request();
String connect_get_ip();
uint16_t connect_get_port();
String connect_get_token();
