package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"desktop/internal/acp"
	"desktop/internal/acp/agentsessions"

	"github.com/beyond5959/acp-adapter/pkg/claudeacp"
)

// claudeSettingsModel reads the user's ~/.claude/settings.json env block and
// returns the configured model (ANTHROPIC_MODEL, falling back to CLAUDE_MODEL).
//
// This exists because the vendored acp-adapter hardcodes DefaultModel =
// "claude-opus-4-6" and normalizeRuntimeConfig re-fills any empty value back to
// that default. That default is then passed as `--model` to the claude CLI,
// overriding whatever the user configured in settings.json (e.g. a third-party
// gateway model like GLM-5.2), which the gateway rejects with 422
// "Model Not Exist". By resolving the model from settings.json ourselves and
// setting cfg.DefaultModel to a non-empty value, normalizeRuntimeConfig leaves
// it alone and the CLI gets the correct `--model`.
func claudeSettingsModel() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		return ""
	}
	var cfg struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ""
	}
	if m := cfg.Env["ANTHROPIC_MODEL"]; m != "" {
		return m
	}
	return cfg.Env["CLAUDE_MODEL"]
}

// IsClaudeCliInstalled reports whether the official Claude Code CLI is
// available on PATH. The adapter itself is embedded, but it still needs the
// claude binary to do the actual work.
func IsClaudeCliInstalled() bool {
	_, err := exec.LookPath("claude")
	return err == nil
}

// ClaudeAdapter drives Claude Code through an embedded ACP adapter runtime.
// It does not require the user to install a separate Node.js bridge.
type ClaudeAdapter struct {
	info         acp.AgentInfo
	rt           *claudeacp.EmbeddedRuntime
	mu           sync.Mutex
	reqID        int
	sessionID    string
	initDone     bool
	activeStream chan acp.StreamEvent
	streamMu     sync.RWMutex
	stopSub      func()
	systemPrompt string
	permCh       chan acp.PermissionRequest
	cwd          string
	resetPending bool

	initRuntimeFunc   func() error
	clientRequestFunc func(id int, method string, params map[string]any) (map[string]any, error)
}

func NewClaudeAdapter() *ClaudeAdapter {
	return &ClaudeAdapter{
		info: acp.AgentInfo{
			ID:      "claude-code",
			Name:    "Claude Code",
			Command: "claude",
		},
		systemPrompt: DefaultSystemPrompt,
		permCh:       make(chan acp.PermissionRequest, 8),
	}
}

func (a *ClaudeAdapter) SetSystemPrompt(prompt string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.systemPrompt = prompt
}

func (a *ClaudeAdapter) initRuntime() error {
	a.mu.Lock()
	if a.rt != nil {
		a.mu.Unlock()
		return nil
	}

	cfg := claudeacp.DefaultRuntimeConfig()
	cfg.LogLevel = "warn"
	// DefaultRuntimeConfig hardcodes DefaultModel = "claude-opus-4-6" and
	// normalizeRuntimeConfig re-fills it when empty, then passes it as
	// `--model` to the claude CLI — overriding the user's settings.json model
	// and causing 422 "Model Not Exist" on third-party gateways. Resolve the
	// model the user actually configured so the CLI gets the right one.
	if m := claudeSettingsModel(); m != "" {
		cfg.DefaultModel = m
	}
	if wrapper := ensureClaudeWrapper(); wrapper != "" {
		cfg.ClaudeBin = wrapper
	}
	rt := claudeacp.NewEmbeddedRuntime(cfg)
	if err := rt.Start(context.Background()); err != nil {
		a.mu.Unlock()
		return fmt.Errorf("start embedded runtime: %w", err)
	}
	a.rt = rt
	a.reqID = 0
	a.mu.Unlock()

	initResp, err := a.clientRequest(1, "initialize", map[string]any{
		"protocolVersion": 1,
		"clientCapabilities": map[string]any{
			"fs":       map[string]bool{"readTextFile": true, "writeTextFile": true},
			"terminal": true,
		},
		"clientInfo": map[string]string{"name": "anya", "version": "1.0.0"},
	})
	if err != nil {
		rt.Close()
		a.mu.Lock()
		a.rt = nil
		a.mu.Unlock()
		return fmt.Errorf("acp initialize: %w", err)
	}
	log.Printf("[claude] initialized (protocol=%v, agent=%v)", initResp["protocolVersion"], initResp["agentInfo"])

	updates, stop := rt.SubscribeUpdates(256)
	a.mu.Lock()
	a.stopSub = stop
	a.initDone = true
	a.mu.Unlock()

	go a.dispatchUpdates(updates)
	return nil
}

func (a *ClaudeAdapter) newSession() error {
	a.mu.Lock()
	a.reqID++
	id := a.reqID
	systemPrompt := a.systemPrompt
	a.mu.Unlock()

	params := map[string]any{
		"cwd":        a.effectiveCWD(),
		"mcpServers": []any{},
	}
	if systemPrompt != "" {
		params["systemInstructions"] = systemPrompt
	}
	sessResp, err := a.clientRequest(id, "session/new", params)
	if err != nil {
		return fmt.Errorf("acp session/new: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if sid, ok := sessResp["sessionId"].(string); ok {
		a.sessionID = sid
	} else {
		log.Printf("[claude] session/new unexpected response: %+v", sessResp)
	}
	log.Printf("[claude] session created: %s", a.sessionID)
	return nil
}

func (a *ClaudeAdapter) ensureInit() error {
	a.mu.Lock()
	if a.initDone && a.sessionID != "" {
		a.mu.Unlock()
		return nil
	}
	a.mu.Unlock()

	if err := a.ensureRuntime(); err != nil {
		return err
	}

	a.mu.Lock()
	hasSession := a.sessionID != ""
	a.mu.Unlock()
	if hasSession {
		return nil
	}

	return a.newSession()
}

func (a *ClaudeAdapter) Send(prompt string, history []acp.Message) (<-chan acp.StreamEvent, error) {
	if err := a.ensureInit(); err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.reqID++
	id := a.reqID
	systemPrompt := a.systemPrompt
	sessionID := a.sessionID
	ch := make(chan acp.StreamEvent, 256)
	a.streamMu.Lock()
	a.activeStream = ch
	a.streamMu.Unlock()
	a.mu.Unlock()

	params := map[string]any{
		"sessionId": sessionID,
		"prompt":    []map[string]any{{"type": "text", "text": prompt}},
	}
	if systemPrompt != "" {
		params["systemInstructions"] = systemPrompt
	}

	go func() {
		_, err := a.clientRequest(id, "session/prompt", params)
		a.finishStream(err)
	}()

	return ch, nil
}

// finishStream is invoked once the session/prompt request returns. It delivers a
// terminal event (error and/or done) to the active stream, closes it, and then
// handles any pending session reset. It must be called with neither a.streamMu
// nor a.mu held: it acquires them itself, and stopLocked (reached via the reset
// path) also takes streamMu, so holding streamMu across that call would deadlock.
func (a *ClaudeAdapter) finishStream(sendErr error) {
	a.streamMu.Lock()
	active := a.activeStream
	if active != nil {
		if sendErr != nil {
			select {
			case active <- acp.StreamEvent{Type: "error", Error: sendErr}:
			default:
			}
		}
		select {
		case active <- acp.StreamEvent{Type: "done"}:
		default:
		}
		close(active)
		a.activeStream = nil
	}
	a.streamMu.Unlock()

	// Check for pending reset after stream completes.
	a.mu.Lock()
	if a.resetPending {
		a.resetPending = false
		if err := a.stopLocked(); err != nil {
			log.Printf("[claude] delayed stop after reset failed: %v", err)
		}
	}
	a.mu.Unlock()
}

func (a *ClaudeAdapter) CurrentSessionID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sessionID
}

func (a *ClaudeAdapter) LoadSession(acpSessionID string, history []acp.Message) error {
	if err := a.ensureRuntime(); err != nil {
		return err
	}

	a.mu.Lock()
	if a.sessionID == acpSessionID {
		a.mu.Unlock()
		return nil
	}
	a.mu.Unlock()

	// Try to load the session. If the backend does not support loading this
	// particular session (e.g. Claude backend may fail with "begin turn failed"),
	// return the error so the caller can create a fresh session instead.
	a.mu.Lock()
	a.reqID++
	id := a.reqID
	a.mu.Unlock()

	resp, err := a.clientRequest(id, "session/load", map[string]any{
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
	loaded := a.sessionID
	a.mu.Unlock()
	log.Printf("[claude] session loaded: %s", loaded)
	return nil
}

func (a *ClaudeAdapter) ListAgentSessions(limit int) ([]acp.AgentSession, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return agentsessions.ListClaudeSessions(home, limit)
}

func (a *ClaudeAdapter) LoadAgentSession(id, cwd string) error {
	a.SetCWD(cwd)
	return a.LoadSession(id, nil)
}

func (a *ClaudeAdapter) StartNewAgentSession(cwd string) error {
	a.SetCWD(cwd)
	return a.Stop()
}

func (a *ClaudeAdapter) Info() acp.AgentInfo { return a.info }

func (a *ClaudeAdapter) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.rt != nil
}

func (a *ClaudeAdapter) Stop() error {
	a.mu.Lock()
	err := a.stopLocked()
	a.mu.Unlock()
	return err
}

func (a *ClaudeAdapter) stopLocked() error {
	rt := a.rt
	a.rt = nil
	a.initDone = false
	a.sessionID = ""
	a.resetPending = false
	if a.stopSub != nil {
		a.stopSub()
		a.stopSub = nil
	}
	a.streamMu.Lock()
	if a.activeStream != nil {
		close(a.activeStream)
		a.activeStream = nil
	}
	a.streamMu.Unlock()

	if rt != nil {
		return rt.Close()
	}
	return nil
}

func (a *ClaudeAdapter) SetCWD(cwd string) {
	a.mu.Lock()
	a.cwd = cwd
	// Check if there's an active stream; if not, stop immediately.
	a.streamMu.RLock()
	hasActive := a.activeStream != nil
	a.streamMu.RUnlock()
	if !hasActive {
		a.mu.Unlock()
		if err := a.Stop(); err != nil {
			log.Printf("[claude] immediate stop after SetCWD failed: %v", err)
		}
		return
	}
	a.resetPending = true
	a.mu.Unlock()
}

func (a *ClaudeAdapter) effectiveCWD() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.effectiveCWDLocked()
}

func (a *ClaudeAdapter) effectiveCWDLocked() string {
	if a.cwd == "" {
		return "."
	}
	return a.cwd
}

func (a *ClaudeAdapter) ensureRuntime() error {
	if a.initRuntimeFunc != nil {
		return a.initRuntimeFunc()
	}
	return a.initRuntime()
}

func (a *ClaudeAdapter) clientRequest(id int, method string, params map[string]any) (map[string]any, error) {
	if a.clientRequestFunc != nil {
		return a.clientRequestFunc(id, method, params)
	}

	paramsRaw, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	idRaw := json.RawMessage(fmt.Sprintf("%d", id))

	a.mu.Lock()
	rt := a.rt
	a.mu.Unlock()
	if rt == nil {
		return nil, fmt.Errorf("runtime not initialized")
	}

	resp, err := rt.ClientRequest(context.Background(), claudeacp.RPCMessage{
		JSONRPC: "2.0",
		ID:      &idRaw,
		Method:  method,
		Params:  paramsRaw,
	})
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("acp error: %s", resp.Error.Message)
	}

	var result map[string]any
	if len(resp.Result) > 0 {
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return nil, fmt.Errorf("unmarshal result: %w", err)
		}
	}
	return result, nil
}

func (a *ClaudeAdapter) dispatchUpdates(updates <-chan claudeacp.RPCMessage) {
	for msg := range updates {
		if msg.Method == "session/request_permission" {
			a.handlePermissionRequest(msg)
			continue
		}
		if msg.Method != "session/update" {
			continue
		}
		var payload struct {
			SessionID string `json:"sessionId"`
			Update    struct {
				SessionUpdate string `json:"sessionUpdate"`
				MessageID     string `json:"messageId"`
				Content       struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"update"`
		}
		if err := json.Unmarshal(msg.Params, &payload); err != nil {
			log.Printf("[claude] update parse error: %v", err)
			continue
		}
		a.mu.Lock()
		sessionID := a.sessionID
		a.mu.Unlock()
		if payload.SessionID != sessionID {
			continue
		}

		evt := acp.StreamEvent{}
		switch payload.Update.SessionUpdate {
		case "agent_message_chunk":
			text, ok := sanitizeACPText(payload.Update.Content.Text)
			if !ok {
				continue
			}
			if isClaudeCLIErrorText(text) {
				evt.Type = "error"
				evt.Error = fmt.Errorf("%s", text)
				break
			}
			evt.Type = "text_delta"
			evt.Content = text
		case "agent_thought_chunk":
			text, ok := sanitizeACPText(payload.Update.Content.Text)
			if !ok {
				continue
			}
			if isClaudeCLIErrorText(text) {
				evt.Type = "error"
				evt.Error = fmt.Errorf("%s", text)
				break
			}
			// Surface thinking chunks as text so callers can see progress.
			evt.Type = "text_delta"
			evt.Content = text
		case "tool_call_update":
			text, ok := sanitizeACPText(payload.Update.Content.Text)
			if !ok {
				continue
			}
			evt.Type = "tool_use"
			evt.Content = text
		default:
			continue
		}

		a.streamMu.RLock()
		ch := a.activeStream
		a.streamMu.RUnlock()
		if ch == nil {
			continue
		}
		select {
		case ch <- evt:
		default:
		}
	}
}

func isClaudeCLIErrorText(text string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(text)), "claude cli error:")
}

func (a *ClaudeAdapter) handlePermissionRequest(msg claudeacp.RPCMessage) {
	var payload struct {
		Options []struct {
			OptionID string `json:"optionId"`
			Name     string `json:"name"`
			Kind     string `json:"kind"`
		} `json:"options"`
		ToolCall *struct {
			Title string `json:"title"`
		} `json:"toolCall"`
		Approval string   `json:"approval"`
		Command  string   `json:"command"`
		Files    []string `json:"files"`
		Host     string   `json:"host"`
	}
	if err := json.Unmarshal(msg.Params, &payload); err != nil {
		log.Printf("[claude] permission request parse error: %v", err)
		return
	}

	req := acp.PermissionRequest{Options: make([]acp.PermissionOption, 0, len(payload.Options))}
	if msg.ID != nil {
		req.ID = string(*msg.ID)
	}
	if payload.ToolCall != nil && payload.ToolCall.Title != "" {
		req.Prompt = payload.ToolCall.Title
	} else if payload.Approval != "" {
		req.Prompt = payload.Approval
	} else if payload.Command != "" {
		req.Prompt = payload.Command
	} else if payload.Host != "" {
		req.Prompt = payload.Host
	} else if len(payload.Files) > 0 {
		req.Prompt = payload.Files[0]
	} else {
		req.Prompt = "Permission required"
	}
	for _, o := range payload.Options {
		req.Options = append(req.Options, acp.PermissionOption{
			ID:    o.OptionID,
			Label: o.Name,
		})
	}
	if len(req.Options) == 0 {
		req.Options = append(req.Options,
			acp.PermissionOption{ID: "accept", Label: "Allow"},
			acp.PermissionOption{ID: "decline", Label: "Deny"},
		)
	}

	a.mu.Lock()
	ch := a.permCh
	a.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- req:
	default:
		log.Printf("[claude] permission request dropped (channel full): %s", req.ID)
	}
}

func (a *ClaudeAdapter) PermissionRequests() <-chan acp.PermissionRequest {
	return a.permCh
}

func (a *ClaudeAdapter) RespondPermission(requestID, optionID string) error {
	if requestID == "" {
		return fmt.Errorf("request id is required")
	}
	a.mu.Lock()
	rt := a.rt
	a.mu.Unlock()
	if rt == nil {
		return fmt.Errorf("runtime not initialized")
	}

	decision := claudeacp.PermissionDecision{}
	if optionID != "" {
		decision.SelectedOptionID = optionID
	} else {
		decision.Outcome = "cancelled"
	}
	return rt.RespondPermission(context.Background(), json.RawMessage(requestID), decision)
}

var _ acp.ACPAdapter = (*ClaudeAdapter)(nil)
