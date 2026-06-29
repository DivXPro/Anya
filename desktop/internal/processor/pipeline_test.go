package processor

import (
	"strings"
	"sync"
	"testing"
	"time"

	"desktop/internal/acp"
)

func TestAssistantResponseExcludesThinkingAndToolUse(t *testing.T) {
	events := make(chan acp.StreamEvent, 4)
	events <- acp.StreamEvent{Type: "thinking", Content: "private chain of thought"}
	events <- acp.StreamEvent{Type: "tool_use", Content: "internal tool payload"}
	events <- acp.StreamEvent{Type: "text_delta", Content: "用户可以看到的答案。"}
	events <- acp.StreamEvent{Type: "done"}
	close(events)

	resp, result, err := NewPipeline(events).Process()
	if err != nil {
		t.Fatalf("process response: %v", err)
	}
	if result != ResultComplete {
		t.Fatalf("expected complete result, got %v", result)
	}
	if resp.Content != "用户可以看到的答案。" {
		t.Fatalf("expected only user-visible answer, got %q", resp.Content)
	}
	forbidden := []string{"private chain of thought", "internal tool payload"}
	for _, text := range forbidden {
		if strings.Contains(resp.Content, text) || strings.Contains(resp.Summary, text) {
			t.Fatalf("private agent event leaked into response: %q", text)
		}
	}
}

// A backend that keeps streaming without ever sending done/error must still be
// bounded by the hard max-turn cap, so the caller regains control.
func TestPipelineMaxTurnTimeoutBoundsEndlessStream(t *testing.T) {
	events := make(chan acp.StreamEvent)
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		ticker := time.NewTicker(2 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				select {
				case events <- acp.StreamEvent{Type: "thinking", Content: "still going"}:
				case <-stop:
					return
				}
			}
		}
	}()

	p := NewPipeline(events)
	// Idle timeout must not fire (events arrive every 2ms); only the hard cap can stop it.
	p.noResponseTimeout = 2 * time.Second
	p.heartbeatInterval = 10 * time.Millisecond
	p.maxTurnTimeout = 60 * time.Millisecond

	start := time.Now()
	_, result, err := p.Process()
	if result != ResultExecTimeout {
		t.Fatalf("expected ResultExecTimeout, got %v", result)
	}
	if err == nil {
		t.Fatal("expected an error describing the max-turn timeout")
	}
	if elapsed := time.Since(start); elapsed > 1*time.Second {
		t.Fatalf("max-turn cap took too long to fire: %v", elapsed)
	}
}

// When the backend goes completely silent, the idle timeout returns promptly.
func TestPipelineNoResponseTimeoutOnSilence(t *testing.T) {
	events := make(chan acp.StreamEvent) // never sends, never closes
	p := NewPipeline(events)
	p.noResponseTimeout = 40 * time.Millisecond
	p.heartbeatInterval = 10 * time.Millisecond
	p.maxTurnTimeout = 5 * time.Second

	_, result, err := p.Process()
	if result != ResultNoResponseTimeout {
		t.Fatalf("expected ResultNoResponseTimeout, got %v", result)
	}
	if err == nil {
		t.Fatal("expected an error describing the no-response timeout")
	}
}

// The heartbeat callback must fire periodically while a turn is still pending,
// so the device keeps receiving a liveness signal.
func TestPipelineHeartbeatFiresWhileWaiting(t *testing.T) {
	events := make(chan acp.StreamEvent) // silent until idle timeout
	p := NewPipeline(events)
	p.noResponseTimeout = 120 * time.Millisecond
	p.heartbeatInterval = 20 * time.Millisecond
	p.maxTurnTimeout = 5 * time.Second

	var mu sync.Mutex
	beats := 0
	p.SetHeartbeatCallback(func() {
		mu.Lock()
		beats++
		mu.Unlock()
	})

	if _, result, _ := p.Process(); result != ResultNoResponseTimeout {
		t.Fatalf("expected ResultNoResponseTimeout, got %v", result)
	}
	mu.Lock()
	defer mu.Unlock()
	if beats < 1 {
		t.Fatalf("expected at least one heartbeat while waiting, got %d", beats)
	}
}
