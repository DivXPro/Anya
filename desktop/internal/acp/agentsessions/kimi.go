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

func ListKimiSessions(home string, limit int) ([]acp.AgentSession, error) {
	if limit <= 0 {
		return []acp.AgentSession{}, nil
	}
	path := filepath.Join(home, ".kimi-code", "session_index.jsonl")
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []acp.AgentSession{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var sessions []acp.AgentSession
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		var entry struct {
			SessionID  string `json:"sessionId"`
			SessionDir string `json:"sessionDir"`
			WorkDir    string `json:"workDir"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.SessionID == "" {
			continue
		}
		state := kimiSessionState(entry.SessionDir)
		if strings.TrimSpace(state.LastPrompt) == "" {
			continue
		}
		title := state.Title
		if strings.TrimSpace(title) == "" {
			title = state.LastPrompt
		}
		if strings.TrimSpace(title) == "" {
			title = entry.SessionID
		}
		updatedAt := state.UpdatedAt
		if updatedAt.IsZero() {
			updatedAt = fileModTime(filepath.Join(entry.SessionDir, "state.json"))
		}
		if updatedAt.IsZero() {
			updatedAt = fileModTime(entry.SessionDir)
		}
		sessions = append(sessions, acp.AgentSession{
			ID:        entry.SessionID,
			Title:     trimTitle(title),
			CWD:       entry.WorkDir,
			UpdatedAt: updatedAt,
			Source:    "kimi",
			CanResume: true,
		})
	}
	if err := scanner.Err(); err != nil {
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

type kimiState struct {
	Title      string
	LastPrompt string
	UpdatedAt  time.Time
}

func kimiSessionState(sessionDir string) kimiState {
	path := filepath.Join(sessionDir, "state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return kimiState{}
	}
	var raw struct {
		Title      string `json:"title"`
		LastPrompt string `json:"lastPrompt"`
		UpdatedAt  string `json:"updatedAt"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return kimiState{}
	}
	updatedAt, _ := time.Parse(time.RFC3339Nano, raw.UpdatedAt)
	return kimiState{
		Title:      raw.Title,
		LastPrompt: raw.LastPrompt,
		UpdatedAt:  updatedAt,
	}
}

func fileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
