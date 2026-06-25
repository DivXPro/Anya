#include "protocol.h"
#include "ws_client.h"
#include "state.h"
#include "elf_wifi.h"
#include <ArduinoJson.h>
#include <cstring>

void protocol_init() {
    ws_on_text([](const char* msg) {
        protocol_handle_message(msg);
    });
    ws_on_binary([](const uint8_t* data, size_t len) {
        protocol_handle_binary(data, len);
    });
    ws_on_close([]() {
        state_force_idle();
    });
}

void protocol_handle_message(const char* json) {
    StaticJsonDocument<512> doc;
    DeserializationError err = deserializeJson(doc, json);
    if (err) return;

    const char* type = doc["type"];
    if (!type) return;

    if (strcmp(type, "summary") == 0) {
        const char* text = doc["text"] | "";
        state_set_summary(text);
        state_transition(State::PLAYING);
    } else if (strcmp(type, "status") == 0) {
        const char* s = doc["state"] | "";
        if (strcmp(s, "listening") == 0) state_transition(State::LISTENING);
        else if (strcmp(s, "processing") == 0) state_transition(State::PROCESSING);
        else if (strcmp(s, "connected") == 0) state_transition(State::IDLE);
    } else if (strcmp(type, "session") == 0) {
        const char* agent = doc["payload"]["agent_id"] | "claude";
        const char* desktopID = doc["payload"]["desktop_id"] | "";
        state_set_agent(agent);
        if (strlen(desktopID) > 0) {
            wifi_save_bound_desktop(desktopID,
                wifi_get_desktop_ip().c_str(),
                wifi_get_desktop_port());
        }
        state_transition(State::IDLE);
    } else if (strcmp(type, "pairing_required") == 0) {
        String boundID = wifi_get_bound_desktop_id();
        if (boundID.length() > 0) {
            ws_disconnect();
            return;
        }
        state_transition(State::PAIRING);
    } else if (strcmp(type, "tts_start") == 0) {
        // prepare for TTS audio
    } else if (strcmp(type, "tts_end") == 0) {
        state_transition(State::IDLE);
    }
}

void protocol_handle_binary(const uint8_t* data, size_t len) {
    state_play_audio(data, len);
}

void protocol_send_audio_start() {
    ws_send_text("{\"type\":\"audio_start\",\"format\":\"pcm\",\"sample_rate\":16000}");
}

void protocol_send_audio_chunk(const uint8_t* data, size_t len) {
    ws_send_binary(data, len);
}

void protocol_send_audio_end() {
    ws_send_text("{\"type\":\"audio_end\"}");
}

void protocol_send_button(const char* action) {
    char json[64];
    snprintf(json, sizeof(json), "{\"type\":\"button\",\"action\":\"%s\"}", action);
    ws_send_text(json);
}
