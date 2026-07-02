#include <M5Unified.h>
#include <driver/i2s.h>

static constexpr uint8_t ES8311_ADDR = 0x18;
static constexpr uint8_t M5PM1_ADDR = 0x6E;
static constexpr uint32_t I2C_FREQ = 100000;
static bool i2sStarted = false;

struct I2sTestMode {
    const char* name;
    uint32_t sampleRate;
    i2s_comm_format_t commFormat;
};

static const I2sTestMode modes[] = {
    {"i2s-44k1", 44100, I2S_COMM_FORMAT_I2S},
    {"i2s-16k", 16000, I2S_COMM_FORMAT_I2S},
    {"msb-16k", 16000, I2S_COMM_FORMAT_I2S_MSB},
};

static uint8_t read_reg(uint8_t addr, uint8_t reg) {
    return M5.In_I2C.readRegister8(addr, reg, I2C_FREQ);
}

static void write_reg(uint8_t addr, uint8_t reg, uint8_t value) {
    M5.In_I2C.writeRegister8(addr, reg, value, I2C_FREQ);
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
    write_reg(ES8311_ADDR, 0x32, 0xFF);
    write_reg(ES8311_ADDR, 0x37, 0x08);
}

static void print_regs(const char* phase) {
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

static void stop_i2s() {
    if (i2sStarted) {
        i2s_stop(I2S_NUM_0);
        i2s_driver_uninstall(I2S_NUM_0);
        i2sStarted = false;
    }
}

static bool start_i2s(const I2sTestMode& mode) {
    stop_i2s();

    i2s_config_t i2sCfg = {};
    i2sCfg.mode = static_cast<i2s_mode_t>(I2S_MODE_MASTER | I2S_MODE_TX);
    i2sCfg.sample_rate = mode.sampleRate;
    i2sCfg.bits_per_sample = I2S_BITS_PER_SAMPLE_16BIT;
    i2sCfg.channel_format = I2S_CHANNEL_FMT_RIGHT_LEFT;
    i2sCfg.communication_format = mode.commFormat;
    i2sCfg.intr_alloc_flags = ESP_INTR_FLAG_LEVEL1;
    i2sCfg.dma_buf_count = 8;
    i2sCfg.dma_buf_len = 256;
    i2sCfg.use_apll = false;
    i2sCfg.tx_desc_auto_clear = true;
    i2sCfg.fixed_mclk = 0;

    esp_err_t err = i2s_driver_install(I2S_NUM_0, &i2sCfg, 0, nullptr);
    if (err != ESP_OK) {
        Serial.printf("[i2s] driver_install err=%d\n", err);
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
        Serial.printf("[i2s] set_pin err=%d\n", err);
        stop_i2s();
        return false;
    }

    i2sStarted = true;
    i2s_start(I2S_NUM_0);
    return true;
}

static void play_square(const I2sTestMode& mode) {
    static int16_t frames[512 * 2];
    const uint32_t freq = 880;
    uint32_t phase = 0;
    const uint32_t phaseStep = (freq * 2u * 65536u) / mode.sampleRate;

    uint32_t loops = (mode.sampleRate * 2u) / 512u;
    for (uint32_t n = 0; n < loops; ++n) {
        for (size_t i = 0; i < 512; ++i) {
            phase += phaseStep;
            int16_t sample = (phase & 0x10000) ? 22000 : -22000;
            frames[i * 2] = sample;
            frames[i * 2 + 1] = sample;
        }
        size_t written = 0;
        esp_err_t err = i2s_write(I2S_NUM_0, frames, sizeof(frames), &written, pdMS_TO_TICKS(1000));
        if (n == 0 || err != ESP_OK || written != sizeof(frames)) {
            Serial.printf("[i2s] write err=%d written=%u\n", err, (unsigned)written);
        }
        M5.update();
    }
}

void setup() {
    auto cfg = M5.config();
    cfg.fallback_board = m5::board_t::board_M5StickS3;
    M5.begin(cfg);
    M5.Display.setRotation(1);
    Serial.begin(115200);
    delay(1000);

    M5.Mic.end();
    M5.Speaker.end();
    configure_amp();
    configure_es8311();

    Serial.printf("[setup] board=%d\n", (int)M5.getBoard());
    print_regs("setup");
    M5.Display.fillScreen(TFT_BLACK);
    M5.Display.setTextColor(TFT_WHITE, TFT_BLACK);
    M5.Display.setCursor(4, 4);
    M5.Display.println("Direct I2S smoke");
}

void loop() {
    for (const auto& mode : modes) {
        configure_amp();
        configure_es8311();
        Serial.printf("[loop] mode=%s rate=%u comm=%d\n",
                      mode.name, (unsigned)mode.sampleRate, (int)mode.commFormat);
        print_regs("pre-i2s");
        if (start_i2s(mode)) {
            play_square(mode);
            stop_i2s();
        }
        delay(1200);
        M5.update();
    }
}
