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
#include "ota.h"
#include "lang.h"

#ifndef FIRMWARE_VERSION
#define FIRMWARE_VERSION "0.0.0-dev"
#endif

static char deviceID[37];
static char deviceName[32];
static bool advertising = false;
static bool httpStarted = false;

static bool inMenu = false;
enum class MenuLevel : uint8_t { MAIN, LANGUAGE };
static MenuLevel menuLevel = MenuLevel::MAIN;
static int menuSelected = 0;
static State menuReturnState = State::IDLE;

static const Str mainMenuItems[] = {
    Str::MenuChooseWifi,
    Str::MenuRepair,
    Str::MenuTestSpeaker,
    Str::MenuLanguage,
    Str::MenuBack,
};
static const int mainMenuCount = sizeof(mainMenuItems) / sizeof(mainMenuItems[0]);

static const Str langMenuItems[] = {
    Str::LangEnglish,
    Str::LangChinese,
    Str::MenuBack,
};
static const int langMenuCount = sizeof(langMenuItems) / sizeof(langMenuItems[0]);

static void show_menu() {
    if (menuLevel == MenuLevel::LANGUAGE) {
        disp_menu(deviceName, menuSelected, langMenuItems, langMenuCount,
                  wifi_rssi(), wifi_connected(), ws_connected());
    } else {
        disp_menu(deviceName, menuSelected, mainMenuItems, mainMenuCount,
                  wifi_rssi(), wifi_connected(), ws_connected());
    }
}

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
        snprintf(deviceName, sizeof(deviceName), "anya-%02x%02x", mac[4], mac[5]);
        prefs.putString("device_id", deviceID);
        prefs.putString("device_name", deviceName);
    } else {
        strncpy(deviceID, storedID.c_str(), sizeof(deviceID) - 1);
        deviceID[sizeof(deviceID) - 1] = '\0';
        if (storedName.length() == 0) storedName = "anya-device";
        strncpy(deviceName, storedName.c_str(), sizeof(deviceName) - 1);
        deviceName[sizeof(deviceName) - 1] = '\0';
    }
    prefs.end();
}

static void ensure_http_server() {
    if (!httpStarted) {
        http_setup_connect_endpoint();
        httpStarted = true;
    }
}

static void resume_after_portal(PortalResult pr) {
    if (pr == PortalResult::CANCELLED) {
        return;
    }
    if (pr == PortalResult::FAILED) {
        disp_error(tr(Str::WifiSetupFailed), deviceName);
        delay(2000);
        inMenu = true;
        menuLevel = MenuLevel::MAIN;
        menuSelected = 0;
        state_transition(State::MENU);
        show_menu();
        return;
    }

    ensure_http_server();

    String boundID = wifi_get_bound_desktop_id();
    String boundIP = wifi_get_bound_desktop_ip();
    uint16_t boundPort = wifi_get_bound_desktop_port();
    if (boundIP.length() > 0) {
        state_transition(State::IDLE);
        ws_set_hello_data(deviceID, deviceName, boundID.c_str(), "");
        ws_connect(boundIP.c_str(), boundPort);
    } else {
        state_transition(State::PAIR_READY);
        if (!advertising) {
            mdns_start_advertise(deviceID, deviceName);
            advertising = true;
        }
    }
}

static void register_button_callbacks() {
    btn_on_ptt_press([]() {
        if (ota_in_progress()) {
            ESP_LOGI("main", "PTT press ignored: OTA in progress");
            return;
        }
        if (inMenu) {
            if (menuLevel == MenuLevel::LANGUAGE) {
                if (menuSelected == 0) {
                    lang_set(Lang::EN);
                    ESP_LOGI("main", "language set to EN");
                } else if (menuSelected == 1) {
                    lang_set(Lang::ZH);
                    ESP_LOGI("main", "language set to ZH");
                }
                // Back (index 2) simply returns to the main menu.
                menuLevel = MenuLevel::MAIN;
                menuSelected = 3; // Language
                show_menu();
                return;
            }
            ESP_LOGI("main", "menu confirm: %s", tr(mainMenuItems[menuSelected]));
            if (menuSelected == 0) {
                // Choose WiFi: open captive portal to reconfigure network
                inMenu = false;
                state_transition(State::WIFI_SETUP);
                resume_after_portal(wifi_portal_begin());
            } else if (menuSelected == 1) {
                // Repair: clear binding. If WiFi is available, start advertising;
                // otherwise open the captive portal so the user can configure it first.
                inMenu = false;
                wifi_clear_bound_desktop();
                ws_disconnect();
                if (wifi_connected()) {
                    state_transition(State::PAIR_READY);
                    if (!advertising) {
                        mdns_start_advertise(deviceID, deviceName);
                        advertising = true;
                    }
                } else {
                    if (advertising) {
                        mdns_stop_advertise();
                        advertising = false;
                    }
                    state_transition(State::WIFI_SETUP);
                    resume_after_portal(wifi_portal_begin());
                }
            } else if (menuSelected == 2) {
                // Test Speaker: play a 1kHz tone for 1 second
                ESP_LOGI("main", "menu: test speaker");
                audio_play_test_tone();
                disp_playing(tr(Str::PlayingTestTone), deviceName);
                delay(1200);
                show_menu();
            } else if (menuSelected == 3) {
                // Language: enter the second-level language menu
                menuLevel = MenuLevel::LANGUAGE;
                menuSelected = (lang_get() == Lang::EN) ? 0 : 1;
                show_menu();
            } else if (menuSelected == 4) {
                // Back: return to the screen we were on before opening the menu.
                inMenu = false;
                State target = menuReturnState;
                state_transition(target);
                if (target == State::WIFI_SETUP) {
                    // Resume the captive portal so the user can keep configuring WiFi.
                    resume_after_portal(wifi_portal_begin());
                }
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
        if (ota_in_progress()) return;
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
        if (ota_in_progress()) {
            ESP_LOGI("main", "menu navigation ignored: OTA in progress");
            return;
        }
        if (!inMenu) {
            ESP_LOGI("main", "enter menu");
            menuReturnState = state_current();
            inMenu = true;
            menuLevel = MenuLevel::MAIN;
            menuSelected = 0;
            state_transition(State::MENU);
            show_menu();
        } else {
            int count = (menuLevel == MenuLevel::LANGUAGE) ? langMenuCount : mainMenuCount;
            menuSelected = (menuSelected + 1) % count;
            ESP_LOGI("main", "menu next -> %d", menuSelected);
            show_menu();
        }
    });
}

void setup() {
    auto cfg = M5.config();
    M5.begin(cfg);
    M5.Display.setFont(&fonts::efontCN_12);
    lang_init();
    ESP_LOGI("elf", "firmware setup start, version=%s", FIRMWARE_VERSION);
    init_device_identity();

    state_init();
    audio_init();
    btn_init();
    ws_init();
    protocol_init();
    register_button_callbacks();

    bool wifiOK = wifi_init();
    PortalResult portalResult = PortalResult::SUCCESS;
    if (!wifiOK) {
        state_transition(State::WIFI_SETUP);
        portalResult = wifi_portal_begin();
    }
    if (portalResult == PortalResult::FAILED) {
        disp_error(tr(Str::WifiSetupFailed), deviceName);
        return;
    }
    if (portalResult == PortalResult::CANCELLED) {
        inMenu = true;
        menuLevel = MenuLevel::MAIN;
        menuSelected = 0;
        menuReturnState = State::WIFI_SETUP;
        state_transition(State::MENU);
        show_menu();
    } else {
        ensure_http_server();

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
    }
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

    // Animate mascot on screens that show it, and scroll agent reply text.
    disp_animate_mascot();
    disp_animate_text();

    // Return to idle if the agent reply has been on screen for 5 minutes.
    if (!ota_in_progress() && disp_text_showing_for(5UL * 60UL * 1000UL)) {
        state_force_idle();
    }

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
    // When disconnected, keep advertising so a desktop can find us, but show the
    // disconnected idle screen (status bar "No agent", bottom "Disconnected").
    // The "Ready to pair" screen is only shown on first boot or when the user
    // explicitly chooses Repair from the menu; we never switch back to it
    // automatically after a disconnect.
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
            if (state_current() != State::IDLE && state_current() != State::PAIR_READY) {
                state_transition(State::IDLE);
            }
        }
    }

    if (!ota_in_progress() && connect_is_requested()) {
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

    if (!ota_in_progress() && audio_is_recording()) {
        static uint8_t buf[1024];
        size_t len = audio_capture(buf, sizeof(buf));
        if (len > 0) {
            protocol_send_audio_chunk(buf, len);
        }
    }

    delay(10);
}
