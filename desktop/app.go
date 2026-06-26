package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"net"
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
	"github.com/wailsapp/wails/v3/pkg/application"
)

type App struct {
	ctx            context.Context
	db             *sql.DB
	desktopID      string
	localIP        string
	router         *acp.Router
	wsServer       *gateway.Server
	stt            speech.STTEngine
	tts            speech.TTSEngine
	trayDeviceItem *application.MenuItem
	trayAgentMenu  *application.Menu
}

func (a *App) SetTrayDeviceItem(item *application.MenuItem) {
	a.trayDeviceItem = item
}

func (a *App) SetTrayAgentMenu(menu *application.Menu) {
	a.trayAgentMenu = menu
}

func NewApp() *App {
	return &App{}
}

// ServiceStartup is called by Wails v3 when the service is initialized.
func (a *App) ServiceStartup(ctx context.Context, opts application.ServiceOptions) error {
	a.ctx = ctx
	log.Println("[elf] starting up...")

	dataDir := filepath.Join(os.Getenv("HOME"), ".elf")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	// 1. Init store
	db, err := store.InitDB(filepath.Join(dataDir, "elf.db"))
	if err != nil {
		return fmt.Errorf("init db: %w", err)
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
	libDir, modelPath, err := speech.EnsureAssets(dataDir)
	if err != nil {
		return fmt.Errorf("ensure stt assets: %w", err)
	}
	a.stt, err = speech.NewBuckySTT(libDir, modelPath, sttLang)
	if err != nil {
		return fmt.Errorf("init stt: %w", err)
	}

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
		if err := store.AuthorizeDevice(a.db, deviceID, deviceName); err != nil {
			log.Printf("[elf] auto-authorize device %s error: %v", deviceID, err)
			return
		}
		if err := a.wsServer.AuthorizePendingDevice(deviceID); err != nil {
			log.Printf("[elf] authorize pending device %s error: %v", deviceID, err)
		} else {
			log.Printf("[elf] auto-authorized device %s", deviceID)
		}
	})
	if err := a.wsServer.Start(); err != nil {
		return fmt.Errorf("start ws server: %w", err)
	}

	// Populate tray Agent submenu
	a.refreshTrayAgentMenu()

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
	return nil
}

func (a *App) ScanDevices() ([]discovery.DiscoveredDevice, error) {
	return discovery.ScanDevices()
}

// ServiceShutdown is called by Wails v3 when the service is shutting down.
func (a *App) ServiceShutdown() error {
	log.Println("[elf] shutting down...")
	if a.wsServer != nil {
		a.wsServer.Stop()
	}
	if a.db != nil {
		a.db.Close()
	}
	log.Println("[elf] shutdown complete")
	return nil
}

func (a *App) handleDeviceConnect(dev gateway.DeviceAdapter) {
	info := dev.Info()
	log.Printf("[elf] device connected: %s (%s)", info.ID, info.Model)

	// Use the user's alias if one is set, otherwise fall back to the device name.
	displayName := info.Name
	if alias, err := store.GetDeviceAlias(a.db, info.ID); err == nil && alias != "" {
		displayName = alias
	}

	// Update tray menu
	if a.trayDeviceItem != nil {
		if len(displayName) > 16 {
			displayName = displayName[:16]
		}
		a.trayDeviceItem.SetLabel(displayName + "  ● 已连接")
	}

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

	// Update tray menu
	if a.trayDeviceItem != nil {
		a.trayDeviceItem.SetLabel("未连接设备")
	}
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
				// Don't switch to processing immediately; let the device show
				// "Sending..." until we actually start processing.
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
	dev.SendText(gateway.StatusMessage("processing"))

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

// ConnectToDevice notifies a scanned device to open a WebSocket back to the desktop.
// The device is NOT authorized here; authorization happens after the device sends
// a valid hello and the user approves it via AuthorizePendingDevice. This prevents
// the auto-reconnect loop from repeatedly notifying the device while the first
// handshake is still in progress.
func (a *App) ConnectToDevice(deviceIP string, devicePort int, deviceID string, deviceName string) error {
	a.localIP = discovery.LocalIPFor(net.ParseIP(deviceIP))
	log.Printf("[elf] user connecting to device %s (%s) at %s:%d via local %s", deviceName, deviceID, deviceIP, devicePort, a.localIP)
	if a.db == nil {
		log.Printf("[elf] db is nil, cannot connect device")
		return fmt.Errorf("database not initialized")
	}
	return a.notifyDeviceToConnect(deviceIP, devicePort, deviceID)
}

func (a *App) notifyDeviceToConnect(deviceIP string, devicePort int, deviceID string) error {
	log.Printf("[elf] notifyDeviceToConnect enter %s", deviceID)
	a.localIP = discovery.LocalIPFor(net.ParseIP(deviceIP))
	log.Printf("[elf] localIP for %s is %s", deviceIP, a.localIP)
	token := make([]byte, 16)
	log.Printf("[elf] reading random token for %s", deviceID)
	if _, err := rand.Read(token); err != nil {
		log.Printf("[elf] rand.Read failed: %v", err)
		return err
	}
	log.Printf("[elf] token generated for %s", deviceID)
	pairingToken := hex.EncodeToString(token)
	log.Printf("[elf] setting pairing token for %s", deviceID)
	a.wsServer.SetPairingToken(deviceID, pairingToken)
	log.Printf("[elf] notifying device %s to connect back to %s:%d", deviceID, a.localIP, 9876)
	if err := discovery.NotifyDevice(deviceIP, devicePort, a.localIP, 9876, a.desktopID, pairingToken); err != nil {
		log.Printf("[elf] notify device %s failed: %v", deviceID, err)
		return err
	}
	log.Printf("[elf] notify device %s succeeded", deviceID)
	return nil
}

func (a *App) autoReconnectKnownDevices(ctx context.Context) {
	// Only run reconnect scans periodically. A longer interval is fine here
	// because this is just a background self-healing mechanism.
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			authorized, err := store.ListAuthorizedDevices(a.db)
			if err != nil {
				continue
			}
			// Build a list of authorized devices that are currently offline.
			// If every authorized device is already connected, skip scanning
			// entirely to avoid unnecessary mDNS traffic and log noise.
			offline := make([]store.AuthorizedDevice, 0)
			for _, d := range authorized {
				if !d.Revoked && !a.wsServer.IsDeviceConnected(d.DeviceID) {
					offline = append(offline, d)
				}
			}
			if len(offline) == 0 {
				continue
			}
			discovered, err := discovery.ScanDevices()
			if err != nil {
				continue
			}
			offlineIDs := map[string]bool{}
			for _, d := range offline {
				offlineIDs[d.DeviceID] = true
			}
			for _, d := range discovered {
				if offlineIDs[d.DeviceID] {
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

func (a *App) SetDeviceAlias(deviceID, alias string) error {
	return store.SetDeviceAlias(a.db, deviceID, alias)
}

func (a *App) GetDeviceAlias(deviceID string) (string, error) {
	return store.GetDeviceAlias(a.db, deviceID)
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
	if err := store.UpdateAgent(a.db, &agent); err != nil {
		return err
	}
	a.refreshTrayAgentMenu()
	return nil
}

// SelectAgent enables only the chosen agent and disables all others.
func (a *App) SelectAgent(agentID string) error {
	agents, err := store.ListAgents(a.db)
	if err != nil {
		return err
	}
	for _, ag := range agents {
		enabled := ag.ID == agentID
		if ag.Enabled != enabled {
			ag.Enabled = enabled
			if err := store.UpdateAgent(a.db, &ag); err != nil {
				return err
			}
		}
	}
	a.refreshTrayAgentMenu()
	return nil
}

// refreshTrayAgentMenu rebuilds the tray Agent submenu from the database.
func (a *App) refreshTrayAgentMenu() {
	if a.trayAgentMenu == nil || a.db == nil {
		return
	}
	a.trayAgentMenu.Clear()

	agents, err := store.ListAgents(a.db)
	if err != nil {
		log.Printf("[elf] list agents for tray error: %v", err)
		return
	}

	for _, ag := range agents {
		id := ag.ID
		name := ag.Name
		if name == "" {
			name = id
		}
		item := a.trayAgentMenu.AddRadio(name, ag.Enabled)
		item.OnClick(func(_ *application.Context) {
			if err := a.SelectAgent(id); err != nil {
				log.Printf("[elf] select agent %s error: %v", id, err)
			}
		})
	}
	a.trayAgentMenu.Update()
}

func (a *App) ListSessions(limit, offset int) ([]store.Session, error) {
	return store.ListSessions(a.db, limit, offset)
}

func (a *App) GetSessionMessages(sessionID string) ([]store.Message, error) {
	return store.GetSessionMessages(a.db, sessionID)
}
