package adapters

import (
	"testing"

	"desktop/internal/acp"
)

func TestIsPiCliInstalled(t *testing.T) {
	// Result depends on the test environment; just ensure it does not panic.
	_ = IsPiCliInstalled()
}

func TestNewPiAdapterInfo(t *testing.T) {
	a := NewPiAdapter()
	info := a.Info()
	if info.ID != "pi" {
		t.Fatalf("unexpected id: %s", info.ID)
	}
	if info.Name != "Pi" {
		t.Fatalf("unexpected name: %s", info.Name)
	}
	if info.Command != "pi --mode rpc --no-session" {
		t.Fatalf("unexpected command: %s", info.Command)
	}
}

func TestPiAdapterLoadSessionUnit(t *testing.T) {
	a := NewPiAdapter()
	if err := a.LoadSession("session-pi-test", nil); err != nil {
		t.Fatalf("load session: %v", err)
	}
	if a.CurrentSessionID() != "session-pi-test" {
		t.Fatalf("expected session session-pi-test, got %s", a.CurrentSessionID())
	}
}

func TestPiAdapterImplementsInterface(t *testing.T) {
	var _ acp.ACPAdapter = (*PiAdapter)(nil)
}
