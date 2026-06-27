package firmware

import (
	"bufio"
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.bug.st/serial"
)

var versionRe = regexp.MustCompile(`(?i)firmware\s+setup\s+start,\s*version\s*=\s*([^\s\r\n]+)`)

// ReadDeviceFirmwareVersion opens the given serial port and listens for the
// firmware's startup banner that contains the version string. It returns the
// version if heard within the timeout, otherwise an empty string.
func ReadDeviceFirmwareVersion(port string, timeout time.Duration) (string, error) {
	mode := &serial.Mode{
		BaudRate: 115200,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	p, err := serial.Open(port, mode)
	if err != nil {
		return "", fmt.Errorf("open serial port: %w", err)
	}
	defer p.Close()

	// Give the device a moment to output any buffered startup log.
	_ = p.SetReadTimeout(100)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	reader := bufio.NewReader(p)
	result := make(chan string, 1)

	go func() {
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				// Ignore transient read errors; keep listening.
				continue
			}
			if m := versionRe.FindStringSubmatch(line); len(m) > 1 {
				result <- strings.TrimSpace(m[1])
				return
			}
		}
	}()

	select {
	case v := <-result:
		return v, nil
	case <-ctx.Done():
		return "", fmt.Errorf("timeout waiting for firmware version")
	}
}
