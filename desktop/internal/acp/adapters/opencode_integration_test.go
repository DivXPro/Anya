//go:build integration

package adapters

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"desktop/internal/acp"
)

func TestOpenCodeAdapterSendAndReceive(t *testing.T) {
	if _, err := exec.LookPath("opencode"); err != nil {
		t.Skip("opencode not installed")
	}

	adapter := NewOpenCodeAdapter()
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
			t.Fatalf("content contains lifecycle marker %q", marker)
		}
	}

	info := adapter.Info()
	if info.ID != "opencode" {
		t.Fatalf("unexpected adapter id: %s", info.ID)
	}
}

func TestOpenCodeAdapterLoadSession(t *testing.T) {
	if _, err := exec.LookPath("opencode"); err != nil {
		t.Skip("opencode not installed")
	}

	adapter := NewOpenCodeAdapter()
	defer adapter.Stop()

	if _, err := adapter.Send("say hi", nil); err != nil {
		t.Fatalf("first send: %v", err)
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

var _ acp.ACPAdapter = (*OpenCodeAdapter)(nil)
