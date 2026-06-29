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

func TestKimiAdapterResetPending(t *testing.T) {
	a := NewKimiAdapter()

	// SetCWD should set resetPending flag when no active stream (pm not running, so immediate stop is no-op)
	a.SetCWD("/tmp/new")
	if a.resetPending {
		t.Fatal("expected resetPending to be false after SetCWD with no active stream (immediate stop consumed it)")
	}

	// Simulate active stream by setting activeStream directly
	ch := make(chan acp.StreamEvent)
	a.streamMu.Lock()
	a.activeStream = ch
	a.streamMu.Unlock()

	a.SetCWD("/tmp/another")
	if !a.resetPending {
		t.Fatal("expected resetPending to be true after SetCWD with active stream")
	}

	// Stop should clear resetPending
	if err := a.Stop(); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
	if a.resetPending {
		t.Fatal("expected resetPending to be false after Stop")
	}

	// Clean up
	a.streamMu.Lock()
	if a.activeStream != nil {
		close(a.activeStream)
		a.activeStream = nil
	}
	a.streamMu.Unlock()
}
