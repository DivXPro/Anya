#include "vad.h"
#include <cmath>
#include <cstdlib>

VAD::VAD(const VADConfig& cfg) : cfg_(cfg) {}

void VAD::reset() {
    silence_count_ = 0;
    speech_count_ = 0;
    total_frames_ = 0;
}

static int rms_energy(const int16_t* samples, size_t n) {
    if (n == 0) return 0;
    int64_t sum = 0;
    for (size_t i = 0; i < n; ++i) {
        int32_t v = samples[i];
        sum += v * v;
    }
    return static_cast<int>(std::sqrt(static_cast<double>(sum) / n));
}

bool VAD::process(const int16_t* samples, size_t sample_count) {
    size_t frame_samples = static_cast<size_t>(cfg_.sample_rate * cfg_.frame_ms / 1000);
    if (sample_count != frame_samples) {
        // 只接受整帧；非整帧按继续处理，但不计数
        return true;
    }

    ++total_frames_;
    if (total_frames_ >= cfg_.max_frames) {
        return false;
    }

    int energy = rms_energy(samples, sample_count);
    if (energy >= cfg_.threshold) {
        ++speech_count_;
        silence_count_ = 0;
    } else {
        ++silence_count_;
    }

    if (speech_count_ < cfg_.min_speech_frames) {
        return true; // 还没录够有效语音，不结束
    }

    return silence_count_ < cfg_.silence_frames;
}
