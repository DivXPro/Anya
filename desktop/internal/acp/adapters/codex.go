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

// CodexAdapter implements the Anya ACP interface on top of the OpenAI Codex CLI
// app-server protocol (JSON-RPC over stdio, newline-delimited).
type CodexAdapter struct {
	pm                 *acp.ProcessManager
	info               acp.AgentInfo
	mu                 sync.Mutex
	initMu             sync.Mutex
	dispatchMu         sync.Mutex
	dispatchRunning    bool
	reqID              int
	pending            map[string]chan acp.StreamEvent
	activeStream       chan acp.StreamEvent
	streamMu           sync.RWMutex
	sessionID          string
	initDone           bool
	lastTurnStartReqID int
	activeTurnID       string
	systemPrompt       string
}

func NewCodexAdapter() *CodexAdapter {
	return &CodexAdapter{
		info: acp.AgentInfo{
			ID:      "codex",
			Name:    "Codex",
			Command: "codex app-server --stdio",
		},
		pm:           acp.NewProcessManagerWithFraming("codex app-server --stdio", acp.NDJSONFraming),
		pending:      make(map[string]chan acp.StreamEvent),
		systemPrompt: DefaultSystemPrompt,
	}
}

func (a *CodexAdapter) SetSystemPrompt(prompt string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.systemPrompt = prompt
}

func (a *CodexAdapter) Info() acp.AgentInfo { return a.info }
func (a *CodexAdapter) IsRunning() bool     { return a.pm.IsRunning() }
func (a *CodexAdapter) Stop() error         { return a.pm.Stop() }

func (a *CodexAdapter) CurrentSessionID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sessionID
}

// ensureInit starts the Codex app-server and creates a fresh thread when needed.
func (a *CodexAdapter) ensureInit() error {
	return a.ensureInitWithSkip(false)
}

// ensureInitWithSkip performs initialization but does not create a new thread
// when skipNewThread is true. This is used by LoadSession so it can resume a
// previously persisted thread instead of creating a throwaway one.
func (a *CodexAdapter) ensureInitWithSkip(skipNewThread bool) error {
	a.initMu.Lock()
	defer a.initMu.Unlock()

	a.mu.Lock()
	if a.initDone && a.sessionID != "" {
		a.mu.Unlock()
		return nil
	}
	a.mu.Unlock()

	if !a.pm.IsRunning() {
		if err := a.pm.Start(); err != nil {
			return fmt.Errorf("start codex app-server: %w", err)
		}
	}

	a.dispatchMu.Lock()
	if !a.dispatchRunning {
		a.dispatchRunning = true
		go a.dispatchLoop(a.pm)
	}
	a.dispatchMu.Unlock()

	if _, err := a.sendRequest("initialize", map[string]any{
		"clientInfo": map[string]string{"name": "anya", "version": "1.0.0"},
		"capabilities": map[string]any{
			"experimentalApi": true,
		},
	}); err != nil {
		return fmt.Errorf("acp initialize: %w", err)
	}

	a.mu.Lock()
	a.initDone = true
	sessionID := a.sessionID
	a.mu.Unlock()

	if sessionID != "" {
		return nil
	}
	if skipNewThread {
		return nil
	}

	params := map[string]any{
		"ephemeral":         false,
		"approvalPolicy":    "never",
		"approvalsReviewer": "auto_review",
		"sandbox":           "workspace-write",
	}
	if a.systemPrompt != "" {
		params["baseInstructions"] = a.systemPrompt
	}

	resp, err := a.sendRequest("thread/start", params)
	if err != nil {
		return fmt.Errorf("acp thread/start: %w", err)
	}

	sid := threadIDFromResult(resp)
	if sid == "" {
		log.Printf("[codex] thread/start unexpected response: %+v", resp)
		return fmt.Errorf("acp thread/start: missing thread id")
	}

	a.mu.Lock()
	a.sessionID = sid
	a.mu.Unlock()
	log.Printf("[codex] thread created: %s", sid)
	return nil
}

// Send starts a new user turn on the active Codex thread and streams back the
// assistant response as Anya StreamEvents.
func (a *CodexAdapter) Send(prompt string, history []acp.Message) (<-chan acp.StreamEvent, error) {
	if err := a.ensureInit(); err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.reqID++
	id := a.reqID
	a.lastTurnStartReqID = id
	sessionID := a.sessionID
	ch := make(chan acp.StreamEvent, 256)
	a.streamMu.Lock()
	a.activeStream = ch
	a.streamMu.Unlock()
	a.mu.Unlock()

	if sessionID == "" {
		return nil, fmt.Errorf("no codex thread available")
	}

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "turn/start",
		"params": map[string]any{
			"threadId": sessionID,
			"input": []map[string]any{
				{"type": "text", "text": prompt},
			},
		},
	}
	if err := a.pm.SendJSON(req); err != nil {
		return nil, fmt.Errorf("send turn/start: %w", err)
	}

	return ch, nil
}

// LoadSession resumes a previously persisted Codex thread by its thread id.
func (a *CodexAdapter) LoadSession(acpSessionID string, history []acp.Message) error {
	if err := a.ensureInitWithSkip(true); err != nil {
		return err
	}

	a.mu.Lock()
	if a.sessionID == acpSessionID {
		a.mu.Unlock()
		return nil
	}
	a.mu.Unlock()

	params := map[string]any{
		"threadId":          acpSessionID,
		"approvalPolicy":    "never",
		"approvalsReviewer": "auto_review",
		"sandbox":           "workspace-write",
	}
	if a.systemPrompt != "" {
		params["baseInstructions"] = a.systemPrompt
	}

	resp, err := a.sendRequest("thread/resume", params)
	if err != nil {
		// Clear the stale session id so the next Send creates a new thread.
		a.mu.Lock()
		a.sessionID = ""
		a.mu.Unlock()
		return fmt.Errorf("acp thread/resume: %w", err)
	}

	sid := threadIDFromResult(resp)
	if sid == "" {
		// thread/resume returned a response but no thread id; fall back to the
		// requested id so the caller can continue.
		sid = acpSessionID
	}

	a.mu.Lock()
	a.sessionID = sid
	a.mu.Unlock()
	log.Printf("[codex] thread resumed: %s", sid)
	return nil
}

// dispatchLoop reads NDJSON messages from the Codex app-server process and
// routes them into the adapter's event model.
func (a *CodexAdapter) dispatchLoop(pm *acp.ProcessManager) {
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
				ID     *int            `json:"id"`
				Method string          `json:"method"`
				Result json.RawMessage `json:"result"`
				Error  *struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
				Params json.RawMessage `json:"params"`
			}
			if err := json.Unmarshal(raw, &msg); err != nil {
				continue
			}

			if msg.ID != nil {
				a.handleResponse(&msg)
				continue
			}
			if msg.Method != "" {
				a.handleNotification(&msg)
			}

		case <-done:
			return
		}
	}
}

func (a *CodexAdapter) handleResponse(msg *struct {
	ID     *int            `json:"id"`
	Method string          `json:"method"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Params json.RawMessage `json:"params"`
}) {
	a.mu.Lock()

	// Route JSON-RPC responses to callers waiting in sendRequest.
	key := fmt.Sprintf("resp-%d", *msg.ID)
	if waiter, ok := a.pending[key]; ok {
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
	}

	// Capture the turn id from the turn/start ack so we can correlate the
	// matching turn/completed notification.
	if *msg.ID == a.lastTurnStartReqID {
		if msg.Error != nil {
			a.streamMu.RLock()
			active := a.activeStream
			a.streamMu.RUnlock()
			if active != nil {
				select {
				case active <- acp.StreamEvent{Type: "error", Error: fmt.Errorf("%s", msg.Error.Message)}:
				default:
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
		} else if len(msg.Result) > 0 {
			var res struct {
				Turn struct {
					ID string `json:"id"`
				} `json:"turn"`
			}
			if json.Unmarshal(msg.Result, &res) == nil && res.Turn.ID != "" {
				a.activeTurnID = res.Turn.ID
			}
		}
	}

	a.mu.Unlock()
}

func (a *CodexAdapter) handleNotification(msg *struct {
	ID     *int            `json:"id"`
	Method string          `json:"method"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Params json.RawMessage `json:"params"`
}) {
	switch msg.Method {
	case "item/agentMessage/delta":
		var p struct {
			Delta string `json:"delta"`
		}
		if json.Unmarshal(msg.Params, &p) != nil {
			return
		}
		text, ok := sanitizeACPText(p.Delta)
		if !ok {
			return
		}
		a.streamMu.RLock()
		active := a.activeStream
		a.streamMu.RUnlock()
		if active == nil {
			return
		}
		select {
		case active <- acp.StreamEvent{Type: "text_delta", Content: text}:
		default:
		}

	case "turn/completed":
		var p struct {
			Turn struct {
				ID string `json:"id"`
			} `json:"turn"`
		}
		if json.Unmarshal(msg.Params, &p) != nil {
			return
		}

		a.mu.Lock()
		activeTurnID := a.activeTurnID
		// If we never captured a turn id, close on any turn/completed so the
		// caller does not hang.
		match := activeTurnID == "" || p.Turn.ID == activeTurnID
		a.activeTurnID = ""
		a.mu.Unlock()

		if !match {
			return
		}

		a.streamMu.RLock()
		active := a.activeStream
		a.streamMu.RUnlock()
		if active == nil {
			return
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

	case "error":
		var p struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(msg.Params, &p) != nil {
			return
		}
		a.streamMu.RLock()
		active := a.activeStream
		a.streamMu.RUnlock()
		if active == nil {
			return
		}
		select {
		case active <- acp.StreamEvent{Type: "error", Error: fmt.Errorf("%s", p.Message)}:
		default:
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

func (a *CodexAdapter) sendRequest(method string, params map[string]any) (map[string]any, error) {
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
		if err := json.Unmarshal([]byte(evt.Content), &result); err != nil {
			return nil, err
		}
		return result, nil
	case <-time.After(30 * time.Second):
		a.mu.Lock()
		delete(a.pending, key)
		a.mu.Unlock()
		return nil, fmt.Errorf("timeout waiting for %s", method)
	}
}

func threadIDFromResult(result map[string]any) string {
	if result == nil {
		return ""
	}
	thread, ok := result["thread"].(map[string]any)
	if !ok {
		return ""
	}
	id, ok := thread["id"].(string)
	if !ok {
		return ""
	}
	return id
}

// IsCodexCliInstalled reports whether the Codex CLI is available on PATH.
func IsCodexCliInstalled() bool {
	_, err := exec.LookPath("codex")
	return err == nil
}

var _ acp.ACPAdapter = (*CodexAdapter)(nil)
