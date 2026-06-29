#pragma once
#include <Arduino.h>

enum class PortalResult : uint8_t {
    SUCCESS = 0,
    CANCELLED,   // user opened the device menu before configuring WiFi
    FAILED
};

PortalResult wifi_portal_begin();
