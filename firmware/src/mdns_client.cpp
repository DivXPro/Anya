#include "mdns_client.h"
#include "wifi.h"
#include <WiFi.h>
#include <ArduinoJson.h>

static WebServer httpServer(80);
static bool connectRequested = false;
static String connectIP = "";
static uint16_t connectPort = 9876;
static String connectDesktopID = "";
static String connectPairingToken = "";

void mdns_start_advertise(const char* deviceID, const char* deviceName) {
    MDNS.begin(deviceName);
    MDNS.addService("_elf-device", "_tcp", 80);
    MDNS.addServiceTxt("_elf-device", "_tcp", "device_id", deviceID);
    MDNS.addServiceTxt("_elf-device", "_tcp", "name", deviceName);
}

void mdns_stop_advertise() { MDNS.end(); }

void http_setup_connect_endpoint() {
    httpServer.on("/connect", HTTP_POST, []() {
        String body = httpServer.arg("plain");
        StaticJsonDocument<256> doc;
        deserializeJson(doc, body);
        const char* desktopIP = doc["desktop_ip"];
        uint16_t desktopPort = doc["desktop_port"] | 9876;
        const char* desktopID = doc["desktop_id"];
        const char* pairingToken = doc["pairing_token"] | "";
        if (!desktopIP || !desktopID) {
            httpServer.send(400, "text/plain", "Missing fields");
            return;
        }
        String boundID = wifi_get_bound_desktop_id();
        if (boundID.length() > 0 && boundID != desktopID) {
            httpServer.send(403, "text/plain", "Wrong desktop");
            return;
        }
        wifi_save_desktop_ip(desktopIP, desktopPort);
        httpServer.send(200, "text/plain", "OK");
        connectRequested = true;
        connectIP = desktopIP;
        connectPort = desktopPort;
        connectDesktopID = desktopID;
        connectPairingToken = pairingToken;
    });
    httpServer.begin();
}

void http_loop() { httpServer.handleClient(); }
bool connect_is_requested() { return connectRequested; }
void connect_clear_request() { connectRequested = false; }
String connect_get_ip() { return connectIP; }
uint16_t connect_get_port() { return connectPort; }
String connect_get_token() { return connectPairingToken; }
