package agentsessions

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"desktop/internal/acp"
)

func ListPiSessions(home string, limit int) ([]acp.AgentSession, error) {
	if limit <= 0 {
		return []acp.AgentSession{}, nil
	}
	root := filepath.Join(home, ".pi", "agent", "sessions")
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return []acp.AgentSession{}, nil
		}
		return nil, err
	}

	var sessions []acp.AgentSession
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		session := piSessionFromFile(path)
		if session.ID == "" {
			return nil
		}
		sessions = append(sessions, session)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})
	if len(sessions) > limit {
		sessions = sessions[:limit]
	}
	return sessions, nil
}

func piSessionFromFile(path string) acp.AgentSession {
	file, err := os.Open(path)
	if err != nil {
		return acp.AgentSession{}
	}
	defer file.Close()

	var cwd string
	var updatedAt time.Time
	var title string
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if cwd == "" || updatedAt.IsZero() {
			var raw struct {
				Type      string `json:"type"`
				CWD       string `json:"cwd"`
				Timestamp string `json:"timestamp"`
			}
			if err := json.Unmarshal(line, &raw); err == nil && raw.Type == "session" {
				cwd = raw.CWD
				updatedAt, _ = time.Parse(time.RFC3339Nano, raw.Timestamp)
			}
		}
		if text := userTextFromClaudeLine(line); text != "" {
			title = text
		}
	}
	if strings.TrimSpace(title) == "" {
		title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if updatedAt.IsZero() {
		updatedAt = fileModTime(path)
	}
	return acp.AgentSession{
		ID:        path,
		Title:     trimTitle(title),
		CWD:       cwd,
		UpdatedAt: updatedAt,
		Source:    "pi",
		CanResume: true,
	}
}
