package firmware

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	"desktop/internal/gateway"
)

// DefaultOTAChunkSize is the firmware chunk size sent over WebSocket.
const DefaultOTAChunkSize = 4096

// OTAManager handles wireless firmware updates for connected devices.
type OTAManager struct {
	mu             sync.Mutex
	states         map[string]*otaState
	deviceVersions map[string]string
}

type otaState struct {
	deviceID    string
	dev         gateway.DeviceAdapter
	running     bool
	cancel      context.CancelFunc
	ctx         context.Context
	firmware    []byte
	version     string
	total       int
	chunkSize   int
	totalChunks int
	ackedChunks int
	startCh     chan struct{}
	ackCh       chan int
	doneCh      chan struct{}
	doneOnce    sync.Once
	progress    OTAProgress
}

// NewOTAManager creates an OTA manager with an idle state.
func NewOTAManager() *OTAManager {
	return &OTAManager{
		states:         make(map[string]*otaState),
		deviceVersions: make(map[string]string),
	}
}

// CheckVersion sends a version request to the device.
func (m *OTAManager) CheckVersion(deviceID string, dev gateway.DeviceAdapter) error {
	m.mu.Lock()
	if s, ok := m.states[deviceID]; ok && s.running {
		m.mu.Unlock()
		return fmt.Errorf("ota already in progress for %s", deviceID)
	}
	m.setProgressLocked(deviceID, OTAProgress{
		Running: true,
		Stage:   string(OTAStageChecking),
		Message: "checking device version",
	})
	m.mu.Unlock()

	if err := dev.SendText(gateway.FirmwareVersionReqMessage()); err != nil {
		m.setProgress(deviceID, OTAProgress{
			Stage:   string(OTAStageError),
			Message: "failed to request version",
			Error:   err.Error(),
		})
		return err
	}
	return nil
}

// StartUpdate begins streaming the embedded firmware to the device over WebSocket.
func (m *OTAManager) StartUpdate(deviceID string, dev gateway.DeviceAdapter, firmware []byte, version string) error {
	if len(firmware) == 0 {
		return fmt.Errorf("no firmware binary available")
	}

	m.mu.Lock()
	if s, ok := m.states[deviceID]; ok && s.running {
		m.mu.Unlock()
		return fmt.Errorf("ota already in progress for %s", deviceID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	s := &otaState{
		deviceID:    deviceID,
		dev:         dev,
		running:     true,
		cancel:      cancel,
		ctx:         ctx,
		firmware:    firmware,
		version:     version,
		total:       len(firmware),
		chunkSize:   DefaultOTAChunkSize,
		totalChunks: (len(firmware) + DefaultOTAChunkSize - 1) / DefaultOTAChunkSize,
		startCh:     make(chan struct{}),
		ackCh:       make(chan int, 4),
		doneCh:      make(chan struct{}),
		progress: OTAProgress{
			Running: true,
			Stage:   string(OTAStageAwaiting),
			Message: "waiting for device to accept update",
		},
	}
	m.states[deviceID] = s
	m.mu.Unlock()

	hash := md5.Sum(firmware)
	md5Str := hex.EncodeToString(hash[:])

	if err := dev.SendText(gateway.FirmwareUpdateMessage(version, len(firmware), md5Str, DefaultOTAChunkSize)); err != nil {
		m.setError(deviceID, fmt.Sprintf("send update offer: %v", err))
		return err
	}

	go m.runUpdate(s)
	return nil
}

// runUpdate waits for the device ack, streams chunks, and waits for completion.
func (m *OTAManager) runUpdate(s *otaState) {
	defer func() {
		m.mu.Lock()
		s.running = false
		if s.cancel != nil {
			s.cancel()
		}
		m.mu.Unlock()
		s.doneOnce.Do(func() { close(s.doneCh) })
	}()

	// Wait for firmware_update_ack from the device.
	select {
	case <-s.startCh:
	case <-s.ctx.Done():
		if s.ctx.Err() == context.Canceled {
			m.setProgress(s.deviceID, OTAProgress{Stage: string(OTAStageCancelled), Message: "cancelled by user"})
		} else {
			m.setError(s.deviceID, "timed out waiting for device to accept update")
		}
		return
	}

	m.setProgress(s.deviceID, OTAProgress{
		Running: true,
		Stage:   string(OTAStageWriting),
		Percent: 0,
		Message: fmt.Sprintf("writing %d bytes", s.total),
	})

	for i := 0; i < s.totalChunks; i++ {
		select {
		case <-s.ctx.Done():
			if s.ctx.Err() == context.Canceled {
				m.setProgress(s.deviceID, OTAProgress{Stage: string(OTAStageCancelled), Message: "cancelled by user"})
			} else {
				m.setError(s.deviceID, "update timed out")
			}
			return
		default:
		}

		start := i * s.chunkSize
		end := start + s.chunkSize
		if end > s.total {
			end = s.total
		}
		if err := s.dev.SendBinary(s.firmware[start:end]); err != nil {
			m.setError(s.deviceID, fmt.Sprintf("send chunk %d: %v", i, err))
			return
		}

		// Wait for the device to acknowledge this chunk before sending the next one.
		select {
		case seq := <-s.ackCh:
			if seq != i {
				m.setError(s.deviceID, fmt.Sprintf("chunk ack out of order: got %d want %d", seq, i))
				return
			}
			s.ackedChunks = i + 1
			percent := s.ackedChunks * 100 / s.totalChunks
			m.setProgress(s.deviceID, OTAProgress{
				Running: true,
				Stage:   string(OTAStageWriting),
				Percent: percent,
				Message: fmt.Sprintf("writing chunk %d / %d", s.ackedChunks, s.totalChunks),
			})
		case <-s.ctx.Done():
			if s.ctx.Err() == context.Canceled {
				m.setProgress(s.deviceID, OTAProgress{Stage: string(OTAStageCancelled), Message: "cancelled by user"})
			} else {
				m.setError(s.deviceID, fmt.Sprintf("timed out waiting for chunk %d ack", i))
			}
			return
		}
	}

	m.setProgress(s.deviceID, OTAProgress{
		Running: true,
		Stage:   string(OTAStageVerifying),
		Percent: 95,
		Message: "waiting for device verification",
	})

	// Wait for firmware_update_complete (or error/cancel) from the device.
	select {
	case <-s.doneCh:
	case <-s.ctx.Done():
		if s.ctx.Err() == context.Canceled {
			m.setProgress(s.deviceID, OTAProgress{Stage: string(OTAStageCancelled), Message: "cancelled by user"})
		} else {
			m.setError(s.deviceID, "timed out waiting for update completion")
		}
	}
}

// HandleEvent processes OTA-related events from the device.
func (m *OTAManager) HandleEvent(deviceID string, evt *gateway.DeviceEvent) {
	if evt == nil {
		return
	}

	switch evt.Type {
	case "firmware_version":
		ver, _ := evt.Payload["version"].(string)
		m.mu.Lock()
		m.deviceVersions[deviceID] = ver
		if s, ok := m.states[deviceID]; ok {
			s.progress.DeviceVersion = ver
			if s.progress.Stage == string(OTAStageChecking) {
				s.progress.Running = false
				s.progress.Message = fmt.Sprintf("device version: %s", ver)
			}
		}
		m.mu.Unlock()

	case "firmware_update_ack":
		m.mu.Lock()
		s, ok := m.states[deviceID]
		if ok {
			close(s.startCh)
		}
		m.mu.Unlock()

	case "firmware_chunk_ack":
		seq, _ := evt.Payload["seq"].(float64)
		m.mu.Lock()
		s, ok := m.states[deviceID]
		if ok && s.running {
			select {
			case s.ackCh <- int(seq):
			default:
			}
		}
		m.mu.Unlock()

	case "firmware_progress":
		sent, _ := evt.Payload["sent"].(float64)
		total, _ := evt.Payload["total"].(float64)
		percent, _ := evt.Payload["percent"].(float64)
		m.mu.Lock()
		if s, ok := m.states[deviceID]; ok {
			if percent > 0 {
				s.progress.Percent = int(percent)
			} else if total > 0 {
				s.progress.Percent = int(sent / total * 100)
			}
			if msg, _ := evt.Payload["message"].(string); msg != "" {
				s.progress.Message = msg
			}
		}
		m.mu.Unlock()

	case "firmware_update_complete":
		m.mu.Lock()
		s, ok := m.states[deviceID]
		if ok {
			s.doneOnce.Do(func() { close(s.doneCh) })
		}
		m.mu.Unlock()
		if !ok {
			return
		}
		m.setProgress(deviceID, OTAProgress{
			Running: true,
			Stage:   string(OTAStageCommitting),
			Percent: 98,
			Message: "committing firmware",
		})
		if err := s.dev.SendText(gateway.FirmwareCommitMessage()); err != nil {
			m.setError(deviceID, fmt.Sprintf("send commit: %v", err))
			return
		}
		m.setProgress(deviceID, OTAProgress{
			Stage:   string(OTAStageDone),
			Percent: 100,
			Message: "update complete; device is restarting",
		})

	case "firmware_update_error":
		msg, _ := evt.Payload["message"].(string)
		m.setError(deviceID, msg)

	default:
		log.Printf("[ota] unknown firmware event type %s from %s", evt.Type, deviceID)
	}
}

// Progress returns the current OTA progress for a device.
func (m *OTAManager) Progress(deviceID string) OTAProgress {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.states[deviceID]; ok {
		p := s.progress
		p.Running = s.running
		return p
	}
	ver := m.deviceVersions[deviceID]
	return OTAProgress{Stage: string(OTAStageIdle), DeviceVersion: ver}
}

// DeviceVersion returns the last reported firmware version for a device.
func (m *OTAManager) DeviceVersion(deviceID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.deviceVersions[deviceID]
}

// Cancel aborts the running OTA for a device and notifies it.
func (m *OTAManager) Cancel(deviceID string, dev gateway.DeviceAdapter) error {
	m.mu.Lock()
	s, ok := m.states[deviceID]
	if ok {
		s.doneOnce.Do(func() { close(s.doneCh) })
	}
	m.mu.Unlock()
	if !ok || !s.running {
		return nil
	}
	if s.cancel != nil {
		s.cancel()
	}
	if dev != nil {
		_ = dev.SendText(gateway.FirmwareUpdateCancelMessage())
	}
	m.setProgress(deviceID, OTAProgress{Stage: string(OTAStageCancelled), Message: "cancelled by user"})
	return nil
}

// DeviceDisconnected marks any running OTA for the device as failed.
func (m *OTAManager) DeviceDisconnected(deviceID string) {
	m.mu.Lock()
	s, ok := m.states[deviceID]
	if ok {
		s.doneOnce.Do(func() { close(s.doneCh) })
	}
	if !ok || !s.running {
		m.mu.Unlock()
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	m.mu.Unlock()
	m.setError(deviceID, "device disconnected")
}

func (m *OTAManager) setProgress(deviceID string, p OTAProgress) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setProgressLocked(deviceID, p)
}

func (m *OTAManager) setProgressLocked(deviceID string, p OTAProgress) {
	if s, ok := m.states[deviceID]; ok {
		if p.Running || s.running {
			s.progress = p
			s.progress.Running = s.running
		}
	}
}

func (m *OTAManager) setError(deviceID, msg string) {
	log.Printf("[ota] %s error: %s", deviceID, msg)
	m.mu.Lock()
	if s, ok := m.states[deviceID]; ok {
		s.doneOnce.Do(func() { close(s.doneCh) })
	}
	m.mu.Unlock()
	m.setProgress(deviceID, OTAProgress{Stage: string(OTAStageError), Message: msg, Error: msg})
}
