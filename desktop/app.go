package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"desktop/internal/acp"
	"desktop/internal/acp/adapters"
	"desktop/internal/agentinstall"
	"desktop/internal/appupdate"
	logoassets "desktop/internal/assets"
	"desktop/internal/discovery"
	"desktop/internal/firmware"
	"desktop/internal/gateway"
	gatewayadapters "desktop/internal/gateway/adapters"
	"desktop/internal/processor"
	"desktop/internal/speech"
	"desktop/internal/store"
	"desktop/internal/version"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type App struct {
	ctx            context.Context
	db             *sql.DB
	dataDir        string
	desktopID      string
	localIP        string
	router         *acp.Router
	wsServer       *gateway.Server
	stt            speech.STTEngine
	sttMu          sync.RWMutex
	tts            speech.TTSEngine
	trayDeviceItem *application.MenuItem
	trayAgentMenu  *application.Menu
	trayOpenItem   *application.MenuItem
	trayQuitItem   *application.MenuItem
	trayCWDItem    *application.MenuItem
	// menuCheckUpdateItem is the "Check for Updates" item in the macOS menu bar.
	menuCheckUpdateItem *application.MenuItem
	trayDeviceName      string
	trayUILanguage      string
	agentCWD            string
	flashMgr            *firmware.Manager
	otaMgr              *firmware.OTAManager
	agentInstaller      *agentinstall.Installer
	updater             *appupdate.Manager

	confirmMu    sync.Mutex
	confirmWaits map[string]chan string

	// turnMu guards turnActive, which tracks sessions with an in-flight turn so
	// a new voice request can't run concurrently with one already being processed.
	turnMu     sync.Mutex
	turnActive map[string]bool
}

// CurrentVersion returns the running application version (bound to the frontend).
func (a *App) CurrentVersion() string {
	return version.Version
}

// CheckForUpdate queries for a newer release; returns nil when up to date.
func (a *App) CheckForUpdate() (*appupdate.UpdateInfo, error) {
	if a.updater == nil {
		return nil, fmt.Errorf("self-update unavailable in this build")
	}
	return a.updater.CheckForUpdate(a.ctx)
}

// DownloadAndApplyUpdate downloads, verifies, applies the update and relaunches.
// While a download is in flight the "Check for Updates" menu item is disabled so
// the user can't kick off a second check/download; it is re-enabled only if this
// call owned the download and it failed (on success the app relaunches).
func (a *App) DownloadAndApplyUpdate() error {
	if a.updater == nil {
		return fmt.Errorf("self-update unavailable in this build")
	}
	a.setCheckUpdateMenuEnabled(false)
	err := a.updater.DownloadAndApply(a.ctx)
	if err != nil && !errors.Is(err, appupdate.ErrUpdateInProgress) {
		a.setCheckUpdateMenuEnabled(true)
	}
	return err
}

// setCheckUpdateMenuEnabled toggles the macOS menu bar "Check for Updates" item.
func (a *App) setCheckUpdateMenuEnabled(enabled bool) {
	if a.menuCheckUpdateItem != nil {
		a.menuCheckUpdateItem.SetEnabled(enabled)
	}
}

// AvailableUpdate returns the update found by the most recent check (cached, no
// network). The frontend calls this on mount so a late-subscribing window can
// still render the "update available" tag for a background-detected release.
func (a *App) AvailableUpdate() *appupdate.UpdateInfo {
	if a.updater == nil {
		return nil
	}
	return a.updater.Available()
}

// CheckForUpdateInteractive runs a check from the native menu bar and shows a
// native dialog with the result: a confirm-to-update prompt when a newer
// version exists, otherwise an "up to date" / error notice. Run it in a
// goroutine — it performs network I/O.
func (a *App) CheckForUpdateInteractive() {
	en := a.trayUILanguage == "en"
	if a.updater == nil {
		a.showUpdateInfo(a.trayText("updateUnavailableTitle"), a.trayText("updateUnavailableMsg"), true)
		return
	}
	info, err := a.updater.CheckForUpdate(a.ctx)
	if err != nil {
		a.showUpdateInfo(a.trayText("updateCheckFailedTitle"), err.Error(), true)
		return
	}
	if info == nil {
		a.showUpdateInfo(a.trayText("upToDateTitle"), a.trayText("upToDateMsg"), false)
		return
	}

	app := application.Get()
	if app == nil {
		return
	}
	dialog := app.Dialog.Question()
	dialog.SetTitle(a.trayText("updateAvailableTitle"))
	if en {
		dialog.SetMessage(fmt.Sprintf("Version %s is available. Update now?", info.Version))
	} else {
		dialog.SetMessage(fmt.Sprintf("发现新版本 %s，是否立即更新？", info.Version))
	}
	confirm := dialog.AddButton(a.trayText("updateNow"))
	cancel := dialog.AddButton(a.trayText("later"))
	confirm.OnClick(func() {
		go func() {
			if err := a.DownloadAndApplyUpdate(); err != nil {
				log.Printf("[update] interactive download failed: %v", err)
			}
		}()
	})
	cancel.SetAsCancel()
	dialog.SetDefaultButton(confirm)
	dialog.Show()
}

// showUpdateInfo shows a simple native message dialog (info or error styled).
func (a *App) showUpdateInfo(title, message string, isError bool) {
	app := application.Get()
	if app == nil {
		return
	}
	var dialog *application.MessageDialog
	if isError {
		dialog = app.Dialog.Error()
	} else {
		dialog = app.Dialog.Info()
	}
	dialog.SetTitle(title)
	dialog.SetMessage(message)
	dialog.Show()
}

func (a *App) backgroundUpdateCheck() {
	time.Sleep(8 * time.Second) // don't block startup
	for {
		if a.updateAutoCheckEnabled() {
			if _, err := a.updater.CheckForUpdate(a.ctx); err != nil {
				log.Printf("[update] background check failed: %v", err)
			}
		}
		time.Sleep(24 * time.Hour)
	}
}

func (a *App) updateAutoCheckEnabled() bool {
	v, err := store.GetSetting(a.db, "update_auto_check")
	if err != nil {
		return true // default on (key absent)
	}
	return v != "false"
}

// wailsEmitter implements appupdate.Emitter via the Wails event bus.
type wailsEmitter struct{}

func (wailsEmitter) Emit(name string, data any) {
	if app := application.Get(); app != nil && app.Event != nil {
		app.Event.Emit(name, data)
	}
}

func (a *App) SetTrayDeviceItem(item *application.MenuItem) {
	a.trayDeviceItem = item
	a.refreshTrayDeviceStatus()
}

func (a *App) SetTrayAgentMenu(menu *application.Menu) {
	a.trayAgentMenu = menu
	a.refreshTrayAgentMenu()
}

func (a *App) SetTrayOpenItem(item *application.MenuItem) {
	a.trayOpenItem = item
	a.refreshTrayOpenItem()
}

func (a *App) SetTrayQuitItem(item *application.MenuItem) {
	a.trayQuitItem = item
	a.refreshTrayQuitItem()
}

func (a *App) SetTrayCWDItem(item *application.MenuItem) {
	a.trayCWDItem = item
	a.refreshTrayCWD()
}

// SetMenuCheckUpdateItem registers the macOS menu bar "Check for Updates" item
// so its label can follow the UI language.
func (a *App) SetMenuCheckUpdateItem(item *application.MenuItem) {
	a.menuCheckUpdateItem = item
	a.refreshMenuCheckUpdateItem()
}

func (a *App) refreshMenuCheckUpdateItem() {
	if a.menuCheckUpdateItem != nil {
		a.menuCheckUpdateItem.SetLabel(a.trayText("checkUpdate"))
	}
}

func (a *App) trayText(key string) string {
	if a.trayUILanguage == "en" {
		switch key {
		case "noDevice":
			return "No device connected"
		case "connected":
			return "Connected"
		case "open":
			return "Open Anya"
		case "quit":
			return "Quit"
		case "workingDirectory":
			return "Working Directory"
		case "defaultWorkingDirectory":
			return "📁 Default Working Directory"
		case "checkUpdate":
			return "Check for Updates…"
		case "updateAvailableTitle":
			return "Update Available"
		case "updateNow":
			return "Update Now"
		case "later":
			return "Later"
		case "upToDateTitle":
			return "Up to Date"
		case "upToDateMsg":
			return "You're running the latest version."
		case "updateCheckFailedTitle":
			return "Update Check Failed"
		case "updateUnavailableTitle":
			return "Updates Unavailable"
		case "updateUnavailableMsg":
			return "Self-update is not available in this build."
		}
		return key
	}
	switch key {
	case "noDevice":
		return "未连接设备"
	case "connected":
		return "已连接"
	case "open":
		return "打开 Anya"
	case "quit":
		return "退出"
	case "workingDirectory":
		return "工作目录"
	case "defaultWorkingDirectory":
		return "📁 默认工作目录"
	case "checkUpdate":
		return "检查更新…"
	case "updateAvailableTitle":
		return "发现新版本"
	case "updateNow":
		return "立即更新"
	case "later":
		return "稍后"
	case "upToDateTitle":
		return "已是最新版本"
	case "upToDateMsg":
		return "当前已是最新版本。"
	case "updateCheckFailedTitle":
		return "检查更新失败"
	case "updateUnavailableTitle":
		return "暂不支持更新"
	case "updateUnavailableMsg":
		return "此版本不支持自动更新。"
	}
	return key
}

func (a *App) refreshTrayDeviceStatus() {
	if a.trayDeviceItem == nil {
		return
	}
	if a.trayDeviceName == "" {
		a.trayDeviceItem.SetLabel(a.trayText("noDevice"))
		return
	}
	name := a.trayDeviceName
	if len(name) > 16 {
		name = name[:16]
	}
	a.trayDeviceItem.SetLabel(name + "  ● " + a.trayText("connected"))
}

func (a *App) refreshTrayOpenItem() {
	if a.trayOpenItem != nil {
		a.trayOpenItem.SetLabel(a.trayText("open"))
	}
}

func (a *App) refreshTrayQuitItem() {
	if a.trayQuitItem != nil {
		a.trayQuitItem.SetLabel(a.trayText("quit"))
	}
}

func (a *App) refreshTrayLanguage() {
	a.refreshTrayDeviceStatus()
	a.refreshTrayOpenItem()
	a.refreshTrayQuitItem()
	a.refreshTrayCWD()
	a.refreshMenuCheckUpdateItem()
}

func (a *App) refreshTrayCWD() {
	if a.trayCWDItem == nil {
		return
	}
	path := a.agentCWD
	label := a.trayText("workingDirectory")
	if path == "" {
		label = a.trayText("defaultWorkingDirectory")
	} else {
		if utf8.RuneCountInString(path) > 40 {
			runes := []rune(path)
			path = "..." + string(runes[len(runes)-37:])
		}
		label = "📁 " + path
	}
	a.trayCWDItem.SetLabel(label)
	if a.agentCWD != "" {
		a.trayCWDItem.SetTooltip(a.agentCWD)
	}
}

func validateWorkingDirectory(path string) error {
	if path == "" {
		return nil
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("working directory must be an absolute path: %q", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("invalid working directory %q: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("working directory is not a directory: %q", path)
	}
	return nil
}

func NewApp() *App {
	return &App{}
}

// ServiceStartup is called by Wails v3 when the service is initialized.
func (a *App) ServiceStartup(ctx context.Context, opts application.ServiceOptions) error {
	a.ctx = ctx
	log.Println("[elf] starting up...")

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get user home dir: %w", err)
	}
	dataDir := filepath.Join(home, ".elf")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	a.dataDir = dataDir

	// 1. Init store
	db, err := store.InitDB(filepath.Join(dataDir, "elf.db"))
	if err != nil {
		return fmt.Errorf("init db: %w", err)
	}
	a.db = db

	// 1.1 Load agent working directory
	if cwd, err := store.GetSetting(a.db, "agent_cwd"); err == nil {
		a.agentCWD = cwd
	} else {
		log.Printf("[elf] agent_cwd setting not found, using default")
	}

	// 1.5 Generate or load desktop identity
	a.desktopID, _ = store.GetSetting(a.db, "desktop_id")
	if a.desktopID == "" {
		a.desktopID = uuid.NewString()
		store.SetSetting(a.db, "desktop_id", a.desktopID)
	}

	// 2. Init ACP router + register all adapters
	a.router = acp.NewRouter()
	a.router.SetCWD(a.agentCWD)
	a.confirmWaits = make(map[string]chan string)
	a.router.Register(adapters.NewClaudeAdapter())
	a.router.Register(adapters.NewOpenCodeAdapter())
	a.router.Register(adapters.NewKimiAdapter())
	a.router.Register(adapters.NewPiAdapter())
	a.router.Register(adapters.NewHermesAdapter())
	a.router.Register(adapters.NewCodexAdapter())

	// 2.0 Init firmware flash manager
	a.flashMgr = firmware.NewManager()
	a.otaMgr = firmware.NewOTAManager()

	// 2.1 Detect which agent CLIs are installed and select the first available one.
	a.agentInstaller = agentinstall.New(application.Get(), a.db)
	if verifier, err := appupdate.DefaultVerifier(); err != nil {
		log.Printf("[update] self-update disabled: %v", err)
	} else {
		a.updater = appupdate.NewManager(
			version.Version,
			appupdate.NewChecker(version.RepoOwner, version.RepoName),
			verifier,
			appupdate.NewApplier(),
			wailsEmitter{},
		)
		go a.backgroundUpdateCheck()
	}
	if err := a.agentInstaller.DetectAll(); err != nil {
		log.Printf("[elf] detect agents error: %v", err)
	}
	if err := a.refreshAgentInstallStatus(); err != nil {
		log.Printf("[elf] refresh agent install status error: %v", err)
	}

	// 3. Init TTS immediately; load STT assets in the background so the UI
	// can open before the large whisper model finishes downloading.
	ttsVoice := "zh-CN-XiaoxiaoNeural"
	ttsSpeed, _ := store.GetSetting(a.db, "tts_speed")
	if ttsSpeed == "" {
		ttsSpeed = "+0%"
	}
	a.tts = speech.NewEdgeTTS(ttsVoice, ttsSpeed)

	sttLang, _ := store.GetSetting(a.db, "stt_language")
	if sttLang == "" {
		sttLang = "zh"
	}
	go a.loadSTTAssets(dataDir, sttLang)

	uiLang, _ := store.GetSetting(a.db, "ui_language")
	if uiLang == "" {
		uiLang = "zh"
	}
	a.trayUILanguage = uiLang

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

// refreshAgentInstallStatus keeps the currently selected agent if it is still
// installed, otherwise it selects the first installed one. If no agent is
// installed, the current selection is cleared.
func (a *App) refreshAgentInstallStatus() error {
	agents, err := store.ListAgents(a.db)
	if err != nil {
		return fmt.Errorf("list agents: %w", err)
	}

	var currentSelected, firstInstalled string
	selectedInstalled := false
	for _, ag := range agents {
		if ag.Selected {
			currentSelected = ag.ID
			selectedInstalled = ag.Installed
		}
		if ag.Installed && firstInstalled == "" {
			firstInstalled = ag.ID
		}
	}

	switch {
	case currentSelected != "" && selectedInstalled:
		log.Printf("[elf] keeping selected agent: %s", currentSelected)
	case firstInstalled != "":
		if err := a.SelectAgent(firstInstalled); err != nil {
			return fmt.Errorf("select first installed agent %s: %w", firstInstalled, err)
		}
		log.Printf("[elf] active agent: %s", firstInstalled)
	default:
		if err := store.ClearAgentSelection(a.db); err != nil {
			return fmt.Errorf("clear agent selection: %w", err)
		}
		log.Println("[elf] no agent installed; selection cleared")
	}

	a.refreshTrayAgentMenu()
	return nil
}

// loadSTTAssets downloads the whisper.cpp library and model if needed, then
// initializes the STT engine. It runs in the background so application startup
// is not blocked by large downloads.
func (a *App) loadSTTAssets(dataDir, lang string) {
	log.Println("[elf] loading STT assets in background...")
	libDir, modelPath, err := speech.EnsureAssets(dataDir)
	if err != nil {
		log.Printf("[elf] ensure stt assets error: %v", err)
		return
	}
	stt, err := speech.NewBuckySTT(libDir, modelPath, lang)
	if err != nil {
		log.Printf("[elf] init stt error: %v", err)
		return
	}
	a.setSTT(stt)
	log.Println("[elf] STT ready")
}

// reloadSTT re-initializes the STT engine with a new language without
// restarting the application.
func (a *App) reloadSTT(lang string) {
	a.loadSTTAssets(a.dataDir, lang)
}

func (a *App) setSTT(stt speech.STTEngine) {
	a.sttMu.Lock()
	defer a.sttMu.Unlock()
	a.stt = stt
}

// sttReady returns true if the STT engine has finished initializing.
func (a *App) sttReady() bool {
	a.sttMu.RLock()
	defer a.sttMu.RUnlock()
	return a.stt != nil
}

// STTReady exposes the STT initialization status to the frontend.
func (a *App) STTReady() bool {
	return a.sttReady()
}

// GetSTTDownloadProgress exposes the local whisper model download progress.
func (a *App) GetSTTDownloadProgress() speech.DownloadProgress {
	return speech.GetDownloadProgress()
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
	if len(displayName) > 16 {
		displayName = displayName[:16]
	}
	a.trayDeviceName = displayName
	a.refreshTrayDeviceStatus()

	agents, err := store.ListAgents(a.db)
	if err != nil || len(agents) == 0 {
		log.Printf("[elf] no agents configured")
		return
	}
	agentID := ""
	for _, ag := range agents {
		if ag.Selected && ag.Installed {
			agentID = ag.ID
			break
		}
	}
	if agentID == "" {
		for _, ag := range agents {
			if ag.Installed {
				agentID = ag.ID
				break
			}
		}
	}
	if agentID == "" {
		agentID = agents[0].ID
	}

	session, err := a.recoverOrCreateSession(info.ID, agentID)
	if err != nil {
		log.Printf("[elf] session setup error: %v", err)
		return
	}

	dev.SendText(gateway.WelcomeMessage(info.ID, agentID, session.ID, a.desktopID))

	go a.handleDeviceEvents(dev, session.ID)
}

// recoverOrCreateSession looks for an existing open session for the device.
// If one exists and the agent hasn't changed, it attempts to load the ACP
// session so the conversation can continue. Otherwise it creates a new session.
func (a *App) recoverOrCreateSession(deviceID, agentID string) (*store.Session, error) {
	existing, err := store.GetOpenSessionForDevice(a.db, deviceID)
	if err != nil {
		return nil, fmt.Errorf("lookup open session: %w", err)
	}
	if existing != nil && existing.AgentID == agentID && existing.ACPSessionID != nil && *existing.ACPSessionID != "" {
		history, err := store.GetSessionMessages(a.db, existing.ID)
		if err != nil {
			log.Printf("[elf] failed to load history for recovery: %v", err)
			history = nil
		}
		acpHistory := make([]acp.Message, len(history))
		for i, m := range history {
			acpHistory[i] = acp.Message{Role: m.Role, Content: m.Content}
		}
		if err := a.router.LoadSession(agentID, *existing.ACPSessionID, acpHistory); err != nil {
			log.Printf("[elf] failed to load acp session %s, creating new session: %v", *existing.ACPSessionID, err)
			// Fall through to create a new session.
		} else {
			log.Printf("[elf] recovered session %s (acp=%s)", existing.ID, *existing.ACPSessionID)
			return existing, nil
		}
	}

	session, err := store.CreateSession(a.db, deviceID, agentID)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return session, nil
}

func (a *App) handleDeviceDisconnect(deviceID string) {
	log.Printf("[elf] device disconnected: %s", deviceID)

	a.otaMgr.DeviceDisconnected(deviceID)

	// Update tray menu
	a.trayDeviceName = ""
	a.refreshTrayDeviceStatus()
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
			if strings.HasPrefix(evt.Type, "firmware_") {
				a.otaMgr.HandleEvent(dev.Info().ID, &evt)
				continue
			}
			switch evt.Type {
			case "audio_start":
				audioBuffer = nil
				dev.SendText(gateway.StatusMessage("listening"))
			case "audio_end":
				// Don't switch to processing immediately; let the device show
				// "Sending..." until we actually start processing. Guard against
				// a second turn starting while one is still in flight (which would
				// corrupt the shared ACP session/stream).
				if !a.tryBeginTurn(sessionID) {
					log.Printf("[elf] ignoring audio_end: a turn is already in progress for session %s", sessionID)
					// Signal the device that the request was dropped so it leaves
					// the SENDING state and stops waiting, instead of relying on
					// its turn watchdog to time out ~30s later.
					dev.SendText(gateway.SummaryMessage("正在处理上一条请求，请稍候"))
					continue
				}
				go func(buf []byte) {
					defer a.endTurn(sessionID)
					a.processVoiceRequest(dev, sessionID, buf)
				}(audioBuffer)
			case "button":
				log.Printf("[elf] button: %s", evt.Action)
			case "confirm_response":
				a.handleConfirmResponse(evt.Payload)
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

func (a *App) handleConfirmResponse(payload map[string]interface{}) {
	reqID, _ := payload["request_id"].(string)
	optionID, _ := payload["option_id"].(string)
	if reqID == "" {
		return
	}
	a.confirmMu.Lock()
	ch := a.confirmWaits[reqID]
	a.confirmMu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- optionID:
	default:
	}
}

func (a *App) handlePermissionRequests(ctx context.Context, dev gateway.DeviceAdapter, responder acp.PermissionResponder) {
	for {
		select {
		case req, ok := <-responder.PermissionRequests():
			if !ok {
				return
			}
			go a.runConfirm(ctx, dev, responder, req)
		case <-ctx.Done():
			return
		}
	}
}

func (a *App) runConfirm(ctx context.Context, dev gateway.DeviceAdapter, responder acp.PermissionResponder, req acp.PermissionRequest) {
	opts := make([]gateway.ConfirmOption, len(req.Options))
	for i, o := range req.Options {
		opts[i] = gateway.ConfirmOption{ID: o.ID, Label: o.Label}
	}
	if err := dev.SendText(gateway.ConfirmMessage(req.ID, req.Prompt, opts)); err != nil {
		log.Printf("[elf] send confirm message error: %v", err)
		_ = responder.RespondPermission(req.ID, "")
		return
	}

	ch := make(chan string, 1)
	a.confirmMu.Lock()
	a.confirmWaits[req.ID] = ch
	a.confirmMu.Unlock()
	defer func() {
		a.confirmMu.Lock()
		delete(a.confirmWaits, req.ID)
		a.confirmMu.Unlock()
	}()

	const confirmTimeout = 5 * time.Minute
	select {
	case optionID := <-ch:
		if err := responder.RespondPermission(req.ID, optionID); err != nil {
			log.Printf("[elf] respond permission error: %v", err)
		}
	case <-ctx.Done():
		log.Printf("[elf] confirmation cancelled because turn ended: %s", req.ID)
		_ = dev.SendText(gateway.ConfirmCancelMessage(req.ID))
		_ = responder.RespondPermission(req.ID, "")
	case <-time.After(confirmTimeout):
		log.Printf("[elf] confirmation timed out: %s", req.ID)
		_ = dev.SendText(gateway.ConfirmCancelMessage(req.ID))
		_ = responder.RespondPermission(req.ID, "")
	}
}

// tryBeginTurn marks the session as having an in-flight turn. It returns false
// if a turn is already running for that session, so callers can drop the new
// request instead of processing two turns concurrently on the same ACP session.
func (a *App) tryBeginTurn(sessionID string) bool {
	a.turnMu.Lock()
	defer a.turnMu.Unlock()
	if a.turnActive == nil {
		a.turnActive = make(map[string]bool)
	}
	if a.turnActive[sessionID] {
		return false
	}
	a.turnActive[sessionID] = true
	return true
}

// endTurn clears the in-flight marker for the session.
func (a *App) endTurn(sessionID string) {
	a.turnMu.Lock()
	delete(a.turnActive, sessionID)
	a.turnMu.Unlock()
}

func (a *App) processVoiceRequest(dev gateway.DeviceAdapter, sessionID string, audioData []byte) {
	dev.SendText(gateway.StatusMessage("processing"))

	// 1. STT
	if !a.sttReady() {
		dev.SendText(gateway.SummaryMessage("语音模型加载中，请稍候"))
		dev.SendText(gateway.StatusMessage("connected"))
		return
	}

	a.sttMu.RLock()
	stt := a.stt
	a.sttMu.RUnlock()

	text, err := stt.Transcribe(audioData, "pcm")
	if err != nil {
		dev.SendText(gateway.SummaryMessage("语音识别失败"))
		return
	}
	log.Printf("[elf] STT: %s", text)

	if strings.TrimSpace(text) == "" {
		dev.SendText(gateway.StatusMessage("connected"))
		dev.SendText(gateway.SummaryMessage("没听清，请再说一次"))
		return
	}

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

	// Watch for ACP permission requests during this turn and surface them on the device.
	if adapter, ok := a.router.GetAdapter(session.AgentID); ok {
		if responder, ok := adapter.(acp.PermissionResponder); ok {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go a.handlePermissionRequests(ctx, dev, responder)
		}
	}

	// Persist the ACP session ID so the conversation can be recovered later.
	if session.ACPSessionID == nil || *session.ACPSessionID == "" {
		if acpSessionID, err := a.router.CurrentSessionID(session.AgentID); err == nil && acpSessionID != "" {
			if err := store.UpdateSessionACPSession(a.db, sessionID, acpSessionID, session.AgentID); err != nil {
				log.Printf("[elf] failed to save acp session id: %v", err)
			}
		}
	}

	// 5. Process response with timeout monitoring
	pipeline := processor.NewPipeline(events)
	pipeline.SetHeartbeatCallback(func() {
		dev.SendText(gateway.StatusMessage("processing"))
	})
	resp, result, err := pipeline.Process()
	switch result {
	case processor.ResultNoResponseTimeout:
		dev.SendText(gateway.SummaryMessage("Agent 超时无响应，已中断"))
		return
	case processor.ResultExecTimeout:
		dev.SendText(gateway.SummaryMessage("Agent 执行超时，已中断"))
		return
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

	// 7. Send the full reply text to the device for on-screen display.
	dev.SendText(gateway.SummaryMessage(resp.Content))

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

func (a *App) ListConnectedDeviceIDs() []string {
	if a.wsServer == nil {
		return nil
	}
	return a.wsServer.ConnectedDeviceIDs()
}

func (a *App) GetSettings() (map[string]string, error) {
	return store.GetAllSettings(a.db)
}

func (a *App) SetSetting(key, value string) error {
	if key == "agent_cwd" {
		if value != "" {
			if err := validateWorkingDirectory(value); err != nil {
				return err
			}
		}
		if err := store.SetSetting(a.db, key, value); err != nil {
			return err
		}
		a.agentCWD = value
		a.refreshTrayCWD()
		if a.router != nil {
			a.router.SetCWD(value)
		}
		return nil
	}
	if key == "stt_language" {
		old, _ := store.GetSetting(a.db, key)
		if old != value {
			if err := store.SetSetting(a.db, key, value); err != nil {
				return err
			}
			log.Printf("[elf] stt language changed from %s to %s, reloading STT...", old, value)
			go a.reloadSTT(value)
			return nil
		}
	}
	if key == "ui_language" {
		old, _ := store.GetSetting(a.db, key)
		if old != value {
			if err := store.SetSetting(a.db, key, value); err != nil {
				return err
			}
			a.trayUILanguage = value
			a.refreshTrayLanguage()
			return nil
		}
	}
	return store.SetSetting(a.db, key, value)
}

func (a *App) ListAgents() ([]store.Agent, error) {
	return store.ListAgents(a.db)
}

// DetectAgents rescans the filesystem for installed agent CLIs and updates the
// database. It returns immediately and performs the work in the background.
func (a *App) DetectAgents() error {
	if a.agentInstaller == nil {
		return fmt.Errorf("agent installer not initialized")
	}
	go func() {
		if err := a.agentInstaller.DetectAll(); err != nil {
			log.Printf("[elf] detect agents error: %v", err)
			return
		}
		if err := a.refreshAgentInstallStatus(); err != nil {
			log.Printf("[elf] refresh agent install status error: %v", err)
		}
	}()
	return nil
}

// InstallAgent starts an asynchronous install of the requested agent.
func (a *App) InstallAgent(agentID string) error {
	if a.agentInstaller == nil {
		return fmt.Errorf("agent installer not initialized")
	}
	return a.agentInstaller.Install(agentID)
}

// IsAgentInstalling reports whether the agent is currently being installed.
func (a *App) IsAgentInstalling(agentID string) bool {
	if a.agentInstaller == nil {
		return false
	}
	return a.agentInstaller.IsInstalling(agentID)
}

// GetPackageManager returns the first available npm-compatible package manager,
// or an empty string if none is found.
func (a *App) GetPackageManager() string {
	pm, _ := agentinstall.DetectPackageManager()
	return pm
}

// GetAgentInstallCommand returns the install command currently stored for an agent,
// or the best platform-specific fallback if nothing has been stored yet.
func (a *App) GetAgentInstallCommand(agentID string) string {
	ag, err := store.GetAgent(a.db, agentID)
	if err != nil {
		return ""
	}
	if ag.InstallCommand != nil && *ag.InstallCommand != "" {
		return *ag.InstallCommand
	}
	_, display, err := agentinstall.PlatformInstallCommand(agentID)
	if err != nil {
		return ""
	}
	return display
}

func (a *App) ListSerialPorts() ([]firmware.SerialPortInfo, error) {
	return firmware.ListSerialPorts()
}

func (a *App) FlashFirmware(port string) error {
	if a.flashMgr == nil {
		return fmt.Errorf("firmware manager not initialized")
	}
	if a.flashMgr.IsRunning() {
		return fmt.Errorf("flash already in progress")
	}
	go func() {
		if err := a.flashMgr.Flash(port); err != nil {
			log.Printf("[elf] flash firmware to %s error: %v", port, err)
		}
	}()
	return nil
}

func (a *App) GetFlashProgress() firmware.FlashProgress {
	if a.flashMgr == nil {
		return firmware.FlashProgress{Stage: firmware.StageIdle}
	}
	return a.flashMgr.Progress()
}

func (a *App) CancelFlash() error {
	if a.flashMgr == nil {
		return nil
	}
	return a.flashMgr.Cancel()
}

func (a *App) HasEmbeddedFirmware() bool {
	return firmware.HasEmbeddedFirmware()
}

func (a *App) CurrentFirmwareVersion() string {
	return firmware.EmbeddedFirmwareVersion()
}

func (a *App) FindEsptool() (string, error) {
	return firmware.FindEsptool()
}

// ReadDeviceFirmwareVersion listens on the given serial port for the device's
// startup banner and returns the firmware version reported by the device.
func (a *App) ReadDeviceFirmwareVersion(port string) (string, error) {
	return firmware.ReadDeviceFirmwareVersion(port, 3*time.Second)
}

func (a *App) CheckDeviceFirmwareVersion(deviceID string) error {
	if a.wsServer == nil {
		return fmt.Errorf("server not initialized")
	}
	dev := a.wsServer.GetDevice(deviceID)
	if dev == nil {
		return fmt.Errorf("device not connected: %s", deviceID)
	}
	return a.otaMgr.CheckVersion(deviceID, dev)
}

func (a *App) StartOTAUpdate(deviceID string) error {
	if a.wsServer == nil {
		return fmt.Errorf("server not initialized")
	}
	dev := a.wsServer.GetDevice(deviceID)
	if dev == nil {
		return fmt.Errorf("device not connected: %s", deviceID)
	}
	if !firmware.HasEmbeddedFirmware() {
		return fmt.Errorf("no firmware binary embedded")
	}
	return a.otaMgr.StartUpdate(deviceID, dev, firmware.EmbeddedFirmware(), firmware.EmbeddedFirmwareVersion())
}

func (a *App) CancelOTAUpdate(deviceID string) error {
	if a.wsServer == nil {
		return fmt.Errorf("server not initialized")
	}
	dev := a.wsServer.GetDevice(deviceID)
	return a.otaMgr.Cancel(deviceID, dev)
}

func (a *App) GetOTAProgress(deviceID string) firmware.OTAProgress {
	return a.otaMgr.Progress(deviceID)
}

func (a *App) GetDeviceFirmwareVersion(deviceID string) string {
	return a.otaMgr.DeviceVersion(deviceID)
}

func (a *App) UpdateAgent(agent store.Agent) error {
	if err := store.UpdateAgent(a.db, &agent); err != nil {
		return err
	}
	a.refreshTrayAgentMenu()
	return nil
}

// SelectAgent marks the chosen agent as selected and clears selection from all others.
// Only installed agents can be selected.
func (a *App) SelectAgent(agentID string) error {
	agents, err := store.ListAgents(a.db)
	if err != nil {
		return err
	}
	for _, ag := range agents {
		if ag.ID == agentID && !ag.Installed {
			return fmt.Errorf("agent %s is not installed", agentID)
		}
		selected := ag.ID == agentID
		if ag.Selected != selected {
			if err := store.UpdateAgentSelected(a.db, ag.ID, selected); err != nil {
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
		item := a.trayAgentMenu.AddRadio(name, ag.Selected)
		if logo, err := logoassets.AgentLogo(id); err != nil {
			log.Printf("[elf] load agent logo %s: %v", id, err)
		} else if logo != nil {
			item.SetBitmap(logo)
		}
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

func (a *App) ListMessages(limit, offset int) ([]store.Message, error) {
	return store.ListMessages(a.db, limit, offset)
}

func (a *App) SearchMessages(query string, limit int) ([]store.Message, error) {
	return store.SearchMessages(a.db, query, limit)
}
