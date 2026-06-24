package gateway

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"desktop/internal/store"
	"github.com/gorilla/websocket"
)

// DeviceFactory creates a DeviceAdapter for a new WebSocket connection.
type DeviceFactory func(ws *websocket.Conn) DeviceAdapter

type Server struct {
	port            int
	httpServer      *http.Server
	upgrader        websocket.Upgrader
	deviceFactory   DeviceFactory
	onConnect       func(DeviceAdapter)
	onDisconnect    func(string)
	mu              sync.Mutex
	devices         map[string]DeviceAdapter
	db              *sql.DB
	desktopID       string
	onPendingDevice func(deviceID, deviceName string)
	pendingAuth     map[string]DeviceAdapter
	pendingNames    map[string]string
	pendingAuthMu   sync.Mutex
	pendingTokens   map[string]string
}

type PendingDevice struct {
	DeviceID string `json:"device_id"`
	Name     string `json:"name"`
}

func NewServer(port int, db *sql.DB, desktopID string) *Server {
	return &Server{
		port:          port,
		db:            db,
		desktopID:     desktopID,
		devices:       make(map[string]DeviceAdapter),
		pendingAuth:   make(map[string]DeviceAdapter),
		pendingNames:  make(map[string]string),
		pendingTokens: make(map[string]string),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (s *Server) SetDeviceFactory(f DeviceFactory)                    { s.deviceFactory = f }
func (s *Server) OnDeviceConnect(cb func(DeviceAdapter))              { s.onConnect = cb }
func (s *Server) OnDeviceDisconnect(cb func(string))                   { s.onDisconnect = cb }
func (s *Server) OnPendingDevice(cb func(deviceID, deviceName string)) { s.onPendingDevice = cb }

func (s *Server) SetPairingToken(deviceID, token string) {
	s.pendingAuthMu.Lock()
	defer s.pendingAuthMu.Unlock()
	s.pendingTokens[deviceID] = token
}

func (s *Server) IsDeviceConnected(deviceID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.devices[deviceID]
	return ok
}

func (s *Server) ListPendingDevices() []PendingDevice {
	s.pendingAuthMu.Lock()
	defer s.pendingAuthMu.Unlock()
	devices := make([]PendingDevice, 0, len(s.pendingAuth))
	for id := range s.pendingAuth {
		devices = append(devices, PendingDevice{DeviceID: id, Name: s.pendingNames[id]})
	}
	return devices
}

func (s *Server) AuthorizePendingDevice(deviceID string) error {
	s.pendingAuthMu.Lock()
	dev, ok := s.pendingAuth[deviceID]
	delete(s.pendingAuth, deviceID)
	delete(s.pendingNames, deviceID)
	s.pendingAuthMu.Unlock()
	if !ok {
		return fmt.Errorf("no pending device: %s", deviceID)
	}
	s.mu.Lock()
	s.devices[deviceID] = dev
	s.mu.Unlock()
	go func(id string) {
		<-dev.OnDisconnect()
		s.mu.Lock()
		delete(s.devices, id)
		s.mu.Unlock()
		if s.onDisconnect != nil {
			s.onDisconnect(id)
		}
	}(deviceID)
	if s.onConnect != nil {
		go s.onConnect(dev)
	}
	return nil
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/device", s.handleWS)

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("listen :%d: %w", s.port, err)
	}

	log.Printf("[gateway] WebSocket server listening on :%d", s.port)
	go s.httpServer.Serve(ln)
	return nil
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[gateway] upgrade error: %v", err)
		return
	}
	adapter := s.deviceFactory(ws)

	events, err := adapter.ReceiveEvent()
	if err != nil {
		adapter.Close()
		return
	}

	var deviceID string
	select {
	case evt := <-events:
		if evt.Type != "hello" || evt.Payload["device_id"] == nil {
			adapter.Close()
			return
		}
		deviceID, _ = evt.Payload["device_id"].(string)
		if deviceID == "" {
			adapter.Close()
			return
		}
		deviceName, _ := evt.Payload["name"].(string)
		boundDesktopID, _ := evt.Payload["bound_desktop_id"].(string)
		adapter.SetDeviceID(deviceID)

		if boundDesktopID != "" && boundDesktopID != s.desktopID {
			log.Printf("[gateway] device %s is bound to another desktop %s", deviceID, boundDesktopID)
			adapter.Close()
			return
		}

		token, _ := evt.Payload["pairing_token"].(string)
		s.pendingAuthMu.Lock()
		expectedToken := s.pendingTokens[deviceID]
		if expectedToken != "" {
			if token != expectedToken {
				s.pendingAuthMu.Unlock()
				log.Printf("[gateway] invalid pairing token from %s", deviceID)
				adapter.Close()
				return
			}
			delete(s.pendingTokens, deviceID)
		}
		s.pendingAuthMu.Unlock()

		authorized, err := store.IsDeviceAuthorized(s.db, deviceID)
		if err != nil {
			log.Printf("[gateway] auth check error for %s: %v", deviceID, err)
			adapter.Close()
			return
		}
		if !authorized {
			adapter.SendText(DeviceMessage{Type: "pairing_required", Payload: map[string]interface{}{"device_id": deviceID}})
			s.pendingAuthMu.Lock()
			s.pendingAuth[deviceID] = adapter
			s.pendingNames[deviceID] = deviceName
			s.pendingAuthMu.Unlock()
			go func(id string) {
				<-adapter.OnDisconnect()
				s.pendingAuthMu.Lock()
				delete(s.pendingAuth, id)
				delete(s.pendingNames, id)
				s.pendingAuthMu.Unlock()
			}(deviceID)
			if s.onPendingDevice != nil {
				s.onPendingDevice(deviceID, deviceName)
			}
			return
		}
		remoteIP, _, _ := net.SplitHostPort(r.RemoteAddr)
		if remoteIP != "" {
			_ = store.UpdateDeviceLastSeen(s.db, deviceID, remoteIP)
		}

		s.mu.Lock()
		s.devices[deviceID] = adapter
		s.mu.Unlock()
	case <-time.After(10 * time.Second):
		adapter.Close()
		return
	}

	go func(id string) {
		<-adapter.OnDisconnect()
		s.mu.Lock()
		delete(s.devices, id)
		s.mu.Unlock()
		if s.onDisconnect != nil {
			s.onDisconnect(id)
		}
	}(deviceID)

	if s.onConnect != nil {
		go s.onConnect(adapter)
	}
}

func (s *Server) Stop() {
	s.mu.Lock()
	for id, dev := range s.devices {
		dev.Close()
		delete(s.devices, id)
	}
	s.mu.Unlock()
	if s.httpServer != nil {
		s.httpServer.Close()
	}
	log.Printf("[gateway] server stopped")
}
