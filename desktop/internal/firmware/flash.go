package firmware

import (
	"context"
	"fmt"
	"sync"
	"time"

	"tinygo.org/x/espflasher/pkg/espflasher"
)

// Manager handles a single firmware flash operation.
type Manager struct {
	mu       sync.Mutex
	progress FlashProgress
	cancel   context.CancelFunc
	running  bool
}

// NewManager creates a flash manager with an idle progress snapshot.
func NewManager() *Manager {
	return &Manager{
		progress: FlashProgress{Stage: StageIdle},
	}
}

// Progress returns a copy of the current flash progress.
func (m *Manager) Progress() FlashProgress {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.progress
	return p
}

func (m *Manager) setProgress(stage FlashStage, percent int, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progress.Running = m.running
	m.progress.Stage = stage
	if percent >= 0 {
		m.progress.Percent = percent
	}
	if message != "" {
		m.progress.Message = message
	}
	if stage == StageError {
		m.progress.Error = message
	}
}

// Flash writes the embedded firmware to the given serial port using the pure-Go
// espflasher library. It blocks until the operation completes or is cancelled.
func (m *Manager) Flash(port string) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("flash already in progress")
	}
	if !HasEmbeddedFirmware() {
		m.mu.Unlock()
		return fmt.Errorf("no firmware binary embedded")
	}
	m.running = true
	m.progress = FlashProgress{Running: true, Stage: StageDetecting, Percent: 0, Message: "connecting"}
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.running = false
		m.progress.Running = false
		m.mu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), flashTimeout)
	m.cancel = cancel
	defer func() { m.cancel = nil }()

	opts := espflasher.DefaultOptions()
	opts.ChipType = espflasher.ChipESP32S3
	opts.BaudRate = 115200
	opts.FlashBaudRate = 460800
	opts.ConnectAttempts = 7
	opts.Compress = true
	opts.FlashMode = "dio"
	opts.FlashFreq = "80m"
	opts.FlashSize = "8MB"
	opts.Logger = &progressLogger{m: m}

	m.setProgress(StageDetecting, 0, fmt.Sprintf("opening %s", port))
	flasher, err := espflasher.New(port, opts)
	if err != nil {
		if ctx.Err() == context.Canceled {
			m.setProgress(StageCancelled, 0, "cancelled by user")
			return fmt.Errorf("cancelled")
		}
		m.setProgress(StageError, 0, fmt.Sprintf("open port: %v", err))
		return fmt.Errorf("open port: %w", err)
	}
	defer flasher.Close()

	m.setProgress(StageDetecting, 5, fmt.Sprintf("connected to %s", flasher.ChipName()))

	// PlatformIO produces an app image that belongs at offset 0x10000 in the
	// default partition table.
	const appOffset = 0x10000
	fw := EmbeddedFirmware()
	m.setProgress(StageWriting, 10, fmt.Sprintf("flashing %d bytes", len(fw)))

	errChan := make(chan error, 1)
	go func() {
		errChan <- flasher.FlashImage(fw, appOffset, func(current, total int) {
			if total <= 0 {
				return
			}
			pct := int(float64(current) / float64(total) * 80)
			if pct > 80 {
				pct = 80
			}
			m.setProgress(StageWriting, 10+pct, fmt.Sprintf("writing %d / %d bytes", current, total))
		})
	}()

	select {
	case err := <-errChan:
		if err != nil {
			if ctx.Err() == context.Canceled {
				m.setProgress(StageCancelled, m.Progress().Percent, "cancelled by user")
				return fmt.Errorf("cancelled")
			}
			m.setProgress(StageError, m.Progress().Percent, fmt.Sprintf("flash failed: %v", err))
			return fmt.Errorf("flash failed: %w", err)
		}
	case <-ctx.Done():
		m.setProgress(StageCancelled, m.Progress().Percent, "cancelled by user")
		return fmt.Errorf("cancelled")
	}

	m.setProgress(StageVerifying, 95, "verifying")
	// espflasher.FlashImage already verifies written data via MD5; we just
	// report the final reset step.
	m.setProgress(StageDone, 100, "resetting device")
	flasher.Reset()

	m.setProgress(StageDone, 100, "flash complete")
	return nil
}

// Cancel requests cancellation of the running flash operation.
func (m *Manager) Cancel() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return nil
	}
	if m.cancel != nil {
		m.cancel()
	}
	return nil
}

// IsRunning reports whether a flash operation is in progress.
func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// FindEsptool always returns nil error now because the flasher is pure Go.
// The method is kept for frontend compatibility.
func FindEsptool() (string, error) {
	return "built-in", nil
}

// flashTimeout is the maximum duration for a flash operation.
const flashTimeout = 5 * time.Minute

type progressLogger struct {
	m *Manager
}

func (l *progressLogger) Logf(format string, args ...interface{}) {
	l.m.setProgress(l.m.Progress().Stage, l.m.Progress().Percent, fmt.Sprintf(format, args...))
}
