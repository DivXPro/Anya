//go:build integration

package adapters

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"desktop/internal/acp"
)

func TestPiAdapterSendAndReceive(t *testing.T) {
	if _, err := exec.LookPath("pi"); err != nil {
		t.Skip("pi not installed")
	}

	adapter := NewPiAdapter()
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
	if info.ID != "pi" {
		t.Fatalf("unexpected adapter id: %s", info.ID)
	}
}

func TestPiAdapterLoadSession(t *testing.T) {
	if _, err := exec.LookPath("pi"); err != nil {
		t.Skip("pi not installed")
	}

	adapter := NewPiAdapter()
	defer adapter.Stop()

	ch1, err := adapter.Send("say hi", nil)
	if err != nil {
		t.Fatalf("first send: %v", err)
	}
	if err := drainStream(t, ch1, 60*time.Second); err != nil {
		t.Fatalf("first send stream: %v", err)
	}

	// Pi --no-session mode does not persist sessions, so LoadSession just keeps
	// the id locally for compatibility with the ACP session layer.
	if err := adapter.LoadSession("session-pi-test", nil); err != nil {
		t.Fatalf("load session: %v", err)
	}
	if adapter.CurrentSessionID() != "session-pi-test" {
		t.Fatalf("expected session session-pi-test, got %s", adapter.CurrentSessionID())
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

var _ acp.ACPAdapter = (*PiAdapter)(nil)
