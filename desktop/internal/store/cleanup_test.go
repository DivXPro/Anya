package store

import (
	"testing"
)

func TestDeleteOldMessages(t *testing.T) {
	db := testDB(t)
	session, err := CreateSession(db, "test-device", "claude-code")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	msg := &Message{SessionID: session.ID, Role: "user", Content: "old"}
	if err := InsertMessage(db, msg); err != nil {
		t.Fatalf("insert message: %v", err)
	}

	if _, err := db.Exec("UPDATE messages SET created_at = datetime('now', '-91 days') WHERE id = ?", msg.ID); err != nil {
		t.Fatalf("backdate message: %v", err)
	}

	deleted, err := DeleteOldMessages(db, 90)
	if err != nil {
		t.Fatalf("delete old: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	msgs, err := GetSessionMessages(db, session.ID)
	if err != nil {
		t.Fatalf("get session messages: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}
