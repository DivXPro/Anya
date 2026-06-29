package adapters

import (
	"testing"

	"desktop/internal/acp"
)

func TestNewOpenCodeAdapterInfo(t *testing.T) {
	a := NewOpenCodeAdapter()
	info := a.Info()
	if info.ID != "opencode" {
		t.Fatalf("unexpected id: %s", info.ID)
	}
	if info.Name != "OpenCode" {
		t.Fatalf("unexpected name: %s", info.Name)
	}
	if info.Command != "opencode acp" {
		t.Fatalf("unexpected command: %s", info.Command)
	}
}

func TestOpenCodeAdapterImplementsInterface(t *testing.T) {
	var _ acp.ACPAdapter = (*OpenCodeAdapter)(nil)
}

func TestOpenCodeAdapterResetPending(t *testing.T) {
	a := NewOpenCodeAdapter()

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
