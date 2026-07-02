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

func ListCodexSessions(home string, limit int) ([]acp.AgentSession, error) {
	if limit <= 0 {
		return []acp.AgentSession{}, nil
	}
	path := filepath.Join(home, ".codex", "state_5.sqlite")
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
		`SELECT id, title, cwd, recency_at_ms
		 FROM threads
		 WHERE archived = 0
		 ORDER BY recency_at_ms DESC
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
		s.Source = "codex"
		s.CanResume = true
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}
