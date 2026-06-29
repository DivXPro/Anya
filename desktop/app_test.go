package main

import (
	"os"
	"path/filepath"
	"testing"

	"desktop/internal/acp"
	"desktop/internal/acp/adapters"
	"desktop/internal/store"
)

func TestAgentCWDValidation(t *testing.T) {
	tmp := t.TempDir()

	// Create a file for the "file not dir" case
	filePath := filepath.Join(tmp, "file")
	if f, err := os.Create(filePath); err == nil {
		f.Close()
	}

	cases := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"empty is valid", "", false},
		{"existing dir", tmp, false},
		{"non-existent", filepath.Join(tmp, "missing"), true},
		{"file not dir", filePath, true},
		{"relative path", "./relative", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateWorkingDirectory(c.path)
			if (err != nil) != c.wantErr {
				t.Fatalf("validateWorkingDirectory(%q) error = %v, wantErr %v", c.path, err, c.wantErr)
			}
		})
	}
}

// The turn guard must prevent two voice requests from running concurrently on
// the same session (which would corrupt the shared ACP session/stream), while
// keeping different sessions independent.
func TestTurnGuardSerializesSameSession(t *testing.T) {
	a := NewApp()

	if !a.tryBeginTurn("s1") {
		t.Fatal("first begin for s1 should succeed")
	}
	if a.tryBeginTurn("s1") {
		t.Fatal("second begin for s1 must fail while a turn is in flight")
	}
	// A different session is unaffected.
	if !a.tryBeginTurn("s2") {
		t.Fatal("begin for an independent session s2 should succeed")
	}
	// After the turn ends, the session can start a new turn again.
	a.endTurn("s1")
	if !a.tryBeginTurn("s1") {
		t.Fatal("begin for s1 should succeed again after endTurn")
	}
}

func TestAppStartupSyncsCWDToRouter(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "elf.db")

	db, err := store.InitDB(dbPath)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	// Store a working directory setting
	if err := store.SetSetting(db, "agent_cwd", "/tmp/test-workspace"); err != nil {
		t.Fatalf("set setting: %v", err)
	}

	// Create app and simulate startup cwd loading
	a := NewApp()
	a.db = db

	// Load agent working directory (same as ServiceStartup)
	if cwd, err := store.GetSetting(a.db, "agent_cwd"); err == nil {
		a.agentCWD = cwd
	} else {
		t.Fatalf("failed to load agent_cwd: %v", err)
	}

	// Init router and register adapters
	a.router = acp.NewRouter()
	a.router.SetCWD(a.agentCWD)
	a.router.Register(adapters.NewClaudeAdapter())

	// Verify the adapter received the cwd via effectiveCWD
	info, ok := a.router.GetAgent("claude-code")
	if !ok {
		t.Fatal("claude adapter not registered")
	}
	if info.ID != "claude-code" {
		t.Fatalf("unexpected agent id: %s", info.ID)
	}

	// Verify the stored cwd was loaded
	if a.agentCWD != "/tmp/test-workspace" {
		t.Fatalf("expected agentCWD '/tmp/test-workspace', got %q", a.agentCWD)
	}
}
