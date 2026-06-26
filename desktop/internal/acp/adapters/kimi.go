package adapters

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"desktop/internal/acp"
)

const kimiBaseURL = "https://api.moonshot.cn/v1"

type KimiAdapter struct {
	info      acp.AgentInfo
	apiKey    string
	model     string
	mu        sync.Mutex
	sessionID string
}

func NewKimiAdapter(apiKey, model string) *KimiAdapter {
	if model == "" {
		model = "moonshot-v1-8k"
	}
	return &KimiAdapter{
		info: acp.AgentInfo{
			ID:      "kimi",
			Name:    "Kimi",
			Command: "kimi-api",
		},
		apiKey: apiKey,
		model:  model,
	}
}

func (a *KimiAdapter) Info() acp.AgentInfo { return a.info }

func (a *KimiAdapter) CurrentSessionID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sessionID
}

func (a *KimiAdapter) IsRunning() bool { return a.apiKey != "" }

func (a *KimiAdapter) Stop() error { return nil }

func (a *KimiAdapter) LoadSession(acpSessionID string, history []acp.Message) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessionID = acpSessionID
	return nil
}

func (a *KimiAdapter) Send(prompt string, history []acp.Message) (<-chan acp.StreamEvent, error) {
	if a.apiKey == "" {
		return nil, fmt.Errorf("kimi api key not configured")
	}

	messages := make([]map[string]string, 0, len(history)+1)
	for _, m := range history {
		role := m.Role
		if role == "assistant" || role == "user" || role == "system" {
			messages = append(messages, map[string]string{"role": role, "content": m.Content})
		}
	}
	messages = append(messages, map[string]string{"role": "user", "content": prompt})

	reqBody := map[string]any{
		"model":    a.model,
		"messages": messages,
		"stream":   true,
	}
	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", kimiBaseURL+"/chat/completions", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kimi request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("kimi http %d: %s", resp.StatusCode, string(body))
	}

	ch := make(chan acp.StreamEvent, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					select {
					case ch <- acp.StreamEvent{Type: "error", Error: err}:
					default:
					}
				}
				break
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if line == "data: [DONE]" {
				break
			}
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			if len(chunk.Choices) == 0 {
				continue
			}
			content := chunk.Choices[0].Delta.Content
			if content == "" {
				continue
			}
			select {
			case ch <- acp.StreamEvent{Type: "text_delta", Content: content}:
			default:
			}
		}

		select {
		case ch <- acp.StreamEvent{Type: "done"}:
		default:
		}
	}()

	return ch, nil
}

var _ acp.ACPAdapter = (*KimiAdapter)(nil)
