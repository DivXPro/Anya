#include <M5Unified.h>

#include "../generated/space_animal_pcm.h"

static constexpr uint8_t ES8311_ADDR = 0x18;
static constexpr uint8_t M5PM1_ADDR = 0x6E;
static constexpr uint32_t I2C_FREQ = 100000;
static constexpr uint32_t WAV_RATE = 16000;
static constexpr uint32_t WAV_SECONDS = 1;
static constexpr uint32_t WAV_SAMPLES = WAV_RATE * WAV_SECONDS;
static constexpr uint32_t WAV_BYTES = WAV_SAMPLES * sizeof(int16_t);
static constexpr size_t PCM_OFFSET_SAMPLES = 8000;
static constexpr size_t PCM_TEST_SAMPLES = 32000;
static uint8_t wavTest[44 + WAV_BYTES];

static void put_le16(uint8_t* dst, uint16_t value) {
    dst[0] = value & 0xFF;
    dst[1] = value >> 8;
}

static void put_le32(uint8_t* dst, uint32_t value) {
    dst[0] = value & 0xFF;
    dst[1] = (value >> 8) & 0xFF;
    dst[2] = (value >> 16) & 0xFF;
    dst[3] = (value >> 24) & 0xFF;
}

static void build_wav_test() {
    memcpy(&wavTest[0], "RIFF", 4);
    put_le32(&wavTest[4], sizeof(wavTest) - 8);
    memcpy(&wavTest[8], "WAVE", 4);
    memcpy(&wavTest[12], "fmt ", 4);
    put_le32(&wavTest[16], 16);
    put_le16(&wavTest[20], 1);
    put_le16(&wavTest[22], 1);
    put_le32(&wavTest[24], WAV_RATE);
    put_le32(&wavTest[28], WAV_RATE * sizeof(int16_t));
    put_le16(&wavTest[32], sizeof(int16_t));
    put_le16(&wavTest[34], 16);
    memcpy(&wavTest[36], "data", 4);
    put_le32(&wavTest[40], WAV_BYTES);

    int16_t* pcm = reinterpret_cast<int16_t*>(&wavTest[44]);
    for (uint32_t i = 0; i < WAV_SAMPLES; ++i) {
        uint32_t phase = (i * 880 * 2) / WAV_RATE;
        pcm[i] = (phase & 1) ? 18000 : -18000;
    }
}

static uint8_t read_reg(uint8_t addr, uint8_t reg) {
    return M5.In_I2C.readRegister8(addr, reg, I2C_FREQ);
}

static void write_reg(uint8_t addr, uint8_t reg, uint8_t value) {
    M5.In_I2C.writeRegister8(addr, reg, value, I2C_FREQ);
}

static void configure_spk_pulse_pin() {
    M5.In_I2C.bitOff(M5PM1_ADDR, 0x16, 1 << 3, I2C_FREQ);
    M5.In_I2C.bitOn(M5PM1_ADDR, 0x10, 1 << 3, I2C_FREQ);
    M5.In_I2C.bitOff(M5PM1_ADDR, 0x13, 1 << 3, I2C_FREQ);
}

static void set_spk_pulse(bool high) {
    if (high) {
        M5.In_I2C.bitOn(M5PM1_ADDR, 0x11, 1 << 3, I2C_FREQ);
    } else {
        M5.In_I2C.bitOff(M5PM1_ADDR, 0x11, 1 << 3, I2C_FREQ);
    }
}

static void apply_amp_mode(uint8_t pulseCount) {
    configure_spk_pulse_pin();
    if (pulseCount == 0) {
        set_spk_pulse(true);
        M5.delay(10);
        Serial.println("[amp] mode=hold-high");
        return;
    }

    set_spk_pulse(false);
    M5.delay(10);
    for (uint8_t i = 0; i < pulseCount; ++i) {
        set_spk_pulse(true);
        M5.delay(2);
        set_spk_pulse(false);
        M5.delay(2);
    }
    set_spk_pulse(true);
    M5.delay(10);
    Serial.printf("[amp] mode=pulses-%u\n", pulseCount);
}

static void force_audio_regs(uint8_t dacVolume) {
    configure_spk_pulse_pin();
    M5.In_I2C.bitOn(M5PM1_ADDR, 0x11, 1 << 3, I2C_FREQ);

    write_reg(ES8311_ADDR, 0x00, 0x80);
    write_reg(ES8311_ADDR, 0x01, 0xB5);
    write_reg(ES8311_ADDR, 0x02, 0x18);
    write_reg(ES8311_ADDR, 0x0D, 0x01);
    write_reg(ES8311_ADDR, 0x12, 0x00);
    write_reg(ES8311_ADDR, 0x13, 0x10);
    write_reg(ES8311_ADDR, 0x32, dacVolume);
    write_reg(ES8311_ADDR, 0x37, 0x08);
}

static void print_audio_regs(const char* phase) {
    Serial.printf("[%s] pwr 06=%02X 10=%02X 11=%02X 13=%02X 16=%02X\n",
                  phase,
                  read_reg(M5PM1_ADDR, 0x06),
                  read_reg(M5PM1_ADDR, 0x10),
                  read_reg(M5PM1_ADDR, 0x11),
                  read_reg(M5PM1_ADDR, 0x13),
                  read_reg(M5PM1_ADDR, 0x16));
    Serial.printf("[%s] es8311 00=%02X 01=%02X 02=%02X 0D=%02X 12=%02X 13=%02X 32=%02X 37=%02X\n",
                  phase,
                  read_reg(ES8311_ADDR, 0x00),
                  read_reg(ES8311_ADDR, 0x01),
                  read_reg(ES8311_ADDR, 0x02),
                  read_reg(ES8311_ADDR, 0x0D),
                  read_reg(ES8311_ADDR, 0x12),
                  read_reg(ES8311_ADDR, 0x13),
                  read_reg(ES8311_ADDR, 0x32),
                  read_reg(ES8311_ADDR, 0x37));
}

static void print_audio_config(const char* phase, bool beginResult) {
    auto spk = M5.Speaker.config();
    auto mic = M5.Mic.config();

    Serial.printf("[%s] board=%d speaker_begin=%d enabled=%d running=%d\n",
                  phase, (int)M5.getBoard(), beginResult,
                  M5.Speaker.isEnabled(), M5.Speaker.isRunning());
    Serial.printf("[%s] spk dout=%d bck=%d ws=%d mck=%d port=%d stereo=%d buzzer=%d dac=%d rate=%u vol=%u\n",
                  phase, spk.pin_data_out, spk.pin_bck, spk.pin_ws, spk.pin_mck,
                  (int)spk.i2s_port, spk.stereo, spk.buzzer, spk.use_dac,
                  (unsigned)spk.sample_rate, M5.Speaker.getVolume());
    Serial.printf("[%s] mic din=%d bck=%d ws=%d mck=%d port=%d enabled=%d\n",
                  phase, mic.pin_data_in, mic.pin_bck, mic.pin_ws, mic.pin_mck,
                  (int)mic.i2s_port, M5.Mic.isEnabled());
}

static void draw_status(const char* line1, const char* line2) {
    M5.Display.fillScreen(TFT_BLACK);
    M5.Display.setTextColor(TFT_WHITE, TFT_BLACK);
    M5.Display.setTextSize(1);
    M5.Display.setCursor(4, 4);
    M5.Display.println("Speaker smoke test");
    M5.Display.printf("board=%d\n", (int)M5.getBoard());
    M5.Display.println(line1);
    M5.Display.println(line2);
}

void setup() {
    auto cfg = M5.config();
    cfg.fallback_board = m5::board_t::board_M5StickS3;
    M5.begin(cfg);
    M5.Display.setRotation(1);

    Serial.begin(115200);
    delay(1000);
    build_wav_test();

    M5.Mic.end();
    M5.Speaker.setVolume(255);
    M5.Speaker.setAllChannelVolume(255);
    M5.Speaker.setChannelVolume(0, 255);
    bool ok = M5.Speaker.begin();
    force_audio_regs(0xFF);
    print_audio_config("setup", ok);
    Serial.printf("[setup] speaker master=%u ch0=%u\n",
                  M5.Speaker.getVolume(), M5.Speaker.getChannelVolume(0));
    print_audio_regs("setup");

    auto spk = M5.Speaker.config();
    char line1[64];
    char line2[64];
    snprintf(line1, sizeof(line1), "spk ok=%d dout=%d", ok, spk.pin_data_out);
    snprintf(line2, sizeof(line2), "bck=%d ws=%d mck=%d", spk.pin_bck, spk.pin_ws, spk.pin_mck);
    draw_status(line1, line2);
}

void loop() {
    M5.update();
    Serial.println("[loop] tone 7000Hz");
    force_audio_regs(0xFF);
    print_audio_regs("pre-tone");
    M5.Speaker.tone(7000, 300, 0, true);
    while (M5.Speaker.isPlaying(0)) {
        M5.delay(1);
        M5.update();
    }
    M5.delay(250);

    Serial.println("[loop] tone 1000Hz");
    M5.Speaker.tone(1000, 500, 0, true);
    while (M5.Speaker.isPlaying(0)) {
        M5.delay(1);
        M5.update();
    }
    M5.delay(250);

    Serial.println("[loop] wav 880Hz 16kHz/16bit/mono");
    force_audio_regs(0xFF);
    print_audio_regs("pre-wav");
    bool wavOk = M5.Speaker.playWav(wavTest, sizeof(wavTest), 1, 0, true);
    Serial.printf("[loop] wav playWav=%d\n", wavOk);
    while (M5.Speaker.isPlaying(0)) {
        M5.delay(1);
        M5.update();
    }
    M5.delay(250);

    static uint8_t ampMode = 0;
    Serial.printf("[loop] mp3-pcm amp-mode=%u samples=%u/%u bytes=%u\n",
                  ampMode,
                  (unsigned)PCM_TEST_SAMPLES,
                  (unsigned)(space_animal_pcm_len / sizeof(int16_t)),
                  (unsigned)space_animal_pcm_len);
    force_audio_regs(0xFF);
    apply_amp_mode(ampMode);
    print_audio_regs("pre-mp3-pcm");
    const int16_t* pcmStart = reinterpret_cast<const int16_t*>(space_animal_pcm) + PCM_OFFSET_SAMPLES;
    bool pcmOk = M5.Speaker.playRaw(pcmStart,
                                    PCM_TEST_SAMPLES,
                                    16000,
                                    false,
                                    1,
                                    0,
                                    true);
    Serial.printf("[loop] mp3-pcm playRaw=%d\n", pcmOk);
    while (M5.Speaker.isPlaying(0)) {
        M5.delay(1);
        M5.update();
    }
    ampMode = (ampMode + 1) % 9;
    M5.delay(1500);
}
