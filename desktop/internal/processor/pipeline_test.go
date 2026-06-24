package processor

import (
	"strings"
	"testing"

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
