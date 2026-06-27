package firmware

import (
	"testing"
	"time"
)

func TestVersionRe(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{"firmware setup start, version=1.2.3", "1.2.3"},
		{"[elf] firmware setup start, version=abc123", "abc123"},
		{"FIRMWARE SETUP START, VERSION = v0.1.0-dirty", "v0.1.0-dirty"},
		{"hello world", ""},
	}
	for _, c := range cases {
		m := versionRe.FindStringSubmatch(c.line)
		var got string
		if len(m) > 1 {
			got = m[1]
		}
		if got != c.want {
			t.Errorf("versionRe(%q) = %q, want %q", c.line, got, c.want)
		}
	}
}

func TestReadDeviceFirmwareVersionEmptyPort(t *testing.T) {
	// /dev/null should fail to open as a serial port quickly.
	_, err := ReadDeviceFirmwareVersion("/dev/null", 500*time.Millisecond)
	if err == nil {
		t.Fatal("expected error for /dev/null")
	}
}
