#include <unity.h>
#include "vad.h"

static const int SAMPLE_RATE = 16000;
static const int FRAME_MS = 20;
static const size_t FRAME_SAMPLES = SAMPLE_RATE * FRAME_MS / 1000;

void setUp() {}
void tearDown() {}

void generate_silence(int16_t* buf, size_t n) {
    for (size_t i = 0; i < n; ++i) buf[i] = 0;
}

void generate_tone(int16_t* buf, size_t n, int freq) {
    for (size_t i = 0; i < n; ++i) {
        float t = static_cast<float>(i) / SAMPLE_RATE;
        buf[i] = static_cast<int16_t>(2000.0f * sinf(2.0f * 3.14159265f * freq * t));
    }
}

void test_vad_silence_never_ends_before_min_speech() {
    VADConfig cfg;
    cfg.sample_rate = SAMPLE_RATE;
    cfg.frame_ms = FRAME_MS;
    cfg.threshold = 500;
    cfg.silence_frames = 30;
    cfg.min_speech_frames = 15;
    cfg.max_frames = 1500;
    VAD vad(cfg);

    int16_t frame[FRAME_SAMPLES];
    generate_silence(frame, FRAME_SAMPLES);

    for (int i = 0; i < 50; ++i) {
        TEST_ASSERT_TRUE(vad.process(frame, FRAME_SAMPLES));
    }
}

void test_vad_ends_after_speech_then_silence() {
    VADConfig cfg;
    cfg.sample_rate = SAMPLE_RATE;
    cfg.frame_ms = FRAME_MS;
    cfg.threshold = 500;
    cfg.silence_frames = 5;
    cfg.min_speech_frames = 2;
    cfg.max_frames = 1500;
    VAD vad(cfg);

    int16_t frame[FRAME_SAMPLES];
    generate_tone(frame, FRAME_SAMPLES, 800);

    TEST_ASSERT_TRUE(vad.process(frame, FRAME_SAMPLES));
    TEST_ASSERT_TRUE(vad.process(frame, FRAME_SAMPLES));

    generate_silence(frame, FRAME_SAMPLES);
    TEST_ASSERT_TRUE(vad.process(frame, FRAME_SAMPLES));
    TEST_ASSERT_TRUE(vad.process(frame, FRAME_SAMPLES));
    TEST_ASSERT_TRUE(vad.process(frame, FRAME_SAMPLES));
    TEST_ASSERT_TRUE(vad.process(frame, FRAME_SAMPLES));
    TEST_ASSERT_FALSE(vad.process(frame, FRAME_SAMPLES));
}

void test_vad_resets() {
    VADConfig cfg;
    cfg.sample_rate = SAMPLE_RATE;
    cfg.frame_ms = FRAME_MS;
    cfg.threshold = 500;
    cfg.silence_frames = 2;
    cfg.min_speech_frames = 1;
    cfg.max_frames = 1500;
    VAD vad(cfg);

    int16_t frame[FRAME_SAMPLES];
    generate_tone(frame, FRAME_SAMPLES, 800);
    TEST_ASSERT_TRUE(vad.process(frame, FRAME_SAMPLES));

    generate_silence(frame, FRAME_SAMPLES);
    TEST_ASSERT_TRUE(vad.process(frame, FRAME_SAMPLES));   // silence frame 1 of silence_frames=2
    TEST_ASSERT_FALSE(vad.process(frame, FRAME_SAMPLES));  // silence frame 2 -> end detected

    vad.reset();
    TEST_ASSERT_TRUE(vad.process(frame, FRAME_SAMPLES));
}

int main(int argc, char** argv) {
    UNITY_BEGIN();
    RUN_TEST(test_vad_silence_never_ends_before_min_speech);
    RUN_TEST(test_vad_ends_after_speech_then_silence);
    RUN_TEST(test_vad_resets);
    UNITY_END();
    return 0;
}
