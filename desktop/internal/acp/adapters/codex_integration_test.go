//go:build integration

package adapters

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"desktop/internal/acp"
)

func TestCodexAdapterSendAndReceive(t *testing.T) {
	if _, err := exec.LookPath("codex"); err != nil {
		t.Skip("codex not installed")
	}

	adapter := NewCodexAdapter()
	defer adapter.Stop()

	ch, err := adapter.Send("say hi in one sentence", nil)
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	var sawDelta, sawDone bool
	var content strings.Builder
	timeout := time.After(120 * time.Second)
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
		t.Fatalf("expected at least one text_delta event; content=%q", content.String())
	}
	if !sawDone {
		t.Fatal("expected done event")
	}
	for _, marker := range acpLifecycleMarkers {
		if strings.Contains(content.String(), marker) {
			t.Fatalf("content contains lifecycle marker %q", marker)
		}
	}

	info := adapter.Info()
	if info.ID != "codex" {
		t.Fatalf("unexpected adapter id: %s", info.ID)
	}
}

func TestCodexAdapterLoadSession(t *testing.T) {
	if _, err := exec.LookPath("codex"); err != nil {
		t.Skip("codex not installed")
	}

	adapter := NewCodexAdapter()
	defer adapter.Stop()

	ch1, err := adapter.Send("say hi", nil)
	if err != nil {
		t.Fatalf("first send: %v", err)
	}
	timeout := time.After(120 * time.Second)
firstLoop:
	for {
		select {
		case evt, ok := <-ch1:
			if !ok {
				break firstLoop
			}
			if evt.IsError() {
				t.Fatalf("first stream error: %v", evt.Error)
			}
			if evt.IsDone() {
				break firstLoop
			}
		case <-timeout:
			t.Fatal("timeout waiting for first response")
		}
	}

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
	timeout2 := time.After(120 * time.Second)
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
		case <-timeout2:
			t.Fatal("timeout waiting for response")
		}
	}
	if !sawDone {
		t.Fatal("expected done event after loaded session")
	}
}

var _ acp.ACPAdapter = (*CodexAdapter)(nil)
