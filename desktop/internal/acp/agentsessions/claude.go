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

func ListClaudeSessions(home string, limit int) ([]acp.AgentSession, error) {
	if limit <= 0 {
		return []acp.AgentSession{}, nil
	}
	root := filepath.Join(home, ".claude", "projects")
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
		info, err := d.Info()
		if err != nil {
			return err
		}
		cwd := decodeClaudeProjectDir(filepath.Base(filepath.Dir(path)))
		chat := claudeSessionChat(path)
		if !chat.HasChat {
			return nil
		}
		updatedAt := chat.UpdatedAt
		if updatedAt.IsZero() {
			updatedAt = info.ModTime()
		}
		sessions = append(sessions, acp.AgentSession{
			ID:        strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
			Title:     chat.Title,
			CWD:       cwd,
			UpdatedAt: updatedAt,
			Source:    "claude-code",
			CanResume: true,
		})
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

func decodeClaudeProjectDir(encoded string) string {
	if encoded == "" {
		return ""
	}
	if strings.HasPrefix(encoded, "-") {
		return "/" + strings.ReplaceAll(strings.TrimPrefix(encoded, "-"), "-", "/")
	}
	return strings.ReplaceAll(encoded, "-", string(filepath.Separator))
}

type jsonlChatInfo struct {
	Title     string
	UpdatedAt time.Time
	HasChat   bool
}

func claudeSessionChat(path string) jsonlChatInfo {
	file, err := os.Open(path)
	if err != nil {
		return jsonlChatInfo{}
	}
	defer file.Close()

	var info jsonlChatInfo
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		text, updatedAt, ok := userChatFromJSONLine(scanner.Bytes())
		if ok {
			info.Title = text
			if !updatedAt.IsZero() {
				info.UpdatedAt = updatedAt
			}
			info.HasChat = true
		}
	}
	info.Title = trimTitle(info.Title)
	return info
}

func userChatFromJSONLine(line []byte) (string, time.Time, bool) {
	text := userTextFromClaudeLine(line)
	if text == "" {
		return "", time.Time{}, false
	}
	var raw struct {
		Timestamp string `json:"timestamp"`
	}
	if err := json.Unmarshal(line, &raw); err != nil {
		return text, time.Time{}, true
	}
	updatedAt, _ := time.Parse(time.RFC3339Nano, raw.Timestamp)
	return text, updatedAt, true
}

func userTextFromClaudeLine(line []byte) string {
	var raw struct {
		Type    string `json:"type"`
		Message struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &raw); err != nil {
		return ""
	}
	if raw.Type != "user" && raw.Message.Role != "user" {
		return ""
	}

	var text string
	if err := json.Unmarshal(raw.Message.Content, &text); err == nil {
		return text
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw.Message.Content, &parts); err != nil {
		return ""
	}
	for _, p := range parts {
		if p.Type == "text" && strings.TrimSpace(p.Text) != "" {
			return p.Text
		}
	}
	return ""
}

func trimTitle(title string) string {
	title = strings.Join(strings.Fields(title), " ")
	runes := []rune(title)
	const maxRunes = 80
	if len(runes) > maxRunes {
		return string(runes[:maxRunes])
	}
	return title
}
