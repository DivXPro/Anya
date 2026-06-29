package adapters

import (
	"testing"

	"desktop/internal/acp"
)

func TestIsClaudeCliInstalled(t *testing.T) {
	// Result depends on the test environment; just ensure it does not panic.
	_ = IsClaudeCliInstalled()
}

func TestNewClaudeAdapterInfo(t *testing.T) {
	a := NewClaudeAdapter()
	info := a.Info()
	if info.ID != "claude-code" {
		t.Fatalf("unexpected id: %s", info.ID)
	}
	if info.Name != "Claude Code" {
		t.Fatalf("unexpected name: %s", info.Name)
	}
	if info.Command != "claude" {
		t.Fatalf("unexpected command: %s", info.Command)
	}
}

func TestClaudeAdapterImplementsInterface(t *testing.T) {
	var _ acp.ACPAdapter = (*ClaudeAdapter)(nil)
}

func TestClaudeAdapterSetCWD(t *testing.T) {
	a := &ClaudeAdapter{}

	// Default behavior
	if got := a.effectiveCWD(); got != "." {
		t.Fatalf("expected default cwd '.', got %q", got)
	}

	// Set custom cwd
	a.SetCWD("/tmp/project")
	if got := a.effectiveCWD(); got != "/tmp/project" {
		t.Fatalf("expected cwd '/tmp/project', got %q", got)
	}

	// Empty string resets to default
	a.SetCWD("")
	if got := a.effectiveCWD(); got != "." {
		t.Fatalf("expected default cwd '.', got %q", got)
	}
}

func TestClaudeAdapterResetPending(t *testing.T) {
	a := NewClaudeAdapter()

	// SetCWD with no active stream should call Stop() immediately, consuming resetPending
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
