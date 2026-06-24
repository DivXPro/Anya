package adapters

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"desktop/internal/acp"
)

type OpenCodeAdapter struct {
	pm              *acp.ProcessManager
	info            acp.AgentInfo
	mu              sync.Mutex
	dispatchMu      sync.Mutex
	dispatchRunning bool
	streamMu        sync.RWMutex
	reqID           int
	pending         map[string]chan acp.StreamEvent
	activeStream    chan acp.StreamEvent
	sessionID       string
	initDone        bool
}

func NewOpenCodeAdapter() *OpenCodeAdapter {
	return &OpenCodeAdapter{
		info: acp.AgentInfo{
			ID:      "opencode",
			Name:    "OpenCode",
			Command: "opencode acp",
		},
		pm:      acp.NewProcessManager("opencode acp"),
		pending: make(map[string]chan acp.StreamEvent),
	}
}

func (a *OpenCodeAdapter) ensureInit() error {
	if a.initDone && a.sessionID != "" {
		return nil
	}
	if !a.pm.IsRunning() {
		if err := a.pm.Start(); err != nil {
			return fmt.Errorf("start opencode: %w", err)
		}
	}

	a.dispatchMu.Lock()
	if !a.dispatchRunning {
		a.dispatchRunning = true
		go a.dispatchLoop(a.pm)
	}
	a.dispatchMu.Unlock()

	initResp, err := a.sendRequest("initialize", map[string]any{
		"protocolVersion": 1,
		"clientCapabilities": map[string]any{
			"fs":       map[string]bool{"readTextFile": true, "writeTextFile": true},
			"terminal": true,
		},
		"clientInfo": map[string]string{"name": "elf", "version": "1.0.0"},
	})
	if err != nil {
		return fmt.Errorf("acp initialize: %w", err)
	}
	log.Printf("[opencode] initialized: %v", initResp)

	sessResp, err := a.sendRequest("session/new", map[string]any{
		"cwd":        ".",
		"mcpServers": []any{},
	})
	if err != nil {
		return fmt.Errorf("acp session/new: %w", err)
	}

	if result, ok := sessResp["result"].(map[string]any); ok {
		if sid, ok := result["sessionId"].(string); ok {
			a.sessionID = sid
		}
	}
	a.initDone = true
	log.Printf("[opencode] session created: %s", a.sessionID)
	return nil
}

func (a *OpenCodeAdapter) Send(prompt string, history []acp.Message) (<-chan acp.StreamEvent, error) {
	if err := a.ensureInit(); err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.reqID++
	ch := make(chan acp.StreamEvent, 256)
	a.mu.Unlock()

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      a.reqID,
		"method":  "session/prompt",
		"params": map[string]any{
			"sessionId": a.sessionID,
			"prompt":    []map[string]any{{"type": "text", "text": prompt}},
		},
	}

	if err := a.pm.SendJSON(req); err != nil {
		a.mu.Lock()
		delete(a.pending, fmt.Sprintf("resp-%d", a.reqID))
		a.mu.Unlock()
		return nil, fmt.Errorf("send request: %w", err)
	}

	a.streamMu.Lock()
	a.activeStream = ch
	a.streamMu.Unlock()

	return ch, nil
}

func (a *OpenCodeAdapter) dispatchLoop(pm *acp.ProcessManager) {
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
			var msg struct {
				ID     int            `json:"id"`
				Method string         `json:"method"`
				Result map[string]any `json:"result"`
				Error  *struct{ Message string } `json:"error"`
				Params struct {
					SessionID string `json:"sessionId"`
					Update    struct {
						Type string `json:"type"`
						Text string `json:"text,omitempty"`
					} `json:"update"`
				} `json:"params"`
			}

			if err := json.Unmarshal(raw, &msg); err != nil {
				continue
			}

			if msg.ID != 0 || msg.Result != nil || msg.Error != nil {
				a.mu.Lock()
				key := fmt.Sprintf("resp-%d", msg.ID)
				ch, ok := a.pending[key]
				a.mu.Unlock()
				if !ok {
					continue
				}

				if msg.Error != nil {
					ch <- acp.StreamEvent{Type: "error", Error: fmt.Errorf("%s", msg.Error.Message)}
				} else {
					resultJSON, err := json.Marshal(msg.Result)
					if err != nil {
						ch <- acp.StreamEvent{Type: "error", Error: err}
					} else {
						ch <- acp.StreamEvent{Type: "text_delta", Content: string(resultJSON)}
						ch <- acp.StreamEvent{Type: "done"}
					}
				}
				close(ch)
				a.mu.Lock()
				delete(a.pending, key)
				a.mu.Unlock()
				continue
			}

			if msg.Method == "session/update" && msg.Params.SessionID == a.sessionID {
				evt := acp.StreamEvent{}
				switch msg.Params.Update.Type {
				case "agent_message_chunk":
					evt.Type = "text_delta"
					evt.Content = msg.Params.Update.Text
				case "tool_call":
					evt.Type = "tool_use"
					evt.Content = msg.Params.Update.Text
				case "turn_end":
					evt.Type = "done"
				default:
					continue
				}
				a.mu.Lock()
				a.streamMu.RLock()
				if a.activeStream != nil {
					select {
					case a.activeStream <- evt:
					default:
					}
				}
				a.streamMu.RUnlock()
				a.mu.Unlock()
			}

		case <-done:
			return
		}
	}
}

func (a *OpenCodeAdapter) sendRequest(method string, params map[string]any) (map[string]any, error) {
	a.mu.Lock()
	a.reqID++
	reqID := a.reqID
	key := fmt.Sprintf("resp-%d", reqID)
	ch := make(chan acp.StreamEvent, 2)
	a.pending[key] = ch
	a.mu.Unlock()

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      reqID,
		"method":  method,
		"params":  params,
	}
	if err := a.pm.SendJSON(req); err != nil {
		a.mu.Lock()
		delete(a.pending, key)
		a.mu.Unlock()
		return nil, err
	}

	select {
	case evt, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("channel closed for %s", method)
		}
		if evt.IsError() {
			return nil, evt.Error
		}
		var result map[string]any
		json.Unmarshal([]byte(evt.Content), &result)
		return result, nil
	case <-time.After(15 * time.Second):
		a.mu.Lock()
		delete(a.pending, key)
		a.mu.Unlock()
		return nil, fmt.Errorf("timeout waiting for %s", method)
	}
}

func (a *OpenCodeAdapter) Info() acp.AgentInfo { return a.info }
func (a *OpenCodeAdapter) IsRunning() bool      { return a.pm.IsRunning() }
func (a *OpenCodeAdapter) Stop() error          { return a.pm.Stop() }
