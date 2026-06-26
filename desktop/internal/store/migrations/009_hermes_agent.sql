-- +goose Up
INSERT OR IGNORE INTO agents (id, name, command, enabled) VALUES
    ('hermes', 'Hermes', 'hermes acp', 0);

-- +goose Down
DELETE FROM agents WHERE id = 'hermes';
