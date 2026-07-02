package store

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

func InsertMessage(db *sql.DB, msg *Message) error {
	msg.ID = uuid.NewString()
	dialogueID := msg.DialogueID
	if dialogueID == "" {
		dialogueID = msg.SessionID
		msg.DialogueID = dialogueID
	}
	msg.SessionID = dialogueID
	_, err := db.Exec(
		`INSERT INTO messages (id, dialogue_id, role, content, audio_url, summary) VALUES (?, ?, ?, ?, ?, ?)`,
		msg.ID, dialogueID, msg.Role, msg.Content, msg.AudioURL, msg.Summary,
	)
	return err
}

func GetMessagesByDialogue(db *sql.DB, dialogueID string, limit, offset int) ([]Message, error) {
	rows, err := db.Query(
		`SELECT id, dialogue_id, role, content, audio_url, summary, created_at
		 FROM messages WHERE dialogue_id = ? ORDER BY created_at ASC LIMIT ? OFFSET ?`,
		dialogueID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.DialogueID, &m.Role, &m.Content, &m.AudioURL, &m.Summary, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.SessionID = m.DialogueID
		msgs = append(msgs, m)
	}
	return msgs, nil
}

func GetMessagesBySession(db *sql.DB, sessionID string, limit, offset int) ([]Message, error) {
	return GetMessagesByDialogue(db, sessionID, limit, offset)
}

func SearchMessages(db *sql.DB, query string, limit int) ([]Message, error) {
	rows, err := db.Query(
		`SELECT m.id, m.dialogue_id, m.role, m.content, m.audio_url, m.summary, m.created_at
		 FROM messages m WHERE m.content LIKE ? ORDER BY m.created_at DESC LIMIT ?`,
		fmt.Sprintf("%%%s%%", query), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.DialogueID, &m.Role, &m.Content, &m.AudioURL, &m.Summary, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.SessionID = m.DialogueID
		msgs = append(msgs, m)
	}
	return msgs, nil
}

func DeleteOldMessages(db *sql.DB, beforeDays int) (int64, error) {
	res, err := db.Exec(
		"DELETE FROM messages WHERE created_at < datetime('now', ?)",
		fmt.Sprintf("-%d days", beforeDays),
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func GetDialogueMessages(db *sql.DB, dialogueID string) ([]Message, error) {
	return GetMessagesByDialogue(db, dialogueID, 1000, 0)
}

func GetSessionMessages(db *sql.DB, sessionID string) ([]Message, error) {
	return GetDialogueMessages(db, sessionID)
}

// ListMessages returns recent messages across all sessions, newest first.
func ListMessages(db *sql.DB, limit, offset int) ([]Message, error) {
	rows, err := db.Query(
		`SELECT id, dialogue_id, role, content, audio_url, summary, created_at
		 FROM messages ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.DialogueID, &m.Role, &m.Content, &m.AudioURL, &m.Summary, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.SessionID = m.DialogueID
		msgs = append(msgs, m)
	}
	return msgs, nil
}
