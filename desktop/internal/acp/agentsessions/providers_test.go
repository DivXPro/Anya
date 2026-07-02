package agentsessions

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestCodexProviderListsRecentThreads(t *testing.T) {
	home := t.TempDir()
	dbPath := filepath.Join(home, ".codex", "state_5.sqlite")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		t.Fatalf("mkdir codex dir: %v", err)
	}
	db := openFixtureDB(t, dbPath)
	defer db.Close()
	execFixture(t, db, `CREATE TABLE threads (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		cwd TEXT NOT NULL,
		recency_at_ms INTEGER NOT NULL,
		archived INTEGER NOT NULL
	)`)
	execFixture(t, db, `INSERT INTO threads (id, title, cwd, recency_at_ms, archived) VALUES
		('old', 'Old Thread', '/tmp/old', 1000, 0),
		('recent', 'Recent Thread', '/tmp/recent', 3000, 0),
		('archived', 'Archived Thread', '/tmp/archived', 4000, 1)`)

	got, err := ListCodexSessions(home, 10)
	if err != nil {
		t.Fatalf("list codex sessions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 visible sessions, got %+v", got)
	}
	if got[0].ID != "recent" || got[0].Title != "Recent Thread" || got[0].CWD != "/tmp/recent" {
		t.Fatalf("unexpected first session: %+v", got[0])
	}
	if got[0].Source != "codex" || !got[0].CanResume {
		t.Fatalf("unexpected source/resume flags: %+v", got[0])
	}
	if !got[0].UpdatedAt.Equal(time.UnixMilli(3000)) {
		t.Fatalf("updated_at = %s", got[0].UpdatedAt)
	}
}

func TestOpenCodeProviderListsRecentSessions(t *testing.T) {
	home := t.TempDir()
	dbPath := filepath.Join(home, ".local", "share", "opencode", "opencode.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		t.Fatalf("mkdir opencode dir: %v", err)
	}
	db := openFixtureDB(t, dbPath)
	defer db.Close()
	execFixture(t, db, `CREATE TABLE session (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		directory TEXT NOT NULL,
		time_updated INTEGER NOT NULL,
		time_archived INTEGER
	)`)
	execFixture(t, db, `INSERT INTO session (id, title, directory, time_updated, time_archived) VALUES
		('old', 'Old Session', '/tmp/old', 1000, NULL),
		('recent', 'Recent Session', '/tmp/recent', 3000, NULL),
		('archived', 'Archived Session', '/tmp/archived', 4000, 4000)`)

	got, err := ListOpenCodeSessions(home, 10)
	if err != nil {
		t.Fatalf("list opencode sessions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 visible sessions, got %+v", got)
	}
	if got[0].ID != "recent" || got[0].Title != "Recent Session" || got[0].CWD != "/tmp/recent" {
		t.Fatalf("unexpected first session: %+v", got[0])
	}
	if got[0].Source != "opencode" || !got[0].CanResume {
		t.Fatalf("unexpected source/resume flags: %+v", got[0])
	}
	if !got[0].UpdatedAt.Equal(time.UnixMilli(3000)) {
		t.Fatalf("updated_at = %s", got[0].UpdatedAt)
	}
}

func TestClaudeProviderListsRecentProjectJsonl(t *testing.T) {
	home := t.TempDir()
	projectDir := filepath.Join(home, ".claude", "projects", "-tmp-claude-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("mkdir claude project dir: %v", err)
	}
	sessionPath := filepath.Join(projectDir, "session-1.jsonl")
	content := `{"type":"user","message":{"role":"user","content":[{"type":"text","text":"Older title"}]}}` + "\n" +
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"ok"}]}}` + "\n" +
		`{"type":"user","message":{"role":"user","content":[{"type":"text","text":"Recent Claude task"}]}}` + "\n"
	if err := os.WriteFile(sessionPath, []byte(content), 0644); err != nil {
		t.Fatalf("write claude jsonl: %v", err)
	}
	modTime := time.Unix(1234, 0)
	if err := os.Chtimes(sessionPath, modTime, modTime); err != nil {
		t.Fatalf("chtimes claude jsonl: %v", err)
	}

	got, err := ListClaudeSessions(home, 10)
	if err != nil {
		t.Fatalf("list claude sessions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 session, got %+v", got)
	}
	if got[0].ID != sessionPath || got[0].Title != "Recent Claude task" || got[0].CWD != "/tmp/claude/project" {
		t.Fatalf("unexpected session: %+v", got[0])
	}
	if got[0].Source != "claude-code" || !got[0].CanResume {
		t.Fatalf("unexpected source/resume flags: %+v", got[0])
	}
	if !got[0].UpdatedAt.Equal(modTime) {
		t.Fatalf("updated_at = %s", got[0].UpdatedAt)
	}
}

func openFixtureDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open fixture db: %v", err)
	}
	return db
}

func execFixture(t *testing.T, db *sql.DB, stmt string) {
	t.Helper()
	if _, err := db.Exec(stmt); err != nil {
		t.Fatalf("exec fixture: %v", err)
	}
}
