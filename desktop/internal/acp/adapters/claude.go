package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"

	"desktop/internal/acp"

	"github.com/beyond5959/acp-adapter/pkg/claudeacp"
)

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
		"cwd":        ".",
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

	if err := a.initRuntime(); err != nil {
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
	ch := make(chan acp.StreamEvent, 256)
	a.streamMu.Lock()
	a.activeStream = ch
	a.streamMu.Unlock()
	a.mu.Unlock()

	params := map[string]any{
		"sessionId": a.sessionID,
		"prompt":    []map[string]any{{"type": "text", "text": prompt}},
	}
	if systemPrompt != "" {
		params["systemInstructions"] = systemPrompt
	}

	go func() {
		_, err := a.clientRequest(id, "session/prompt", params)

		a.streamMu.Lock()
		defer a.streamMu.Unlock()
		active := a.activeStream
		if active == nil {
			return
		}
		if err != nil {
			select {
			case active <- acp.StreamEvent{Type: "error", Error: err}:
			default:
			}
		}
		select {
		case active <- acp.StreamEvent{Type: "done"}:
		default:
		}
		close(active)
		a.activeStream = nil
	}()

	return ch, nil
}

func (a *ClaudeAdapter) CurrentSessionID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sessionID
}

func (a *ClaudeAdapter) LoadSession(acpSessionID string, history []acp.Message) error {
	if err := a.initRuntime(); err != nil {
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
	// fall back to keeping the requested session id so the caller can decide
	// whether to create a new session.
	a.mu.Lock()
	a.reqID++
	id := a.reqID
	a.mu.Unlock()

	resp, err := a.clientRequest(id, "session/load", map[string]any{
		"sessionId": acpSessionID,
	})
	if err != nil {
		log.Printf("[claude] session/load failed, keeping requested id: %v", err)
		a.mu.Lock()
		a.sessionID = acpSessionID
		a.mu.Unlock()
		return nil
	}

	a.mu.Lock()
	if sid, ok := resp["sessionId"].(string); ok && sid != "" {
		a.sessionID = sid
	} else {
		a.sessionID = acpSessionID
	}
	a.mu.Unlock()
	log.Printf("[claude] session loaded: %s", a.sessionID)
	return nil
}

func (a *ClaudeAdapter) Info() acp.AgentInfo { return a.info }

func (a *ClaudeAdapter) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.rt != nil
}

func (a *ClaudeAdapter) Stop() error {
	a.mu.Lock()
	rt := a.rt
	a.rt = nil
	a.initDone = false
	a.sessionID = ""
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
	a.mu.Unlock()

	if rt != nil {
		return rt.Close()
	}
	return nil
}

func (a *ClaudeAdapter) clientRequest(id int, method string, params map[string]any) (map[string]any, error) {
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
		type textContent struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		type toolCallContentItem struct {
			Type    string       `json:"type"`
			Content *textContent `json:"content,omitempty"`
		}
		var payload struct {
			SessionID string `json:"sessionId"`
			Update    struct {
				SessionUpdate  string          `json:"sessionUpdate"`
				MessageID      string          `json:"messageId"`
				Title          string          `json:"title,omitempty"`
				ToolCallID     string          `json:"toolCallId,omitempty"`
				Content        json.RawMessage `json:"content"`
				ToolCallContent []toolCallContentItem `json:"toolCallContent,omitempty"`
			} `json:"update"`
		}
		if err := json.Unmarshal(msg.Params, &payload); err != nil {
			log.Printf("[claude] update parse error: %v", err)
			continue
		}
		if payload.SessionID != a.sessionID {
			continue
		}

		evt := acp.StreamEvent{}
		switch payload.Update.SessionUpdate {
		case "agent_message_chunk", "agent_thought_chunk":
			var tc textContent
			if err := json.Unmarshal(payload.Update.Content, &tc); err != nil {
				continue
			}
			text, ok := sanitizeACPText(tc.Text)
			if !ok {
				continue
			}
			evt.Type = "text_delta"
			evt.Content = text
		case "tool_call_update":
			var text string
			// Tool call content may be in the top-level ToolCallContent array.
			for _, item := range payload.Update.ToolCallContent {
				if item.Content != nil {
					text += item.Content.Text
				}
			}
			// Fall back to the legacy single content object.
			if text == "" {
				var tc textContent
				if err := json.Unmarshal(payload.Update.Content, &tc); err == nil {
					text = tc.Text
				}
			}
			text, ok := sanitizeACPText(text)
			if !ok {
				continue
			}
			evt.ToolCallID = payload.Update.ToolCallID
			evt.ToolName = payload.Update.Title
			if isAskUserTool(payload.Update.Title, text) {
				evt.Type = "ask_user"
				evt.Content = text
			} else {
				evt.Type = "tool_use"
				evt.Content = text
			}
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

func isAskUserTool(toolName, text string) bool {
	name := strings.ToLower(toolName)
	askNames := []string{"ask_user", "ask-user", "askuser", "request_user_input", "user_input", "askuser", "user_input_request"}
	for _, n := range askNames {
		if name == n {
			return true
		}
	}
	// Heuristic: if the tool has no recognizable name but its text is a direct
	// question, surface it to the user as well.
	if toolName == "" && strings.Contains(text, "?") {
		return true
	}
	return false
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
