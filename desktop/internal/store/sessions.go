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
		"SELECT id, device_id, agent_id, created_at, closed_at FROM sessions WHERE id = ?",
		sessionID,
	).Scan(&s.ID, &s.DeviceID, &s.AgentID, &s.CreatedAt, &s.ClosedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func ListSessions(db *sql.DB, limit, offset int) ([]Session, error) {
	rows, err := db.Query(
		"SELECT id, device_id, agent_id, created_at, closed_at FROM sessions ORDER BY created_at DESC LIMIT ? OFFSET ?",
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.DeviceID, &s.AgentID, &s.CreatedAt, &s.ClosedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}
