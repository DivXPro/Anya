PRAGMA foreign_keys=off;

CREATE TABLE IF NOT EXISTS dialogues (
    id                     TEXT PRIMARY KEY,
    device_id              TEXT NOT NULL,
    agent_id               TEXT NOT NULL,
    agent_session_id       TEXT,
    agent_session_provider TEXT,
    created_at             TEXT NOT NULL DEFAULT (datetime('now')),
    closed_at              TEXT
);

INSERT OR IGNORE INTO dialogues (
    id, device_id, agent_id, agent_session_id, agent_session_provider, created_at, closed_at
)
SELECT id, device_id, agent_id, acp_session_id, acp_agent_id, created_at, closed_at
FROM sessions;

CREATE TABLE IF NOT EXISTS messages_new (
    id          TEXT PRIMARY KEY,
    dialogue_id TEXT NOT NULL REFERENCES dialogues(id),
    role        TEXT NOT NULL CHECK(role IN ('user', 'assistant', 'system')),
    content     TEXT NOT NULL,
    audio_url   TEXT,
    summary     TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT OR IGNORE INTO messages_new (
    id, dialogue_id, role, content, audio_url, summary, created_at
)
SELECT id, session_id, role, content, audio_url, summary, created_at
FROM messages;

DROP TABLE IF EXISTS messages;
ALTER TABLE messages_new RENAME TO messages;
DROP TABLE IF EXISTS sessions;

CREATE INDEX IF NOT EXISTS idx_messages_dialogue ON messages(dialogue_id);
CREATE INDEX IF NOT EXISTS idx_messages_created ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_dialogues_agent_session ON dialogues(agent_session_id);

PRAGMA foreign_keys=on;
