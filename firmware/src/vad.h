#pragma once
#include <cstddef>
#include <cstdint>

struct VADConfig {
    int sample_rate = 16000;
    int frame_ms = 20;           // 每帧时长
    int threshold = 500;         // RMS 能量阈值
    int silence_frames = 30;     // 连续静音帧数视为结束（600ms）
    int min_speech_frames = 15;  // 最短有效语音帧数（300ms）
    int max_frames = 1500;       // 最长录音帧数（30s）
};

class VAD {
public:
    explicit VAD(const VADConfig& cfg = VADConfig{});
    // 返回 true 表示继续录音，false 表示检测到结束
    bool process(const int16_t* samples, size_t sample_count);
    void reset();
private:
    VADConfig cfg_;
    int silence_count_ = 0;
    int speech_count_ = 0;
    int total_frames_ = 0;
};
