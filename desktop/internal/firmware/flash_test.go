package firmware

import (
	"testing"
)

func TestExtractPercent(t *testing.T) {
	cases := []struct {
		line string
		want int
	}{
		{"Writing at 0x00010000... (25%)", 25},
		{"Compressed 123456 bytes to 65432...", -1},
		{"Writing at 0x00020000... (50%)", 50},
		{"Hash of data verified.", -1},
	}
	for _, c := range cases {
		got := extractPercent(c.line)
		if got != c.want {
			t.Errorf("extractPercent(%q) = %d, want %d", c.line, got, c.want)
		}
	}
}

func TestManagerNew(t *testing.T) {
	m := NewManager()
	p := m.Progress()
	if p.Stage != StageIdle {
		t.Fatalf("expected idle stage, got %v", p.Stage)
	}
	if m.IsRunning() {
		t.Fatal("new manager should not be running")
	}
}

func TestManagerFlashWithoutFirmware(t *testing.T) {
	// This test relies on the dev placeholder firmware.bin being empty.
	if HasEmbeddedFirmware() {
		t.Skip("embedded firmware is present")
	}
	m := NewManager()
	err := m.Flash("/dev/null")
	if err == nil {
		t.Fatal("expected error when no firmware is embedded")
	}
}
