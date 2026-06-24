#include "wifi.h"

String wifi_local_ip = "";
String wifi_ssid = "";
Preferences prefs;

bool wifi_has_credentials() {
    prefs.begin("elf-wifi", true);
    String ssid = prefs.getString("ssid", "");
    prefs.end();
    return ssid.length() > 0;
}

void wifi_save_credentials(const char* ssid, const char* password) {
    prefs.begin("elf-wifi", false);
    prefs.putString("ssid", ssid);
    prefs.putString("pass", password);
    prefs.end();
}

void wifi_clear_credentials() {
    prefs.begin("elf-wifi", false);
    prefs.remove("ssid");
    prefs.remove("pass");
    prefs.end();
}

bool wifi_init() {
    prefs.begin("elf-wifi", true);
    String ssid = prefs.getString("ssid", "");
    String pass = prefs.getString("pass", "");
    prefs.end();

    if (ssid.length() == 0) {
        return false;
    }

    WiFi.mode(WIFI_STA);
    WiFi.begin(ssid.c_str(), pass.c_str());

    int attempts = 0;
    while (WiFi.status() != WL_CONNECTED && attempts < 40) {
        delay(500);
        attempts++;
    }

    if (WiFi.status() == WL_CONNECTED) {
        wifi_local_ip = WiFi.localIP().toString();
        wifi_ssid = ssid;
        return true;
    }
    return false;
}

bool wifi_connected() {
    return WiFi.status() == WL_CONNECTED;
}

String wifi_get_local_ip() {
    return WiFi.localIP().toString();
}

void wifi_save_desktop_ip(const char* ip, uint16_t port) {
    prefs.begin("elf-wifi", false);
    prefs.putString("desktop_ip", ip);
    prefs.putUShort("desktop_port", port);
    prefs.end();
}

String wifi_get_desktop_ip() {
    prefs.begin("elf-wifi", true);
    String ip = prefs.getString("desktop_ip", "");
    prefs.end();
    return ip;
}

uint16_t wifi_get_desktop_port() {
    prefs.begin("elf-wifi", true);
    uint16_t port = prefs.getUShort("desktop_port", 9876);
    prefs.end();
    return port;
}

void wifi_save_bound_desktop(const char* desktop_id, const char* ip, uint16_t port) {
    prefs.begin("elf-wifi", false);
    prefs.putString("bound_desktop_id", desktop_id);
    prefs.putString("bound_desktop_ip", ip);
    prefs.putUShort("bound_desktop_port", port);
    prefs.end();
}

String wifi_get_bound_desktop_id() {
    prefs.begin("elf-wifi", true);
    String id = prefs.getString("bound_desktop_id", "");
    prefs.end();
    return id;
}

String wifi_get_bound_desktop_ip() {
    prefs.begin("elf-wifi", true);
    String ip = prefs.getString("bound_desktop_ip", "");
    prefs.end();
    return ip;
}

uint16_t wifi_get_bound_desktop_port() {
    prefs.begin("elf-wifi", true);
    uint16_t port = prefs.getUShort("bound_desktop_port", 9876);
    prefs.end();
    return port;
}
