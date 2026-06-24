package store

import (
	"testing"
)

func TestInsertAndGetMessages(t *testing.T) {
	db := testDB(t)
	session, err := CreateSession(db, "test-device", "claude-code")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	msg := &Message{SessionID: session.ID, Role: "user", Content: "hello"}
	if err := InsertMessage(db, msg); err != nil {
		t.Fatalf("insert message: %v", err)
	}
	if msg.ID == "" {
		t.Error("expected message ID to be set")
	}

	msgs, err := GetMessagesBySession(db, session.ID, 10, 0)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content != "hello" {
		t.Errorf("expected 'hello', got '%s'", msgs[0].Content)
	}
}
