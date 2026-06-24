package store

import "testing"

func TestCreateAndCloseSession(t *testing.T) {
	db := testDB(t)
	s, err := CreateSession(db, "device-1", "claude-code")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if s.ID == "" {
		t.Error("expected session ID")
	}
	if s.DeviceID != "device-1" {
		t.Errorf("expected device-1, got %s", s.DeviceID)
	}

	if err := CloseSession(db, s.ID); err != nil {
		t.Fatalf("close session: %v", err)
	}

	got, err := GetSession(db, s.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if got.ClosedAt == nil {
		t.Error("expected closed_at to be set")
	}
}

func TestListAgents(t *testing.T) {
	db := testDB(t)
	agents, err := ListAgents(db)
	if err != nil {
		t.Fatalf("list agents: %v", err)
	}
	if len(agents) < 2 {
		t.Errorf("expected at least 2 seeded agents, got %d", len(agents))
	}
}
