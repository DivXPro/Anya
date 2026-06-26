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
