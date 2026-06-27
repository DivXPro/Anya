//go:build integration

package adapters

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	"desktop/internal/acp"
)

var errStreamTimeout = errors.New("stream timed out")

func drainStream(t *testing.T, ch <-chan acp.StreamEvent, timeout time.Duration) error {
	t.Helper()
	deadline := time.After(timeout)
loop:
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				break loop
			}
			if evt.IsError() {
				return evt.Error
			}
			if evt.IsDone() {
				break loop
			}
		case <-deadline:
			return errStreamTimeout
		}
	}
	return nil
}

func TestClaudeEmbeddedRuntimeInitialize(t *testing.T) {
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude not installed")
	}

	a := NewClaudeAdapter()
	if err := a.ensureInit(); err != nil {
		t.Fatalf("ensureInit: %v", err)
	}
	defer a.Stop()

	if a.sessionID == "" {
		t.Fatal("expected non-empty session id")
	}
}

func TestClaudeAdapterSendAndReceive(t *testing.T) {
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude not installed")
	}

	adapter := NewClaudeAdapter()
	defer adapter.Stop()

	ch, err := adapter.Send("say hi", nil)
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	var sawDelta, sawDone bool
	var content strings.Builder
	timeout := time.After(60 * time.Second)
loop:
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				break loop
			}
			if evt.IsError() {
				t.Fatalf("stream error: %v", evt.Error)
			}
			if evt.IsContent() {
				sawDelta = true
				content.WriteString(evt.Content)
			}
			if evt.IsDone() {
				sawDone = true
				break loop
			}
		case <-timeout:
			t.Fatal("timeout waiting for response")
		}
	}

	if !sawDelta {
		t.Fatal("expected at least one text_delta event")
	}
	if !sawDone {
		t.Fatal("expected done event")
	}

	for _, marker := range acpLifecycleMarkers {
		if strings.Contains(content.String(), marker) {
			t.Fatalf("content contains lifecycle marker %q: %q", marker, content.String())
		}
	}

	info := adapter.Info()
	if info.ID != "claude-code" {
		t.Fatalf("unexpected adapter id: %s", info.ID)
	}
}

func TestClaudeAdapterLoadSession(t *testing.T) {
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude not installed")
	}

	adapter := NewClaudeAdapter()
	defer adapter.Stop()

	ch1, err := adapter.Send("say hi", nil)
	if err != nil {
		t.Fatalf("first send: %v", err)
	}
	if err := drainStream(t, ch1, 60*time.Second); err != nil {
		t.Fatalf("first send stream: %v", err)
	}

	// Give the first turn time to fully settle before loading/reusing the session.
	time.Sleep(2 * time.Second)

	acpSessionID := adapter.CurrentSessionID()
	if acpSessionID == "" {
		t.Fatal("expected non-empty acp session id")
	}

	if err := adapter.LoadSession(acpSessionID, nil); err != nil {
		t.Fatalf("load session: %v", err)
	}
	if adapter.CurrentSessionID() != acpSessionID {
		t.Fatalf("expected session %s, got %s", acpSessionID, adapter.CurrentSessionID())
	}

	ch, err := adapter.Send("say hi again", nil)
	if err != nil {
		t.Fatalf("second send after load: %v", err)
	}

	var sawDone bool
	timeout := time.After(60 * time.Second)
loop:
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				break loop
			}
			if evt.IsError() {
				t.Fatalf("stream error: %v", evt.Error)
			}
			if evt.IsDone() {
				sawDone = true
				break loop
			}
		case <-timeout:
			t.Fatal("timeout waiting for response")
		}
	}
	if !sawDone {
		t.Fatal("expected done event after loaded session")
	}
}

var _ acp.ACPAdapter = (*ClaudeAdapter)(nil)
