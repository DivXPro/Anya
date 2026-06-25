package gateway

import (
	"encoding/json"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"desktop/internal/store"
	"github.com/gorilla/websocket"
)

type loopbackAdapter struct {
	conn       *websocket.Conn
	disconnect chan struct{}
	textIn     chan DeviceEvent
}

func newLoopbackAdapter(ws *websocket.Conn) DeviceAdapter {
	a := &loopbackAdapter{
		conn:       ws,
		disconnect: make(chan struct{}),
		textIn:     make(chan DeviceEvent, 64),
	}
	go func() {
		defer close(a.disconnect)
		defer close(a.textIn)
		for {
			mt, data, err := ws.ReadMessage()
			if err != nil {
				return
			}
			if mt == websocket.TextMessage {
				var evt DeviceEvent
				if json.Unmarshal(data, &evt) == nil {
					a.textIn <- evt
				}
			}
		}
	}()
	return a
}

func (a *loopbackAdapter) SetDeviceID(id string) {}
func (a *loopbackAdapter) Info() DeviceInfo       { return DeviceInfo{ID: "loopback", Model: "test"} }
func (a *loopbackAdapter) SendText(msg DeviceMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return a.conn.WriteMessage(websocket.TextMessage, data)
}
func (a *loopbackAdapter) SendBinary(data []byte) error { return nil }
func (a *loopbackAdapter) ReceiveEvent() (<-chan DeviceEvent, error) { return a.textIn, nil }
func (a *loopbackAdapter) ReceiveBinary() (<-chan []byte, error)    { return nil, nil }
func (a *loopbackAdapter) OnDisconnect() <-chan struct{}     { return a.disconnect }
func (a *loopbackAdapter) Close() error                      { return a.conn.Close() }

func TestServerHelloHandshake(t *testing.T) {
	dir := t.TempDir()
	db, err := store.InitDB(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	srv := NewServer(0, db, "desktop-test")
	srv.SetDeviceFactory(func(ws *websocket.Conn) DeviceAdapter {
		return newLoopbackAdapter(ws)
	})

	pendingCalled := make(chan struct{}, 1)
	srv.OnPendingDevice(func(deviceID, deviceName string) {
		pendingCalled <- struct{}{}
	})

	if err := srv.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer srv.Stop()

	time.Sleep(100 * time.Millisecond)

	u := url.URL{Scheme: "ws", Host: srv.Addr(), Path: "/device"}
	ws, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("dial server: %v", err)
	}
	defer ws.Close()

	srv.SetPairingToken("dev-1", "token-123")

	hello := `{"type":"hello","payload":{"device_id":"dev-1","name":"test-dev","pairing_token":"token-123"}}`
	if err := ws.WriteMessage(websocket.TextMessage, []byte(hello)); err != nil {
		t.Fatalf("write hello: %v", err)
	}

	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	mt, data, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("read message: %v", err)
	}
	if mt != websocket.TextMessage {
		t.Fatalf("expected text message, got %d", mt)
	}
	t.Logf("received: %s", string(data))

	select {
	case <-pendingCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("pending device callback not called")
	}
}
