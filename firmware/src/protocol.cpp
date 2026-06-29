#include "protocol.h"
#include "ws_client.h"
#include "state.h"
#include "elf_wifi.h"
#include "ota.h"
#include <ArduinoJson.h>
#include <cstring>

static size_t otaChunkSize = 0;
static size_t otaTotalSize = 0;
static size_t otaReceived = 0;
static int otaSeq = 0;

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
    JsonDocument doc;
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
            // The desktop no longer authorizes this device. Clear the binding
            // so we stop trying to reconnect and go back to advertising.
            wifi_clear_bound_desktop();
            ws_disconnect();
            return;
        }
        state_transition(State::PAIRING);
    } else if (strcmp(type, "firmware_version_req") == 0) {
        char json[128];
        snprintf(json, sizeof(json), "{\"type\":\"firmware_version\",\"payload\":{\"version\":\"%s\"}}", FIRMWARE_VERSION);
        ws_send_text(json);
    } else if (strcmp(type, "firmware_update") == 0) {
        if (state_current() != State::IDLE) {
            ws_send_text("{\"type\":\"firmware_update_error\",\"payload\":{\"message\":\"device not idle\"}}");
            return;
        }
        const char* version = doc["payload"]["version"] | "";
        size_t size = doc["payload"]["size"] | 0;
        const char* md5 = doc["payload"]["md5"] | "";
        size_t chunkSize = doc["payload"]["chunk_size"] | 4096;
        ESP_LOGI("ota", "update offer v=%s size=%u md5=%s chunk=%u", version, (unsigned)size, md5, (unsigned)chunkSize);
        if (size == 0 || chunkSize == 0) {
            ws_send_text("{\"type\":\"firmware_update_error\",\"payload\":{\"message\":\"invalid update parameters\"}}");
            return;
        }
        otaChunkSize = chunkSize;
        otaTotalSize = size;
        otaReceived = 0;
        otaSeq = 0;
        if (!ota_begin(size, md5, chunkSize)) {
            ws_send_text("{\"type\":\"firmware_update_error\",\"payload\":{\"message\":\"flash init failed\"}}");
            return;
        }
        state_transition(State::UPDATING);
        ws_send_text("{\"type\":\"firmware_update_ack\"}");
    } else if (strcmp(type, "firmware_commit") == 0) {
        if (!ota_in_progress()) {
            ws_send_text("{\"type\":\"firmware_update_error\",\"payload\":{\"message\":\"no update to commit\"}}");
            return;
        }
        ota_commit();
    } else if (strcmp(type, "firmware_update_cancel") == 0) {
        ota_abort();
        state_force_idle();
        ws_send_text("{\"type\":\"firmware_update_cancelled\"}");
    } else if (strcmp(type, "confirm") == 0) {
        const char* requestId = doc["payload"]["request_id"] | "";
        const char* text = doc["payload"]["text"] | "";
        JsonArray opts = doc["payload"]["options"];
        ConfirmOption options[8];
        int count = 0;
        for (JsonObject opt : opts) {
            if (count >= 8) break;
            const char* id = opt["id"] | "";
            const char* label = opt["label"] | "";
            strncpy(options[count].id, id, sizeof(options[count].id) - 1);
            options[count].id[sizeof(options[count].id) - 1] = '\0';
            strncpy(options[count].label, label, sizeof(options[count].label) - 1);
            options[count].label[sizeof(options[count].label) - 1] = '\0';
            count++;
        }
        if (count > 0) {
            state_set_confirm(requestId, text, options, count);
            state_transition(State::CONFIRM);
        }
    } else if (strcmp(type, "confirm_cancel") == 0) {
        state_force_idle();
    } else if (strcmp(type, "tts_start") == 0) {
        // prepare for TTS audio
    } else if (strcmp(type, "tts_end") == 0) {
        state_transition(State::IDLE);
    }
}

void protocol_handle_binary(const uint8_t* data, size_t len) {
    if (ota_in_progress()) {
        if (!ota_write_chunk(data, len)) {
            ota_abort();
            state_force_idle();
            ws_send_text("{\"type\":\"firmware_update_error\",\"payload\":{\"message\":\"chunk write failed\"}}");
            return;
        }
        otaReceived += len;
        int percent = (int)(otaReceived * 100 / otaTotalSize);
        char ack[128];
        snprintf(ack, sizeof(ack), "{\"type\":\"firmware_chunk_ack\",\"payload\":{\"seq\":%d,\"percent\":%d}}", otaSeq, percent);
        ws_send_text(ack);
        otaSeq++;
        state_set_ota_progress((int8_t)percent, "");
        if (otaReceived >= otaTotalSize) {
            ws_send_text("{\"type\":\"firmware_update_complete\"}");
        }
        return;
    }
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
