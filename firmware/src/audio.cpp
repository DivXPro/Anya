#include "audio.h"
#include <M5Unified.h>
#include <cmath>
#include <cstring>

static bool recording = false;

void audio_init() {
    auto cfg = M5.Mic.config();
    cfg.sample_rate = 16000;
    M5.Mic.config(cfg);
    M5.Mic.begin();

    auto spkCfg = M5.Speaker.config();
    spkCfg.sample_rate = 16000;
    M5.Speaker.config(spkCfg);
    M5.Speaker.begin();
}

void audio_start_recording() { recording = true; }
void audio_stop_recording() { recording = false; }
bool audio_is_recording() { return recording; }

size_t audio_capture(uint8_t* buffer, size_t maxLen) {
    if (!recording || !M5.Mic.isEnabled()) return 0;

    // Use M5.Mic.record() with int16_t buffer
    size_t samples = maxLen / sizeof(int16_t);
    if (M5.Mic.record(reinterpret_cast<int16_t*>(buffer), samples)) {
        return samples * sizeof(int16_t);
    }
    return 0;
}

void audio_play(const uint8_t* data, size_t len) {
    // playRaw(const uint8_t*, size_t, sample_rate, stereo)
    M5.Speaker.playRaw(data, len, 16000, false);
}

void audio_play_test_tone() {
    // Generate a 1 kHz beep instead of embedding a sample.
    M5.Speaker.tone(1000, 1000);
}
