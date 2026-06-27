package firmware

import (
	"encoding/json"
	"sync"
	"testing"

	"desktop/internal/gateway"
)

type mockDevice struct {
	mu        sync.Mutex
	texts     []gateway.DeviceMessage
	binaries  [][]byte
	info      gateway.DeviceInfo
	disconnect chan struct{}
}

func newMockDevice(id string) *mockDevice {
	return &mockDevice{
		info: gateway.DeviceInfo{ID: id, Name: "test"},
		disconnect: make(chan struct{}),
	}
}

func (m *mockDevice) Info() gateway.DeviceInfo { return m.info }
func (m *mockDevice) SetDeviceID(id string)    { m.info.ID = id }
func (m *mockDevice) SetDeviceName(name string) {
	if name != "" {
		m.info.Name = name
	}
}
func (m *mockDevice) SendText(msg gateway.DeviceMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.texts = append(m.texts, msg)
	return nil
}
func (m *mockDevice) SendBinary(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	buf := make([]byte, len(data))
	copy(buf, data)
	m.binaries = append(m.binaries, buf)
	return nil
}
func (m *mockDevice) ReceiveEvent() (<-chan gateway.DeviceEvent, error) { return nil, nil }
func (m *mockDevice) ReceiveBinary() (<-chan []byte, error)             { return nil, nil }
func (m *mockDevice) OnDisconnect() <-chan struct{}                     { return m.disconnect }
func (m *mockDevice) Close() error                                      { return nil }

func (m *mockDevice) lastText() gateway.DeviceMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.texts) == 0 {
		return gateway.DeviceMessage{}
	}
	return m.texts[len(m.texts)-1]
}

func TestOTACheckVersion(t *testing.T) {
	mgr := NewOTAManager()
	dev := newMockDevice("dev1")

	if err := mgr.CheckVersion("dev1", dev); err != nil {
		t.Fatalf("CheckVersion: %v", err)
	}

	last := dev.lastText()
	if last.Type != "firmware_version_req" {
		t.Errorf("expected firmware_version_req, got %s", last.Type)
	}
}

func TestOTAStartUpdate(t *testing.T) {
	mgr := NewOTAManager()
	dev := newMockDevice("dev1")
	fw := []byte("firmware payload")

	if err := mgr.StartUpdate("dev1", dev, fw, "1.0.0"); err != nil {
		t.Fatalf("StartUpdate: %v", err)
	}

	last := dev.lastText()
	if last.Type != "firmware_update" {
		t.Fatalf("expected firmware_update, got %s", last.Type)
	}
	if last.Payload["version"] != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %v", last.Payload["version"])
	}
	if last.Payload["size"] != len(fw) {
		t.Errorf("expected size %d, got %v", len(fw), last.Payload["size"])
	}
}

func TestOTAHandleVersion(t *testing.T) {
	mgr := NewOTAManager()
	mgr.HandleEvent("dev1", &gateway.DeviceEvent{
		Type:    "firmware_version",
		Payload: map[string]interface{}{"version": "0.9.0"},
	})

	if got := mgr.DeviceVersion("dev1"); got != "0.9.0" {
		t.Errorf("expected device version 0.9.0, got %s", got)
	}
	p := mgr.Progress("dev1")
	if p.DeviceVersion != "0.9.0" {
		t.Errorf("expected progress device version 0.9.0, got %s", p.DeviceVersion)
	}
}

func TestOTAProgressEvent(t *testing.T) {
	mgr := NewOTAManager()
	dev := newMockDevice("dev1")
	if err := mgr.StartUpdate("dev1", dev, []byte("firmware payload"), "1.0.0"); err != nil {
		t.Fatalf("StartUpdate: %v", err)
	}
	mgr.HandleEvent("dev1", &gateway.DeviceEvent{
		Type: "firmware_progress",
		Payload: map[string]interface{}{
			"sent":    500.0,
			"total":   1000.0,
			"percent": 50.0,
		},
	})

	p := mgr.Progress("dev1")
	if p.Percent != 50 {
		t.Errorf("expected percent 50, got %d", p.Percent)
	}
}

func TestOTAEventPayloadJSON(t *testing.T) {
	// Sanity check that the gateway event payload matches what the firmware sends.
	raw := `{"type":"firmware_update_ack","payload":{"seq":0}}`
	var evt gateway.DeviceEvent
	if err := json.Unmarshal([]byte(raw), &evt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if evt.Type != "firmware_update_ack" {
		t.Errorf("unexpected type: %s", evt.Type)
	}
}
