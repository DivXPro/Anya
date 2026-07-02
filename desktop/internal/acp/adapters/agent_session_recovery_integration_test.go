//go:build integration

package adapters

import (
	"os/exec"
	"testing"

	"desktop/internal/acp"
)

func TestClaudeAgentSessionRecoveryFromRecentList(t *testing.T) {
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude not installed")
	}
	adapter := NewClaudeAdapter()
	defer adapter.Stop()

	session := recentAgentSession(t, adapter, "claude-code")
	if err := adapter.LoadAgentSession(session.ID, session.CWD); err != nil {
		t.Fatalf("load recent claude agent session %q: %v", session.ID, err)
	}
	if got := adapter.CurrentSessionID(); got != session.ID {
		t.Fatalf("current session = %q, want %q", got, session.ID)
	}
}

func TestCodexAgentSessionRecoveryFromRecentList(t *testing.T) {
	if _, err := exec.LookPath("codex"); err != nil {
		t.Skip("codex not installed")
	}
	adapter := NewCodexAdapter()
	defer adapter.Stop()

	session := recentAgentSession(t, adapter, "codex")
	if err := adapter.LoadAgentSession(session.ID, session.CWD); err != nil {
		t.Fatalf("load recent codex agent session %q: %v", session.ID, err)
	}
	if got := adapter.CurrentSessionID(); got != session.ID {
		t.Fatalf("current session = %q, want %q", got, session.ID)
	}
}

func TestOpenCodeAgentSessionRecoveryFromRecentList(t *testing.T) {
	if _, err := exec.LookPath("opencode"); err != nil {
		t.Skip("opencode not installed")
	}
	adapter := NewOpenCodeAdapter()
	defer adapter.Stop()

	session := recentAgentSession(t, adapter, "opencode")
	if err := adapter.LoadAgentSession(session.ID, session.CWD); err != nil {
		t.Fatalf("load recent opencode agent session %q: %v", session.ID, err)
	}
	if got := adapter.CurrentSessionID(); got != session.ID {
		t.Fatalf("current session = %q, want %q", got, session.ID)
	}
}

func recentAgentSession(t *testing.T, provider acp.AgentSessionProvider, name string) acp.AgentSession {
	t.Helper()
	sessions, err := provider.ListAgentSessions(10)
	if err != nil {
		t.Fatalf("list %s agent sessions: %v", name, err)
	}
	for _, session := range sessions {
		if session.ID != "" && session.CanResume {
			return session
		}
	}
	t.Skipf("no resumable %s agent sessions found", name)
	return acp.AgentSession{}
}
