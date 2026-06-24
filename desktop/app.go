package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"log"
	"os"
	"path/filepath"
	"time"

	"desktop/internal/acp"
	"desktop/internal/acp/adapters"
	"desktop/internal/discovery"
	"desktop/internal/gateway"
	gatewayadapters "desktop/internal/gateway/adapters"
	"desktop/internal/processor"
	"desktop/internal/speech"
	"desktop/internal/store"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type App struct {
	ctx       context.Context
	db        *sql.DB
	desktopID string
	localIP   string
	router    *acp.Router
	wsServer  *gateway.Server
	stt       speech.STTEngine
	tts       speech.TTSEngine
}

func NewApp() *App {
	return &App{}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	log.Println("[elf] starting up...")

	dataDir := filepath.Join(os.Getenv("HOME"), ".elf")
	os.MkdirAll(dataDir, 0755)

	// 1. Init store
	db, err := store.InitDB(filepath.Join(dataDir, "elf.db"))
	if err != nil {
		log.Fatalf("[elf] init db: %v", err)
	}
	a.db = db

	// 1.5 Generate or load desktop identity
	a.desktopID, _ = store.GetSetting(a.db, "desktop_id")
	if a.desktopID == "" {
		a.desktopID = uuid.NewString()
		store.SetSetting(a.db, "desktop_id", a.desktopID)
	}

	// 2. Init ACP router + register all adapters
	a.router = acp.NewRouter()
	a.router.Register(adapters.NewClaudeAdapter())
	a.router.Register(adapters.NewOpenCodeAdapter())

	// 3. Init speech engines
	sttLang, _ := store.GetSetting(a.db, "stt_language")
	if sttLang == "" {
		sttLang = "zh"
	}
	a.stt = speech.NewWhisperSTT("small", sttLang)

	ttsVoice := "zh-CN-XiaoxiaoNeural"
	ttsSpeed, _ := store.GetSetting(a.db, "tts_speed")
	if ttsSpeed == "" {
		ttsSpeed = "+0%"
	}
	a.tts = speech.NewEdgeTTS(ttsVoice, ttsSpeed)

	// 4. Start WebSocket server with authorization
	a.wsServer = gateway.NewServer(9876, a.db, a.desktopID)
	a.wsServer.SetDeviceFactory(func(ws *websocket.Conn) gateway.DeviceAdapter {
		return gatewayadapters.NewStickCS3Adapter(ws)
	})
	a.wsServer.OnDeviceConnect(a.handleDeviceConnect)
	a.wsServer.OnDeviceDisconnect(a.handleDeviceDisconnect)
	a.wsServer.OnPendingDevice(func(deviceID, deviceName string) {
		log.Printf("[elf] pending device authorization: %s (%s)", deviceID, deviceName)
	})
	if err := a.wsServer.Start(); err != nil {
		log.Fatalf("[elf] start ws server: %v", err)
	}

	// Background IP self-healing: auto-reconnect known devices
	go a.autoReconnectKnownDevices(ctx)

	// Background cleanup: delete messages older than 90 days
	go func() {
		for {
			time.Sleep(24 * time.Hour)
			deleted, err := store.DeleteOldMessages(a.db, 90)
			if err != nil {
				log.Printf("[elf] cleanup error: %v", err)
			} else if deleted > 0 {
				log.Printf("[elf] cleaned up %d old messages", deleted)
			}
		}
	}()

	log.Println("[elf] startup complete")
}

func (a *App) ScanDevices() ([]discovery.DiscoveredDevice, error) {
	return discovery.ScanDevices()
}

func (a *App) Shutdown(ctx context.Context) {
	log.Println("[elf] shutting down...")
	a.wsServer.Stop()
	if a.db != nil {
		a.db.Close()
	}
	log.Println("[elf] shutdown complete")
}

func (a *App) handleDeviceConnect(dev gateway.DeviceAdapter) {
	info := dev.Info()
	log.Printf("[elf] device connected: %s (%s)", info.ID, info.Model)

	agents, err := store.ListAgents(a.db)
	if err != nil || len(agents) == 0 {
		log.Printf("[elf] no agents configured")
		return
	}
	agentID := agents[0].ID
	for _, ag := range agents {
		if ag.Enabled {
			agentID = ag.ID
			break
		}
	}
	session, err := store.CreateSession(a.db, info.ID, agentID)
	if err != nil {
		log.Printf("[elf] create session error: %v", err)
		return
	}

	dev.SendText(gateway.WelcomeMessage(info.ID, agentID, session.ID, a.desktopID))

	go a.handleDeviceEvents(dev, session.ID)
}

func (a *App) handleDeviceDisconnect(deviceID string) {
	log.Printf("[elf] device disconnected: %s", deviceID)
}

func (a *App) handleDeviceEvents(dev gateway.DeviceAdapter, sessionID string) {
	events, err := dev.ReceiveEvent()
	if err != nil {
		log.Printf("[elf] receive event channel error: %v", err)
		return
	}
	audioCh, err := dev.ReceiveBinary()
	if err != nil {
		log.Printf("[elf] receive audio channel error: %v", err)
		return
	}

	var audioBuffer []byte

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return
			}
			switch evt.Type {
			case "audio_start":
				audioBuffer = nil
				dev.SendText(gateway.StatusMessage("listening"))
			case "audio_end":
				dev.SendText(gateway.StatusMessage("processing"))
				go a.processVoiceRequest(dev, sessionID, audioBuffer)
			case "button":
				log.Printf("[elf] button: %s", evt.Action)
			case "ping":
				dev.SendText(gateway.StatusMessage("connected"))
			}

		case chunk, ok := <-audioCh:
			if !ok {
				return
			}
			audioBuffer = append(audioBuffer, chunk...)

		case <-dev.OnDisconnect():
			store.CloseSession(a.db, sessionID)
			return
		}
	}
}

func (a *App) processVoiceRequest(dev gateway.DeviceAdapter, sessionID string, audioData []byte) {
	// 1. STT
	text, err := a.stt.Transcribe(audioData, "pcm")
	if err != nil {
		dev.SendText(gateway.SummaryMessage("语音识别失败"))
		return
	}
	log.Printf("[elf] STT: %s", text)

	// 2. Save user message
	if err := store.InsertMessage(a.db, &store.Message{
		SessionID: sessionID, Role: "user", Content: text,
	}); err != nil {
		dev.SendText(gateway.SummaryMessage("保存消息失败"))
		return
	}

	// 3. Get history
	history, err := store.GetSessionMessages(a.db, sessionID)
	if err != nil {
		dev.SendText(gateway.SummaryMessage("读取历史失败"))
		return
	}
	acpHistory := make([]acp.Message, len(history))
	for i, m := range history {
		acpHistory[i] = acp.Message{Role: m.Role, Content: m.Content}
	}

	// 4. Route to ACP agent
	session, err := store.GetSession(a.db, sessionID)
	if err != nil {
		dev.SendText(gateway.SummaryMessage("会话读取失败"))
		return
	}
	events, err := a.router.Route(session.AgentID, text, acpHistory)
	if err != nil {
		dev.SendText(gateway.SummaryMessage("Agent 调用失败"))
		return
	}

	// 5. Process response with timeout monitoring
	pipeline := processor.NewPipeline(events)
	pipeline.SetExecTimeoutCallback(func() {
		dev.SendText(gateway.StatusMessage("processing"))
	})
	resp, result, err := pipeline.Process()
	switch result {
	case processor.ResultNoResponseTimeout:
		dev.SendText(gateway.SummaryMessage("Agent 超时无响应，已中断"))
		return
	case processor.ResultExecTimeout:
		// fall through
	}
	if err != nil {
		dev.SendText(gateway.SummaryMessage("处理出错"))
		return
	}

	// 6. Save assistant message
	if err := store.InsertMessage(a.db, &store.Message{
		SessionID: sessionID, Role: "assistant",
		Content: resp.Content, Summary: &resp.Summary,
	}); err != nil {
		dev.SendText(gateway.SummaryMessage("保存回复失败"))
		return
	}

	// 7. Send summary to device
	dev.SendText(gateway.SummaryMessage(resp.Summary))

	// 8. TTS → stream audio
	ttsEnabled, err := store.GetSetting(a.db, "tts_enabled")
	if err != nil {
		ttsEnabled = "true"
	}
	if ttsEnabled == "true" {
		dev.SendText(gateway.TTSStartMessage("pcm"))
		audioStream, err := a.tts.Synthesize(resp.Content)
		if err != nil {
			log.Printf("[elf] TTS error: %v", err)
		} else {
			for chunk := range audioStream {
				dev.SendBinary(chunk)
			}
		}
		dev.SendText(gateway.TTSEndMessage())
	}

	dev.SendText(gateway.StatusMessage("connected"))
	log.Printf("[elf] request complete")
}

// ConnectToDevice pre-authorizes a scanned device and sends a one-time token.
func (a *App) ConnectToDevice(deviceIP string, devicePort int, deviceID string, deviceName string) error {
	a.localIP = discovery.LocalIP()
	if err := store.AuthorizeDevice(a.db, deviceID, deviceName); err != nil {
		return err
	}
	return a.notifyDeviceToConnect(deviceIP, devicePort, deviceID)
}

func (a *App) notifyDeviceToConnect(deviceIP string, devicePort int, deviceID string) error {
	a.localIP = discovery.LocalIP()
	token := make([]byte, 16)
	if _, err := rand.Read(token); err != nil {
		return err
	}
	pairingToken := hex.EncodeToString(token)
	a.wsServer.SetPairingToken(deviceID, pairingToken)
	return discovery.NotifyDevice(deviceIP, devicePort, a.localIP, 9876, a.desktopID, pairingToken)
}

func (a *App) autoReconnectKnownDevices(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			discovered, err := discovery.ScanDevices()
			if err != nil {
				continue
			}
			authorized, err := store.ListAuthorizedDevices(a.db)
			if err != nil {
				continue
			}
			known := map[string]bool{}
			for _, d := range authorized {
				if !d.Revoked {
					known[d.DeviceID] = true
				}
			}
			for _, d := range discovered {
				if known[d.DeviceID] && !a.wsServer.IsDeviceConnected(d.DeviceID) {
					if err := a.notifyDeviceToConnect(d.IP, d.Port, d.DeviceID); err != nil {
						log.Printf("[elf] auto reconnect notify failed for %s: %v", d.DeviceID, err)
					}
				}
			}
		}
	}
}

// AuthorizeDevice is called from the frontend when user approves a device.
func (a *App) AuthorizeDevice(deviceID string) error {
	if err := store.AuthorizeDevice(a.db, deviceID, ""); err != nil {
		return err
	}
	return a.wsServer.AuthorizePendingDevice(deviceID)
}

func (a *App) RevokeDevice(deviceID string) error {
	return store.RevokeDevice(a.db, deviceID)
}

func (a *App) ListAuthorizedDevices() ([]store.AuthorizedDevice, error) {
	return store.ListAuthorizedDevices(a.db)
}

func (a *App) ListPendingDevices() []gateway.PendingDevice {
	return a.wsServer.ListPendingDevices()
}

func (a *App) GetSettings() (map[string]string, error) {
	return store.GetAllSettings(a.db)
}

func (a *App) SetSetting(key, value string) error {
	return store.SetSetting(a.db, key, value)
}

func (a *App) ListAgents() ([]store.Agent, error) {
	return store.ListAgents(a.db)
}

func (a *App) UpdateAgent(agent store.Agent) error {
	return store.UpdateAgent(a.db, &agent)
}

func (a *App) ListSessions(limit, offset int) ([]store.Session, error) {
	return store.ListSessions(a.db, limit, offset)
}

func (a *App) GetSessionMessages(sessionID string) ([]store.Message, error) {
	return store.GetSessionMessages(a.db, sessionID)
}
