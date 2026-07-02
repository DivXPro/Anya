package applog

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	logFileName      = "anya.log"
	defaultTailBytes = 128 * 1024
	maxTailBytes     = 1024 * 1024
	rotateBytes      = 5 * 1024 * 1024
)

var (
	mu      sync.Mutex
	logPath string
	logFile *os.File
)

type Info struct {
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modified_at"`
}

func Init(dataDir string) error {
	if dataDir == "" {
		return fmt.Errorf("data dir is empty")
	}
	dir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}
	path := filepath.Join(dir, logFileName)
	if err := rotateIfNeeded(path); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	mu.Lock()
	if logFile != nil {
		_ = logFile.Close()
	}
	logPath = path
	logFile = f
	mu.Unlock()

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	return nil
}

func GetInfo() Info {
	path := currentPath()
	info := Info{Path: path}
	if path == "" {
		return info
	}
	st, err := os.Stat(path)
	if err != nil {
		return info
	}
	info.Size = st.Size()
	info.ModifiedAt = st.ModTime().Format(time.RFC3339)
	return info
}

func ReadTail(maxBytes int64) (string, error) {
	path := currentPath()
	if path == "" {
		return "", fmt.Errorf("log file is not initialized")
	}
	if maxBytes <= 0 {
		maxBytes = defaultTailBytes
	}
	if maxBytes > maxTailBytes {
		maxBytes = maxTailBytes
	}

	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open log file: %w", err)
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("stat log file: %w", err)
	}
	start := st.Size() - maxBytes
	if start < 0 {
		start = 0
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return "", fmt.Errorf("seek log file: %w", err)
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("read log file: %w", err)
	}
	for len(data) > 0 && !utf8.Valid(data) {
		data = data[1:]
	}
	if start > 0 {
		return "...\n" + string(data), nil
	}
	return string(data), nil
}

func Writer() io.Writer {
	return log.Writer()
}

func currentPath() string {
	mu.Lock()
	defer mu.Unlock()
	return logPath
}

func rotateIfNeeded(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat log file: %w", err)
	}
	if st.Size() < rotateBytes {
		return nil
	}
	rotated := path + ".1"
	_ = os.Remove(rotated)
	if err := os.Rename(path, rotated); err != nil {
		return fmt.Errorf("rotate log file: %w", err)
	}
	return nil
}
