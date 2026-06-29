package adapters

import (
	"testing"
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
