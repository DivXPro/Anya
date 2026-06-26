package firmware

import (
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"go.bug.st/serial"
)

// ListSerialPorts returns candidate serial ports that may host an ESP32-S3 / M5StickC S3.
// It uses go.bug.st/serial to enumerate real serial devices across platforms.
func ListSerialPorts() ([]SerialPortInfo, error) {
	paths, err := serial.GetPortsList()
	if err != nil {
		return nil, err
	}

	var ports []SerialPortInfo
	seen := map[string]bool{}
	for _, p := range paths {
		if seen[p] {
			continue
		}
		seen[p] = true
		if !isLikelyESP32Port(p) {
			continue
		}
		ports = append(ports, SerialPortInfo{
			Path: p,
			Name: portDisplayName(p),
		})
	}

	sort.Slice(ports, func(i, j int) bool { return ports[i].Path < ports[j].Path })
	return ports, nil
}

func isLikelyESP32Port(path string) bool {
	lower := strings.ToLower(path)
	switch runtime.GOOS {
	case "darwin":
		// Common USB-to-UART bridges used by ESP32 dev boards.
		return strings.Contains(lower, "usbserial") ||
			strings.Contains(lower, "wchusbserial") ||
			strings.Contains(lower, "slab_usb") ||
			strings.Contains(lower, "cp210") ||
			strings.Contains(lower, "ch340") ||
			strings.Contains(lower, "ftdi") ||
			strings.Contains(lower, "usbmodem")
	case "linux":
		// go.bug.st/serial only returns real serial ports; keep the common
		// USB-to-UART prefixes to reduce noise from built-in serial consoles.
		return strings.Contains(lower, "/dev/ttyusb") ||
			strings.Contains(lower, "/dev/ttyacm") ||
			strings.Contains(lower, "/dev/ttych")
	case "windows":
		return true
	}
	return false
}

func portDisplayName(path string) string {
	base := filepath.Base(path)
	switch runtime.GOOS {
	case "darwin", "linux":
		return base
	case "windows":
		return path
	}
	return base
}
