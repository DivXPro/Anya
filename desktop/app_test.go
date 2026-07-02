package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"desktop/internal/acp"
	"desktop/internal/acp/adapters"
	"desktop/internal/gateway"
	"desktop/internal/store"
)

func TestAgentCWDValidation(t *testing.T) {
	tmp := t.TempDir()

	// Create a file for the "file not dir" case
	filePath := filepath.Join(tmp, "file")
	if f, err := os.Create(filePath); err == nil {
		f.Close()
	}

	cases := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"empty is valid", "", false},
		{"existing dir", tmp, false},
		{"non-existent", filepath.Join(tmp, "missing"), true},
		{"file not dir", filePath, true},
		{"relative path", "./relative", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateWorkingDirectory(c.path)
			if (err != nil) != c.wantErr {
				t.Fatalf("validateWorkingDirectory(%q) error = %v, wantErr %v", c.path, err, c.wantErr)
			}
		})
	}
}

// The turn guard must prevent two voice requests from running concurrently on
// the same session (which would corrupt the shared ACP session/stream), while
// keeping different sessions independent.
func TestTurnGuardSerializesSameSession(t *testing.T) {
	a := NewApp()

	if !a.tryBeginTurn("s1") {
		t.Fatal("first begin for s1 should succeed")
	}
	if a.tryBeginTurn("s1") {
		t.Fatal("second begin for s1 must fail while a turn is in flight")
	}
	// A different session is unaffected.
	if !a.tryBeginTurn("s2") {
		t.Fatal("begin for an independent session s2 should succeed")
	}
	// After the turn ends, the session can start a new turn again.
	a.endTurn("s1")
	if !a.tryBeginTurn("s1") {
		t.Fatal("begin for s1 should succeed again after endTurn")
	}
}

func TestAppStartupSyncsCWDToRouter(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "elf.db")

	db, err := store.InitDB(dbPath)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	// Store a working directory setting
	if err := store.SetSetting(db, "agent_cwd", "/tmp/test-workspace"); err != nil {
		t.Fatalf("set setting: %v", err)
	}

	// Create app and simulate startup cwd loading
	a := NewApp()
	a.db = db

	// Load agent working directory (same as ServiceStartup)
	if cwd, err := store.GetSetting(a.db, "agent_cwd"); err == nil {
		a.agentCWD = cwd
	} else {
		t.Fatalf("failed to load agent_cwd: %v", err)
	}

	// Init router and register adapters
	a.router = acp.NewRouter()
	a.router.SetCWD(a.agentCWD)
	a.router.Register(adapters.NewClaudeAdapter())

	// Verify the adapter received the cwd via effectiveCWD
	info, ok := a.router.GetAgent("claude-code")
	if !ok {
		t.Fatal("claude adapter not registered")
	}
	if info.ID != "claude-code" {
		t.Fatalf("unexpected agent id: %s", info.ID)
	}

	// Verify the stored cwd was loaded
	if a.agentCWD != "/tmp/test-workspace" {
		t.Fatalf("expected agentCWD '/tmp/test-workspace', got %q", a.agentCWD)
	}
}

// fakeDevice is a minimal DeviceAdapter used to drive handleDeviceEvents in
// tests. It records every message sent to the device.
type fakeDevice struct {
	events     chan gateway.DeviceEvent
	binary     chan []byte
	disconnect chan struct{}
	mu         sync.Mutex
	sent       []gateway.DeviceMessage
}

type fakeSTT struct {
	text string
	err  error
}

func (f fakeSTT) Transcribe([]byte, string) (string, error) {
	return f.text, f.err
}

type fakeTTS struct {
	mu     sync.Mutex
	called int
	texts  []string
}

func (f *fakeTTS) Synthesize(text string) (<-chan []byte, error) {
	f.mu.Lock()
	f.called++
	f.texts = append(f.texts, text)
	f.mu.Unlock()
	ch := make(chan []byte)
	close(ch)
	return ch, nil
}

func (f *fakeTTS) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.called
}

type fakeACPAdapter struct {
	events     []acp.StreamEvent
	sendErr    error
	sessions   []acp.AgentSession
	cwd        string
	loadedID   string
	startedCWD string
}

func (f *fakeACPAdapter) Send(string, []acp.Message) (<-chan acp.StreamEvent, error) {
	if f.sendErr != nil {
		return nil, f.sendErr
	}
	ch := make(chan acp.StreamEvent, len(f.events))
	for _, evt := range f.events {
		ch <- evt
	}
	close(ch)
	return ch, nil
}

func (f *fakeACPAdapter) LoadSession(string, []acp.Message) error { return nil }
func (f *fakeACPAdapter) CurrentSessionID() string                { return "fake-acp-session" }
func (f *fakeACPAdapter) Info() acp.AgentInfo {
	return acp.AgentInfo{ID: "fake-agent", Name: "Fake", Command: "fake"}
}
func (f *fakeACPAdapter) IsRunning() bool { return false }
func (f *fakeACPAdapter) Stop() error     { return nil }
func (f *fakeACPAdapter) SetCWD(cwd string) {
	f.cwd = cwd
}
func (f *fakeACPAdapter) ListAgentSessions(limit int) ([]acp.AgentSession, error) {
	if len(f.sessions) > limit {
		return f.sessions[:limit], nil
	}
	return f.sessions, nil
}
func (f *fakeACPAdapter) LoadAgentSession(id, cwd string) error {
	f.loadedID = id
	f.cwd = cwd
	return nil
}
func (f *fakeACPAdapter) StartNewAgentSession(cwd string) error {
	f.startedCWD = cwd
	f.cwd = cwd
	return nil
}

func newFakeDevice() *fakeDevice {
	return &fakeDevice{
		events:     make(chan gateway.DeviceEvent, 8),
		binary:     make(chan []byte, 8),
		disconnect: make(chan struct{}),
	}
}

func (f *fakeDevice) Info() gateway.DeviceInfo { return gateway.DeviceInfo{ID: "dev1"} }
func (f *fakeDevice) SetDeviceID(string)       {}
func (f *fakeDevice) SetDeviceName(string)     {}
func (f *fakeDevice) SendText(msg gateway.DeviceMessage) error {
	f.mu.Lock()
	f.sent = append(f.sent, msg)
	f.mu.Unlock()
	return nil
}
func (f *fakeDevice) SendBinary([]byte) error                           { return nil }
func (f *fakeDevice) ReceiveEvent() (<-chan gateway.DeviceEvent, error) { return f.events, nil }
func (f *fakeDevice) ReceiveBinary() (<-chan []byte, error)             { return f.binary, nil }
func (f *fakeDevice) OnDisconnect() <-chan struct{}                     { return f.disconnect }
func (f *fakeDevice) Close() error                                      { return nil }

func (f *fakeDevice) sentMessages() []gateway.DeviceMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]gateway.DeviceMessage(nil), f.sent...)
}

func waitForDeviceMessage(t *testing.T, dev *fakeDevice, msgType string) gateway.DeviceMessage {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		for _, msg := range dev.sentMessages() {
			if msg.Type == msgType {
				return msg
			}
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for %s; sent=%+v", msgType, dev.sentMessages())
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestDeviceCanRequestAgentSessionList(t *testing.T) {
	tmp := t.TempDir()
	db, err := store.InitDB(filepath.Join(tmp, "elf.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()
	dialogue, err := store.CreateDialogue(db, "dev1", "fake-agent")
	if err != nil {
		t.Fatalf("create dialogue: %v", err)
	}

	adapter := &fakeACPAdapter{sessions: []acp.AgentSession{
		{ID: "agent-session-1", Title: "First task", CWD: "/tmp/first", Source: "fake", CanResume: true},
	}}
	a := NewApp()
	a.db = db
	a.router = acp.NewRouter()
	a.router.Register(adapter)

	dev := newFakeDevice()
	done := make(chan struct{})
	go func() {
		a.handleDeviceEvents(dev, dialogue.ID)
		close(done)
	}()

	dev.events <- gateway.DeviceEvent{Type: "agent_session_list_req"}
	msg := waitForDeviceMessage(t, dev, "agent_session_list")

	if msg.Payload["agent_id"] != "fake-agent" {
		t.Fatalf("agent_id = %v", msg.Payload["agent_id"])
	}
	items, ok := msg.Payload["sessions"].([]map[string]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("unexpected sessions payload: %#v", msg.Payload["sessions"])
	}
	if items[0]["id"] != "agent-session-1" || items[0]["title"] != "First task" {
		t.Fatalf("unexpected first item: %#v", items[0])
	}

	close(dev.events)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handleDeviceEvents did not exit")
	}
}

func TestDeviceCanSelectExistingAgentSessionAndDialogue(t *testing.T) {
	tmp := t.TempDir()
	cwd := filepath.Join(tmp, "workspace")
	if err := os.MkdirAll(cwd, 0755); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}
	db, err := store.InitDB(filepath.Join(tmp, "elf.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()
	dialogue, err := store.CreateDialogue(db, "dev1", "fake-agent")
	if err != nil {
		t.Fatalf("create dialogue: %v", err)
	}

	adapter := &fakeACPAdapter{}
	a := NewApp()
	a.db = db
	a.router = acp.NewRouter()
	a.router.Register(adapter)

	dev := newFakeDevice()
	done := make(chan struct{})
	go func() {
		a.handleDeviceEvents(dev, dialogue.ID)
		close(done)
	}()

	dev.events <- gateway.DeviceEvent{
		Type: "agent_session_select",
		Payload: map[string]interface{}{
			"id":         "agent-session-1",
			"title":      "Selected task",
			"cwd":        cwd,
			"source":     "fake",
			"can_resume": true,
		},
	}
	msg := waitForDeviceMessage(t, dev, "agent_session_changed")

	if adapter.loadedID != "agent-session-1" || adapter.cwd != cwd {
		t.Fatalf("agent session not loaded: loaded=%q cwd=%q", adapter.loadedID, adapter.cwd)
	}
	if msg.Payload["id"] != "agent-session-1" || msg.Payload["cwd"] != cwd {
		t.Fatalf("unexpected changed payload: %#v", msg.Payload)
	}

	close(dev.events)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handleDeviceEvents did not exit")
	}
}

func TestDeviceCanStartNewAgentSession(t *testing.T) {
	tmp := t.TempDir()
	cwd := filepath.Join(tmp, "workspace")
	if err := os.MkdirAll(cwd, 0755); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}
	db, err := store.InitDB(filepath.Join(tmp, "elf.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()
	dialogue, err := store.CreateDialogue(db, "dev1", "fake-agent")
	if err != nil {
		t.Fatalf("create dialogue: %v", err)
	}

	adapter := &fakeACPAdapter{}
	a := NewApp()
	a.db = db
	a.router = acp.NewRouter()
	a.router.Register(adapter)

	dev := newFakeDevice()
	done := make(chan struct{})
	go func() {
		a.handleDeviceEvents(dev, dialogue.ID)
		close(done)
	}()

	dev.events <- gateway.DeviceEvent{
		Type: "agent_session_select",
		Payload: map[string]interface{}{
			"new": true,
			"cwd": cwd,
		},
	}
	msg := waitForDeviceMessage(t, dev, "agent_session_changed")

	if adapter.startedCWD != cwd || adapter.cwd != cwd {
		t.Fatalf("new agent session not started: started=%q cwd=%q", adapter.startedCWD, adapter.cwd)
	}
	if msg.Payload["new_session"] != true || msg.Payload["cwd"] != cwd {
		t.Fatalf("unexpected changed payload: %#v", msg.Payload)
	}

	close(dev.events)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handleDeviceEvents did not exit")
	}
}

func TestEmptyTranscriptUsesUIStateNotConnectionStatus(t *testing.T) {
	a := NewApp()
	a.stt = fakeSTT{text: "   "}

	dev := newFakeDevice()
	a.processVoiceRequest(dev, "sess-empty", []byte{1, 2, 3})

	msgs := dev.sentMessages()
	var sawProcessingUI, sawIdleUI bool
	for _, msg := range msgs {
		if msg.Type == "ui_state" && msg.State == "processing" {
			sawProcessingUI = true
		}
		if msg.Type == "ui_state" && msg.State == "idle" {
			sawIdleUI = true
		}
		if msg.Type == "status" && msg.State == "connected" {
			t.Fatalf("empty transcript must not report connection state as turn completion: sent=%+v", msgs)
		}
	}
	if !sawProcessingUI || !sawIdleUI {
		t.Fatalf("expected processing and idle ui_state messages, sent=%+v", msgs)
	}
}

func TestAgentRouteErrorReturnsDeviceToIdle(t *testing.T) {
	tmp := t.TempDir()
	db, err := store.InitDB(filepath.Join(tmp, "elf.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	session, err := store.CreateSession(db, "dev1", "fake-agent")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	a := NewApp()
	a.db = db
	a.stt = fakeSTT{text: "你好"}
	a.router = acp.NewRouter()
	a.router.Register(&fakeACPAdapter{sendErr: errors.New("agent binary not found")})

	dev := newFakeDevice()
	a.processVoiceRequest(dev, session.ID, []byte{1, 2, 3})

	msgs := dev.sentMessages()
	var sawFailureSummary bool
	if len(msgs) == 0 {
		t.Fatal("expected messages to be sent")
	}
	if last := msgs[len(msgs)-1]; last.Type != "ui_state" || last.State != "idle" {
		t.Fatalf("agent failure must return the device UI to idle; sent=%+v", msgs)
	}
	for _, msg := range msgs {
		if msg.Type == "summary" && msg.Text == "Agent 调用失败" {
			sawFailureSummary = true
		}
	}
	if !sawFailureSummary {
		t.Fatalf("expected agent failure summary, sent=%+v", msgs)
	}
}

func TestAgentTurnErrorReturnsDeviceToIdle(t *testing.T) {
	tmp := t.TempDir()
	db, err := store.InitDB(filepath.Join(tmp, "elf.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	session, err := store.CreateSession(db, "dev1", "fake-agent")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	a := NewApp()
	a.db = db
	a.stt = fakeSTT{text: "你好"}
	a.router = acp.NewRouter()
	a.router.Register(&fakeACPAdapter{events: []acp.StreamEvent{{
		Type:  "error",
		Error: errors.New("Claude CLI error: test failure"),
	}}})

	dev := newFakeDevice()
	a.processVoiceRequest(dev, session.ID, []byte{1, 2, 3})

	msgs := dev.sentMessages()
	var sawFailureSummary bool
	if len(msgs) == 0 {
		t.Fatal("expected messages to be sent")
	}
	if last := msgs[len(msgs)-1]; last.Type != "ui_state" || last.State != "idle" {
		t.Fatalf("agent turn error must return the device UI to idle; sent=%+v", msgs)
	}
	for _, msg := range msgs {
		if msg.Type == "summary" && msg.Text == "处理出错" {
			sawFailureSummary = true
		}
	}
	if !sawFailureSummary {
		t.Fatalf("expected turn failure summary, sent=%+v", msgs)
	}
}

func TestPingDoesNotChangeDeviceUIState(t *testing.T) {
	a := NewApp()
	dev := newFakeDevice()

	done := make(chan struct{})
	go func() {
		a.handleDeviceEvents(dev, "sess-ping")
		close(done)
	}()

	dev.events <- gateway.DeviceEvent{Type: "ping"}
	close(dev.events)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handleDeviceEvents did not exit after events channel closed")
	}

	for _, msg := range dev.sentMessages() {
		if msg.Type == "ui_state" || msg.Type == "status" {
			t.Fatalf("ping must not change UI or connection state, sent=%+v", dev.sentMessages())
		}
	}
}

func TestEmptyAgentResponseDoesNotSendEmptySummaryOrTTS(t *testing.T) {
	tmp := t.TempDir()
	db, err := store.InitDB(filepath.Join(tmp, "elf.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	session, err := store.CreateSession(db, "dev1", "fake-agent")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	tts := &fakeTTS{}
	a := NewApp()
	a.db = db
	a.stt = fakeSTT{text: "你好"}
	a.tts = tts
	a.router = acp.NewRouter()
	a.router.Register(&fakeACPAdapter{events: []acp.StreamEvent{{Type: "done"}}})

	dev := newFakeDevice()
	a.processVoiceRequest(dev, session.ID, []byte{1, 2, 3})

	for _, msg := range dev.sentMessages() {
		if msg.Type == "summary" && strings.TrimSpace(msg.Text) == "" {
			t.Fatalf("empty agent response must not send an empty summary: sent=%+v", dev.sentMessages())
		}
		if msg.Type == "tts_start" || msg.Type == "tts_end" {
			t.Fatalf("empty agent response must not start TTS: sent=%+v", dev.sentMessages())
		}
	}
	if tts.callCount() != 0 {
		t.Fatalf("empty agent response must not synthesize TTS, calls=%d", tts.callCount())
	}
}

// When a second voice request arrives while a turn is already in flight, the
// desktop must notify the device (so it leaves the SENDING state) instead of
// silently dropping the request and leaving the device waiting.
func TestAudioEndWhileTurnInProgressNotifiesDevice(t *testing.T) {
	a := NewApp()
	const sessionID = "sess-busy"

	// Simulate an in-flight turn for this session.
	if !a.tryBeginTurn(sessionID) {
		t.Fatal("precondition: begin turn should succeed")
	}

	dev := newFakeDevice()
	done := make(chan struct{})
	go func() {
		a.handleDeviceEvents(dev, sessionID)
		close(done)
	}()

	// Device finishes recording and submits a second request.
	dev.events <- gateway.DeviceEvent{Type: "audio_end"}

	// The desktop must send a summary back so the device stops waiting.
	deadline := time.After(2 * time.Second)
	for {
		notified := false
		for _, m := range dev.sentMessages() {
			if m.Type == "summary" && m.Text != "" {
				notified = true
			}
		}
		if notified {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("device was not notified after dropped audio_end; sent=%+v", dev.sentMessages())
		case <-time.After(10 * time.Millisecond):
		}
	}

	// Closing the events channel makes the loop exit cleanly.
	close(dev.events)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handleDeviceEvents did not exit after events channel closed")
	}
}
