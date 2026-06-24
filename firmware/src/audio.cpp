#include "audio.h"
#include <M5Unified.h>
#include <cstring>

static bool recording = false;

void audio_init() {
    auto cfg = M5.Mic.config();
    cfg.sample_rate = 16000;
    cfg.bit_depth = 16;
    M5.Mic.begin(cfg);

    auto spkCfg = M5.Speaker.config();
    spkCfg.sample_rate = 16000;
    M5.Speaker.begin(spkCfg);
}

void audio_start_recording() { recording = true; }
void audio_stop_recording() { recording = false; }
bool audio_is_recording() { return recording; }

size_t audio_capture(uint8_t* buffer, size_t maxLen) {
    if (!recording || !M5.Mic.isEnabled()) return 0;

    size_t bytesRead = 0;
    int16_t sample;
    while (M5.Mic.available() && bytesRead + sizeof(sample) <= maxLen) {
        sample = M5.Mic.read();
        memcpy(buffer + bytesRead, &sample, sizeof(sample));
        bytesRead += sizeof(sample);
    }
    return bytesRead;
}

void audio_play(const uint8_t* data, size_t len) {
    M5.Speaker.playRaw(data, len, 16000, false);
}
