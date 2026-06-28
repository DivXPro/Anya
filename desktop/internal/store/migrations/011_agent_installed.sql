-- Add installed status and per-platform install command to agents.
ALTER TABLE agents ADD COLUMN installed INTEGER NOT NULL DEFAULT 0;
ALTER TABLE agents ADD COLUMN install_command TEXT;
