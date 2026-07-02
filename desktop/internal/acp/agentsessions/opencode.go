package agentsessions

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"desktop/internal/acp"

	_ "modernc.org/sqlite"
)

func ListOpenCodeSessions(home string, limit int) ([]acp.AgentSession, error) {
	if limit <= 0 {
		return []acp.AgentSession{}, nil
	}
	path := filepath.Join(home, ".local", "share", "opencode", "opencode.db")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return []acp.AgentSession{}, nil
		}
		return nil, err
	}
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro", path))
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(
		`SELECT s.id, s.title, s.directory, MAX(m.time_created) AS chat_at
		 FROM session s
		 JOIN message m ON m.session_id = s.id AND m.data LIKE '%"role":"user"%'
		 WHERE s.time_archived IS NULL
		 GROUP BY s.id, s.title, s.directory
		 ORDER BY chat_at DESC
		 LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []acp.AgentSession
	for rows.Next() {
		var s acp.AgentSession
		var chatMs int64
		if err := rows.Scan(&s.ID, &s.Title, &s.CWD, &chatMs); err != nil {
			return nil, err
		}
		s.UpdatedAt = time.UnixMilli(chatMs)
		s.Source = "opencode"
		s.CanResume = true
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}
