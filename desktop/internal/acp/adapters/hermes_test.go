package adapters

import (
	"testing"

	"desktop/internal/acp"
)

func TestIsHermesCliInstalled(t *testing.T) {
	// Result depends on the test environment; just ensure it does not panic.
	_ = IsHermesCliInstalled()
}

func TestNewHermesAdapterInfo(t *testing.T) {
	a := NewHermesAdapter()
	info := a.Info()
	if info.ID != "hermes" {
		t.Fatalf("unexpected id: %s", info.ID)
	}
	if info.Name != "Hermes" {
		t.Fatalf("unexpected name: %s", info.Name)
	}
	if info.Command != "hermes acp" {
		t.Fatalf("unexpected command: %s", info.Command)
	}
}

func TestHermesAdapterImplementsInterface(t *testing.T) {
	var _ acp.ACPAdapter = (*HermesAdapter)(nil)
}
