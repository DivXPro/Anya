#include "ws_client.h"

static WebsocketsClient client;
static TextCallback textCb = nullptr;
static BinaryCallback binaryCb = nullptr;
static std::function<void()> closeCb = nullptr;

void ws_init() {}

bool ws_connect(const char* host, uint16_t port) {
    return client.connect(host, port, "/device");
}

void ws_send_text(const char* json) {
    client.send(json);
}

void ws_send_binary(const uint8_t* data, size_t len) {
    client.sendBinary((const char*)data, len);
}

void ws_on_text(TextCallback cb) { textCb = cb; }
void ws_on_binary(BinaryCallback cb) { binaryCb = cb; }
void ws_on_close(std::function<void()> cb) { closeCb = cb; }

void ws_loop() {
    if (!client.available()) return;

    auto msg = client.readBlocking();
    if (msg.isText() && textCb) {
        textCb(msg.data().c_str());
    } else if (msg.isBinary() && binaryCb) {
        binaryCb((const uint8_t*)msg.data().c_str(), msg.data().length());
    }
}

void ws_disconnect() {
    client.close();
}

bool ws_connected() {
    return client.available();
}
