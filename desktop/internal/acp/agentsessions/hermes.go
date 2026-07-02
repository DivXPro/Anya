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
		`SELECT id, COALESCE(title, ''), COALESCE(cwd, ''), started_at
		 FROM sessions
		 WHERE archived = 0
		 ORDER BY started_at DESC
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
		var startedAt float64
		if err := rows.Scan(&s.ID, &s.Title, &s.CWD, &startedAt); err != nil {
			return nil, err
		}
		if strings.TrimSpace(s.Title) == "" {
			s.Title = s.ID
		}
		s.Title = trimTitle(s.Title)
		s.UpdatedAt = unixFloatTime(startedAt)
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
