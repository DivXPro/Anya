#include "ws_client.h"
#include <esp_log.h>

static WebSocketsClient client;
static TextCallback textCb = nullptr;
static BinaryCallback binaryCb = nullptr;
static std::function<void()> closeCb = nullptr;
static bool connected = false;
static bool pendingHello = false;

static char helloDeviceID[40] = "";
static char helloDeviceName[40] = "";
static char helloBoundID[40] = "";
static char helloToken[40] = "";

static void sendHello() {
    char hello[256];
    if (helloToken[0] != '\0') {
        snprintf(hello, sizeof(hello),
            "{\"type\":\"hello\",\"payload\":{\"device_id\":\"%s\",\"name\":\"%s\",\"bound_desktop_id\":\"%s\",\"pairing_token\":\"%s\"}}",
            helloDeviceID, helloDeviceName, helloBoundID, helloToken);
    } else {
        snprintf(hello, sizeof(hello),
            "{\"type\":\"hello\",\"payload\":{\"device_id\":\"%s\",\"name\":\"%s\",\"bound_desktop_id\":\"%s\"}}",
            helloDeviceID, helloDeviceName, helloBoundID);
    }
    ESP_LOGI("ws", "sending hello: %s", hello);
    bool ok = client.sendTXT(hello);
    ESP_LOGI("ws", "sendTXT returned %s", ok ? "true" : "false");
    if (ok) {
        pendingHello = false;
    }
}

static void onEvent(WStype_t type, uint8_t* payload, size_t length) {
    ESP_LOGI("ws", "onEvent called type=%d len=%u", (int)type, (unsigned)length);
    switch (type) {
        case WStype_CONNECTED:
            ESP_LOGI("ws", "event: CONNECTED");
            connected = true;
            pendingHello = true;
            break;
        case WStype_DISCONNECTED:
            ESP_LOGI("ws", "event: DISCONNECTED");
            connected = false;
            pendingHello = false;
            if (closeCb) closeCb();
            break;
        case WStype_TEXT:
            ESP_LOGI("ws", "received text: %s", (const char*)payload);
            if (textCb && payload) {
                textCb((const char*)payload);
            }
            break;
        case WStype_BIN:
            ESP_LOGI("ws", "received binary len=%u", (unsigned)length);
            if (binaryCb && payload) {
                binaryCb(payload, length);
            }
            break;
        default:
            ESP_LOGI("ws", "event: other type=%d len=%u", (int)type, (unsigned)length);
            break;
    }
}

void ws_init() {
    client.onEvent(onEvent);
    client.setReconnectInterval(5000);
    // Ping every 10s; wait 3s for pong. If two consecutive pongs are missed,
    // the library closes the connection so the device notices a dead desktop quickly.
    client.enableHeartbeat(10000, 3000, 2);
}

bool ws_connect(const char* host, uint16_t port) {
    ESP_LOGI("ws", "connecting to ws://%s:%d/device", host, port);
    client.begin(host, port, "/device", "");
    return true;
}

void ws_set_hello_data(const char* deviceID, const char* deviceName, const char* boundDesktopID, const char* pairingToken) {
    strncpy(helloDeviceID, deviceID, sizeof(helloDeviceID) - 1);
    helloDeviceID[sizeof(helloDeviceID) - 1] = '\0';
    strncpy(helloDeviceName, deviceName, sizeof(helloDeviceName) - 1);
    helloDeviceName[sizeof(helloDeviceName) - 1] = '\0';
    strncpy(helloBoundID, boundDesktopID ? boundDesktopID : "", sizeof(helloBoundID) - 1);
    helloBoundID[sizeof(helloBoundID) - 1] = '\0';
    strncpy(helloToken, pairingToken ? pairingToken : "", sizeof(helloToken) - 1);
    helloToken[sizeof(helloToken) - 1] = '\0';
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
    if (connected && pendingHello) {
        sendHello();
    }
}

void ws_disconnect() {
    client.disconnect();
    connected = false;
}

bool ws_connected() {
    return connected;
}
