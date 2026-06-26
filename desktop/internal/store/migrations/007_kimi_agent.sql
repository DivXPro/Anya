-- Add Kimi Code as an ACP-based agent option.
INSERT OR IGNORE INTO agents (id, name, command, enabled) VALUES
    ('kimi', 'Kimi Code', 'kimi acp', 0);
