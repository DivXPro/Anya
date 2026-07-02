package store

import "testing"

func TestCreateAndCloseDialogue(t *testing.T) {
	db := testDB(t)
	d, err := CreateDialogue(db, "device-1", "claude-code")
	if err != nil {
		t.Fatalf("create dialogue: %v", err)
	}
	if d.ID == "" || d.DeviceID != "device-1" || d.AgentID != "claude-code" {
		t.Fatalf("unexpected dialogue: %+v", d)
	}

	if err := CloseDialogue(db, d.ID); err != nil {
		t.Fatalf("close dialogue: %v", err)
	}

	got, err := GetDialogue(db, d.ID)
	if err != nil {
		t.Fatalf("get dialogue: %v", err)
	}
	if got.ClosedAt == nil {
		t.Fatal("expected closed_at")
	}
}

func TestUpdateDialogueAgentSession(t *testing.T) {
	db := testDB(t)
	d, err := CreateDialogue(db, "device-1", "claude-code")
	if err != nil {
		t.Fatalf("create dialogue: %v", err)
	}

	if err := UpdateDialogueAgentSession(db, d.ID, "agent-session-1", "claude-code"); err != nil {
		t.Fatalf("update dialogue agent session: %v", err)
	}

	got, err := GetDialogue(db, d.ID)
	if err != nil {
		t.Fatalf("get dialogue: %v", err)
	}
	if got.AgentSessionID == nil || *got.AgentSessionID != "agent-session-1" {
		t.Fatalf("agent session id = %v", got.AgentSessionID)
	}
	if got.AgentSessionProvider == nil || *got.AgentSessionProvider != "claude-code" {
		t.Fatalf("agent session provider = %v", got.AgentSessionProvider)
	}
}
