package adapters

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"desktop/internal/acp"
)

type ClaudeAdapter struct {
	pm              *acp.ProcessManager
	info            acp.AgentInfo
	mu              sync.Mutex
	dispatchMu      sync.Mutex
	dispatchRunning bool
	reqID           int
	pending         map[string]chan acp.StreamEvent
}

func NewClaudeAdapter() *ClaudeAdapter {
	return &ClaudeAdapter{
		info: acp.AgentInfo{
			ID:      "claude-code",
			Name:    "Claude Code",
			Command: "claude --acp",
		},
		pm:      acp.NewProcessManager("claude --acp"),
		pending: make(map[string]chan acp.StreamEvent),
	}
}

type acpRequest struct {
	ID     string         `json:"id"`
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
}

type acpResponse struct {
	ID     string `json:"id,omitempty"`
	Type   string `json:"type"`
	Result struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	} `json:"result,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (a *ClaudeAdapter) Send(prompt string, history []acp.Message) (<-chan acp.StreamEvent, error) {
	if !a.pm.IsRunning() {
		if err := a.pm.Start(); err != nil {
			return nil, fmt.Errorf("start claude: %w", err)
		}
	}

	a.dispatchMu.Lock()
	if !a.dispatchRunning {
		a.dispatchRunning = true
		go a.dispatchLoop(a.pm)
	}
	a.dispatchMu.Unlock()

	a.mu.Lock()
	a.reqID++
	reqID := fmt.Sprintf("req-%d", a.reqID)
	ch := make(chan acp.StreamEvent, 256)
	a.pending[reqID] = ch
	a.mu.Unlock()

	msgs := make([]map[string]string, 0, len(history)+1)
	for _, h := range history {
		msgs = append(msgs, map[string]string{"role": h.Role, "content": h.Content})
	}
	msgs = append(msgs, map[string]string{"role": "user", "content": prompt})

	req := acpRequest{
		ID:     reqID,
		Method: "chat",
		Params: map[string]any{"messages": msgs},
	}

	if err := a.pm.SendJSON(req); err != nil {
		a.mu.Lock()
		delete(a.pending, reqID)
		a.mu.Unlock()
		return nil, fmt.Errorf("send request: %w", err)
	}

	return ch, nil
}

func (a *ClaudeAdapter) Info() acp.AgentInfo { return a.info }

func (a *ClaudeAdapter) IsRunning() bool { return a.pm.IsRunning() }
func (a *ClaudeAdapter) Stop() error    { return a.pm.Stop() }

func (a *ClaudeAdapter) dispatchLoop(pm *acp.ProcessManager) {
	defer func() {
		a.dispatchMu.Lock()
		a.dispatchRunning = false
		a.dispatchMu.Unlock()
	}()
	events := pm.Events()
	done := pm.Done()
	for {
		select {
		case raw, ok := <-events:
			if !ok {
				return
			}
			var resp acpResponse
			if err := json.Unmarshal(raw, &resp); err != nil {
				log.Printf("[claude] parse error: %v (raw: %s)", err, string(raw))
				continue
			}

			a.mu.Lock()
			ch, ok := a.pending[resp.ID]
			a.mu.Unlock()
			if !ok {
				continue
			}

			evt := acp.StreamEvent{Type: resp.Result.Type, Content: resp.Result.Content}
			if resp.Error != nil {
				evt = acp.StreamEvent{Type: "error", Error: fmt.Errorf("%s", resp.Error.Message)}
			}
			if resp.Type == "done" {
				evt.Type = "done"
			}

			select {
			case ch <- evt:
			default:
				log.Printf("[claude] event dropped for %s", resp.ID)
			}

			if evt.Type == "done" || evt.Type == "error" {
				close(ch)
				a.mu.Lock()
				delete(a.pending, resp.ID)
				a.mu.Unlock()
			}

		case <-done:
			log.Printf("[claude] dispatch loop exited (process done)")
			return
		}
	}
}
