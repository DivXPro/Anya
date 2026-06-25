#pragma once
#include <WebSocketsClient.h>
#include <functional>

typedef std::function<void(const char* msg)> TextCallback;
typedef std::function<void(const uint8_t* data, size_t len)> BinaryCallback;

void ws_init();
bool ws_connect(const char* host, uint16_t port);
void ws_set_hello_data(const char* deviceID, const char* deviceName, const char* boundDesktopID, const char* pairingToken);
void ws_send_text(const char* json);
void ws_send_binary(const uint8_t* data, size_t len);
void ws_on_text(TextCallback cb);
void ws_on_binary(BinaryCallback cb);
void ws_on_close(std::function<void()> cb);
void ws_loop();
void ws_disconnect();
bool ws_connected();
