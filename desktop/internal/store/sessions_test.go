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

func TestGetOpenSessionForDevice(t *testing.T) {
	db := testDB(t)

	// No open session initially.
	if s, err := GetOpenSessionForDevice(db, "device-1"); err != nil {
		t.Fatalf("get open session: %v", err)
	} else if s != nil {
		t.Fatal("expected no open session")
	}

	s, err := CreateSession(db, "device-1", "claude-code")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	got, err := GetOpenSessionForDevice(db, "device-1")
	if err != nil {
		t.Fatalf("get open session: %v", err)
	}
	if got == nil || got.ID != s.ID {
		t.Fatalf("expected session %s, got %v", s.ID, got)
	}

	if err := CloseSession(db, s.ID); err != nil {
		t.Fatalf("close session: %v", err)
	}
	if got, err := GetOpenSessionForDevice(db, "device-1"); err != nil {
		t.Fatalf("get open session: %v", err)
	} else if got != nil {
		t.Fatal("expected no open session after close")
	}
}

func TestUpdateSessionACPSession(t *testing.T) {
	db := testDB(t)
	s, err := CreateSession(db, "device-1", "claude-code")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := UpdateSessionACPSession(db, s.ID, "acp-session-1", "claude-code"); err != nil {
		t.Fatalf("update acp session: %v", err)
	}

	got, err := GetSession(db, s.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if got.ACPSessionID == nil || *got.ACPSessionID != "acp-session-1" {
		t.Fatalf("expected acp session id acp-session-1, got %v", got.ACPSessionID)
	}
	if got.ACPAgentID == nil || *got.ACPAgentID != "claude-code" {
		t.Fatalf("expected acp agent id claude-code, got %v", got.ACPAgentID)
	}
}
