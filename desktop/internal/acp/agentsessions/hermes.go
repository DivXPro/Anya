package agentsessions

import (
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"desktop/internal/acp"

	_ "modernc.org/sqlite"
)

func ListHermesSessions(home string, limit int) ([]acp.AgentSession, error) {
	if limit <= 0 {
		return []acp.AgentSession{}, nil
	}
	path := filepath.Join(home, ".hermes", "state.db")
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
		`SELECT s.id, COALESCE(s.title, ''), COALESCE(s.cwd, ''), MAX(m.timestamp) AS chat_at
		 FROM sessions s
		 JOIN messages m ON m.session_id = s.id AND m.role = 'user'
		 WHERE s.archived = 0
		 GROUP BY s.id, s.title, s.cwd
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
		var chatAt float64
		if err := rows.Scan(&s.ID, &s.Title, &s.CWD, &chatAt); err != nil {
			return nil, err
		}
		if strings.TrimSpace(s.Title) == "" {
			s.Title = s.ID
		}
		s.Title = trimTitle(s.Title)
		s.UpdatedAt = unixFloatTime(chatAt)
		s.Source = "hermes"
		s.CanResume = true
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func unixFloatTime(seconds float64) time.Time {
	sec, frac := math.Modf(seconds)
	return time.Unix(int64(sec), int64(frac*1_000_000_000))
}
