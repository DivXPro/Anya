#include <M5Unified.h>
#include <cmath>

static constexpr uint8_t ES8311_ADDR = 0x18;
static constexpr uint8_t M5PM1_ADDR = 0x6E;
static constexpr uint32_t I2C_FREQ = 100000;
static constexpr uint32_t SAMPLE_RATE = 16000;
static constexpr size_t CHUNK_SAMPLES = 1024;
static constexpr gpio_num_t BTN_FRONT_PIN = GPIO_NUM_11;
static constexpr gpio_num_t BTN_SIDE_PIN = GPIO_NUM_12;

static int16_t chunkBuffer[CHUNK_SAMPLES];
static bool lastFrontDown = false;
static bool lastSideDown = false;

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
    write_reg(ES8311_ADDR, 0x32, 0xBF);
    write_reg(ES8311_ADDR, 0x37, 0x08);
}

static void draw_status(const char* mode, const char* detail) {
    M5.Display.fillScreen(TFT_BLACK);
    M5.Display.setTextColor(TFT_WHITE, TFT_BLACK);
    M5.Display.setTextSize(1);
    M5.Display.setCursor(4, 4);
    M5.Display.println("Speaker stream smoke");
    M5.Display.println("Front: chunked PCM");
    M5.Display.println("Side: mic switch + PCM");
    M5.Display.println();
    M5.Display.printf("Mode: %s\n", mode);
    M5.Display.println(detail);
}

static bool start_speaker() {
    while (M5.Mic.isRecording()) {
        delay(1);
    }
    M5.Mic.end();
    M5.Speaker.end();

    auto cfg = M5.Speaker.config();
    cfg.sample_rate = 44100;
    cfg.stereo = true;
    M5.Speaker.config(cfg);
    M5.Speaker.setVolume(255);
    M5.Speaker.setAllChannelVolume(255);
    M5.Speaker.setChannelVolume(0, 255);

    configure_amp();
    bool ok = M5.Speaker.begin();
    configure_amp();
    configure_es8311();

    auto spk = M5.Speaker.config();
    Serial.printf("[speaker] begin=%d running=%d dout=%d bck=%d ws=%d mck=%d rate=%u queued=%u\n",
                  ok,
                  M5.Speaker.isRunning(),
                  spk.pin_data_out,
                  spk.pin_bck,
                  spk.pin_ws,
                  spk.pin_mck,
                  (unsigned)spk.sample_rate,
                  (unsigned)M5.Speaker.isPlaying(0));
    return ok;
}

static void stop_speaker() {
    M5.Speaker.stop(0);
    while (M5.Speaker.isPlaying(0)) {
        delay(1);
        M5.update();
    }
    M5.Speaker.end();
}

static void fill_tone(int16_t* out, size_t samples, uint32_t startSample, float freq) {
    static constexpr float PI_F = 3.14159265f;
    for (size_t i = 0; i < samples; ++i) {
        float t = (startSample + i) / static_cast<float>(SAMPLE_RATE);
        out[i] = static_cast<int16_t>(9000.0f * sinf(2.0f * PI_F * freq * t));
    }
}

static bool play_chunk(uint32_t cursor) {
    fill_tone(chunkBuffer, CHUNK_SAMPLES, cursor, 880.0f);
    while (M5.Speaker.isPlaying(0)) {
        delay(1);
        M5.update();
    }
    bool ok = M5.Speaker.playRaw(chunkBuffer, CHUNK_SAMPLES, SAMPLE_RATE, false, 1, 0, false);
    Serial.printf("[speaker] playRaw ok=%d queued=%u cursor=%u\n",
                  ok,
                  (unsigned)M5.Speaker.isPlaying(0),
                  (unsigned)cursor);
    return ok;
}

static void play_chunked_pcm() {
    draw_status("speaker", "playing chunked 16-bit PCM");
    if (!start_speaker()) {
        draw_status("speaker", "begin failed");
        return;
    }

    const uint32_t totalSamples = SAMPLE_RATE * 4;
    for (uint32_t cursor = 0; cursor < totalSamples; cursor += CHUNK_SAMPLES) {
        if (!play_chunk(cursor)) {
            draw_status("speaker", "playRaw failed");
            break;
        }
    }

    while (M5.Speaker.isPlaying(0)) {
        delay(1);
        M5.update();
    }
    stop_speaker();
    draw_status("speaker", "done");
}

static void switch_mic_then_play() {
    draw_status("mic", "starting mic first");
    M5.Speaker.end();
    auto micCfg = M5.Mic.config();
    micCfg.sample_rate = SAMPLE_RATE;
    M5.Mic.config(micCfg);
    bool micOk = M5.Mic.begin();
    Serial.printf("[mic] begin=%d enabled=%d\n", micOk, M5.Mic.isEnabled());
    delay(500);
    play_chunked_pcm();
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
    configure_es8311();
    Serial.printf("[setup] board=%d front_gpio=%d side_gpio=%d\n",
                  (int)M5.getBoard(),
                  (int)BTN_FRONT_PIN,
                  (int)BTN_SIDE_PIN);
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

    if (frontPressed || M5.BtnA.wasPressed()) {
        play_chunked_pcm();
    } else if (sidePressed || M5.BtnB.wasPressed()) {
        switch_mic_then_play();
    }
    delay(10);
}
