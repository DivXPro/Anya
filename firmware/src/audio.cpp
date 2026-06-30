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

#include <cmath>

static const float PI_F = 3.14159265f;
static const int TONE_SAMPLE_RATE = 16000;

static void generate_swept_tone(int16_t* buf, size_t samples, float start_freq, float end_freq) {
    for (size_t i = 0; i < samples; ++i) {
        float progress = i / static_cast<float>(samples);
        float freq = start_freq + (end_freq - start_freq) * progress;
        float phase = 2.0f * PI_F * freq * (i / static_cast<float>(TONE_SAMPLE_RATE));
        float envelope = sinf(progress * PI_F);  // 0 → 1 → 0，避免爆音
        buf[i] = static_cast<int16_t>(1800.0f * envelope * sinf(phase));
    }
}

static void play_tone(float start_freq, float end_freq, int duration_ms) {
    size_t samples = TONE_SAMPLE_RATE * duration_ms / 1000;
    int16_t* buf = new int16_t[samples];
    generate_swept_tone(buf, samples, start_freq, end_freq);
    M5.Speaker.playRaw(reinterpret_cast<uint8_t*>(buf), samples * sizeof(int16_t), TONE_SAMPLE_RATE, false);
    delete[] buf;
}

void audio_play_start_tone() {
    play_tone(800.0f, 1200.0f, 150);
}

void audio_play_end_tone() {
    play_tone(600.0f, 400.0f, 100);
}
