# Anya Firmware

M5StickC S3 firmware for the Anya hardware agent voice assistant.

## Prerequisites

- PlatformIO CLI (`pio`)
- M5StickC S3 hardware (or compatible ESP32-S3 board)

## Board Configuration

The M5StickC S3 board is not available in PlatformIO's built-in board list. You have two options:

### Option 1: Custom Board Definition (Recommended)

Add the M5Stack board definitions to PlatformIO by creating `platformio.ini` with:

```ini
[env:m5stickc-s3]
platform = espressif32@6.5.0
board = m5stack-cores3
framework = arduino
board_build.mcu = esp32s3
board_build.f_cpu = 240000000L
```

### Option 2: Use Latest espressif32 Platform

Use the latest platform version and a generic ESP32-S3 board:

```ini
[env:m5stickc-s3]
platform = espressif32
board = esp32-s3-devkitc-1
framework = arduino
```

Then adjust pin definitions as needed.

## Building

```bash
cd firmware
pio run
```

## Uploading

Connect the M5StickC S3 via USB-C:

```bash
pio run --target upload
```

## Monitoring

```bash
pio device monitor
```

## Architecture

```
src/
├── main.cpp         # Entry point + state machine loop
├── wifi.h/cpp       # WiFi manager (STA + NVS credentials)
├── wifi_portal.h/cpp # Captive portal for first-time WiFi setup
├── mdns_client.h/cpp # mDNS advertiser + HTTP /connect endpoint
├── ws_client.h/cpp   # WebSocket client
├── audio.h/cpp       # I2S mic capture + speaker playback
├── display.h/cpp     # Screen rendering for all states
├── buttons.h/cpp     # Push-to-talk button handling
├── protocol.h/cpp    # JSON protocol encoder/decoder
└── state.h/cpp       # State machine (WIFI_SETUP → IDLE → LISTENING → ...)
```

## State Machine

```
WIFI_SETUP → WIFI_CONNECTING → PAIR_READY → PAIRING → IDLE
                                                         ↓
                                              LISTENING ↔ SENDING
                                                         ↓
                                              PROCESSING → PLAYING
```

## Protocol

Uses WebSocket binary frames for PCM 16kHz 16bit mono audio and JSON text frames for control messages. See the design doc for the full protocol specification.
