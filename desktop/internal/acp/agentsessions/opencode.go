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
		`SELECT id, title, directory, time_updated
		 FROM session
		 WHERE time_archived IS NULL
		 ORDER BY time_updated DESC
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
		var updatedMs int64
		if err := rows.Scan(&s.ID, &s.Title, &s.CWD, &updatedMs); err != nil {
			return nil, err
		}
		s.UpdatedAt = time.UnixMilli(updatedMs)
		s.Source = "opencode"
		s.CanResume = true
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}
