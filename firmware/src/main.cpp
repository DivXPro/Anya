#include <M5Unified.h>
#include <Preferences.h>
#include <esp_mac.h>
#include "wifi.h"
#include "wifi_portal.h"
#include "ws_client.h"
#include "mdns_client.h"
#include "audio.h"
#include "display.h"
#include "buttons.h"
#include "protocol.h"
#include "state.h"

static char deviceID[37];
static char deviceName[32];
static bool advertising = false;

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
    init_device_identity();

    state_init();
    audio_init();
    btn_init();
    protocol_init();

    bool wifiOK = wifi_init();
    if (!wifiOK) {
        state_transition(State::WIFI_SETUP);
        wifiOK = wifi_portal_begin();
    }
    if (!wifiOK) {
        disp_error("WiFi setup failed");
        return;
    }
    http_setup_connect_endpoint();
    state_transition(State::PAIR_READY);

    String boundID = wifi_get_bound_desktop_id();
    String boundIP = wifi_get_bound_desktop_ip();
    uint16_t boundPort = wifi_get_bound_desktop_port();
    bool connected = false;

    if (boundIP.length() > 0) {
        connected = ws_connect(boundIP.c_str(), boundPort);
    }

    if (!connected) {
        mdns_start_advertise(deviceID, deviceName);
        advertising = true;
        state_transition(State::PAIR_READY);
    }

    if (connected) {
        char hello[256];
        snprintf(hello, sizeof(hello),
            "{\"type\":\"hello\",\"payload\":{\"device_id\":\"%s\",\"name\":\"%s\",\"bound_desktop_id\":\"%s\"}}",
            deviceID, deviceName, boundID.c_str());
        ws_send_text(hello);
        state_transition(State::IDLE);
    }

    btn_on_ptt_press([]() {
        state_transition(State::LISTENING);
        audio_start_recording();
        protocol_send_audio_start();
    });

    btn_on_ptt_release([]() {
        audio_stop_recording();
        protocol_send_audio_end();
        state_transition(State::SENDING);
    });
}

void loop() {
    M5.update();
    btn_loop();
    ws_loop();
    http_loop();

    // Update status bar with live WiFi RSSI + WS connection (every ~1s)
    {
        static unsigned long lastStatusUpdate = 0;
        unsigned long now = millis();
        if (now - lastStatusUpdate > 1000) {
            lastStatusUpdate = now;
            int8_t rssi = wifi_connected() ? wifi_rssi() : 0;
            state_update_status(rssi, ws_connected());
        }
    }

    if (!ws_connected() && !advertising && wifi_connected()) {
        mdns_start_advertise(deviceID, deviceName);
        advertising = true;
        state_transition(State::PAIR_READY);
    }

    if (connect_is_requested()) {
        connect_clear_request();
        String ip = connect_get_ip();
        uint16_t port = connect_get_port();
        state_transition(State::PAIRING);
        if (ws_connect(ip.c_str(), port)) {
            mdns_stop_advertise();
            advertising = false;
            char hello[256];
            String token = connect_get_token();
            String boundID = wifi_get_bound_desktop_id();
            snprintf(hello, sizeof(hello),
                "{\"type\":\"hello\",\"payload\":{\"device_id\":\"%s\",\"name\":\"%s\",\"bound_desktop_id\":\"%s\",\"pairing_token\":\"%s\"}}",
                deviceID, deviceName, boundID.c_str(), token.c_str());
            ws_send_text(hello);
            state_transition(State::IDLE);
        } else {
            state_transition(State::PAIR_READY);
        }
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
