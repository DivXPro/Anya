-- Add OpenAI Codex CLI as an ACP-based agent option.
INSERT OR IGNORE INTO agents (id, name, command, enabled) VALUES
    ('codex', 'Codex', 'codex app-server --stdio', 0);
