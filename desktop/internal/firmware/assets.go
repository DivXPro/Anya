package firmware

import (
	_ "embed"
	"strings"
)

//go:embed assets/firmware.bin
var firmwareBin []byte

//go:embed assets/version.txt
var firmwareVersionText string

//go:embed assets/esptool.py
var esptoolScript []byte

// EmbeddedFirmwareSize returns the size of the embedded firmware binary.
func EmbeddedFirmwareSize() int {
	return len(firmwareBin)
}

// EmbeddedFirmware returns the embedded firmware binary.
func EmbeddedFirmware() []byte {
	return firmwareBin
}

// HasEmbeddedFirmware reports whether a non-empty firmware binary is embedded.
func HasEmbeddedFirmware() bool {
	return len(firmwareBin) > 0
}

// EmbeddedFirmwareVersion returns the version string of the embedded firmware.
func EmbeddedFirmwareVersion() string {
	v := strings.TrimSpace(firmwareVersionText)
	if v == "" {
		return "0.0.0-dev"
	}
	return v
}

// EsptoolScript returns the embedded esptool.py script bytes.
func EsptoolScript() []byte {
	return esptoolScript
}

// HasEsptoolScript reports whether a non-empty esptool.py script is embedded.
func HasEsptoolScript() bool {
	return len(esptoolScript) > 0
}
