CREATE TABLE IF NOT EXISTS sessions (
    id         TEXT PRIMARY KEY,
    device_id  TEXT NOT NULL,
    agent_id   TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    closed_at  TEXT
);

CREATE TABLE IF NOT EXISTS messages (
    id         TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id),
    role       TEXT NOT NULL CHECK(role IN ('user', 'assistant', 'system')),
    content    TEXT NOT NULL,
    audio_url  TEXT,
    summary    TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);
CREATE INDEX IF NOT EXISTS idx_messages_created ON messages(created_at);

CREATE TABLE IF NOT EXISTS agents (
    id      TEXT PRIMARY KEY,
    name    TEXT NOT NULL,
    command TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    config  TEXT
);

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT OR IGNORE INTO agents (id, name, command, enabled) VALUES
    ('claude-code', 'Claude Code', 'claude --acp', 0),
    ('opencode', 'OpenCode', 'opencode acp', 0);

INSERT OR IGNORE INTO settings (key, value) VALUES
    ('stt_engine', 'whisper'),
    ('stt_language', 'zh'),
    ('tts_engine', 'edge-tts'),
    ('tts_speed', '+0%'),
    ('tts_enabled', 'true');
