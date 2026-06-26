ALTER TABLE sessions ADD COLUMN acp_session_id TEXT;
ALTER TABLE sessions ADD COLUMN acp_agent_id TEXT;

CREATE INDEX IF NOT EXISTS idx_sessions_acp ON sessions(acp_session_id);
