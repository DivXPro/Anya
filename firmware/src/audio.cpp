#include "audio.h"
#include <M5Unified.h>
#include <driver/i2s.h>
#include <soc/i2s_struct.h>
#include <cmath>
#include <cstring>
#include <esp_log.h>

static bool recording = false;
static bool micStarted = false;
static bool i2sStarted = false;
static constexpr const char* TAG = "audio";
static constexpr uint8_t ES8311_ADDR = 0x18;
static constexpr uint8_t M5PM1_ADDR = 0x6E;
static constexpr uint32_t I2C_FREQ = 100000;
static constexpr int AUDIO_SAMPLE_RATE = 16000;
static constexpr size_t I2S_WRITE_FRAMES = 512;

static void play_tone(float start_freq, float end_freq, int duration_ms);

static void calc_clock_div(uint32_t* div_a, uint32_t* div_b, uint32_t* div_n, uint32_t base_clock, uint32_t target_freq) {
    if (base_clock <= target_freq << 1) {
        *div_n = 2;
        *div_a = 1;
        *div_b = 0;
        return;
    }

    uint32_t save_n = 255;
    uint32_t save_a = 63;
    uint32_t save_b = 62;
    if (target_freq) {
        float fdiv = static_cast<float>(base_clock) / target_freq;
        uint32_t n = static_cast<uint32_t>(fdiv);
        if (n < 256) {
            fdiv -= n;
            float check_base = base_clock;
            while (static_cast<int32_t>(target_freq) >= 0) {
                target_freq <<= 1;
                check_base *= 2;
            }
            float check_target = target_freq;

            uint32_t save_diff = UINT32_MAX;
            if (n < 255) {
                save_a = 1;
                save_b = 0;
                save_n = n + 1;
                save_diff = static_cast<uint32_t>(fabsf(check_target - check_base / static_cast<float>(save_n)));
            }

            for (uint32_t a = 1; a < 64; ++a) {
                uint32_t b = static_cast<uint32_t>(roundf(a * fdiv));
                if (a <= b) continue;
                uint32_t diff = static_cast<uint32_t>(fabsf(check_target - ((check_base * a) / (n * a + b))));
                if (save_diff <= diff) continue;
                save_diff = diff;
                save_a = a;
                save_b = b;
                save_n = n;
                if (!diff) break;
            }
        }
    }

    *div_n = save_n;
    *div_a = save_a;
    *div_b = save_b;
}

static uint32_t apply_i2s_clock(uint32_t sample_rate) {
    static constexpr uint32_t PLL_D2_CLK = 120 * 1000 * 1000;
    static constexpr uint32_t BITS_PER_SAMPLE = 16;
    static constexpr uint32_t BCK_DIV_WITH_MCLK = 8;

    uint32_t div_a = 0;
    uint32_t div_b = 0;
    uint32_t div_n = 0;
    calc_clock_div(&div_a, &div_b, &div_n, PLL_D2_CLK, BCK_DIV_WITH_MCLK * BITS_PER_SAMPLE * sample_rate);

    I2S0.tx_conf1.tx_bck_div_num = BCK_DIV_WITH_MCLK - 1;

    float denom = (static_cast<float>(div_b * BCK_DIV_WITH_MCLK * BITS_PER_SAMPLE) / div_a)
                + (div_n * BCK_DIV_WITH_MCLK * BITS_PER_SAMPLE);
    uint32_t actual_rate = static_cast<uint32_t>((PLL_D2_CLK / denom) + 0.5f);

    bool yn1 = div_b > (div_a >> 1);
    if (yn1) {
        div_b = div_a - div_b;
    }

    uint32_t div_x = 0;
    uint32_t div_y = 1;
    if (div_b) {
        div_x = div_a / div_b - 1;
        div_y = div_a % div_b;
        if (div_y == 0) {
            div_y = 1;
            div_b = 511;
        }
    }

    I2S0.tx_clkm_div_conf.tx_clkm_div_x = div_x;
    I2S0.tx_clkm_div_conf.tx_clkm_div_y = div_y;
    I2S0.tx_clkm_div_conf.tx_clkm_div_z = div_b;
    I2S0.tx_clkm_div_conf.tx_clkm_div_yn1 = yn1;
    I2S0.tx_clkm_conf.tx_clkm_div_num = div_n;
    I2S0.tx_clkm_conf.tx_clk_sel = 1;
    I2S0.tx_clkm_conf.clk_en = 1;
    I2S0.tx_clkm_conf.tx_clk_active = 1;
    I2S0.tx_conf.tx_update = 1;
    I2S0.tx_conf.tx_update = 0;

    return actual_rate;
}

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

static void stop_i2s() {
    if (!i2sStarted) return;
    i2s_stop(I2S_NUM_0);
    i2s_driver_uninstall(I2S_NUM_0);
    i2sStarted = false;
}

static bool start_i2s() {
    if (i2sStarted) return true;

    i2s_config_t i2sCfg = {};
    i2sCfg.mode = static_cast<i2s_mode_t>(I2S_MODE_MASTER | I2S_MODE_TX);
    i2sCfg.sample_rate = AUDIO_SAMPLE_RATE;
    i2sCfg.bits_per_sample = I2S_BITS_PER_SAMPLE_16BIT;
    i2sCfg.channel_format = I2S_CHANNEL_FMT_RIGHT_LEFT;
    i2sCfg.communication_format = I2S_COMM_FORMAT_I2S;
    i2sCfg.intr_alloc_flags = ESP_INTR_FLAG_LEVEL1;
    i2sCfg.dma_buf_count = 8;
    i2sCfg.dma_buf_len = 256;
    // use_apll=false / fixed_mclk=0 is the config that produces sound on this
    // board. (An attempt to add a fixed 256*LRCK MCLK via use_apll=true was not
    // verified and risks no-audio if the S3 legacy driver rejects APLL, so it
    // is intentionally left off here.)
    i2sCfg.use_apll = false;
    i2sCfg.tx_desc_auto_clear = true;
    i2sCfg.fixed_mclk = 0;

    esp_err_t err = i2s_driver_install(I2S_NUM_0, &i2sCfg, 0, nullptr);
    if (err != ESP_OK) {
        ESP_LOGW(TAG, "i2s driver_install failed: %d", err);
        return false;
    }

    i2s_pin_config_t pinCfg = {};
    pinCfg.mck_io_num = GPIO_NUM_18;
    pinCfg.bck_io_num = GPIO_NUM_17;
    pinCfg.ws_io_num = GPIO_NUM_15;
    pinCfg.data_out_num = GPIO_NUM_14;
    pinCfg.data_in_num = I2S_PIN_NO_CHANGE;
    err = i2s_set_pin(I2S_NUM_0, &pinCfg);
    if (err != ESP_OK) {
        ESP_LOGW(TAG, "i2s set_pin failed: %d", err);
        stop_i2s();
        return false;
    }

    uint32_t actualRate = apply_i2s_clock(AUDIO_SAMPLE_RATE);
    ESP_LOGI(TAG, "i2s clock target=%d actual=%u mclk=%u",
             AUDIO_SAMPLE_RATE, (unsigned)actualRate, (unsigned)(AUDIO_SAMPLE_RATE * 256));

    i2sStarted = true;
    i2s_start(I2S_NUM_0);
    return true;
}

static bool start_speaker() {
    if (micStarted) {
        while (M5.Mic.isRecording()) { delay(1); }
        M5.Mic.end();
        micStarted = false;
    }
    configure_amp();
    configure_es8311();
    return start_i2s();
}

static void stop_speaker() {
    stop_i2s();
}

static bool start_mic() {
    stop_speaker();
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
    // Only do the full start_speaker() (which RESETS and reconfigures the
    // ES8311 codec + PMU amp over I2C) when the speaker isn't already running.
    // Calling it every chunk reset the the codec every ~128 ms mid-playback,
    // causing distortion and I2C-vs-DMA crashes. Callers (audio_begin_playback,
    // play_tone) already do the full setup first, so here we just keep I2S alive.
    if (!i2sStarted) {
        if (!start_speaker()) return;
    }

    size_t samples = len / sizeof(int16_t);
    // Static so the 2 KB stereo frame buffer lives in BSS, not on the stack of
    // the (deep) WebSocket binary-callback chain. audio_play runs only on the
    // main loopTask and is not reentrant, so a single shared buffer is safe.
    static int16_t frames[I2S_WRITE_FRAMES * 2];

    for (size_t offset = 0; offset < samples;) {
        size_t count = samples - offset;
        if (count > I2S_WRITE_FRAMES) count = I2S_WRITE_FRAMES;
        for (size_t i = 0; i < count; ++i) {
            int16_t sample = read_sample_le(data + ((offset + i) * sizeof(int16_t)));
            // Attenuate -6 dB so full-scale TTS peaks don't over-drive the
            // amp (clipping distortion + brownout resets).
            sample = static_cast<int16_t>(sample / 2);
            frames[i * 2] = sample;
            frames[i * 2 + 1] = sample;
        }
        size_t written = 0;
        esp_err_t err = i2s_write(I2S_NUM_0, frames, count * 2 * sizeof(int16_t), &written, pdMS_TO_TICKS(1000));
        if (err != ESP_OK) {
            ESP_LOGW(TAG, "i2s write failed: %d", err);
            break;
        }
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
    return i2sStarted;
}
