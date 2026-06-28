package agentinstall

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"desktop/internal/store"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func setAgentCommand(t *testing.T, db *sql.DB, id, command string) {
	t.Helper()
	ag, err := store.GetAgent(db, id)
	if err != nil {
		t.Fatalf("get agent %s: %v", id, err)
	}
	ag.Command = command
	if err := store.UpdateAgent(db, ag); err != nil {
		t.Fatalf("update agent %s command: %v", id, err)
	}
}

func TestDetect_UpdatesInstalledAndVersion(t *testing.T) {
	db := testDB(t)
	if err := store.UpdateAgentInstalled(db, "claude-code", false); err != nil {
		t.Fatalf("reset installed: %v", err)
	}

	tmp := t.TempDir()
	writeFakeBinary(t, tmp, "claude", "Claude Code 0.2.0")
	t.Setenv("PATH", tmp)

	inst := New(nil, db)
	if err := inst.Detect("claude-code"); err != nil {
		t.Fatalf("detect: %v", err)
	}

	ag, err := store.GetAgent(db, "claude-code")
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if !ag.Installed {
		t.Fatal("expected agent to be marked installed")
	}
	if ag.Version == nil || !strings.Contains(*ag.Version, "0.2.0") {
		t.Fatalf("expected version 0.2.0, got %v", ag.Version)
	}
}

func TestDetect_BinaryNotFound(t *testing.T) {
	db := testDB(t)
	if err := store.UpdateAgentInstalled(db, "claude-code", true); err != nil {
		t.Fatalf("set installed: %v", err)
	}
	setAgentCommand(t, db, "claude-code", "not-a-real-binary --acp")

	// Empty PATH so the real claude cannot be found.
	t.Setenv("PATH", "")

	inst := New(nil, db)
	if err := inst.Detect("claude-code"); err != nil {
		t.Fatalf("detect: %v", err)
	}

	ag, err := store.GetAgent(db, "claude-code")
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if ag.Installed {
		t.Fatal("expected agent to be marked not installed")
	}
}

func TestDetectAll_UpdatesInstalledStatuses(t *testing.T) {
	db := testDB(t)
	setAgentCommand(t, db, "claude-code", "cc-fake --acp")
	setAgentCommand(t, db, "opencode", "oc-fake acp")
	if err := store.UpdateAgentInstalled(db, "claude-code", false); err != nil {
		t.Fatalf("reset claude-code installed: %v", err)
	}
	if err := store.UpdateAgentInstalled(db, "opencode", true); err != nil {
		t.Fatalf("reset opencode installed: %v", err)
	}

	tmp := t.TempDir()
	writeFakeBinary(t, tmp, "cc-fake", "cc 1.0")
	t.Setenv("PATH", tmp)

	inst := New(nil, db)
	if err := inst.DetectAll(); err != nil {
		t.Fatalf("detect all: %v", err)
	}

	cc, err := store.GetAgent(db, "claude-code")
	if err != nil {
		t.Fatalf("get claude-code: %v", err)
	}
	if !cc.Installed {
		t.Fatal("expected claude-code to be installed")
	}

	oc, err := store.GetAgent(db, "opencode")
	if err != nil {
		t.Fatalf("get opencode: %v", err)
	}
	if oc.Installed {
		t.Fatal("expected opencode to be not installed")
	}
}

func TestInstall_MarksAgentInstalled(t *testing.T) {
	db := testDB(t)
	if err := store.UpdateAgentInstalled(db, "claude-code", false); err != nil {
		t.Fatalf("reset installed: %v", err)
	}

	tmp := t.TempDir()
	writeFakeNPM(t, tmp)
	t.Setenv("PATH", tmp)

	inst := New(nil, db)
	if err := inst.Install("claude-code"); err != nil {
		t.Fatalf("install: %v", err)
	}
	if !inst.IsInstalling("claude-code") {
		t.Fatal("expected install to be running immediately after start")
	}

	deadline := time.Now().Add(5 * time.Second)
	for inst.IsInstalling("claude-code") && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
	}
	if inst.IsInstalling("claude-code") {
		t.Fatal("install did not finish in time")
	}

	ag, err := store.GetAgent(db, "claude-code")
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if !ag.Installed {
		t.Fatal("expected agent to be installed after successful install")
	}
	if ag.Version == nil || !strings.Contains(*ag.Version, "Claude Code") {
		t.Fatalf("expected version to be populated, got %v", ag.Version)
	}
}

func TestInstall_NoPackageManager(t *testing.T) {
	db := testDB(t)
	if err := store.UpdateAgentInstalled(db, "claude-code", false); err != nil {
		t.Fatalf("reset installed: %v", err)
	}

	// Ensure no package manager is found.
	t.Setenv("PATH", "")

	inst := New(nil, db)
	if err := inst.Install("claude-code"); err == nil {
		t.Fatal("expected error when no package manager is available")
	}
	if inst.IsInstalling("claude-code") {
		t.Fatal("expected install task to be cleared after failure")
	}
}

func TestInstall_UnknownAgent(t *testing.T) {
	db := testDB(t)
	tmp := t.TempDir()
	writeFakeNPM(t, tmp)
	t.Setenv("PATH", tmp)

	inst := New(nil, db)
	if err := inst.Install("not-real"); err == nil {
		t.Fatal("expected error for unknown agent")
	}
}
