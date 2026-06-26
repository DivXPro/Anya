package firmware

import (
	"testing"
)

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

func TestFindEsptool(t *testing.T) {
	cmd, err := FindEsptool()
	if err != nil {
		t.Fatalf("FindEsptool error: %v", err)
	}
	if cmd != "built-in" {
		t.Fatalf("expected built-in, got %s", cmd)
	}
}
