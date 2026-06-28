# Anya

[English](README.md) | [中文](README.zh.md)

> A compact voice assistant device for your workspace — just talk, and Anya responds.

Anya is a hands-free voice companion that sits on your desk. A small hardware device listens when you press its talk button, sends your voice to the Anya desktop app on your computer, and speaks back the answer through its built-in speaker while showing a short summary on its screen.

You can ask it to write code, look things up, take notes, or control your tools — whatever your favorite local AI agent can do.

---

## Works with the agents you already use

Anya is not tied to a single AI provider. It speaks to any local agent that supports the ACP protocol, so you can pick the right brain for each task or keep using the tools you already have installed.

Out of the box, Anya can drive:

- **Claude Code**
- **Codex**
- **OpenCode**
- **Kimi**
- **Hermes**
- **Pi**

Switching agents takes a single click in the menu bar or settings window. New ACP-compatible agents can be added without replacing the device or rewriting the core app.

---

## What Anya does

- **Push-to-talk voice control** — Hold the button on the device, speak, and release. Anya handles the rest.
- **Natural responses** — Your voice is transcribed locally on your computer, routed to an AI agent, then turned into speech and sent back to the device.
- **Visual feedback** — The device screen shows connection status, what it heard, and a short summary of the reply.
- **Multiple AI agents** — Switch between ACP-compatible agents such as Claude Code, Codex, OpenCode, Kimi, Hermes, and Pi directly from the menu bar or settings window.
- **Automatic reconnection** — Once paired, the device remembers your computer and reconnects automatically when it comes back online.
- **Over-the-air firmware updates** — Update the device firmware from the desktop app without plugging in any cables.
- **Conversation history** — All your exchanges are stored locally on your computer, searchable from the app.

---

## How it works

```
┌──────────────┐     WiFi / LAN     ┌──────────────────┐
│  Anya device │  ◄──────────────►  │  Anya desktop app│
│  (M5StickC   │      WebSocket     │  on your Mac or  │
│   S3)        │                    │  Windows PC      │
└──────────────┘                    └────────┬─────────┘
                                             │
                              ┌──────────────┼──────────────┐
                              ▼              ▼              ▼
                         ┌────────┐    ┌─────────┐    ┌──────────┐
                         │  STT   │    │  ACP    │    │   TTS    │
                         │ Whisper│    │  agent  │    │ Edge TTS │
                         └────────┘    └─────────┘    └──────────┘
```

1. You press and hold the talk button on the device.
2. Audio is streamed to the desktop app over your local network.
3. The app transcribes your speech, sends the text to the selected agent, and receives a reply.
4. The reply is summarized for the screen, synthesized into speech, and played back on the device.

---

## Hardware

Anya is designed around the **M5StickC S3** (ESP32-S3) with a microphone, speaker, display, and push button. You can build one yourself with off-the-shelf parts.

Required hardware:

- M5StickC S3
- USB-C cable for flashing and power
- A computer on the same WiFi network as the device

---

## Getting started

### 1. Install the desktop app

Download the latest release for macOS or Windows, or build it from source:

```bash
./build.sh
```

The first time you run the app, it creates a local profile in `~/.elf/` on your computer.

### 2. Flash the firmware

Plug the M5StickC S3 into your computer via USB-C and flash the firmware:

```bash
cd firmware
pio run --target upload
```

Alternatively, use **Settings → Device Firmware** in the desktop app to flash a prebuilt firmware image.

### 3. Connect the device to WiFi

When you power on the device for the first time, it creates a hotspot named **Anya-hotspot**:

1. Connect your phone or computer to `Anya-hotspot`.
2. Open the captive portal at `http://192.168.4.1/`.
3. Select your home WiFi network and enter the password.
4. The device will restart and join your network.

### 4. Pair the device

Open the Anya desktop app:

1. Go to the **Devices** tab and click **Scan**.
2. Choose your device from the list and click **Connect**.
3. The device will pair with your computer and remember it for next time.

### 5. Choose an agent and start talking

Select the AI agent you want to use from the menu bar or settings window, then hold the talk button and speak. The screen will show a short summary while the device reads the full answer aloud.

---

## Updating firmware

Anya supports wireless firmware updates:

1. Connect the device to the desktop app.
2. Open **Settings → Device Firmware**.
3. If a newer firmware version is available, click **Update**.
4. Wait for the progress bar to complete. The device will restart automatically.

---

## Privacy

Anya is built to be local-first:

- Your voice is transcribed on your own computer using a local Whisper model.
- Conversation history is stored in a local SQLite database.
- AI agents run through local ACP-compatible clients you already control.
- The device and desktop app communicate only over your local network.

---

## Development

If you want to build or modify Anya yourself:

| Module | Command |
|--------|---------|
| Desktop app + firmware bundle | `./build.sh` |
| Desktop app only | `cd desktop && wails3 build` |
| Run Go tests | `cd desktop && go test ./... -count=1` |
| Firmware only | `cd firmware && pio run` |

See [`CLAUDE.md`](CLAUDE.md) for project conventions and [`firmware/README.md`](firmware/README.md) for hardware-specific details.

---

## Roadmap

- [x] Push-to-talk voice requests
- [x] Multi-agent support (Claude Code, Codex, OpenCode, Kimi, Hermes, Pi)
- [x] mDNS device discovery and auto-reconnection
- [x] Over-the-air firmware updates
- [ ] Wake-word activation
- [ ] Support for additional ESP32-based devices

---

## Contributing

Contributions are welcome. Please open an issue or pull request, and include a summary of the change, the modules affected, and any relevant test results or hardware notes.

Use Conventional Commit-style messages, for example:

```
feat: add device authorization flow
test: add gateway protocol codec tests
```

---

