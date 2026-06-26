package adapters

import (
	"testing"

	"desktop/internal/acp"
)

func TestIsKimiCliInstalled(t *testing.T) {
	// Result depends on the test environment; just ensure it does not panic.
	_ = IsKimiCliInstalled()
}

func TestNewKimiAdapterInfo(t *testing.T) {
	a := NewKimiAdapter()
	info := a.Info()
	if info.ID != "kimi" {
		t.Fatalf("unexpected id: %s", info.ID)
	}
	if info.Name != "Kimi Code" {
		t.Fatalf("unexpected name: %s", info.Name)
	}
	if info.Command != "kimi acp" {
		t.Fatalf("unexpected command: %s", info.Command)
	}
}

func TestKimiAdapterImplementsInterface(t *testing.T) {
	var _ acp.ACPAdapter = (*KimiAdapter)(nil)
}
