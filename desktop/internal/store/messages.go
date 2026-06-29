package store

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

func InsertMessage(db *sql.DB, msg *Message) error {
	msg.ID = uuid.NewString()
	_, err := db.Exec(
		`INSERT INTO messages (id, session_id, role, content, audio_url, summary) VALUES (?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.SessionID, msg.Role, msg.Content, msg.AudioURL, msg.Summary,
	)
	return err
}

func GetMessagesBySession(db *sql.DB, sessionID string, limit, offset int) ([]Message, error) {
	rows, err := db.Query(
		`SELECT id, session_id, role, content, audio_url, summary, created_at
		 FROM messages WHERE session_id = ? ORDER BY created_at ASC LIMIT ? OFFSET ?`,
		sessionID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.AudioURL, &m.Summary, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

func SearchMessages(db *sql.DB, query string, limit int) ([]Message, error) {
	rows, err := db.Query(
		`SELECT m.id, m.session_id, m.role, m.content, m.audio_url, m.summary, m.created_at
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
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.AudioURL, &m.Summary, &m.CreatedAt); err != nil {
			return nil, err
		}
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

func GetSessionMessages(db *sql.DB, sessionID string) ([]Message, error) {
	return GetMessagesBySession(db, sessionID, 1000, 0)
}

func GetLastAssistantMessage(db *sql.DB, sessionID string) (*Message, error) {
	row := db.QueryRow(
		`SELECT id, session_id, role, content, audio_url, summary, created_at
		 FROM messages WHERE session_id = ? AND role = ? ORDER BY created_at DESC LIMIT 1`,
		sessionID, "assistant",
	)
	var m Message
	if err := row.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.AudioURL, &m.Summary, &m.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

func UpdateMessageContent(db *sql.DB, messageID, content string) error {
	_, err := db.Exec(
		`UPDATE messages SET content = ? WHERE id = ?`,
		content, messageID,
	)
	return err
}

// ListMessages returns recent messages across all sessions, newest first.
func ListMessages(db *sql.DB, limit, offset int) ([]Message, error) {
	rows, err := db.Query(
		`SELECT id, session_id, role, content, audio_url, summary, created_at
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
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.AudioURL, &m.Summary, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}
