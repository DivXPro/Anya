#include "audio.h"
#include <M5Unified.h>
#include <cmath>
#include <cstring>
#include <esp_log.h>

static bool recording = false;
static bool micStarted = false;
static bool speakerStarted = false;
static constexpr const char* TAG = "audio";
static constexpr uint8_t ES8311_ADDR = 0x18;
static constexpr uint8_t M5PM1_ADDR = 0x6E;
static constexpr uint32_t I2C_FREQ = 100000;
static constexpr int AUDIO_SAMPLE_RATE = 16000;
static constexpr size_t SPEAKER_CHUNK_SAMPLES = 1024;
static constexpr size_t SPEAKER_BUFFER_COUNT = 3;
static int16_t speakerBuffers[SPEAKER_BUFFER_COUNT][SPEAKER_CHUNK_SAMPLES];
static size_t speakerBufferIndex = 0;

static void play_tone(float start_freq, float end_freq, int duration_ms);

static void write_reg(uint8_t addr, uint8_t reg, uint8_t value) {
    M5.In_I2C.writeRegister8(addr, reg, value, I2C_FREQ);
}

static int16_t read_sample_le(const uint8_t* data) {
    uint16_t value = static_cast<uint16_t>(data[0]) | (static_cast<uint16_t>(data[1]) << 8);
    return static_cast<int16_t>(value);
}

static void configure_amp() {
    M5.In_I2C.bitOn(M5PM1_ADDR, 0x06, 0x08, I2C_FREQ);
    M5.In_I2C.bitOff(M5PM1_ADDR, 0x16, 1 << 3, I2C_FREQ);
    M5.In_I2C.bitOn(M5PM1_ADDR, 0x10, 1 << 3, I2C_FREQ);
    M5.In_I2C.bitOff(M5PM1_ADDR, 0x13, 1 << 3, I2C_FREQ);
    M5.In_I2C.bitOn(M5PM1_ADDR, 0x11, 1 << 3, I2C_FREQ);
}

static void configure_es8311() {
    write_reg(ES8311_ADDR, 0x00, 0x80);
    delay(10);
    write_reg(ES8311_ADDR, 0x01, 0xB5);
    write_reg(ES8311_ADDR, 0x02, 0x18);
    write_reg(ES8311_ADDR, 0x0D, 0x01);
    write_reg(ES8311_ADDR, 0x12, 0x00);
    write_reg(ES8311_ADDR, 0x13, 0x10);
    write_reg(ES8311_ADDR, 0x32, 0xBF);  // DAC volume ±0 dB (was 0xFF = max, over-driving -> clipping/brownout)
    write_reg(ES8311_ADDR, 0x37, 0x08);
}

static bool start_speaker() {
    if (speakerStarted && M5.Speaker.isRunning()) return true;

    if (micStarted) {
        while (M5.Mic.isRecording()) { delay(1); }
        M5.Mic.end();
        micStarted = false;
    }

    M5.Speaker.end();
    auto spkCfg = M5.Speaker.config();
    spkCfg.sample_rate = 44100;
    spkCfg.stereo = true;
    M5.Speaker.config(spkCfg);
    M5.Speaker.setVolume(255);
    M5.Speaker.setAllChannelVolume(255);
    M5.Speaker.setChannelVolume(0, 255);

    configure_amp();
    bool ok = M5.Speaker.begin();
    configure_amp();
    configure_es8311();

    speakerStarted = ok && M5.Speaker.isRunning();
    ESP_LOGI(TAG, "speaker begin=%d running=%d rate=%u vol=%u",
             ok,
             M5.Speaker.isRunning(),
             (unsigned)M5.Speaker.config().sample_rate,
             M5.Speaker.getVolume());
    return speakerStarted;
}

static void stop_speaker(bool stopCurrent = false) {
    if (!speakerStarted && !M5.Speaker.isRunning()) return;
    if (stopCurrent) {
        M5.Speaker.stop(0);
    }
    while (M5.Speaker.isPlaying(0)) {
        delay(1);
        M5.update();
    }
    M5.Speaker.end();
    speakerStarted = false;
}

static bool start_mic() {
    stop_speaker(true);
    if (!micStarted) {
        micStarted = M5.Mic.begin();
        if (!micStarted) {
            auto cfg = M5.Mic.config();
            ESP_LOGW(TAG, "mic begin failed: board=%d data=%d ws=%d bck=%d",
                     (int)M5.getBoard(), cfg.pin_data_in, cfg.pin_ws, cfg.pin_bck);
        }
    }
    return micStarted;
}

void audio_init() {
    auto cfg = M5.Mic.config();
    cfg.sample_rate = AUDIO_SAMPLE_RATE;
    M5.Mic.config(cfg);
    start_mic();
}

void audio_start_recording() {
    if (start_mic()) {
        recording = true;
    }
}
void audio_stop_recording() { recording = false; }
bool audio_is_recording() { return recording; }

size_t audio_capture(uint8_t* buffer, size_t maxLen) {
    if (!recording || !M5.Mic.isEnabled()) return 0;
    size_t samples = maxLen / sizeof(int16_t);
    if (M5.Mic.record(reinterpret_cast<int16_t*>(buffer), samples)) {
        return samples * sizeof(int16_t);
    }
    return 0;
}

void audio_play(const uint8_t* data, size_t len) {
    if (!data || len < sizeof(int16_t)) return;
    if (!start_speaker()) return;

    size_t samples = len / sizeof(int16_t);
    for (size_t offset = 0; offset < samples;) {
        size_t count = samples - offset;
        if (count > SPEAKER_CHUNK_SAMPLES) count = SPEAKER_CHUNK_SAMPLES;

        int16_t* out = speakerBuffers[speakerBufferIndex];
        for (size_t i = 0; i < count; ++i) {
            int16_t sample = read_sample_le(data + ((offset + i) * sizeof(int16_t)));
            // Attenuate -6 dB so full-scale TTS peaks don't over-drive the
            // amp (clipping distortion + brownout resets).
            out[i] = static_cast<int16_t>(sample / 2);
        }

        while (M5.Speaker.isPlaying(0) >= 2) {
            delay(1);
            M5.update();
        }

        bool ok = M5.Speaker.playRaw(out, count, AUDIO_SAMPLE_RATE, false, 1, 0, false);
        if (!ok) {
            ESP_LOGW(TAG, "speaker playRaw failed queued=%u", (unsigned)M5.Speaker.isPlaying(0));
            break;
        }

        speakerBufferIndex = (speakerBufferIndex + 1) % SPEAKER_BUFFER_COUNT;
        offset += count;
        M5.update();
    }
}

void audio_play_test_tone() {
    // Generate a 1 kHz beep instead of embedding a sample.
    play_tone(1000.0f, 1000.0f, 1000);
    audio_finish_playback();
}

#include <cmath>

static const float PI_F = 3.14159265f;

static void generate_swept_tone(int16_t* buf, size_t samples, float start_freq, float end_freq) {
    for (size_t i = 0; i < samples; ++i) {
        float progress = i / static_cast<float>(samples);
        float freq = start_freq + (end_freq - start_freq) * progress;
        float phase = 2.0f * PI_F * freq * (i / static_cast<float>(AUDIO_SAMPLE_RATE));
        float envelope = sinf(progress * PI_F);
        buf[i] = static_cast<int16_t>(9000.0f * envelope * sinf(phase));
    }
}

static void play_tone(float start_freq, float end_freq, int duration_ms) {
    if (!start_speaker()) return;
    size_t samples = AUDIO_SAMPLE_RATE * duration_ms / 1000;
    int16_t* buf = new int16_t[samples];
    generate_swept_tone(buf, samples, start_freq, end_freq);
    audio_play(reinterpret_cast<const uint8_t*>(buf), samples * sizeof(int16_t));
    while (M5.Speaker.isPlaying(0)) {
        delay(1);
        M5.update();
    }
    delete[] buf;
}

void audio_play_start_tone() {
    play_tone(800.0f, 1200.0f, 150);
}

void audio_play_end_tone() {
    play_tone(600.0f, 400.0f, 100);
    audio_finish_playback();
}

void audio_begin_playback() {
    start_speaker();
}

void audio_finish_playback() {
    stop_speaker();
    if (!recording) {
        start_mic();
    }
}

bool audio_is_playing() {
    return speakerStarted || M5.Speaker.isPlaying(0);
}
