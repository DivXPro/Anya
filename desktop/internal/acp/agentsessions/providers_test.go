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
	execFixture(t, db, `CREATE TABLE message (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		time_created INTEGER NOT NULL,
		time_updated INTEGER NOT NULL,
		data TEXT NOT NULL
	)`)
	execFixture(t, db, `INSERT INTO session (id, title, directory, time_updated, time_archived) VALUES
		('old', 'Old Session', '/tmp/old', 1000, NULL),
		('recent', 'Recent Session', '/tmp/recent', 3000, NULL),
		('empty-newer', 'Empty Newer Session', '/tmp/empty', 5000, NULL),
		('archived', 'Archived Session', '/tmp/archived', 4000, 4000)`)
	execFixture(t, db, `INSERT INTO message (id, session_id, time_created, time_updated, data) VALUES
		('m-old', 'old', 1000, 1000, '{"role":"user"}'),
		('m-recent-old', 'recent', 2000, 2000, '{"role":"user"}'),
		('m-recent-new', 'recent', 3500, 3500, '{"role":"user"}'),
		('m-archived', 'archived', 4500, 4500, '{"role":"user"}')`)

	got, err := ListOpenCodeSessions(home, 10)
	if err != nil {
		t.Fatalf("list opencode sessions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 visible sessions with chat, got %+v", got)
	}
	if got[0].ID != "recent" || got[0].Title != "Recent Session" || got[0].CWD != "/tmp/recent" {
		t.Fatalf("unexpected first session: %+v", got[0])
	}
	if got[0].Source != "opencode" || !got[0].CanResume {
		t.Fatalf("unexpected source/resume flags: %+v", got[0])
	}
	if !got[0].UpdatedAt.Equal(time.UnixMilli(3500)) {
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
	emptyPath := filepath.Join(projectDir, "empty-newer.jsonl")
	content := `{"type":"user","message":{"role":"user","content":[{"type":"text","text":"Older title"}]},"timestamp":"2026-07-01T01:00:00Z"}` + "\n" +
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"ok"}]}}` + "\n" +
		`{"type":"user","message":{"role":"user","content":[{"type":"text","text":"Recent Claude task"}]},"timestamp":"2026-07-01T02:00:00Z"}` + "\n"
	if err := os.WriteFile(sessionPath, []byte(content), 0644); err != nil {
		t.Fatalf("write claude jsonl: %v", err)
	}
	if err := os.WriteFile(emptyPath, []byte(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"empty"}]}}`+"\n"), 0644); err != nil {
		t.Fatalf("write empty claude jsonl: %v", err)
	}
	modTime := time.Unix(1234, 0)
	if err := os.Chtimes(sessionPath, modTime, modTime); err != nil {
		t.Fatalf("chtimes claude jsonl: %v", err)
	}
	emptyModTime := time.Unix(9999, 0)
	if err := os.Chtimes(emptyPath, emptyModTime, emptyModTime); err != nil {
		t.Fatalf("chtimes empty claude jsonl: %v", err)
	}

	got, err := ListClaudeSessions(home, 10)
	if err != nil {
		t.Fatalf("list claude sessions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 session, got %+v", got)
	}
	if got[0].ID != "session-1" || got[0].Title != "Recent Claude task" || got[0].CWD != "/tmp/claude/project" {
		t.Fatalf("unexpected session: %+v", got[0])
	}
	if got[0].Source != "claude-code" || !got[0].CanResume {
		t.Fatalf("unexpected source/resume flags: %+v", got[0])
	}
	wantUpdated, _ := time.Parse(time.RFC3339, "2026-07-01T02:00:00Z")
	if !got[0].UpdatedAt.Equal(wantUpdated) {
		t.Fatalf("updated_at = %s", got[0].UpdatedAt)
	}
}

func TestKimiProviderListsRecentSessions(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".kimi-code")
	sessionDir := filepath.Join(root, "sessions", "wd_project_abcd", "ses-recent")
	oldSessionDir := filepath.Join(root, "sessions", "wd_project_abcd", "ses-old")
	emptySessionDir := filepath.Join(root, "sessions", "wd_project_abcd", "ses-empty-newer")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("mkdir recent kimi session dir: %v", err)
	}
	if err := os.MkdirAll(oldSessionDir, 0755); err != nil {
		t.Fatalf("mkdir old kimi session dir: %v", err)
	}
	if err := os.MkdirAll(emptySessionDir, 0755); err != nil {
		t.Fatalf("mkdir empty kimi session dir: %v", err)
	}
	index := `{"sessionId":"ses-old","sessionDir":"` + oldSessionDir + `","workDir":"/tmp/kimi/old"}` + "\n" +
		`{"sessionId":"ses-empty-newer","sessionDir":"` + emptySessionDir + `","workDir":"/tmp/kimi/empty"}` + "\n" +
		`{"sessionId":"ses-recent","sessionDir":"` + sessionDir + `","workDir":"/tmp/kimi/recent"}` + "\n"
	if err := os.WriteFile(filepath.Join(root, "session_index.jsonl"), []byte(index), 0644); err != nil {
		t.Fatalf("write kimi session index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldSessionDir, "state.json"), []byte(`{
		"createdAt":"2026-07-01T01:00:00Z",
		"updatedAt":"2026-07-01T02:00:00Z",
		"title":"Old Kimi Session",
		"lastPrompt":"old prompt"
	}`), 0644); err != nil {
		t.Fatalf("write old kimi state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "state.json"), []byte(`{
		"createdAt":"2026-07-02T01:00:00Z",
		"updatedAt":"2026-07-02T02:00:00Z",
		"title":"",
		"lastPrompt":"Recent Kimi prompt"
	}`), 0644); err != nil {
		t.Fatalf("write recent kimi state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(emptySessionDir, "state.json"), []byte(`{
		"createdAt":"2026-07-03T01:00:00Z",
		"updatedAt":"2026-07-03T02:00:00Z",
		"title":"",
		"lastPrompt":""
	}`), 0644); err != nil {
		t.Fatalf("write empty kimi state: %v", err)
	}

	got, err := ListKimiSessions(home, 10)
	if err != nil {
		t.Fatalf("list kimi sessions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 kimi sessions with chat, got %+v", got)
	}
	if got[0].ID != "ses-recent" || got[0].Title != "Recent Kimi prompt" || got[0].CWD != "/tmp/kimi/recent" {
		t.Fatalf("unexpected first kimi session: %+v", got[0])
	}
	if got[0].Source != "kimi" || !got[0].CanResume {
		t.Fatalf("unexpected kimi source/resume flags: %+v", got[0])
	}
	wantUpdated, _ := time.Parse(time.RFC3339, "2026-07-02T02:00:00Z")
	if !got[0].UpdatedAt.Equal(wantUpdated) {
		t.Fatalf("updated_at = %s", got[0].UpdatedAt)
	}
}

func TestHermesProviderListsRecentSessions(t *testing.T) {
	home := t.TempDir()
	dbPath := filepath.Join(home, ".hermes", "state.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		t.Fatalf("mkdir hermes dir: %v", err)
	}
	db := openFixtureDB(t, dbPath)
	defer db.Close()
	execFixture(t, db, `CREATE TABLE sessions (
		id TEXT PRIMARY KEY,
		title TEXT,
		cwd TEXT,
		started_at REAL NOT NULL,
		archived INTEGER NOT NULL DEFAULT 0
	)`)
	execFixture(t, db, `CREATE TABLE messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT,
		timestamp REAL NOT NULL
	)`)
	execFixture(t, db, `INSERT INTO sessions (id, title, cwd, started_at, archived) VALUES
		('old', 'Old Hermes Session', '/tmp/hermes/old', 1000.25, 0),
		('recent', 'Recent Hermes Session', '/tmp/hermes/recent', 3000.5, 0),
		('empty-newer', 'Empty Hermes Session', '/tmp/hermes/empty', 5000.5, 0),
		('archived', 'Archived Hermes Session', '/tmp/hermes/archived', 4000.75, 1)`)
	execFixture(t, db, `INSERT INTO messages (session_id, role, content, timestamp) VALUES
		('old', 'user', 'old prompt', 1000.25),
		('recent', 'user', 'first recent prompt', 2000.25),
		('recent', 'user', 'latest recent prompt', 3500.5),
		('archived', 'user', 'archived prompt', 4500.75)`)

	got, err := ListHermesSessions(home, 10)
	if err != nil {
		t.Fatalf("list hermes sessions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 visible hermes sessions with chat, got %+v", got)
	}
	if got[0].ID != "recent" || got[0].Title != "Recent Hermes Session" || got[0].CWD != "/tmp/hermes/recent" {
		t.Fatalf("unexpected first hermes session: %+v", got[0])
	}
	if got[0].Source != "hermes" || !got[0].CanResume {
		t.Fatalf("unexpected hermes source/resume flags: %+v", got[0])
	}
	if !got[0].UpdatedAt.Equal(time.Unix(3500, 500_000_000)) {
		t.Fatalf("updated_at = %s", got[0].UpdatedAt)
	}
}

func TestHermesProviderSortsByLatestUserMessageTime(t *testing.T) {
	home := t.TempDir()
	dbPath := filepath.Join(home, ".hermes", "state.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		t.Fatalf("mkdir hermes dir: %v", err)
	}
	db := openFixtureDB(t, dbPath)
	defer db.Close()
	execFixture(t, db, `CREATE TABLE sessions (
		id TEXT PRIMARY KEY,
		title TEXT,
		cwd TEXT,
		started_at REAL NOT NULL,
		archived INTEGER NOT NULL DEFAULT 0
	)`)
	execFixture(t, db, `CREATE TABLE messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT,
		timestamp REAL NOT NULL
	)`)
	execFixture(t, db, `INSERT INTO sessions (id, title, cwd, started_at, archived) VALUES
		('opened-later', 'Opened Later', '/tmp/later', 5000, 0),
		('chatted-later', 'Chatted Later', '/tmp/chat', 1000, 0)`)
	execFixture(t, db, `INSERT INTO messages (session_id, role, content, timestamp) VALUES
		('opened-later', 'user', 'older chat', 2000),
		('chatted-later', 'user', 'newer chat', 4000)`)

	got, err := ListHermesSessions(home, 10)
	if err != nil {
		t.Fatalf("list hermes sessions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 hermes sessions, got %+v", got)
	}
	if got[0].ID != "chatted-later" {
		t.Fatalf("expected session with latest user chat first, got %+v", got)
	}
	if !got[0].UpdatedAt.Equal(time.Unix(4000, 0)) {
		t.Fatalf("updated_at = %s", got[0].UpdatedAt)
	}
}

func TestPiProviderListsRecentJsonl(t *testing.T) {
	home := t.TempDir()
	sessionDir := filepath.Join(home, ".pi", "agent", "sessions", "--tmp-pi-project--")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("mkdir pi session dir: %v", err)
	}
	oldPath := filepath.Join(sessionDir, "2026-07-01T01-00-00-000Z_old.jsonl")
	recentPath := filepath.Join(sessionDir, "2026-07-02T01-00-00-000Z_recent.jsonl")
	emptyPath := filepath.Join(sessionDir, "2026-07-03T01-00-00-000Z_empty.jsonl")
	if err := os.WriteFile(oldPath, []byte(
		`{"type":"session","id":"old","timestamp":"2026-07-01T01:00:00Z","cwd":"/tmp/pi/old"}`+"\n"+
			`{"type":"message","timestamp":"2026-07-01T01:30:00Z","message":{"role":"user","content":[{"type":"text","text":"Old Pi task"}]}}`+"\n",
	), 0644); err != nil {
		t.Fatalf("write old pi jsonl: %v", err)
	}
	if err := os.WriteFile(recentPath, []byte(
		`{"type":"session","id":"recent","timestamp":"2026-07-02T01:00:00Z","cwd":"/tmp/pi/recent"}`+"\n"+
			`{"type":"message","timestamp":"2026-07-02T01:30:00Z","message":{"role":"user","content":[{"type":"text","text":"First Pi task"}]}}`+"\n"+
			`{"type":"message","timestamp":"2026-07-02T02:00:00Z","message":{"role":"user","content":[{"type":"text","text":"Recent Pi task"}]}}`+"\n",
	), 0644); err != nil {
		t.Fatalf("write recent pi jsonl: %v", err)
	}
	if err := os.WriteFile(emptyPath, []byte(
		`{"type":"session","id":"empty","timestamp":"2026-07-03T01:00:00Z","cwd":"/tmp/pi/empty"}`+"\n"+
			`{"type":"message","timestamp":"2026-07-03T02:00:00Z","message":{"role":"assistant","content":[{"type":"text","text":"empty"}]}}`+"\n",
	), 0644); err != nil {
		t.Fatalf("write empty pi jsonl: %v", err)
	}

	got, err := ListPiSessions(home, 10)
	if err != nil {
		t.Fatalf("list pi sessions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 pi sessions with chat, got %+v", got)
	}
	if got[0].ID != recentPath || got[0].Title != "Recent Pi task" || got[0].CWD != "/tmp/pi/recent" {
		t.Fatalf("unexpected first pi session: %+v", got[0])
	}
	if got[0].Source != "pi" || !got[0].CanResume {
		t.Fatalf("unexpected pi source/resume flags: %+v", got[0])
	}
	wantUpdated, _ := time.Parse(time.RFC3339, "2026-07-02T02:00:00Z")
	if !got[0].UpdatedAt.Equal(wantUpdated) {
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
