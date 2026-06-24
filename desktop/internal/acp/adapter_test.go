package acp

import (
	"testing"
	"time"
)

type mockAdapter struct {
	events []StreamEvent
}

func (m *mockAdapter) Send(prompt string, history []Message) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, len(m.events))
	for _, e := range m.events {
		ch <- e
	}
	close(ch)
	return ch, nil
}
func (m *mockAdapter) Info() AgentInfo { return AgentInfo{ID: "mock", Name: "Mock", Command: "mock"} }
func (m *mockAdapter) IsRunning() bool  { return true }
func (m *mockAdapter) Stop() error      { return nil }

func TestRouterRoute(t *testing.T) {
	r := NewRouter()
	mock := &mockAdapter{events: []StreamEvent{
		{Type: "text_delta", Content: "Hello "},
		{Type: "text_delta", Content: "World"},
		{Type: "done"},
	}}
	r.Register(mock)

	ch, err := r.Route("mock", "hi", nil)
	if err != nil {
		t.Fatalf("route: %v", err)
	}

	var content string
	timeout := time.After(2 * time.Second)
loop:
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				break loop
			}
			if evt.IsContent() {
				content += evt.Content
			}
		case <-timeout:
			t.Fatal("timeout waiting for events")
		}
	}

	if content != "Hello World" {
		t.Errorf("expected 'Hello World', got '%s'", content)
	}
}
