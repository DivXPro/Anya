package agentinstall

import (
	"database/sql"
	"fmt"
	"path/filepath"
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
	t.Setenv("PATH", tmp)

	inst := New(nil, db)
	// Inject a hermetic install command that drops a fake claude on PATH, so the
	// end-to-end pipeline (run -> detect -> mark installed) is exercised without
	// running the real vendor installer over the network.
	inst.buildCmd = func(id string, isUpdate bool) ([]string, string, error) {
		script := fmt.Sprintf("cat > %q <<'EOF'\n#!/bin/sh\necho \"Claude Code 0.1.0\"\nEOF\nchmod +x %q\n",
			filepath.Join(tmp, "claude"), filepath.Join(tmp, "claude"))
		return []string{"/bin/sh", "-c", script}, "sh -c fake-install", nil
	}

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

func TestInstall_SyncBuildErrorClearsTask(t *testing.T) {
	db := testDB(t)
	inst := New(nil, db)
	inst.buildCmd = func(id string, isUpdate bool) ([]string, string, error) {
		return nil, "", fmt.Errorf("no package manager found")
	}

	if err := inst.Install("claude-code"); err == nil {
		t.Fatal("expected error when the command cannot be built")
	}
	if inst.IsInstalling("claude-code") {
		t.Fatal("expected no install task after a synchronous build failure")
	}
}

func TestBuildPMCommand_NoPackageManager(t *testing.T) {
	// With an empty PATH no package manager can be located, so the npm-family
	// path must fail (the caller then surfaces the manual-install command).
	t.Setenv("PATH", "")
	info := Registry["claude-code"]
	if _, _, err := buildPMCommand(info, ""); err == nil {
		t.Fatal("expected error when no package manager is available")
	}
}

func TestCancel_StopsRunningInstall(t *testing.T) {
	db := testDB(t)
	inst := New(nil, db)
	// A long-running command stands in for a stalled installer.
	inst.buildCmd = func(id string, isUpdate bool) ([]string, string, error) {
		return []string{"/bin/sh", "-c", "sleep 30"}, "sleep 30", nil
	}

	if err := inst.Install("claude-code"); err != nil {
		t.Fatalf("install: %v", err)
	}

	// Wait until it is actually running.
	deadline := time.Now().Add(2 * time.Second)
	for !inst.IsInstalling("claude-code") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !inst.IsInstalling("claude-code") {
		t.Fatal("expected install to be running")
	}

	inst.Cancel("claude-code")

	// Cancel must tear the process down and clear the task promptly, well before
	// the 30s sleep would end.
	deadline = time.Now().Add(5 * time.Second)
	for inst.IsInstalling("claude-code") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if inst.IsInstalling("claude-code") {
		t.Fatal("expected cancel to stop the install promptly")
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
