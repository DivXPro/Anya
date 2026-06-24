CREATE TABLE IF NOT EXISTS authorized_devices (
    device_id     TEXT PRIMARY KEY,
    name          TEXT NOT NULL DEFAULT '',
    authorized_at TEXT NOT NULL DEFAULT (datetime('now')),
    last_seen_ip  TEXT,
    last_seen_at  TEXT,
    revoked       INTEGER NOT NULL DEFAULT 0
);
