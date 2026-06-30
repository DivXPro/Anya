package adapters

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"sync"
	"time"

	"desktop/internal/acp"
)

type KimiAdapter struct {
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
	systemPrompt    string
	cwd             string
	resetPending    bool
}

func NewKimiAdapter() *KimiAdapter {
	return &KimiAdapter{
		info: acp.AgentInfo{
			ID:      "kimi",
			Name:    "Kimi Code",
			Command: "kimi acp",
		},
		pm:           acp.NewProcessManagerWithFraming("kimi acp", acp.NDJSONFraming),
		pending:      make(map[string]chan acp.StreamEvent),
		systemPrompt: DefaultSystemPrompt,
	}
}

func (a *KimiAdapter) SetSystemPrompt(prompt string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.systemPrompt = prompt
}

func (a *KimiAdapter) ensureInit() error {
	a.mu.Lock()
	initDone := a.initDone
	sessionID := a.sessionID
	a.mu.Unlock()
	if initDone && sessionID != "" {
		return nil
	}
	if !a.pm.IsRunning() {
		if err := a.pm.Start(); err != nil {
			return fmt.Errorf("start kimi: %w", err)
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
		"clientInfo": map[string]string{"name": "anya", "version": "1.0.0"},
	})
	if err != nil {
		return fmt.Errorf("acp initialize: %w", err)
	}
	log.Printf("[kimi] initialized: %v", initResp)
	a.mu.Lock()
	a.initDone = true
	sessionID = a.sessionID
	systemPrompt := a.systemPrompt
	a.mu.Unlock()

	if sessionID != "" {
		log.Printf("[kimi] using loaded session: %s", sessionID)
		return nil
	}

	params := map[string]any{
		"cwd":        a.effectiveCWD(),
		"mcpServers": []any{},
	}
	if systemPrompt != "" {
		params["systemInstructions"] = systemPrompt
	}
	sessResp, err := a.sendRequest("session/new", params)
	if err != nil {
		return fmt.Errorf("acp session/new: %w", err)
	}

	a.mu.Lock()
	sid, ok := sessResp["sessionId"].(string)
	if ok {
		a.sessionID = sid
	}
	sessionID = a.sessionID
	a.mu.Unlock()
	if !ok {
		log.Printf("[kimi] session/new unexpected response: %+v", sessResp)
	}
	log.Printf("[kimi] session created: %s", sessionID)
	return nil
}

func (a *KimiAdapter) Send(prompt string, history []acp.Message) (<-chan acp.StreamEvent, error) {
	if err := a.ensureInit(); err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.reqID++
	id := a.reqID
	a.lastPromptReqID = id
	systemPrompt := a.systemPrompt
	sessionID := a.sessionID
	ch := make(chan acp.StreamEvent, 256)
	a.streamMu.Lock()
	a.activeStream = ch
	a.streamMu.Unlock()
	a.mu.Unlock()

	promptParams := map[string]any{
		"sessionId": sessionID,
		"prompt":    []map[string]any{{"type": "text", "text": prompt}},
	}
	if systemPrompt != "" {
		promptParams["systemInstructions"] = systemPrompt
	}
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "session/prompt",
		"params":  promptParams,
	}

	if err := a.pm.SendJSON(req); err != nil {
		a.streamMu.Lock()
		if a.activeStream == ch {
			close(ch)
			a.activeStream = nil
		}
		a.streamMu.Unlock()
		return nil, fmt.Errorf("send request: %w", err)
	}

	return ch, nil
}

func (a *KimiAdapter) LoadSession(acpSessionID string, history []acp.Message) error {
	if err := a.ensureInit(); err != nil {
		return err
	}
	a.mu.Lock()
	current := a.sessionID
	a.mu.Unlock()
	if current == acpSessionID {
		return nil
	}

	resp, err := a.sendRequest("session/load", map[string]any{
		"sessionId":  acpSessionID,
		"cwd":        a.effectiveCWD(),
		"mcpServers": []any{},
	})
	if err != nil {
		return fmt.Errorf("acp session/load: %w", err)
	}
	a.mu.Lock()
	if sid, ok := resp["sessionId"].(string); ok && sid != "" {
		a.sessionID = sid
	} else {
		a.sessionID = acpSessionID
	}
	sessionID := a.sessionID
	a.mu.Unlock()
	log.Printf("[kimi] session loaded: %s", sessionID)
	return nil
}

func (a *KimiAdapter) CurrentSessionID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sessionID
}

func (a *KimiAdapter) Info() acp.AgentInfo { return a.info }

func (a *KimiAdapter) IsRunning() bool { return a.pm.IsRunning() }
func (a *KimiAdapter) Stop() error {
	a.mu.Lock()
	a.resetPending = false
	a.mu.Unlock()
	return a.pm.Stop()
}

func (a *KimiAdapter) SetCWD(cwd string) {
	a.mu.Lock()
	a.cwd = cwd
	// Check if there's an active stream; if not, stop immediately.
	a.streamMu.RLock()
	hasActive := a.activeStream != nil
	a.streamMu.RUnlock()
	if !hasActive {
		a.mu.Unlock()
		if err := a.Stop(); err != nil {
			log.Printf("[kimi] immediate stop after SetCWD failed: %v", err)
		}
		return
	}
	a.resetPending = true
	a.mu.Unlock()
}

func (a *KimiAdapter) effectiveCWD() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.effectiveCWDLocked()
}

func (a *KimiAdapter) effectiveCWDLocked() string {
	if a.cwd == "" {
		return "."
	}
	return a.cwd
}

func (a *KimiAdapter) dispatchLoop(pm *acp.ProcessManager) {
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
				ID     int                       `json:"id"`
				Method string                    `json:"method"`
				Result map[string]any            `json:"result"`
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

			a.mu.Lock()
			currentSession := a.sessionID
			a.mu.Unlock()
			if msg.Method == "session/update" && msg.Params.SessionID == currentSession {
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
					evt.Type = "text_delta"
					text, ok := sanitizeACPText(msg.Params.Update.Content.Text)
					if !ok {
						continue
					}
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
			// Check for pending reset after stream completes. pm.Stop() blocks on
			// process termination, so decide under a.mu but call Stop() outside it
			// to avoid stalling other a.mu users while the child is torn down.
			a.mu.Lock()
			shouldStop := a.resetPending
			a.resetPending = false
			a.mu.Unlock()
			if shouldStop {
				if err := a.pm.Stop(); err != nil {
					log.Printf("[kimi] delayed stop after reset failed: %v", err)
				}
			}
			return
		}
	}
}

func (a *KimiAdapter) sendRequest(method string, params map[string]any) (map[string]any, error) {
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

var _ acp.ACPAdapter = (*KimiAdapter)(nil)

// IsKimiCliInstalled reports whether the Kimi Code CLI is available on PATH.
func IsKimiCliInstalled() bool {
	_, err := exec.LookPath("kimi")
	return err == nil
}
