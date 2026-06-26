package store

import (
	"database/sql"

	"github.com/google/uuid"
)

func CreateSession(db *sql.DB, deviceID, agentID string) (*Session, error) {
	s := &Session{ID: uuid.NewString(), DeviceID: deviceID, AgentID: agentID}
	_, err := db.Exec(
		"INSERT INTO sessions (id, device_id, agent_id) VALUES (?, ?, ?)",
		s.ID, s.DeviceID, s.AgentID,
	)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func CloseSession(db *sql.DB, sessionID string) error {
	_, err := db.Exec("UPDATE sessions SET closed_at = datetime('now') WHERE id = ?", sessionID)
	return err
}

func GetSession(db *sql.DB, sessionID string) (*Session, error) {
	s := &Session{}
	err := db.QueryRow(
		"SELECT id, device_id, agent_id, acp_session_id, acp_agent_id, created_at, closed_at FROM sessions WHERE id = ?",
		sessionID,
	).Scan(&s.ID, &s.DeviceID, &s.AgentID, &s.ACPSessionID, &s.ACPAgentID, &s.CreatedAt, &s.ClosedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func GetOpenSessionForDevice(db *sql.DB, deviceID string) (*Session, error) {
	s := &Session{}
	err := db.QueryRow(
		`SELECT id, device_id, agent_id, acp_session_id, acp_agent_id, created_at, closed_at
		 FROM sessions WHERE device_id = ? AND closed_at IS NULL
		 ORDER BY created_at DESC LIMIT 1`,
		deviceID,
	).Scan(&s.ID, &s.DeviceID, &s.AgentID, &s.ACPSessionID, &s.ACPAgentID, &s.CreatedAt, &s.ClosedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

func UpdateSessionACPSession(db *sql.DB, sessionID, acpSessionID, acpAgentID string) error {
	_, err := db.Exec(
		"UPDATE sessions SET acp_session_id = ?, acp_agent_id = ? WHERE id = ?",
		acpSessionID, acpAgentID, sessionID,
	)
	return err
}

func ListSessions(db *sql.DB, limit, offset int) ([]Session, error) {
	rows, err := db.Query(
		"SELECT id, device_id, agent_id, acp_session_id, acp_agent_id, created_at, closed_at FROM sessions ORDER BY created_at DESC LIMIT ? OFFSET ?",
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.DeviceID, &s.AgentID, &s.ACPSessionID, &s.ACPAgentID, &s.CreatedAt, &s.ClosedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}
