#pragma once
#include <cstdint>
#include <cstddef>

void audio_init();
void audio_start_recording();
void audio_stop_recording();
size_t audio_capture(uint8_t* buffer, size_t maxLen);
void audio_play(const uint8_t* data, size_t len);
void audio_play_test_tone();
void audio_play_start_tone();
void audio_play_end_tone();
void audio_begin_playback();
void audio_finish_playback();
bool audio_is_recording();
bool audio_is_playing();
