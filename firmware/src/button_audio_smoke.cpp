#include <M5Unified.h>
#include <driver/i2s.h>
#include <soc/i2s_struct.h>
#include <cmath>

static constexpr uint8_t ES8311_ADDR = 0x18;
static constexpr uint8_t M5PM1_ADDR = 0x6E;
static constexpr uint32_t I2C_FREQ = 100000;
static constexpr uint32_t SAMPLE_RATE = 16000;
static constexpr gpio_num_t BTN_FRONT_PIN = GPIO_NUM_11;
static constexpr gpio_num_t BTN_SIDE_PIN = GPIO_NUM_12;

static bool directI2sStarted = false;
static bool lastFrontDown = false;
static bool lastSideDown = false;
static uint32_t lastButtonMs = 0;

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

static void configure_es8311(uint8_t volume) {
    write_reg(ES8311_ADDR, 0x00, 0x80);
    delay(10);
    write_reg(ES8311_ADDR, 0x01, 0xB5);
    write_reg(ES8311_ADDR, 0x02, 0x18);
    write_reg(ES8311_ADDR, 0x0D, 0x01);
    write_reg(ES8311_ADDR, 0x12, 0x00);
    write_reg(ES8311_ADDR, 0x13, 0x10);
    write_reg(ES8311_ADDR, 0x32, volume);
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

static void draw_status(const char* mode, const char* detail) {
    M5.Display.fillScreen(TFT_BLACK);
    M5.Display.setTextColor(TFT_WHITE, TFT_BLACK);
    M5.Display.setTextSize(1);
    M5.Display.setCursor(4, 4);
    M5.Display.println("Button audio smoke");
    M5.Display.println("Front: M5.Speaker");
    M5.Display.println("Side: direct I2S");
    M5.Display.println();
    M5.Display.printf("Last: %s\n", mode);
    M5.Display.println(detail);
}

static void stop_direct_i2s() {
    if (!directI2sStarted) return;
    i2s_stop(I2S_NUM_0);
    i2s_driver_uninstall(I2S_NUM_0);
    directI2sStarted = false;
}

static bool start_direct_i2s() {
    stop_direct_i2s();
    M5.Speaker.end();
    configure_amp();
    configure_es8311(0xBF);

    i2s_config_t i2sCfg = {};
    i2sCfg.mode = static_cast<i2s_mode_t>(I2S_MODE_MASTER | I2S_MODE_TX);
    i2sCfg.sample_rate = SAMPLE_RATE;
    i2sCfg.bits_per_sample = I2S_BITS_PER_SAMPLE_16BIT;
    i2sCfg.channel_format = I2S_CHANNEL_FMT_RIGHT_LEFT;
    i2sCfg.communication_format = I2S_COMM_FORMAT_I2S;
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
        i2s_driver_uninstall(I2S_NUM_0);
        return false;
    }

    uint32_t actualRate = apply_i2s_clock(SAMPLE_RATE);
    Serial.printf("[i2s] clock target=%u actual=%u mclk=%u\n",
                  (unsigned)SAMPLE_RATE,
                  (unsigned)actualRate,
                  (unsigned)(SAMPLE_RATE * 256));

    directI2sStarted = true;
    i2s_start(I2S_NUM_0);
    return true;
}

static void play_direct_i2s() {
    Serial.println("[side] direct I2S tone");
    draw_status("direct I2S", "playing 880Hz / 2s");
    print_regs("pre-direct");
    if (!start_direct_i2s()) {
        draw_status("direct I2S", "start failed");
        return;
    }

    static int16_t frames[512 * 2];
    uint32_t phase = 0;
    const uint32_t phaseStep = (880u * 2u * 65536u) / SAMPLE_RATE;
    const uint32_t loops = (SAMPLE_RATE * 2u) / 512u;

    for (uint32_t n = 0; n < loops; ++n) {
        for (size_t i = 0; i < 512; ++i) {
            phase += phaseStep;
            int16_t sample = (phase & 0x10000) ? 14000 : -14000;
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

    stop_direct_i2s();
    print_regs("post-direct");
    draw_status("direct I2S", "done");
}

static void play_speaker() {
    Serial.println("[front] M5.Speaker tone");
    draw_status("M5.Speaker", "playing 1kHz / 2s");
    stop_direct_i2s();
    M5.Speaker.end();
    configure_amp();
    configure_es8311(0xFF);
    M5.Speaker.setVolume(255);
    M5.Speaker.setAllChannelVolume(255);
    M5.Speaker.setChannelVolume(0, 255);

    bool ok = M5.Speaker.begin();
    configure_amp();
    configure_es8311(0xFF);
    auto cfg = M5.Speaker.config();
    Serial.printf("[speaker] begin=%d enabled=%d running=%d dout=%d bck=%d ws=%d mck=%d rate=%u vol=%u\n",
                  ok,
                  M5.Speaker.isEnabled(),
                  M5.Speaker.isRunning(),
                  cfg.pin_data_out,
                  cfg.pin_bck,
                  cfg.pin_ws,
                  cfg.pin_mck,
                  (unsigned)cfg.sample_rate,
                  M5.Speaker.getVolume());
    print_regs("pre-speaker");

    M5.Speaker.tone(1000, 2000, 0, true);
    while (M5.Speaker.isPlaying(0)) {
        M5.delay(1);
        M5.update();
    }

    M5.Speaker.end();
    print_regs("post-speaker");
    draw_status("M5.Speaker", "done");
}

void setup() {
    auto cfg = M5.config();
    cfg.fallback_board = m5::board_t::board_M5StickS3;
    M5.begin(cfg);
    M5.Display.setRotation(1);

    Serial.begin(115200);
    delay(1000);

    pinMode(BTN_FRONT_PIN, INPUT_PULLUP);
    pinMode(BTN_SIDE_PIN, INPUT_PULLUP);
    lastFrontDown = digitalRead(BTN_FRONT_PIN) == LOW;
    lastSideDown = digitalRead(BTN_SIDE_PIN) == LOW;

    M5.Mic.end();
    M5.Speaker.end();
    configure_amp();
    configure_es8311(0xBF);

    Serial.printf("[setup] board=%d front_gpio=%d side_gpio=%d\n",
                  (int)M5.getBoard(),
                  (int)BTN_FRONT_PIN,
                  (int)BTN_SIDE_PIN);
    print_regs("setup");
    draw_status("ready", "press a button");
}

void loop() {
    M5.update();

    bool frontDown = digitalRead(BTN_FRONT_PIN) == LOW;
    bool sideDown = digitalRead(BTN_SIDE_PIN) == LOW;
    bool frontPressed = frontDown && !lastFrontDown;
    bool sidePressed = sideDown && !lastSideDown;
    lastFrontDown = frontDown;
    lastSideDown = sideDown;

    uint32_t now = millis();
    if ((frontPressed || sidePressed) && now - lastButtonMs < 150) {
        return;
    }

    if (frontPressed || M5.BtnA.wasPressed()) {
        lastButtonMs = now;
        play_speaker();
    } else if (sidePressed || M5.BtnB.wasPressed()) {
        lastButtonMs = now;
        play_direct_i2s();
    }

    delay(10);
}
