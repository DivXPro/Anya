package firmware

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
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

// Flash writes the embedded firmware to the given serial port using esptool.
// It blocks until the operation completes or is cancelled.
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
	m.progress = FlashProgress{Running: true, Stage: StageDetecting, Percent: 0, Message: "starting"}
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.running = false
		m.progress.Running = false
		m.mu.Unlock()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	defer func() { m.cancel = nil }()

	esptool, args, err := buildEsptoolCommand(port)
	if err != nil {
		m.setProgress(StageError, 0, err.Error())
		return err
	}

	// Write the embedded firmware to a temporary file. esptool needs a file path.
	tmpDir, err := os.MkdirTemp("", "elf-firmware-*")
	if err != nil {
		m.setProgress(StageError, 0, fmt.Sprintf("create temp dir: %v", err))
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	binPath := filepath.Join(tmpDir, "firmware.bin")
	if err := os.WriteFile(binPath, EmbeddedFirmware(), 0644); err != nil {
		m.setProgress(StageError, 0, fmt.Sprintf("write firmware file: %v", err))
		return fmt.Errorf("write firmware file: %w", err)
	}

	fullArgs := append(args, binPath)
	cmd := exec.CommandContext(ctx, esptool, fullArgs...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.setProgress(StageError, 0, fmt.Sprintf("stdout pipe: %v", err))
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		m.setProgress(StageError, 0, fmt.Sprintf("stderr pipe: %v", err))
		return fmt.Errorf("stderr pipe: %w", err)
	}

	m.setProgress(StageDetecting, 0, fmt.Sprintf("connecting to %s", port))

	if err := cmd.Start(); err != nil {
		m.setProgress(StageError, 0, fmt.Sprintf("start esptool: %v", err))
		return fmt.Errorf("start esptool: %w", err)
	}

	output := &limitedBuffer{limit: 4096}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		m.scanOutput(stdout, output)
	}()
	go func() {
		defer wg.Done()
		m.scanOutput(stderr, output)
	}()

	if err := cmd.Wait(); err != nil {
		wg.Wait()
		if ctx.Err() == context.Canceled {
			m.setProgress(StageCancelled, m.Progress().Percent, "cancelled by user")
			return fmt.Errorf("cancelled")
		}
		msg := fmt.Sprintf("esptool failed: %v\n%s", err, output.String())
		m.setProgress(StageError, m.Progress().Percent, msg)
		return fmt.Errorf("esptool failed: %w\n%s", err, output.String())
	}
	wg.Wait()

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

func buildEsptoolCommand(port string) (string, []string, error) {
	chip := "esp32s3"
	baud := "460800"
	args := []string{
		"--chip", chip,
		"--port", port,
		"--baud", baud,
		"--before", "default_reset",
		"--after", "hard_reset",
		"write_flash", "-z",
		"--flash_mode", "dio",
		"--flash_freq", "80m",
		"--flash_size", "8MB",
		"0x0",
	}

	if runtime.GOOS == "windows" {
		// On Windows, esptool.py may be invoked via python -m esptool.
		if _, err := exec.LookPath("esptool.py"); err == nil {
			return "esptool.py", args, nil
		}
		if _, err := exec.LookPath("python"); err == nil {
			return "python", append([]string{"-m", "esptool"}, args...), nil
		}
		if _, err := exec.LookPath("python3"); err == nil {
			return "python3", append([]string{"-m", "esptool"}, args...), nil
		}
		return "", nil, fmt.Errorf("esptool not found. Install with: pip install esptool")
	}

	// macOS / Linux: prefer esptool.py, fall back to python -m esptool.
	if _, err := exec.LookPath("esptool.py"); err == nil {
		return "esptool.py", args, nil
	}
	python := "python3"
	if _, err := exec.LookPath("python3"); err != nil {
		python = "python"
		if _, err := exec.LookPath("python"); err != nil {
			return "", nil, fmt.Errorf("esptool not found. Install with: pip install esptool")
		}
	}
	return python, append([]string{"-m", "esptool"}, args...), nil
}

var (
	eraseRe   = regexp.MustCompile(`(?i)erasing flash|` + "`" + `erase` + "`" + `|Chip Erase`)
	writeRe   = regexp.MustCompile(`(?i)writing @|Writing at|Compressed.*bytes to`)
	verifyRe  = regexp.MustCompile(`(?i)verifying|Hash of data verified`)
	percentRe = regexp.MustCompile(`\((\d+)\s*%\)`)
	writePctRe = regexp.MustCompile(`Writing at\s+0x[0-9a-f]+\.\.\.\s*\((\d+)\s*%\)`)
)

func (m *Manager) scanOutput(r io.Reader, output *limitedBuffer) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		output.WriteString(line + "\n")
		m.parseLine(line)
	}
}

func (m *Manager) parseLine(line string) {
	lower := strings.ToLower(line)

	switch {
	case strings.Contains(lower, "connecting"):
		m.setProgress(StageDetecting, 0, line)
	case strings.Contains(lower, "chip is"):
		m.setProgress(StageDetecting, 0, line)
	case eraseRe.MatchString(line):
		m.setProgress(StageErasing, 5, line)
	case writeRe.MatchString(line):
		pct := extractPercent(line)
		if pct >= 0 {
			// Map writing progress from 5% to 90%.
			mapped := 5 + int(float64(pct)*0.85)
			if mapped > 90 {
				mapped = 90
			}
			m.setProgress(StageWriting, mapped, line)
		} else {
			m.setProgress(StageWriting, m.Progress().Percent, line)
		}
	case verifyRe.MatchString(line):
		m.setProgress(StageVerifying, 95, line)
	case strings.Contains(lower, "hard resetting"):
		m.setProgress(StageDone, 100, line)
	case strings.Contains(lower, "failed") || strings.Contains(lower, "error"):
		// Don't set stage to error here; let the command exit handle that.
		m.setProgress(m.Progress().Stage, m.Progress().Percent, line)
	}
}

func extractPercent(line string) int {
	if m := writePctRe.FindStringSubmatch(line); len(m) > 1 {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n
		}
	}
	if m := percentRe.FindStringSubmatch(line); len(m) > 1 {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n
		}
	}
	return -1
}

// limitedBuffer keeps the last N bytes of output for error reports.
type limitedBuffer struct {
	buf   []byte
	limit int
	mu    sync.Mutex
}

func (b *limitedBuffer) WriteString(s string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, []byte(s)...)
	if len(b.buf) > b.limit {
		b.buf = b.buf[len(b.buf)-b.limit:]
	}
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

// FindEsptool returns the resolved esptool command, or an error if not installed.
func FindEsptool() (string, error) {
	cmd, _, err := buildEsptoolCommand("/dev/null")
	return cmd, err
}

// ShortFlashTimeout is the max duration for a flash operation.
const ShortFlashTimeout = 5 * time.Minute
