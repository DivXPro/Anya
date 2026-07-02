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

struct AgentSessionOption {
    char id[80];
    char title[80];
    char cwd[120];
    bool canResume;
};

using AgentSessionListCallback = void (*)(const AgentSessionOption* sessions, int count);

void protocol_on_agent_session_list(AgentSessionListCallback cb);
void protocol_send_agent_session_list_req();
void protocol_send_agent_session_select(const char* id, bool createNew, const char* cwd = nullptr);
