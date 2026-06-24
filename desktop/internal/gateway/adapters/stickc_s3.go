package adapters

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"

	"desktop/internal/gateway"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type StickCS3Adapter struct {
	conn       *websocket.Conn
	info       gateway.DeviceInfo
	deviceID   string
	disconnect chan struct{}
	textIn     chan gateway.DeviceEvent
	binaryIn   chan []byte
	mu         sync.Mutex
	closed     bool
}

func (a *StickCS3Adapter) SetDeviceID(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.deviceID = id
	a.info.ID = id
}

func NewStickCS3Adapter(ws *websocket.Conn) gateway.DeviceAdapter {
	a := &StickCS3Adapter{
		conn: ws,
		info: gateway.DeviceInfo{
			ID:           uuid.NewString(),
			Name:         fmt.Sprintf("elf-stick-%s", uuid.NewString()[:8]),
			Model:        "m5stickc-s3",
			Capabilities: []string{"audio_input", "audio_output", "display", "buttons"},
		},
		disconnect: make(chan struct{}, 1),
		textIn:     make(chan gateway.DeviceEvent, 64),
		binaryIn:   make(chan []byte, 64),
	}
	go a.readLoop()
	return a
}

func (a *StickCS3Adapter) readLoop() {
	defer func() {
		a.mu.Lock()
		a.closed = true
		a.mu.Unlock()
		close(a.disconnect)
		close(a.textIn)
		close(a.binaryIn)
	}()

	for {
		msgType, data, err := a.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("[stickc] read error: %v", err)
			}
			return
		}

		switch msgType {
		case websocket.TextMessage:
			var evt gateway.DeviceEvent
			if err := json.Unmarshal(data, &evt); err != nil {
				log.Printf("[stickc] parse error: %v", err)
				continue
			}
			a.textIn <- evt

		case websocket.BinaryMessage:
			buf := make([]byte, len(data))
			copy(buf, data)
			a.binaryIn <- buf
		}
	}
}

func (a *StickCS3Adapter) Info() gateway.DeviceInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.info
}

func (a *StickCS3Adapter) SendText(msg gateway.DeviceMessage) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return net.ErrClosed
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return a.conn.WriteMessage(websocket.TextMessage, data)
}

func (a *StickCS3Adapter) SendBinary(data []byte) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return net.ErrClosed
	}
	return a.conn.WriteMessage(websocket.BinaryMessage, data)
}

func (a *StickCS3Adapter) ReceiveEvent() (<-chan gateway.DeviceEvent, error) {
	return a.textIn, nil
}

func (a *StickCS3Adapter) ReceiveBinary() (<-chan []byte, error) {
	return a.binaryIn, nil
}

func (a *StickCS3Adapter) OnDisconnect() <-chan struct{} {
	return a.disconnect
}

func (a *StickCS3Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closed = true
	return a.conn.Close()
}
