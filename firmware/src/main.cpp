#include <M5Unified.h>
#include <Preferences.h>
#include <esp_mac.h>
#include <esp_log.h>
#include "elf_wifi.h"
#include "wifi_portal.h"
#include "ws_client.h"
#include "mdns_client.h"
#include "audio.h"
#include "display.h"
#include "buttons.h"
#include "protocol.h"
#include "state.h"

#ifndef FIRMWARE_VERSION
#define FIRMWARE_VERSION "0.0.0-dev"
#endif

static char deviceID[37];
static char deviceName[32];
static bool advertising = false;

static const char* menuItems[] = {"Choose WiFi", "Repair", "Test Speaker", "Back"};
static const int menuCount = 4;
static int menuSelected = 0;
static bool inMenu = false;

void init_device_identity() {
    Preferences prefs;
    prefs.begin("elf-device", false);
    String storedID = prefs.getString("device_id", "");
    String storedName = prefs.getString("device_name", "");
    if (storedID.length() == 0) {
        uint8_t mac[6];
        esp_read_mac(mac, ESP_MAC_WIFI_STA);
        snprintf(deviceID, sizeof(deviceID), "%02x%02x%02x%02x%02x%02x-%04x",
            mac[0], mac[1], mac[2], mac[3], mac[4], mac[5], esp_random() & 0xFFFF);
        snprintf(deviceName, sizeof(deviceName), "elf-%02x%02x", mac[4], mac[5]);
        prefs.putString("device_id", deviceID);
        prefs.putString("device_name", deviceName);
    } else {
        strncpy(deviceID, storedID.c_str(), sizeof(deviceID) - 1);
        deviceID[sizeof(deviceID) - 1] = '\0';
        if (storedName.length() == 0) storedName = "elf-device";
        strncpy(deviceName, storedName.c_str(), sizeof(deviceName) - 1);
        deviceName[sizeof(deviceName) - 1] = '\0';
    }
    prefs.end();
}

void setup() {
    auto cfg = M5.config();
    M5.begin(cfg);
    ESP_LOGI("elf", "firmware setup start, version=%s", FIRMWARE_VERSION);
    init_device_identity();

    state_init();
    audio_init();
    btn_init();
    ws_init();
    protocol_init();

    bool wifiOK = wifi_init();
    if (!wifiOK) {
        state_transition(State::WIFI_SETUP);
        wifiOK = wifi_portal_begin();
    }
    if (!wifiOK) {
        disp_error("WiFi setup failed", "Elf");
        return;
    }
    http_setup_connect_endpoint();
    state_transition(State::PAIR_READY);

    String boundID = wifi_get_bound_desktop_id();
    String boundIP = wifi_get_bound_desktop_ip();
    uint16_t boundPort = wifi_get_bound_desktop_port();

    if (boundIP.length() > 0) {
        ESP_LOGI("main", "bound reconnect to %s:%d", boundIP.c_str(), boundPort);
        ws_set_hello_data(deviceID, deviceName, boundID.c_str(), "");
        ws_connect(boundIP.c_str(), boundPort);
        state_transition(State::IDLE);
    } else {
        mdns_start_advertise(deviceID, deviceName);
        advertising = true;
        state_transition(State::PAIR_READY);
    }

    btn_on_ptt_press([]() {
        if (inMenu) {
            ESP_LOGI("main", "menu confirm: %s", menuItems[menuSelected]);
            if (menuSelected == 0) {
                // Choose WiFi: open captive portal to reconfigure network
                inMenu = false;
                state_transition(State::WIFI_SETUP);
                if (wifi_portal_begin()) {
                    state_transition(State::IDLE);
                    // Reconnect to the bound desktop if available
                    String boundID = wifi_get_bound_desktop_id();
                    String boundIP = wifi_get_bound_desktop_ip();
                    uint16_t boundPort = wifi_get_bound_desktop_port();
                    if (boundIP.length() > 0) {
                        ws_set_hello_data(deviceID, deviceName, boundID.c_str(), "");
                        ws_connect(boundIP.c_str(), boundPort);
                    }
                } else {
                    disp_error("WiFi setup failed", "Elf");
                }
            } else if (menuSelected == 1) {
                // Repair: clear binding and start fresh advertising
                inMenu = false;
                wifi_clear_bound_desktop();
                ws_disconnect();
                state_transition(State::PAIR_READY);
                if (!advertising) {
                    mdns_start_advertise(deviceID, deviceName);
                    advertising = true;
                }
            } else if (menuSelected == 2) {
                // Test Speaker: play a 1kHz tone for 1 second
                ESP_LOGI("main", "menu: test speaker");
                audio_play_test_tone();
                disp_playing("Playing test tone...", deviceName);
                delay(1200);
                state_transition(State::MENU);
                disp_menu(deviceName, menuSelected, menuItems, menuCount, wifi_rssi(), wifi_connected(), ws_connected());
            } else if (menuSelected == 3) {
                // Back: return to normal idle screen
                inMenu = false;
                state_transition(State::IDLE);
            }
            return;
        }
        if (!ws_connected()) {
            ESP_LOGI("main", "PTT press ignored: not connected");
            return;
        }
        ESP_LOGI("main", "PTT press -> LISTENING");
        state_transition(State::LISTENING);
        audio_start_recording();
        protocol_send_audio_start();
    });

    btn_on_ptt_release([]() {
        if (inMenu) return;
        if (!ws_connected()) {
            ESP_LOGI("main", "PTT release ignored: not connected");
            return;
        }
        ESP_LOGI("main", "PTT release -> SENDING");
        audio_stop_recording();
        protocol_send_audio_end();
        state_transition(State::SENDING);
    });

    btn_on_next([]() {
        if (!inMenu) {
            ESP_LOGI("main", "enter menu");
            inMenu = true;
            menuSelected = 0;
            state_transition(State::MENU);
        } else {
            menuSelected = (menuSelected + 1) % menuCount;
            ESP_LOGI("main", "menu next -> %d", menuSelected);
            disp_menu(deviceName, menuSelected, menuItems, menuCount, wifi_rssi(), wifi_connected(), ws_connected());
        }
    });
}

void loop() {
    M5.update();
    btn_loop();
    ws_loop();
    http_loop();

    // Keep menu flag consistent if something else (e.g. disconnect) changed the state.
    if (inMenu && state_current() != State::MENU) {
        inMenu = false;
    }

    // Animate mascot on screens that show it.
    disp_animate_mascot();

    // Update status bar with live WiFi RSSI + WS connection (every ~1s)
    {
        static unsigned long lastStatusUpdate = 0;
        unsigned long now = millis();
        if (now - lastStatusUpdate > 1000) {
            lastStatusUpdate = now;
            bool wifiConn = wifi_connected();
            int8_t rssi = wifiConn ? wifi_rssi() : 0;
            state_update_status(rssi, wifiConn, ws_connected());
        }
    }

    // WebSocket lifecycle: hello is sent from the onEvent(CONNECTED) callback.
    // When connected, stop advertising to avoid showing up in the desktop scan list.
    // When disconnected, keep advertising so a desktop can find us, but show:
    //   - "Ready to pair" if this device has never been paired,
    //   - "Disconnected" if it has a bound desktop but cannot reach it.
    // Menu mode overrides automatic state transitions.
    if (!inMenu) {
        if (ws_connected()) {
            if (advertising) {
                mdns_stop_advertise();
                advertising = false;
            }
        } else if (wifi_connected()) {
            if (!advertising) {
                mdns_start_advertise(deviceID, deviceName);
                advertising = true;
            }
            String boundID = wifi_get_bound_desktop_id();
            if (boundID.length() == 0) {
                state_transition(State::PAIR_READY);
            } else if (state_current() != State::IDLE) {
                state_transition(State::IDLE);
            }
        }
    }

    if (connect_is_requested()) {
        connect_clear_request();
        String ip = connect_get_ip();
        uint16_t port = connect_get_port();
        String token = connect_get_token();
        String boundID = wifi_get_bound_desktop_id();
        ESP_LOGI("main", "pairing request to %s:%d token=%s", ip.c_str(), port, token.c_str());
        ws_set_hello_data(deviceID, deviceName, boundID.c_str(), token.c_str());
        state_transition(State::PAIRING);
        // Give the HTTP server time to finish sending the 200 OK response before
        // this task starts the WebSocket handshake.
        delay(100);
        ws_connect(ip.c_str(), port);
    }

    if (audio_is_recording()) {
        static uint8_t buf[1024];
        size_t len = audio_capture(buf, sizeof(buf));
        if (len > 0) {
            protocol_send_audio_chunk(buf, len);
        }
    }

    delay(10);
}
