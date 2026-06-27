#include "ota.h"
#include <Update.h>
#include <esp_log.h>

static bool updating = false;
static size_t updateSize = 0;
static size_t written = 0;

bool ota_in_progress() {
    return updating;
}

void ota_abort() {
    if (updating) {
        Update.abort();
        updating = false;
        written = 0;
        ESP_LOGI("ota", "update aborted");
    }
}

bool ota_begin(size_t size, const char* md5, size_t chunkSize) {
    if (updating) {
        ESP_LOGW("ota", "begin called while update already in progress");
        return false;
    }
    if (!Update.begin(size, U_FLASH)) {
        ESP_LOGE("ota", "Update.begin failed: %s", Update.errorString());
        return false;
    }
    if (md5 && md5[0]) {
        Update.setMD5(md5);
    }
    updating = true;
    updateSize = size;
    written = 0;
    ESP_LOGI("ota", "update started, size=%u chunk=%u", (unsigned)size, (unsigned)chunkSize);
    return true;
}

bool ota_write_chunk(const uint8_t* data, size_t len) {
    if (!updating) {
        ESP_LOGW("ota", "write chunk while not updating");
        return false;
    }
    size_t n = Update.write(const_cast<uint8_t*>(data), len);
    if (n != len) {
        ESP_LOGE("ota", "write failed: %u/%u bytes, %s", (unsigned)n, (unsigned)len, Update.errorString());
        return false;
    }
    written += n;
    return true;
}

bool ota_commit() {
    if (!updating) {
        ESP_LOGW("ota", "commit while not updating");
        return false;
    }
    if (!Update.end(true)) {
        ESP_LOGE("ota", "Update.end failed: %s", Update.errorString());
        return false;
    }
    ESP_LOGI("ota", "update committed, restarting");
    updating = false;
    written = 0;
    ESP.restart();
    return true;
}
