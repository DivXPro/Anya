ALTER TABLE agents ADD COLUMN selected INTEGER NOT NULL DEFAULT 0;

-- Migrate existing data: if exactly one agent is enabled, mark it selected.
UPDATE agents SET selected = 1 WHERE enabled = 1;
UPDATE agents SET enabled = 1;
