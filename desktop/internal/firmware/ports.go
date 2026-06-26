package firmware

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// ListSerialPorts returns candidate serial ports that may host an ESP32-S3 / M5StickC S3.
func ListSerialPorts() ([]SerialPortInfo, error) {
	paths, err := enumerateSerialPorts()
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

func enumerateSerialPorts() ([]string, error) {
	switch runtime.GOOS {
	case "darwin":
		return listDarwinPorts()
	case "linux":
		return listLinuxPorts()
	case "windows":
		return listWindowsPorts()
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func listDarwinPorts() ([]string, error) {
	matches, err := filepath.Glob("/dev/cu.*")
	if err != nil {
		return nil, err
	}
	return matches, nil
}

func listLinuxPorts() ([]string, error) {
	var all []string
	for _, pat := range []string{"/dev/ttyUSB*", "/dev/ttyACM*"} {
		m, err := filepath.Glob(pat)
		if err != nil {
			return nil, err
		}
		all = append(all, m...)
	}
	return all, nil
}

func listWindowsPorts() ([]string, error) {
	// On Windows, serial ports are COM1..COM256. We cannot easily test existence
	// without opening them, so enumerate a reasonable range.
	var ports []string
	for i := 1; i <= 64; i++ {
		ports = append(ports, fmt.Sprintf("COM%d", i))
	}
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
			strings.Contains(lower, "ftdi")
	case "linux":
		return true // ttyUSB/ttyACM are already filtered by glob.
	case "windows":
		return true // COM list is explicit.
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

// IsPortAccessible returns true if the port can be opened for reading.
// This is a best-effort check; on some platforms opening may require exclusive access.
func IsPortAccessible(path string) bool {
	if runtime.GOOS == "windows" {
		// Opening COM ports on Windows can block or require exclusive access;
		// skip the accessibility probe and rely on esptool to report errors.
		return true
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}
