package main

import (
	"testing"
)

func TestExtractAskUserMarker(t *testing.T) {
	content := `Some intro text.
[ASK_USER] {"question": "Pick a color?", "options": [{"id": "A", "label": "Red"}, {"id": "B", "label": "Blue"}]} [/ASK_USER]
More text.`
	clean, ask := extractAskUserMarker(content)
	if ask == nil {
		t.Fatal("expected ask prompt")
	}
	if ask.Question != "Pick a color?" {
		t.Errorf("question=%q, want %q", ask.Question, "Pick a color?")
	}
	if len(ask.Options) != 2 {
		t.Fatalf("options len=%d, want 2", len(ask.Options))
	}
	if ask.Options[0].ID != "A" || ask.Options[0].Label != "Red" {
		t.Errorf("option[0]=%+v", ask.Options[0])
	}
	if ask.Options[1].ID != "B" || ask.Options[1].Label != "Blue" {
		t.Errorf("option[1]=%+v", ask.Options[1])
	}
	if clean == "" {
		t.Error("expected non-empty cleaned content")
	}
}

func TestExtractAskUserMarkerNoMarker(t *testing.T) {
	content := "Just a normal reply."
	clean, ask := extractAskUserMarker(content)
	if ask != nil {
		t.Fatal("expected no ask prompt")
	}
	if clean != content {
		t.Errorf("clean=%q, want %q", clean, content)
	}
}
