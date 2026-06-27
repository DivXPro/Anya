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
	reqID           int
	pending         map[string]chan acp.StreamEvent
	activeStream    chan acp.StreamEvent
	streamMu        sync.RWMutex
	sessionID       string
	initDone        bool
	lastPromptReqID int
}

func NewOpenCodeAdapter() *OpenCodeAdapter {
	return &OpenCodeAdapter{
		info: acp.AgentInfo{
			ID:      "opencode",
			Name:    "OpenCode",
			Command: "opencode acp",
		},
		pm:      acp.NewProcessManagerWithFraming("opencode acp", acp.NDJSONFraming),
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
	a.initDone = true

	// If a session was already loaded via LoadSession, skip creating a new one.
	if a.sessionID != "" {
		log.Printf("[opencode] using loaded session: %s", a.sessionID)
		return nil
	}

	sessResp, err := a.sendRequest("session/new", map[string]any{
		"cwd":        ".",
		"mcpServers": []any{},
	})
	if err != nil {
		return fmt.Errorf("acp session/new: %w", err)
	}

	if sid, ok := sessResp["sessionId"].(string); ok {
		a.sessionID = sid
	} else {
		log.Printf("[opencode] session/new unexpected response: %+v", sessResp)
	}
	log.Printf("[opencode] session created: %s", a.sessionID)
	return nil
}

func (a *OpenCodeAdapter) Send(prompt string, history []acp.Message) (<-chan acp.StreamEvent, error) {
	if err := a.ensureInit(); err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.reqID++
	id := a.reqID
	a.lastPromptReqID = id
	ch := make(chan acp.StreamEvent, 256)
	a.streamMu.Lock()
	a.activeStream = ch
	a.streamMu.Unlock()
	a.mu.Unlock()

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "session/prompt",
		"params": map[string]any{
			"sessionId": a.sessionID,
			"prompt":    []map[string]any{{"type": "text", "text": prompt}},
		},
	}

	if err := a.pm.SendJSON(req); err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	return ch, nil
}

func (a *OpenCodeAdapter) LoadSession(acpSessionID string, history []acp.Message) error {
	if err := a.ensureInit(); err != nil {
		return err
	}
	if a.sessionID == acpSessionID {
		return nil
	}

	resp, err := a.sendRequest("session/load", map[string]any{
		"sessionId": acpSessionID,
	})
	if err != nil {
		return fmt.Errorf("acp session/load: %w", err)
	}
	if sid, ok := resp["sessionId"].(string); ok && sid != "" {
		a.sessionID = sid
	} else {
		a.sessionID = acpSessionID
	}
	log.Printf("[opencode] session loaded: %s", a.sessionID)
	return nil
}

func (a *OpenCodeAdapter) CurrentSessionID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sessionID
}

func (a *OpenCodeAdapter) Info() acp.AgentInfo { return a.info }

func (a *OpenCodeAdapter) IsRunning() bool { return a.pm.IsRunning() }
func (a *OpenCodeAdapter) Stop() error     { return a.pm.Stop() }

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
						SessionUpdate string `json:"sessionUpdate"`
						MessageID     string `json:"messageId"`
						Content       struct {
							Type string `json:"type"`
							Text string `json:"text"`
						} `json:"content"`
					} `json:"update"`
				} `json:"params"`
			}

			if err := json.Unmarshal(raw, &msg); err != nil {
				continue
			}

			// JSON-RPC response: route to pending waiter if one exists, or treat
			// it as the completion signal for the current prompt.
			if msg.ID != 0 || msg.Result != nil || msg.Error != nil {
				a.mu.Lock()
				key := fmt.Sprintf("resp-%d", msg.ID)
				waiter, ok := a.pending[key]
				if ok {
					if msg.Error != nil {
						waiter <- acp.StreamEvent{Type: "error", Error: fmt.Errorf("%s", msg.Error.Message)}
					} else {
						resultJSON, err := json.Marshal(msg.Result)
						if err != nil {
							waiter <- acp.StreamEvent{Type: "error", Error: err}
						} else {
							waiter <- acp.StreamEvent{Type: "text_delta", Content: string(resultJSON)}
						}
					}
					close(waiter)
					delete(a.pending, key)
					a.mu.Unlock()
					continue
				}

				if msg.ID == a.lastPromptReqID {
					a.streamMu.RLock()
					active := a.activeStream
					a.streamMu.RUnlock()
					if active != nil {
						if msg.Error != nil {
							select {
							case active <- acp.StreamEvent{Type: "error", Error: fmt.Errorf("%s", msg.Error.Message)}:
							default:
							}
						}
						select {
						case active <- acp.StreamEvent{Type: "done"}:
						default:
						}
						a.streamMu.Lock()
						if a.activeStream == active {
							close(a.activeStream)
							a.activeStream = nil
						}
						a.streamMu.Unlock()
					}
				}
				a.mu.Unlock()
				continue
			}

			if msg.Method == "session/update" && msg.Params.SessionID == a.sessionID {
				evt := acp.StreamEvent{}
				switch msg.Params.Update.SessionUpdate {
				case "agent_message_chunk":
					text, ok := sanitizeACPText(msg.Params.Update.Content.Text)
					if !ok {
						continue
					}
					evt.Type = "text_delta"
					evt.Content = text
				case "agent_thought_chunk":
					// Surface thinking chunks as text so callers can see progress.
					text, ok := sanitizeACPText(msg.Params.Update.Content.Text)
					if !ok {
						continue
					}
					evt.Type = "text_delta"
					evt.Content = text
				case "tool_call_update":
					text, ok := sanitizeACPText(msg.Params.Update.Content.Text)
					if !ok {
						continue
					}
					evt.Type = "tool_use"
					evt.Content = text
				default:
					continue
				}

				a.streamMu.RLock()
				active := a.activeStream
				a.streamMu.RUnlock()
				if active == nil {
					continue
				}
				select {
				case active <- evt:
				default:
				}
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
