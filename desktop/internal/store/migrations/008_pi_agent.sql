-- +goose Up
-- Add Pi as an RPC-based agent option.
INSERT OR IGNORE INTO agents (id, name, command, enabled) VALUES
    ('pi', 'Pi', 'pi --mode rpc --no-session', 0);

-- +goose Down
DELETE FROM agents WHERE id = 'pi';
