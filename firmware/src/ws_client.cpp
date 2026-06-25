#include "ws_client.h"

static WebSocketsClient client;
static TextCallback textCb = nullptr;
static BinaryCallback binaryCb = nullptr;
static std::function<void()> closeCb = nullptr;
static bool connected = false;

static void onEvent(WStype_t type, uint8_t* payload, size_t length) {
    switch (type) {
        case WStype_CONNECTED:
            connected = true;
            break;
        case WStype_DISCONNECTED:
            connected = false;
            if (closeCb) closeCb();
            break;
        case WStype_TEXT:
            if (textCb && payload) {
                textCb((const char*)payload);
            }
            break;
        case WStype_BIN:
            if (binaryCb && payload) {
                binaryCb(payload, length);
            }
            break;
        default:
            break;
    }
}

void ws_init() {
    client.onEvent(onEvent);
}

bool ws_connect(const char* host, uint16_t port) {
    client.begin(host, port, "/device");
    return true; // connection is async, status comes via WStype_CONNECTED
}

void ws_send_text(const char* json) {
    client.sendTXT(json);
}

void ws_send_binary(const uint8_t* data, size_t len) {
    client.sendBIN(data, len);
}

void ws_on_text(TextCallback cb) { textCb = cb; }
void ws_on_binary(BinaryCallback cb) { binaryCb = cb; }
void ws_on_close(std::function<void()> cb) { closeCb = cb; }

void ws_loop() {
    client.loop();
}

void ws_disconnect() {
    client.disconnect();
    connected = false;
}

bool ws_connected() {
    return connected;
}
