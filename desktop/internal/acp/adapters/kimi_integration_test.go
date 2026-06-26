//go:build integration

package adapters

import (
	"os"
	"testing"
	"time"

	"desktop/internal/acp"
)

func TestKimiAdapterSendAndReceive(t *testing.T) {
	apiKey := os.Getenv("KIMI_API_KEY")
	if apiKey == "" {
		t.Skip("KIMI_API_KEY not set")
	}

	adapter := NewKimiAdapter(apiKey, "moonshot-v1-8k")
	defer adapter.Stop()

	ch, err := adapter.Send("say hi", nil)
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	var sawDelta, sawDone bool
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

	info := adapter.Info()
	if info.ID != "kimi" {
		t.Fatalf("unexpected adapter id: %s", info.ID)
	}
}

func TestKimiAdapterLoadSession(t *testing.T) {
	apiKey := os.Getenv("KIMI_API_KEY")
	if apiKey == "" {
		t.Skip("KIMI_API_KEY not set")
	}

	adapter := NewKimiAdapter(apiKey, "moonshot-v1-8k")
	defer adapter.Stop()

	ch1, err := adapter.Send("say hi", nil)
	if err != nil {
		t.Fatalf("first send: %v", err)
	}
	if err := drainStream(t, ch1, 60*time.Second); err != nil {
		t.Fatalf("first send stream: %v", err)
	}

	// Kimi adapter does not track real ACP sessions; LoadSession simply stores
	// the requested id so the caller can keep session continuity.
	if err := adapter.LoadSession("session-kimi-test", nil); err != nil {
		t.Fatalf("load session: %v", err)
	}
	if adapter.CurrentSessionID() != "session-kimi-test" {
		t.Fatalf("expected session session-kimi-test, got %s", adapter.CurrentSessionID())
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

var _ acp.ACPAdapter = (*KimiAdapter)(nil)
