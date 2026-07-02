package adapters

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"desktop/internal/acp"

	"github.com/beyond5959/acp-adapter/pkg/claudeacp"
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

func TestClaudeAdapterLoadSessionPropagatesLoadFailure(t *testing.T) {
	a := NewClaudeAdapter()
	a.initRuntimeFunc = func() error { return nil }
	a.clientRequestFunc = func(id int, method string, params map[string]any) (map[string]any, error) {
		if method != "session/load" {
			t.Fatalf("expected session/load request, got %s", method)
		}
		if got := params["sessionId"]; got != "stale-session" {
			t.Fatalf("expected stale-session, got %v", got)
		}
		return nil, errors.New("begin turn failed")
	}

	err := a.LoadSession("stale-session", nil)
	if err == nil {
		t.Fatal("expected load failure to be returned")
	}
	if !strings.Contains(err.Error(), "begin turn failed") {
		t.Fatalf("expected original load error, got %v", err)
	}
	if got := a.CurrentSessionID(); got == "stale-session" {
		t.Fatalf("failed load should not keep stale session id %q", got)
	}
}

func TestClaudeAdapterDispatchTreatsClaudeCLITextAsError(t *testing.T) {
	a := NewClaudeAdapter()
	a.sessionID = "session-1"

	ch := make(chan acp.StreamEvent, 2)
	a.streamMu.Lock()
	a.activeStream = ch
	a.streamMu.Unlock()

	updates := make(chan claudeacp.RPCMessage, 1)
	params, err := json.Marshal(map[string]any{
		"sessionId": "session-1",
		"update": map[string]any{
			"sessionUpdate": "agent_thought_chunk",
			"content": map[string]any{
				"type": "text",
				"text": "claude cli error: ",
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	updates <- claudeacp.RPCMessage{Method: "session/update", Params: params}
	close(updates)

	a.dispatchUpdates(updates)

	select {
	case evt := <-ch:
		if !evt.IsError() {
			t.Fatalf("expected error event, got %v", evt)
		}
		if evt.Error == nil || !strings.HasPrefix(strings.ToLower(evt.Error.Error()), "claude cli error:") {
			t.Fatalf("unexpected error: %v", evt.Error)
		}
	default:
		t.Fatal("expected dispatched error event")
	}
}

// TestClaudeAdapterFinishStreamReleasesLock guards against a self-deadlock
// regression: finishStream must release streamMu so that subsequent turns
// (Send) and update dispatch (dispatchUpdates) can re-acquire it. Before the
// fix, finishStream locked streamMu twice and hung forever holding the write
// lock, so only the first turn ever produced a response.
func TestClaudeAdapterFinishStreamReleasesLock(t *testing.T) {
	a := NewClaudeAdapter()

	ch := make(chan acp.StreamEvent, 4)
	a.streamMu.Lock()
	a.activeStream = ch
	a.streamMu.Unlock()

	finished := make(chan struct{})
	go func() {
		a.finishStream(nil)
		close(finished)
	}()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("finishStream deadlocked")
	}

	// A terminal "done" event must have been delivered and the channel closed.
	// The range loop returns only once the channel is closed.
	var sawDone bool
	for evt := range ch {
		if evt.IsDone() {
			sawDone = true
		}
	}
	if !sawDone {
		t.Fatal("expected done event on stream")
	}

	// activeStream must be cleared and the lock must be re-acquirable, i.e. it
	// was not leaked by a deadlocked goroutine.
	acquired := make(chan struct{})
	go func() {
		a.streamMu.Lock()
		a.activeStream = nil
		a.streamMu.Unlock()
		close(acquired)
	}()
	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		t.Fatal("streamMu was leaked: could not re-acquire after finishStream")
	}
}

// TestClaudeAdapterFinishStreamResetPending guards the reset path: when a cwd
// change set resetPending during an active stream, finishStream runs stopLocked
// (which itself takes streamMu) after the stream completes. This must not
// deadlock and must clear resetPending.
func TestClaudeAdapterFinishStreamResetPending(t *testing.T) {
	a := NewClaudeAdapter()

	ch := make(chan acp.StreamEvent, 4)
	a.streamMu.Lock()
	a.activeStream = ch
	a.streamMu.Unlock()
	a.resetPending = true

	finished := make(chan struct{})
	go func() {
		a.finishStream(nil)
		close(finished)
	}()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("finishStream with resetPending deadlocked")
	}

	if a.resetPending {
		t.Fatal("expected resetPending to be cleared after finishStream")
	}
}
