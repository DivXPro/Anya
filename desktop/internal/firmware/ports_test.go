package firmware

import (
	"runtime"
	"testing"
)

func TestPortDisplayName(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/dev/cu.usbserial-1234", "cu.usbserial-1234"},
		{"/dev/ttyUSB0", "ttyUSB0"},
		{"COM3", "COM3"},
	}
	for _, c := range cases {
		got := portDisplayName(c.path)
		if got != c.want {
			t.Errorf("portDisplayName(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

func TestIsLikelyESP32Port(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/dev/cu.usbserial-1234", runtime.GOOS == "darwin"},
		{"/dev/cu.wchusbserial1234", runtime.GOOS == "darwin"},
		{"/dev/cu.Bluetooth-Incoming-Port", false},
		{"/dev/ttyUSB0", runtime.GOOS == "linux"},
		{"COM1", runtime.GOOS == "windows"},
	}
	for _, c := range cases {
		got := isLikelyESP32Port(c.path)
		if got != c.want {
			t.Errorf("isLikelyESP32Port(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestListSerialPortsNoPanic(t *testing.T) {
	// Just ensure the function does not panic on any supported platform.
	_, err := ListSerialPorts()
	if err != nil && runtime.GOOS != "windows" {
		t.Fatalf("ListSerialPorts error: %v", err)
	}
}
