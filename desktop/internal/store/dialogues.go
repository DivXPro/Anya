package store

import (
	"database/sql"

	"github.com/google/uuid"
)

func CreateDialogue(db *sql.DB, deviceID, agentID string) (*Dialogue, error) {
	d := &Dialogue{ID: uuid.NewString(), DeviceID: deviceID, AgentID: agentID}
	_, err := db.Exec(
		"INSERT INTO dialogues (id, device_id, agent_id) VALUES (?, ?, ?)",
		d.ID, d.DeviceID, d.AgentID,
	)
	if err != nil {
		return nil, err
	}
	syncDialogueCompatFields(d)
	return d, nil
}

func CloseDialogue(db *sql.DB, dialogueID string) error {
	_, err := db.Exec("UPDATE dialogues SET closed_at = datetime('now') WHERE id = ?", dialogueID)
	return err
}

func GetDialogue(db *sql.DB, dialogueID string) (*Dialogue, error) {
	d := &Dialogue{}
	err := db.QueryRow(
		`SELECT id, device_id, agent_id, agent_session_id, agent_session_provider, created_at, closed_at
		 FROM dialogues WHERE id = ?`,
		dialogueID,
	).Scan(&d.ID, &d.DeviceID, &d.AgentID, &d.AgentSessionID, &d.AgentSessionProvider, &d.CreatedAt, &d.ClosedAt)
	if err != nil {
		return nil, err
	}
	syncDialogueCompatFields(d)
	return d, nil
}

func GetOpenDialogueForDevice(db *sql.DB, deviceID string) (*Dialogue, error) {
	d := &Dialogue{}
	err := db.QueryRow(
		`SELECT id, device_id, agent_id, agent_session_id, agent_session_provider, created_at, closed_at
		 FROM dialogues WHERE device_id = ? AND closed_at IS NULL
		 ORDER BY created_at DESC LIMIT 1`,
		deviceID,
	).Scan(&d.ID, &d.DeviceID, &d.AgentID, &d.AgentSessionID, &d.AgentSessionProvider, &d.CreatedAt, &d.ClosedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	syncDialogueCompatFields(d)
	return d, nil
}

func UpdateDialogueAgentSession(db *sql.DB, dialogueID, agentSessionID, provider string) error {
	_, err := db.Exec(
		"UPDATE dialogues SET agent_session_id = ?, agent_session_provider = ? WHERE id = ?",
		agentSessionID, provider, dialogueID,
	)
	return err
}

func GetOpenDialogueForAgentSession(db *sql.DB, deviceID, agentID, agentSessionID string) (*Dialogue, error) {
	d := &Dialogue{}
	err := db.QueryRow(
		`SELECT id, device_id, agent_id, agent_session_id, agent_session_provider, created_at, closed_at
		 FROM dialogues
		 WHERE device_id = ? AND agent_id = ? AND agent_session_id = ? AND closed_at IS NULL
		 ORDER BY created_at DESC LIMIT 1`,
		deviceID, agentID, agentSessionID,
	).Scan(&d.ID, &d.DeviceID, &d.AgentID, &d.AgentSessionID, &d.AgentSessionProvider, &d.CreatedAt, &d.ClosedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return d, nil
}

func ListDialogues(db *sql.DB, limit, offset int) ([]Dialogue, error) {
	rows, err := db.Query(
		`SELECT id, device_id, agent_id, agent_session_id, agent_session_provider, created_at, closed_at
		 FROM dialogues ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var dialogues []Dialogue
	for rows.Next() {
		var d Dialogue
		if err := rows.Scan(&d.ID, &d.DeviceID, &d.AgentID, &d.AgentSessionID, &d.AgentSessionProvider, &d.CreatedAt, &d.ClosedAt); err != nil {
			return nil, err
		}
		syncDialogueCompatFields(&d)
		dialogues = append(dialogues, d)
	}
	return dialogues, nil
}

func syncDialogueCompatFields(d *Dialogue) {
	d.ACPSessionID = d.AgentSessionID
	d.ACPAgentID = d.AgentSessionProvider
}

func CreateSession(db *sql.DB, deviceID, agentID string) (*Session, error) {
	return CreateDialogue(db, deviceID, agentID)
}

func CloseSession(db *sql.DB, sessionID string) error {
	return CloseDialogue(db, sessionID)
}

func GetSession(db *sql.DB, sessionID string) (*Session, error) {
	return GetDialogue(db, sessionID)
}

func GetOpenSessionForDevice(db *sql.DB, deviceID string) (*Session, error) {
	return GetOpenDialogueForDevice(db, deviceID)
}

func UpdateSessionACPSession(db *sql.DB, sessionID, acpSessionID, acpAgentID string) error {
	return UpdateDialogueAgentSession(db, sessionID, acpSessionID, acpAgentID)
}

func ListSessions(db *sql.DB, limit, offset int) ([]Session, error) {
	dialogues, err := ListDialogues(db, limit, offset)
	if err != nil {
		return nil, err
	}
	return dialogues, nil
}
