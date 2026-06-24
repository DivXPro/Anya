#pragma once
#include <cstdint>
#include <cstddef>

void protocol_init();
void protocol_handle_message(const char* json);
void protocol_handle_binary(const uint8_t* data, size_t len);
void protocol_send_audio_start();
void protocol_send_audio_chunk(const uint8_t* data, size_t len);
void protocol_send_audio_end();
void protocol_send_button(const char* action);
