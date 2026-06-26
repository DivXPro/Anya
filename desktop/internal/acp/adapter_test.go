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
func (m *mockAdapter) LoadSession(acpSessionID string, history []Message) error { return nil }
func (m *mockAdapter) CurrentSessionID() string { return "" }
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

func TestRouterLoadSession(t *testing.T) {
	r := NewRouter()
	mock := &mockAdapter{}
	r.Register(mock)

	if err := r.LoadSession("mock", "acp-session-1", nil); err != nil {
		t.Fatalf("load session: %v", err)
	}

	if _, err := r.CurrentSessionID("missing"); err == nil {
		t.Fatal("expected error for unknown agent")
	}
}

func TestRouterUnknownAgent(t *testing.T) {
	r := NewRouter()
	_, err := r.Route("missing", "hi", nil)
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
}

func TestStreamEventHelpers(t *testing.T) {
	evt := StreamEvent{Type: "text_delta", Content: "hello"}
	if !evt.IsContent() {
		t.Fatal("expected content event")
	}
	if evt.IsDone() || evt.IsError() {
		t.Fatal("unexpected done/error")
	}
	if evt.IsSkippable() {
		t.Fatal("text_delta should not be skippable")
	}

	done := StreamEvent{Type: "done"}
	if !done.IsDone() {
		t.Fatal("expected done")
	}

	err := StreamEvent{Type: "error", Error: nil}
	if !err.IsError() {
		t.Fatal("expected error")
	}

	// Ensure String does not panic on nil error.
	_ = evt.String()
	_ = done.String()
}
