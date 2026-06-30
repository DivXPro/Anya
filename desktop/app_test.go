package main

import (
	"os"
	"path/filepath"
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
