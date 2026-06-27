package adapters

import (
	"testing"

	"desktop/internal/acp"
)

func TestIsCodexCliInstalled(t *testing.T) {
	// Result depends on the test environment; just ensure it does not panic.
	_ = IsCodexCliInstalled()
}

func TestNewCodexAdapterInfo(t *testing.T) {
	a := NewCodexAdapter()
	info := a.Info()
	if info.ID != "codex" {
		t.Fatalf("unexpected id: %s", info.ID)
	}
	if info.Name != "Codex" {
		t.Fatalf("unexpected name: %s", info.Name)
	}
	if info.Command != "codex app-server --stdio" {
		t.Fatalf("unexpected command: %s", info.Command)
	}
}

func TestCodexAdapterImplementsInterface(t *testing.T) {
	var _ acp.ACPAdapter = (*CodexAdapter)(nil)
}

func TestCodexThreadIDFromResult(t *testing.T) {
	cases := []struct {
		name   string
		result map[string]any
		want   string
	}{
		{
			name: "valid thread",
			result: map[string]any{
				"thread": map[string]any{"id": "abc-123"},
			},
			want: "abc-123",
		},
		{
			name:   "nil result",
			result: nil,
			want:   "",
		},
		{
			name: "missing thread",
			result: map[string]any{
				"model": "gpt-5.5",
			},
			want: "",
		},
		{
			name: "missing id",
			result: map[string]any{
				"thread": map[string]any{},
			},
			want: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := threadIDFromResult(c.result)
			if got != c.want {
				t.Fatalf("threadIDFromResult() = %q, want %q", got, c.want)
			}
		})
	}
}
